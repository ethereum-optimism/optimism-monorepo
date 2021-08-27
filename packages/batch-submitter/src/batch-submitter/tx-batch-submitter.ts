/* External Imports */
import { Promise as bPromise } from 'bluebird'
import { Signer, ethers, Contract, providers } from 'ethers'
import { TransactionReceipt } from '@ethersproject/abstract-provider'
import { getContractInterface, getContractFactory } from 'old-contracts'
import { getContractInterface as getNewContractInterface } from '@eth-optimism/contracts'
import {
  L2Block,
  RollupInfo,
  BatchElement,
  Batch,
  QueueOrigin,
} from '@eth-optimism/core-utils'
import { Logger, Metrics } from '@eth-optimism/common-ts'

/* Internal Imports */
import {
  CanonicalTransactionChainContract,
  encodeAppendSequencerBatch,
  BatchContext,
  AppendSequencerBatchParams,
} from '../transaction-chain-contract'

import { BlockRange, BatchSubmitter } from '.'
import { TransactionSubmitter } from '../utils'

export interface AutoFixBatchOptions {
  fixDoublePlayedDeposits: boolean
  fixMonotonicity: boolean
  fixSkippedDeposits: boolean
}

export class TransactionBatchSubmitter extends BatchSubmitter {
  protected chainContract: CanonicalTransactionChainContract
  protected l2ChainId: number
  protected syncing: boolean
  private disableQueueBatchAppend: boolean
  private autoFixBatchOptions: AutoFixBatchOptions
  private transactionSubmitter: TransactionSubmitter
  private gasThresholdInGwei: number

  constructor(
    signer: Signer,
    l2Provider: providers.StaticJsonRpcProvider,
    minTxSize: number,
    maxTxSize: number,
    maxBatchSize: number,
    maxBatchSubmissionTime: number,
    numConfirmations: number,
    resubmissionTimeout: number,
    addressManagerAddress: string,
    minBalanceEther: number,
    gasThresholdInGwei: number,
    transactionSubmitter: TransactionSubmitter,
    blockOffset: number,
    logger: Logger,
    metrics: Metrics,
    disableQueueBatchAppend: boolean,
    autoFixBatchOptions: AutoFixBatchOptions = {
      fixDoublePlayedDeposits: false,
      fixMonotonicity: false,
      fixSkippedDeposits: false,
    } // TODO: Remove this
  ) {
    super(
      signer,
      l2Provider,
      minTxSize,
      maxTxSize,
      maxBatchSize,
      maxBatchSubmissionTime,
      numConfirmations,
      resubmissionTimeout,
      0, // Supply dummy value because it is not used.
      addressManagerAddress,
      minBalanceEther,
      blockOffset,
      logger,
      metrics
    )
    this.disableQueueBatchAppend = disableQueueBatchAppend
    this.autoFixBatchOptions = autoFixBatchOptions
    this.gasThresholdInGwei = gasThresholdInGwei
    this.transactionSubmitter = transactionSubmitter
  }

  /*****************************
   * Batch Submitter Overrides *
   ****************************/

  public async _updateChainInfo(): Promise<void> {
    const info: RollupInfo = await this._getRollupInfo()
    if (info.mode === 'verifier') {
      this.logger.error(
        'Verifier mode enabled! Batch submitter only compatible with sequencer mode'
      )
      process.exit(1)
    }
    this.syncing = info.syncing
    const addrs = await this._getChainAddresses()
    const ctcAddress = addrs.ctcAddress

    if (
      typeof this.chainContract !== 'undefined' &&
      ctcAddress === this.chainContract.address
    ) {
      this.logger.debug('Chain contract already initialized', {
        ctcAddress,
      })
      return
    }

    const unwrapped_OVM_CanonicalTransactionChain = (
      await getContractFactory('OVM_CanonicalTransactionChain', this.signer)
    ).attach(ctcAddress)

    this.chainContract = new CanonicalTransactionChainContract(
      unwrapped_OVM_CanonicalTransactionChain.address,
      getContractInterface('OVM_CanonicalTransactionChain'),
      this.signer
    )
    this.logger.info('Initialized new CTC', {
      address: this.chainContract.address,
    })
    return
  }

  public async _onSync(): Promise<TransactionReceipt> {
    const pendingQueueElements =
      await this.chainContract.getNumPendingQueueElements()
    this.logger.debug('Got number of pending queue elements', {
      pendingQueueElements,
    })

    if (pendingQueueElements !== 0) {
      this.logger.info(
        'Syncing mode enabled! Skipping batch submission and clearing queue elements',
        { pendingQueueElements }
      )

      if (!this.disableQueueBatchAppend) {
        return this.submitAppendQueueBatch()
      }
    }
    this.logger.info('Syncing mode enabled but queue is empty. Skipping...')
    return
  }

  public async _getBatchStartAndEnd(): Promise<BlockRange> {
    this.logger.info(
      'Getting batch start and end for transaction batch submitter...'
    )
    const startBlock =
      (await this.chainContract.getTotalElements()).toNumber() +
      this.blockOffset
    this.logger.info('Retrieved start block number from CTC', {
      startBlock,
    })

    const endBlock =
      Math.min(
        startBlock + this.maxBatchSize,
        await this.l2Provider.getBlockNumber()
      ) + 1 // +1 because the `endBlock` is *exclusive*
    this.logger.info('Retrieved end block number from L2 sequencer', {
      endBlock,
    })

    if (startBlock >= endBlock) {
      if (startBlock > endBlock) {
        this.logger
          .error(`More chain elements in L1 (${startBlock}) than in the L2 node (${endBlock}).
                   This shouldn't happen because we don't submit batches if the sequencer is syncing.`)
      }
      this.logger.info('No txs to submit. Skipping batch submission...')
      return
    }
    return {
      start: startBlock,
      end: endBlock,
    }
  }

  public async _submitBatch(
    startBlock: number,
    endBlock: number
  ): Promise<TransactionReceipt> {
    // Do not submit batch if gas price above threshold
    const gasPriceInGwei = parseInt(
      ethers.utils.formatUnits(await this.signer.getGasPrice(), 'gwei'),
      10
    )
    if (gasPriceInGwei > this.gasThresholdInGwei) {
      this.logger.warn(
        'Gas price is higher than gas price threshold; aborting batch submission',
        {
          gasPriceInGwei,
          gasThresholdInGwei: this.gasThresholdInGwei,
        }
      )
      return
    }

    const [batchParams, wasBatchTruncated] =
      await this._generateSequencerBatchParams(startBlock, endBlock)
    const batchSizeInBytes = encodeAppendSequencerBatch(batchParams).length / 2
    this.logger.debug('Sequencer batch generated', {
      batchSizeInBytes,
    })

    // Only submit batch if one of the following is true:
    // 1. it was truncated
    // 2. it is large enough
    // 3. enough time has passed since last submission
    if (!wasBatchTruncated && !this._shouldSubmitBatch(batchSizeInBytes)) {
      return
    }
    this.metrics.numTxPerBatch.observe(endBlock - startBlock)
    const l1tipHeight = await this.signer.provider.getBlockNumber()
    this.logger.debug('Submitting batch.', {
      calldata: batchParams,
      l1tipHeight,
    })

    return this.submitAppendSequencerBatch(batchParams)
  }

  /*********************
   * Private Functions *
   ********************/

  private async submitAppendQueueBatch(): Promise<TransactionReceipt> {
    const tx = await this.chainContract.populateTransaction.appendQueueBatch(
      ethers.constants.MaxUint256 // Completely empty the queue by appending (up to) an enormous number of queue elements.
    )
    const submitTransaction = (): Promise<TransactionReceipt> => {
      return this.transactionSubmitter.submitTransaction(
        tx,
        this._makeHooks('appendQueueBatch')
      )
    }
    // Empty the queue with a huge `appendQueueBatch(..)` call
    return this._submitAndLogTx(submitTransaction, 'Cleared queue!')
  }

  private async submitAppendSequencerBatch(
    batchParams: AppendSequencerBatchParams
  ): Promise<TransactionReceipt> {
    const tx =
      await this.chainContract.customPopulateTransaction.appendSequencerBatch(
        batchParams
      )
    const submitTransaction = (): Promise<TransactionReceipt> => {
      return this.transactionSubmitter.submitTransaction(
        tx,
        this._makeHooks('appendSequencerBatch')
      )
    }
    return this._submitAndLogTx(submitTransaction, 'Submitted batch!')
  }

  private async _generateSequencerBatchParams(
    startBlock: number,
    endBlock: number
  ): Promise<[AppendSequencerBatchParams, boolean]> {
    // Get all L2 BatchElements for the given range
    const blockRange = endBlock - startBlock
    let batch: Batch = await bPromise.map(
      [...Array(blockRange).keys()],
      (i) => {
        this.logger.debug('Fetching L2BatchElement', {
          blockNo: startBlock + i,
        })
        return this._getL2BatchElement(startBlock + i)
      },
      { concurrency: 100 }
    )

    // Fix our batches if we are configured to. TODO: Remove this.
    batch = await this._fixBatch(batch)
    if (!(await this._validateBatch(batch))) {
      this.metrics.malformedBatches.inc()
      return
    }
    let sequencerBatchParams = await this._getSequencerBatchParams(
      startBlock,
      batch
    )
    let wasBatchTruncated = false
    let encoded = encodeAppendSequencerBatch(sequencerBatchParams)
    while (encoded.length / 2 > this.maxTxSize) {
      this.logger.debug('Splicing batch...', {
        batchSizeInBytes: encoded.length / 2,
      })
      batch.splice(Math.ceil((batch.length * 2) / 3)) // Delete 1/3rd of all of the batch elements
      sequencerBatchParams = await this._getSequencerBatchParams(
        startBlock,
        batch
      )
      encoded = encodeAppendSequencerBatch(sequencerBatchParams)
      //  This is to prevent against the case where a batch is oversized,
      //  but then gets truncated to the point where it is under the minimum size.
      //  In this case, we want to submit regardless of the batch's size.
      wasBatchTruncated = true
    }

    this.logger.info('Generated sequencer batch params', {
      contexts: sequencerBatchParams.contexts,
      transactions: sequencerBatchParams.transactions,
      wasBatchTruncated,
    })
    return [sequencerBatchParams, wasBatchTruncated]
  }

  /**
   * Returns true if the batch is valid.
   */
  protected async _validateBatch(batch: Batch): Promise<boolean> {
    // Verify all of the queue elements are what we expect
    let nextQueueIndex = await this.chainContract.getNextQueueIndex()
    for (const ele of batch) {
      this.logger.debug('Verifying batch element', { ele })
      if (!ele.isSequencerTx) {
        this.logger.debug('Checking queue equality against L1 queue index', {
          nextQueueIndex,
        })
        if (!(await this._doesQueueElementMatchL1(nextQueueIndex, ele))) {
          return false
        }
        nextQueueIndex++
      }
    }

    // Verify all of the batch elements are monotonic
    let lastTimestamp: number
    let lastBlockNumber: number
    for (const [idx, ele] of batch.entries()) {
      if (ele.timestamp < lastTimestamp) {
        this.logger.error('Timestamp monotonicity violated! Element', {
          idx,
          ele,
        })
        return false
      }
      if (ele.blockNumber < lastBlockNumber) {
        this.logger.error('Block Number monotonicity violated! Element', {
          idx,
          ele,
        })
        return false
      }
      lastTimestamp = ele.timestamp
      lastBlockNumber = ele.blockNumber
    }
    return true
  }

  private async _doesQueueElementMatchL1(
    queueIndex: number,
    queueElement: BatchElement
  ): Promise<boolean> {
    const logEqualityError = (name, index, expected, got) => {
      this.logger.error('Observed mismatched values', {
        index,
        expected,
        got,
      })
    }

    let isEqual = true
    const [queueEleHash, timestamp, blockNumber] =
      await this.chainContract.getQueueElement(queueIndex)

    // TODO: Verify queue element hash equality. The queue element hash can be computed with:
    // keccak256( abi.encode( msg.sender, _target, _gasLimit, _data))

    // Check timestamp & blockNumber equality
    if (timestamp !== queueElement.timestamp) {
      isEqual = false
      logEqualityError(
        'Timestamp',
        queueIndex,
        timestamp,
        queueElement.timestamp
      )
    }
    if (blockNumber !== queueElement.blockNumber) {
      isEqual = false
      logEqualityError(
        'Block Number',
        queueIndex,
        blockNumber,
        queueElement.blockNumber
      )
    }
    return isEqual
  }

  /**
   * Takes in a batch which is potentially malformed & returns corrected version.
   * Current fixes that are supported:
   * - Double played deposits.
   */
  private async _fixBatch(batch: Batch): Promise<Batch> {
    const fixDoublePlayedDeposits = async (b: Batch): Promise<Batch> => {
      let nextQueueIndex = await this.chainContract.getNextQueueIndex()
      const fixedBatch: Batch = []
      for (const ele of b) {
        if (!ele.isSequencerTx) {
          if (!(await this._doesQueueElementMatchL1(nextQueueIndex, ele))) {
            this.logger.warn('Fixing double played queue element.', {
              nextQueueIndex,
            })
            fixedBatch.push(
              await this._fixDoublePlayedDepositQueueElement(
                nextQueueIndex,
                ele
              )
            )
            continue
          }
          nextQueueIndex++
        }
        fixedBatch.push(ele)
      }
      return fixedBatch
    }

    const fixSkippedDeposits = async (b: Batch): Promise<Batch> => {
      this.logger.debug('Fixing skipped deposits...')
      let nextQueueIndex = await this.chainContract.getNextQueueIndex()
      const fixedBatch: Batch = []
      for (const ele of b) {
        // Look for skipped deposits
        while (true) {
          const pendingQueueElements =
            await this.chainContract.getNumPendingQueueElements()
          const nextRemoteQueueElements =
            await this.chainContract.getNextQueueIndex()
          const totalQueueElements =
            pendingQueueElements + nextRemoteQueueElements
          // No more queue elements so we clearly haven't skipped anything
          if (nextQueueIndex >= totalQueueElements) {
            break
          }
          const [queueEleHash, timestamp, blockNumber] =
            await this.chainContract.getQueueElement(nextQueueIndex)

          if (timestamp < ele.timestamp || blockNumber < ele.blockNumber) {
            this.logger.warn('Fixing skipped deposit', {
              badTimestamp: ele.timestamp,
              skippedQueueTimestamp: timestamp,
              badBlockNumber: ele.blockNumber,
              skippedQueueBlockNumber: blockNumber,
            })
            // Push a dummy queue element
            fixedBatch.push({
              stateRoot: ele.stateRoot,
              isSequencerTx: false,
              rawTransaction: undefined,
              timestamp,
              blockNumber,
            })
            nextQueueIndex++
          } else {
            // The next queue element's timestamp is after this batch element so
            // we must not have skipped anything.
            break
          }
        }
        if (!ele.isSequencerTx) {
          nextQueueIndex++
        }
      }
      return fixedBatch
    }

    // TODO: Remove this super complex logic and rely on Geth to actually supply correct block data.
    const fixMonotonicity = async (b: Batch): Promise<Batch> => {
      this.logger.debug('Fixing monotonicity...')
      // The earliest allowed timestamp/blockNumber is the last timestamp submitted on chain.
      const { lastTimestamp, lastBlockNumber } =
        await this._getLastTimestampAndBlockNumber()
      let earliestTimestamp = lastTimestamp
      let earliestBlockNumber = lastBlockNumber
      this.logger.debug('Determined earliest timestamp and blockNumber', {
        earliestTimestamp,
        earliestBlockNumber,
      })

      // The latest allowed timestamp/blockNumber is the next queue element!
      let nextQueueIndex = await this.chainContract.getNextQueueIndex()
      let latestTimestamp: number
      let latestBlockNumber: number

      // updateLatestTimestampAndBlockNumber is a helper which updates
      // the latest timestamp and block number based on the pending queue elements.
      const updateLatestTimestampAndBlockNumber = async () => {
        const pendingQueueElements =
          await this.chainContract.getNumPendingQueueElements()
        const nextRemoteQueueElements =
          await this.chainContract.getNextQueueIndex()
        const totalQueueElements =
          pendingQueueElements + nextRemoteQueueElements
        if (nextQueueIndex < totalQueueElements) {
          const [queueEleHash, queueTimestamp, queueBlockNumber] =
            await this.chainContract.getQueueElement(nextQueueIndex)
          latestTimestamp = queueTimestamp
          latestBlockNumber = queueBlockNumber
        } else {
          // If there are no queue elements left then just allow any timestamp/blocknumber
          latestTimestamp = Number.MAX_SAFE_INTEGER
          latestBlockNumber = Number.MAX_SAFE_INTEGER
        }
      }
      // Actually update the latest timestamp and block number
      await updateLatestTimestampAndBlockNumber()
      this.logger.debug('Determined latest timestamp and blockNumber', {
        latestTimestamp,
        latestBlockNumber,
      })

      // Now go through our batch and fix the timestamps and block numbers
      // to automatically enforce monotonicity.
      const fixedBatch: Batch = []
      for (const ele of b) {
        if (!ele.isSequencerTx) {
          // Set the earliest allowed timestamp to the old latest and set the new latest
          // to the next queue element's timestamp / blockNumber
          earliestTimestamp = latestTimestamp
          earliestBlockNumber = latestBlockNumber
          nextQueueIndex++
          await updateLatestTimestampAndBlockNumber()
        }
        // Fix the element if its timestammp/blockNumber is too small
        if (
          ele.timestamp < earliestTimestamp ||
          ele.blockNumber < earliestBlockNumber
        ) {
          this.logger.warn('Fixing timestamp/blockNumber too small', {
            oldTimestamp: ele.timestamp,
            newTimestamp: earliestTimestamp,
            oldBlockNumber: ele.blockNumber,
            newBlockNumber: earliestBlockNumber,
          })
          ele.timestamp = earliestTimestamp
          ele.blockNumber = earliestBlockNumber
        }
        // Fix the element if its timestammp/blockNumber is too large
        if (
          ele.timestamp > latestTimestamp ||
          ele.blockNumber > latestBlockNumber
        ) {
          this.logger.warn('Fixing timestamp/blockNumber too large.', {
            oldTimestamp: ele.timestamp,
            newTimestamp: latestTimestamp,
            oldBlockNumber: ele.blockNumber,
            newBlockNumber: latestBlockNumber,
          })
          ele.timestamp = latestTimestamp
          ele.blockNumber = latestBlockNumber
        }
        earliestTimestamp = ele.timestamp
        earliestBlockNumber = ele.blockNumber
        fixedBatch.push(ele)
      }
      return fixedBatch
    }

    // NOTE: It is unsafe to combine multiple autoFix options.
    // If you must combine them, manually verify the output before proceeding.
    if (this.autoFixBatchOptions.fixDoublePlayedDeposits) {
      batch = await fixDoublePlayedDeposits(batch)
    }
    if (this.autoFixBatchOptions.fixMonotonicity) {
      batch = await fixMonotonicity(batch)
    }
    if (this.autoFixBatchOptions.fixSkippedDeposits) {
      batch = await fixSkippedDeposits(batch)
    }
    return batch
  }

  private async _getLastTimestampAndBlockNumber(): Promise<{
    lastTimestamp: number
    lastBlockNumber: number
  }> {
    const manager = new Contract(
      this.addressManagerAddress,
      getNewContractInterface('Lib_AddressManager'),
      this.signer.provider
    )

    const addr = await manager.getAddress(
      'OVM_ChainStorageContainer-CTC-batches'
    )
    const container = new Contract(
      addr,
      getNewContractInterface('iOVM_ChainStorageContainer'),
      this.signer.provider
    )

    let meta = await container.getGlobalMetadata()
    // remove 0x
    meta = meta.slice(2)
    // convert to bytes27
    meta = meta.slice(10)

    const totalElements = meta.slice(-10)
    const nextQueueIndex = meta.slice(-20, -10)
    const lastTimestamp = parseInt(meta.slice(-30, -20), 16)
    const lastBlockNumber = parseInt(meta.slice(-40, -30), 16)
    this.logger.debug('Retrieved timestamp and block number from CTC', {
      lastTimestamp,
      lastBlockNumber,
    })

    return { lastTimestamp, lastBlockNumber }
  }

  private async _fixDoublePlayedDepositQueueElement(
    queueIndex: number,
    queueElement: BatchElement
  ): Promise<BatchElement> {
    const [queueEleHash, timestamp, blockNumber] =
      await this.chainContract.getQueueElement(queueIndex)

    if (
      timestamp > queueElement.timestamp &&
      blockNumber > queueElement.blockNumber
    ) {
      this.logger.warn(
        'Double deposit detected. Fixing by skipping the deposit & replacing with a dummy tx.',
        {
          timestamp,
          blockNumber,
          queueElementTimestamp: queueElement.timestamp,
          queueElementBlockNumber: queueElement.blockNumber,
        }
      )
      const dummyTx: string = '0x1234'
      return {
        stateRoot: queueElement.stateRoot,
        isSequencerTx: true,
        rawTransaction: dummyTx,
        timestamp: queueElement.timestamp,
        blockNumber: queueElement.blockNumber,
      }
    }
    if (
      timestamp < queueElement.timestamp &&
      blockNumber < queueElement.blockNumber
    ) {
      this.logger.error('A deposit seems to have been skipped!')
      throw new Error('Skipped deposit?!')
    }
    throw new Error('Unable to fix queue element!')
  }

  private async _getSequencerBatchParams(
    shouldStartAtIndex: number,
    blocks: Batch
  ): Promise<AppendSequencerBatchParams> {
    const totalElementsToAppend = blocks.length

    // Generate contexts
    const contexts: BatchContext[] = []
    let lastBlockIsSequencerTx = false
    let lastTimestamp = 0
    let lastBlockNumber = 0
    const groupedBlocks: Array<{
      sequenced: BatchElement[]
      queued: BatchElement[]
    }> = []
    for (const block of blocks) {
      if (
        (lastBlockIsSequencerTx === false && block.isSequencerTx === true) ||
        groupedBlocks.length === 0 ||
        (block.timestamp !== lastTimestamp && block.isSequencerTx === true) ||
        (block.blockNumber !== lastBlockNumber && block.isSequencerTx === true)
      ) {
        groupedBlocks.push({
          sequenced: [],
          queued: [],
        })
      }
      const cur = groupedBlocks.length - 1
      block.isSequencerTx
        ? groupedBlocks[cur].sequenced.push(block)
        : groupedBlocks[cur].queued.push(block)
      lastBlockIsSequencerTx = block.isSequencerTx
      lastTimestamp = block.timestamp
      lastBlockNumber = block.blockNumber
    }
    for (const groupedBlock of groupedBlocks) {
      if (
        groupedBlock.sequenced.length === 0 &&
        groupedBlock.queued.length === 0
      ) {
        throw new Error(
          'Attempted to generate batch context with 0 queued and 0 sequenced txs!'
        )
      }
      contexts.push({
        numSequencedTransactions: groupedBlock.sequenced.length,
        numSubsequentQueueTransactions: groupedBlock.queued.length,
        timestamp:
          groupedBlock.sequenced.length > 0
            ? groupedBlock.sequenced[0].timestamp
            : groupedBlock.queued[0].timestamp,
        blockNumber:
          groupedBlock.sequenced.length > 0
            ? groupedBlock.sequenced[0].blockNumber
            : groupedBlock.queued[0].blockNumber,
      })
    }

    // Generate sequencer transactions
    const transactions: string[] = []
    for (const block of blocks) {
      if (!block.isSequencerTx) {
        continue
      }
      transactions.push(block.rawTransaction)
    }

    return {
      shouldStartAtElement: shouldStartAtIndex - this.blockOffset,
      totalElementsToAppend,
      contexts,
      transactions,
    }
  }

  private async _getL2BatchElement(blockNumber: number): Promise<BatchElement> {
    const block = await this._getBlock(blockNumber)
    this.logger.debug('Fetched L2 block', {
      block,
    })

    const batchElement = {
      stateRoot: block.stateRoot,
      timestamp: block.timestamp,
      blockNumber: block.transactions[0].l1BlockNumber,
      isSequencerTx: false,
      rawTransaction: undefined,
    }

    if (this._isSequencerTx(block)) {
      batchElement.isSequencerTx = true
      batchElement.rawTransaction = block.transactions[0].rawTransaction
    }

    return batchElement
  }

  private async _getBlock(blockNumber: number): Promise<L2Block> {
    const p = this.l2Provider.getBlockWithTransactions(blockNumber)
    return p as Promise<L2Block>
  }

  private _isSequencerTx(block: L2Block): boolean {
    return block.transactions[0].queueOrigin === QueueOrigin.Sequencer
  }
}
