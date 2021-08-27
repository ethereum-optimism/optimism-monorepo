/* External Imports */
import {
  Contract,
  Signer,
  utils,
  providers,
  PopulatedTransaction,
} from 'ethers'
import {
  TransactionReceipt,
  TransactionResponse,
} from '@ethersproject/abstract-provider'
import { Gauge, Histogram, Counter } from 'prom-client'
import { RollupInfo, sleep } from '@eth-optimism/core-utils'
import { Logger, Metrics } from '@eth-optimism/common-ts'
import { getContractFactory } from 'old-contracts'
/* Internal Imports */
import { TxSubmissionHooks } from '..'

export interface BlockRange {
  start: number
  end: number
}

interface BatchSubmitterMetrics {
  batchSubmitterETHBalance: Gauge<string>
  batchSizeInBytes: Histogram<string>
  numTxPerBatch: Histogram<string>
  submissionTimestamp: Histogram<string>
  submissionGasUsed: Histogram<string>
  batchesSubmitted: Counter<string>
  failedSubmissions: Counter<string>
  malformedBatches: Counter<string>
}

export abstract class BatchSubmitter {
  protected rollupInfo: RollupInfo
  protected chainContract: Contract
  protected l2ChainId: number
  protected syncing: boolean
  protected lastBatchSubmissionTimestamp: number = 0
  protected metrics: BatchSubmitterMetrics

  constructor(
    readonly signer: Signer,
    readonly l2Provider: providers.StaticJsonRpcProvider,
    readonly minTxSize: number,
    readonly maxTxSize: number,
    readonly maxBatchSize: number,
    readonly maxBatchSubmissionTime: number,
    readonly numConfirmations: number,
    readonly resubmissionTimeout: number,
    readonly finalityConfirmations: number,
    readonly addressManagerAddress: string,
    readonly minBalanceEther: number,
    readonly blockOffset: number,
    readonly logger: Logger,
    readonly defaultMetrics: Metrics
  ) {
    this.metrics = this._registerMetrics(defaultMetrics)
  }

  public abstract _submitBatch(
    startBlock: number,
    endBlock: number
  ): Promise<TransactionReceipt>
  public abstract _onSync(): Promise<TransactionReceipt>
  public abstract _getBatchStartAndEnd(): Promise<BlockRange>
  public abstract _updateChainInfo(): Promise<void>

  public async submitNextBatch(): Promise<TransactionReceipt> {
    if (typeof this.l2ChainId === 'undefined') {
      this.l2ChainId = await this._getL2ChainId()
    }
    await this._updateChainInfo()

    if (!(await this._hasEnoughETHToCoverGasCosts())) {
      await sleep(this.resubmissionTimeout)
      return
    }

    this.logger.info('Readying to submit next batch...', {
      l2ChainId: this.l2ChainId,
      batchSubmitterAddress: await this.signer.getAddress(),
    })

    if (this.syncing === true) {
      this.logger.info(
        'Syncing mode enabled! Skipping batch submission and clearing queue...'
      )
      return this._onSync()
    }
    const range = await this._getBatchStartAndEnd()
    if (!range) {
      return
    }

    return this._submitBatch(range.start, range.end)
  }

  protected async _hasEnoughETHToCoverGasCosts(): Promise<boolean> {
    const address = await this.signer.getAddress()
    const balance = await this.signer.getBalance()
    const ether = utils.formatEther(balance)
    const num = parseFloat(ether)

    this.logger.info('Checked balance', {
      address,
      ether,
    })

    this.metrics.batchSubmitterETHBalance.set(num)

    if (num < this.minBalanceEther) {
      this.logger.fatal('Current balance lower than min safe balance', {
        current: num,
        safeBalance: this.minBalanceEther,
      })
      return false
    }

    return true
  }

  protected async _getRollupInfo(): Promise<RollupInfo> {
    return this.l2Provider.send('rollup_getInfo', [])
  }

  protected async _getL2ChainId(): Promise<number> {
    return this.l2Provider.send('eth_chainId', [])
  }

  protected async _getChainAddresses(): Promise<{
    ctcAddress: string
    sccAddress: string
  }> {
    const addressManager = (
      await getContractFactory('Lib_AddressManager', this.signer)
    ).attach(this.addressManagerAddress)
    const sccAddress = await addressManager.getAddress(
      'OVM_StateCommitmentChain'
    )
    const ctcAddress = await addressManager.getAddress(
      'OVM_CanonicalTransactionChain'
    )
    return {
      ctcAddress,
      sccAddress,
    }
  }

  protected _shouldSubmitBatch(
    batchSizeInBytes: number,
    numberOfElements: number
  ): boolean {
    const currentTimestamp = Date.now()
    if (
      batchSizeInBytes < this.minTxSize &&
      numberOfElements < this.maxBatchSize
    ) {
      const timeSinceLastSubmission =
        currentTimestamp - this.lastBatchSubmissionTimestamp
      if (timeSinceLastSubmission < this.maxBatchSubmissionTime) {
        this.logger.info(
          'Skipping batch submission. Batch too small & max submission timeout not reached.',
          {
            batchSizeInBytes,
            timeSinceLastSubmission,
            maxBatchSubmissionTime: this.maxBatchSubmissionTime,
            minTxSize: this.minTxSize,
            lastBatchSubmissionTimestamp: this.lastBatchSubmissionTimestamp,
            currentTimestamp,
          }
        )
        return false
      }
      this.logger.info('Timeout reached, proceeding with batch submission.', {
        batchSizeInBytes,
        timeSinceLastSubmission,
        maxBatchSubmissionTime: this.maxBatchSubmissionTime,
        lastBatchSubmissionTimestamp: this.lastBatchSubmissionTimestamp,
        currentTimestamp,
      })
      this.metrics.batchSizeInBytes.observe(batchSizeInBytes)
      return true
    }
    this.logger.info(
      'Sufficient batch size, proceeding with batch submission.',
      {
        batchSizeInBytes,
        lastBatchSubmissionTimestamp: this.lastBatchSubmissionTimestamp,
        currentTimestamp,
      }
    )
    this.metrics.batchSizeInBytes.observe(batchSizeInBytes)
    return true
  }

  protected _makeHooks(txName: string): TxSubmissionHooks {
    return {
      beforeSendTransaction: (tx: PopulatedTransaction) => {
        this.logger.info(`Submitting ${txName} transaction`, {
          gasPrice: tx.gasPrice,
          nonce: tx.nonce,
          contractAddr: this.chainContract.address,
        })
      },
      onTransactionResponse: (txResponse: TransactionResponse) => {
        this.logger.info(`Submitted ${txName} transaction`, {
          txHash: txResponse.hash,
          from: txResponse.from,
        })
        this.logger.debug(`${txName} transaction data`, {
          data: txResponse.data,
        })
      },
    }
  }

  protected async _submitAndLogTx(
    submitTransaction: () => Promise<TransactionReceipt>,
    successMessage: string
  ): Promise<TransactionReceipt> {
    this.lastBatchSubmissionTimestamp = Date.now()
    this.logger.debug('Submitting transaction & waiting for receipt...')

    let receipt: TransactionReceipt
    try {
      receipt = await submitTransaction()
    } catch (err) {
      this.metrics.failedSubmissions.inc()
      if (err.reason) {
        this.logger.error(`Transaction invalid: ${err.reason}, aborting`, {
          message: err.toString(),
          stack: err.stack,
          code: err.code,
        })
        return
      }

      this.logger.error('Encountered error at submission, aborting', {
        message: err.toString(),
        stack: err.stack,
        code: err.code,
      })
      return
    }

    this.logger.info('Received transaction receipt', { receipt })
    this.logger.info(successMessage)
    this.metrics.batchesSubmitted.inc()
    this.metrics.submissionGasUsed.observe(receipt.gasUsed.toNumber())
    this.metrics.submissionTimestamp.observe(Date.now())
    return receipt
  }

  private _registerMetrics(metrics: Metrics): BatchSubmitterMetrics {
    metrics.registry.clear()

    return {
      batchSubmitterETHBalance: new metrics.client.Gauge({
        name: 'batch_submitter_eth_balance',
        help: 'ETH balance of the batch submitter',
        registers: [metrics.registry],
      }),
      batchSizeInBytes: new metrics.client.Histogram({
        name: 'batch_size_in_bytes',
        help: 'Size of batches in bytes',
        registers: [metrics.registry],
      }),
      numTxPerBatch: new metrics.client.Histogram({
        name: 'num_txs_per_batch',
        help: 'Number of transactions in each batch',
        registers: [metrics.registry],
      }),
      submissionTimestamp: new metrics.client.Histogram({
        name: 'submission_timestamp',
        help: 'Timestamp of each batch submitter submission',
        registers: [metrics.registry],
      }),
      submissionGasUsed: new metrics.client.Histogram({
        name: 'submission_gash_used',
        help: 'Gas used to submit each batch',
        registers: [metrics.registry],
      }),
      batchesSubmitted: new metrics.client.Counter({
        name: 'batches_submitted',
        help: 'Count of batches submitted',
        registers: [metrics.registry],
      }),
      failedSubmissions: new metrics.client.Counter({
        name: 'failed_submissions',
        help: 'Count of failed batch submissions',
        registers: [metrics.registry],
      }),
      malformedBatches: new metrics.client.Counter({
        name: 'malformed_batches',
        help: 'Count of malformed batches',
        registers: [metrics.registry],
      }),
    }
  }
}
