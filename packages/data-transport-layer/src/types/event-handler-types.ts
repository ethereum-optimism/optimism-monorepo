import { BaseProvider } from '@ethersproject/providers'
import { BigNumber, Event } from 'ethers'

import { TransportDB } from '../db/transport-db'
import {
  TransactionBatchEntry,
  TransactionEntry,
  StateRootBatchEntry,
  StateRootEntry,
} from './database-types'

export type TypedEthersEvent<T> = Event & {
  args: T
}

export type GetExtraDataHandler<TEventArgs, TExtraData> = (
  event?: TypedEthersEvent<TEventArgs>,
  l1RpcProvider?: BaseProvider
) => Promise<TExtraData>

export type ParseEventHandler<TEventArgs, TExtraData, TParsedEvent> = (
  event: TypedEthersEvent<TEventArgs>,
  extraData: TExtraData,
  l2ChainId: number
) => TParsedEvent

export type StoreEventHandler<TParsedEvent> = (
  parsedEvent: TParsedEvent,
  db: TransportDB
) => Promise<void>

export interface EventHandlerSet<TEventArgs, TExtraData, TParsedEvent> {
  getExtraData: GetExtraDataHandler<TEventArgs, TExtraData>
  parseEvent: ParseEventHandler<TEventArgs, TExtraData, TParsedEvent>
  storeEvent: StoreEventHandler<TParsedEvent>
}

export interface SequencerBatchAppendedExtraData {
  timestamp: number
  blockNumber: number
  submitter: string
  l1TransactionData: string
  l1TransactionHash: string
  gasLimit: string

  // Stuff from TransactionBatchAppended.
  prevTotalElements: BigNumber
  batchIndex: BigNumber
  batchSize: BigNumber
  batchRoot: string
  batchExtraData: string
}

export interface SequencerBatchAppendedParsedEvent {
  transactionBatchEntry: TransactionBatchEntry
  transactionEntries: TransactionEntry[]
}

export interface StateBatchAppendedExtraData {
  timestamp: number
  blockNumber: number
  submitter: string
  l1TransactionHash: string
  l1TransactionData: string
}

export interface StateBatchAppendedParsedEvent {
  stateRootBatchEntry: StateRootBatchEntry
  stateRootEntries: StateRootEntry[]
}
