/* External Imports */
import { ethers } from 'ethers'
import { Provider, TransactionReceipt } from '@ethersproject/abstract-provider'

export interface Layer {
  provider: Provider
  messengerAddress: string
}

export interface WatcherOptions {
  l1: Layer
  l2: Layer
  pollInterval?: number
  blocksToFetch?: number
}

export class Watcher {
  public l1: Layer
  public l2: Layer
  public pollInterval = 3000
  public blocksToFetch = 1500

  constructor(opts: WatcherOptions) {
    this.l1 = opts.l1
    this.l2 = opts.l2
    if (opts.pollInterval) {
      this.pollInterval = opts.pollInterval
    }
    if (opts.blocksToFetch) {
      this.blocksToFetch = opts.blocksToFetch
    }
  }

  public async getMessageHashesFromL1Tx(l1TxHash: string): Promise<string[]> {
    return this.getMessageHashesFromTx(this.l1, l1TxHash)
  }
  public async getMessageHashesFromL2Tx(l2TxHash: string): Promise<string[]> {
    return this.getMessageHashesFromTx(this.l2, l2TxHash)
  }

  public async getL1TransactionReceipt(
    l2ToL1MsgHash: string,
    pollForPending = true
  ): Promise<TransactionReceipt> {
    return this.getTransactionReceipt(this.l1, l2ToL1MsgHash, pollForPending)
  }

  public async getL2TransactionReceipt(
    l1ToL2MsgHash: string,
    pollForPending = true
  ): Promise<TransactionReceipt> {
    return this.getTransactionReceipt(this.l2, l1ToL2MsgHash, pollForPending)
  }

  public async getMessageHashesFromTx(
    layer: Layer,
    txHash: string
  ): Promise<string[]> {
    const receipt = await layer.provider.getTransactionReceipt(txHash)
    if (!receipt) {
      return []
    }

    const msgHashes = []
    for (const log of receipt.logs) {
      if (
        log.address === layer.messengerAddress &&
        log.topics[0] === ethers.utils.id('SentMessage(bytes)')
      ) {
        const [message] = ethers.utils.defaultAbiCoder.decode(
          ['bytes'],
          log.data
        )
        msgHashes.push(ethers.utils.solidityKeccak256(['bytes'], [message]))
      }
    }
    return msgHashes
  }

  public async getTransactionReceipt(
    layer: Layer,
    msgHash: string,
    pollForPending = true
  ): Promise<TransactionReceipt> {
    let matches: ethers.providers.Log[] = []

    // scan for transaction with specified message
    while (matches.length === 0) {
      const blockNumber = await layer.provider.getBlockNumber()
      const startingBlock = Math.max(blockNumber - this.blocksToFetch, 0)
      const successFilter: ethers.providers.Filter = {
        address: layer.messengerAddress,
        topics: [ethers.utils.id(`RelayedMessage(bytes32)`)],
        fromBlock: startingBlock,
      }
      const failureFilter: ethers.providers.Filter = {
        address: layer.messengerAddress,
        topics: [ethers.utils.id(`FailedRelayedMessage(bytes32)`)],
        fromBlock: startingBlock,
      }
      const successLogs = await layer.provider.getLogs(successFilter)
      const failureLogs = await layer.provider.getLogs(failureFilter)
      const logs = successLogs.concat(failureLogs)
      matches = logs.filter((log: ethers.providers.Log) => log.data === msgHash)

      // exit loop after first iteration if not polling
      if (!pollForPending) {
        break
      }

      // pause awhile before trying again
      await new Promise((r) => setTimeout(r, this.pollInterval))
    }

    // Message was relayed in the past
    if (matches.length > 0) {
      if (matches.length > 1) {
        throw Error(
          'Found multiple transactions relaying the same message hash.'
        )
      }
      return layer.provider.getTransactionReceipt(matches[0].transactionHash)
    } else {
      return Promise.resolve(undefined)
    }
  }
}
