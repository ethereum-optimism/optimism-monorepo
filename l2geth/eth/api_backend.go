// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/diffdb"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

// EthAPIBackend implements ethapi.Backend for full nodes
type EthAPIBackend struct {
	extRPCEnabled   bool
	eth             *Ethereum
	gpo             *gasprice.Oracle
	rollupGpo       *gasprice.RollupOracle
	verifier        bool
	gasLimit        uint64
	UsingOVM        bool
	MaxCallDataSize int
}

func (b *EthAPIBackend) IsVerifier() bool {
	return b.verifier
}

func (b *EthAPIBackend) IsSyncing() bool {
	return b.eth.syncService.IsSyncing()
}

func (b *EthAPIBackend) GasLimit() uint64 {
	return b.gasLimit
}

func (b *EthAPIBackend) GetEthContext() (uint64, uint64) {
	bn := b.eth.syncService.GetLatestL1BlockNumber()
	ts := b.eth.syncService.GetLatestL1Timestamp()
	return bn, ts
}

func (b *EthAPIBackend) GetRollupContext() (uint64, uint64, uint64) {
	index := uint64(0)
	queueIndex := uint64(0)
	verifiedIndex := uint64(0)

	if latest := b.eth.syncService.GetLatestIndex(); latest != nil {
		index = *latest
	}
	if latest := b.eth.syncService.GetLatestEnqueueIndex(); latest != nil {
		queueIndex = *latest
	}
	if latest := b.eth.syncService.GetLatestVerifiedIndex(); latest != nil {
		verifiedIndex = *latest
	}
	return index, queueIndex, verifiedIndex
}

// ChainConfig returns the active chain configuration.
func (b *EthAPIBackend) ChainConfig() *params.ChainConfig {
	return b.eth.blockchain.Config()
}

func (b *EthAPIBackend) CurrentBlock() *types.Block {
	return b.eth.blockchain.CurrentBlock()
}

func (b *EthAPIBackend) GetDiff(block *big.Int) (diffdb.Diff, error) {
	return b.eth.blockchain.GetDiff(block)
}

func (b *EthAPIBackend) SetHead(number uint64) {
	if number == 0 {
		log.Info("Cannot reset to genesis")
		return
	}
	if !b.UsingOVM {
		b.eth.protocolManager.downloader.Cancel()
	}
	b.eth.blockchain.SetHead(number)

	// Make sure to reset the LatestL1{Timestamp,BlockNumber}
	block := b.eth.blockchain.CurrentBlock()
	txs := block.Transactions()
	if len(txs) == 0 {
		log.Error("No transactions found in block", "number", number)
		return
	}

	tx := txs[0]
	blockNumber := tx.L1BlockNumber()
	if blockNumber == nil {
		log.Error("No L1BlockNumber found in transaction", "number", number)
		return
	}

	b.eth.syncService.SetLatestL1Timestamp(tx.L1Timestamp())
	b.eth.syncService.SetLatestL1BlockNumber(blockNumber.Uint64())
}

func (b *EthAPIBackend) IngestTransactions(txs []*types.Transaction) error {
	for _, tx := range txs {
		err := b.eth.syncService.IngestTransaction(tx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *EthAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if !vm.UsingOVM && number == rpc.PendingBlockNumber {
		block := b.eth.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.eth.blockchain.CurrentBlock().Header(), nil
	}
	return b.eth.blockchain.GetHeaderByNumber(uint64(number)), nil
}

func (b *EthAPIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.eth.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.eth.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *EthAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.eth.blockchain.GetHeaderByHash(hash), nil
}

func (b *EthAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if !vm.UsingOVM && number == rpc.PendingBlockNumber {
		block := b.eth.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.eth.blockchain.CurrentBlock(), nil
	}
	return b.eth.blockchain.GetBlockByNumber(uint64(number)), nil
}

func (b *EthAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.eth.blockchain.GetBlockByHash(hash), nil
}

func (b *EthAPIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.eth.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.eth.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.eth.blockchain.GetBlock(hash, header.Number.Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *EthAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if !vm.UsingOVM && number == rpc.PendingBlockNumber {
		block, state := b.eth.miner.Pending()
		return state, block.Header(), nil
	}

	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := b.eth.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *EthAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := b.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.eth.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := b.eth.BlockChain().StateAt(header.Root)
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *EthAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.eth.blockchain.GetReceiptsByHash(hash), nil
}

func (b *EthAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	receipts := b.eth.blockchain.GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *EthAPIBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.eth.blockchain.GetTdByHash(blockHash)
}

func (b *EthAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.eth.BlockChain(), nil)
	return vm.NewEVM(context, state, b.eth.blockchain.Config(), *b.eth.blockchain.GetVMConfig()), vmError, nil
}

func (b *EthAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *EthAPIBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.eth.miner.SubscribePendingLogs(ch)
}

func (b *EthAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainEvent(ch)
}

func (b *EthAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *EthAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *EthAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.eth.BlockChain().SubscribeLogsEvent(ch)
}

// Transactions originating from the RPC endpoints are added to remotes so that
// a lock can be used around the remotes for when the sequencer is reorganizing.
func (b *EthAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	if b.UsingOVM {
		to := signedTx.To()
		if to != nil {
			if *to == (common.Address{}) {
				return errors.New("Cannot send transaction to zero address")
			}
			// Prevent QueueOriginSequencer transactions that are too large to
			// be included in a batch. The `MaxCallDataSize` should be set to
			// the layer one consensus max transaction size in bytes minus the
			// constant sized overhead of a batch. This will prevent
			// a layer two transaction from not being able to be batch submitted
			// to layer one.
			if len(signedTx.Data()) > b.MaxCallDataSize {
				return fmt.Errorf("Calldata cannot be larger than %d, sent %d", b.MaxCallDataSize, len(signedTx.Data()))
			}
			// If there is a value field set then reject transactions that
			// contain calldata. The feature of sending transactions with value
			// and calldata will be added in the future.
			if signedTx.Value().Cmp(common.Big0) != 0 {
				if len(signedTx.Data()) > 0 {
					return errors.New("Cannot send transactions with value and calldata")
				}
			}
		}
		return b.eth.syncService.ValidateAndApplySequencerTransaction(signedTx)
	}
	// OVM Disabled
	return b.eth.txPool.AddLocal(signedTx)
}

func (b *EthAPIBackend) SetTimestamp(timestamp int64) {
	b.eth.blockchain.SetCurrentTimestamp(timestamp)
}

func (b *EthAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.eth.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *EthAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.eth.txPool.Get(hash)
}

func (b *EthAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(b.eth.ChainDb(), txHash)
	return tx, blockHash, blockNumber, index, nil
}

func (b *EthAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.eth.txPool.Nonce(addr), nil
}

func (b *EthAPIBackend) Stats() (pending int, queued int) {
	return b.eth.txPool.Stats()
}

func (b *EthAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.eth.TxPool().Content()
}

func (b *EthAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.eth.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *EthAPIBackend) Downloader() *downloader.Downloader {
	return b.eth.Downloader()
}

func (b *EthAPIBackend) ProtocolVersion() int {
	return b.eth.EthVersion()
}

func (b *EthAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *EthAPIBackend) SuggestL1GasPrice(ctx context.Context) (*big.Int, error) {
	return b.rollupGpo.SuggestL1GasPrice(ctx)
}

func (b *EthAPIBackend) SuggestL2GasPrice(ctx context.Context) (*big.Int, error) {
	return b.rollupGpo.SuggestL2GasPrice(ctx)
}

func (b *EthAPIBackend) SetL1GasPrice(ctx context.Context, gasPrice *big.Int) error {
	return b.rollupGpo.SetL1GasPrice(gasPrice)
}

func (b *EthAPIBackend) SetL2GasPrice(ctx context.Context, gasPrice *big.Int) error {
	return b.rollupGpo.SetL2GasPrice(gasPrice)
}

func (b *EthAPIBackend) ChainDb() ethdb.Database {
	return b.eth.ChainDb()
}

func (b *EthAPIBackend) EventMux() *event.TypeMux {
	return b.eth.EventMux()
}

func (b *EthAPIBackend) AccountManager() *accounts.Manager {
	return b.eth.AccountManager()
}

func (b *EthAPIBackend) ExtRPCEnabled() bool {
	return b.extRPCEnabled
}

func (b *EthAPIBackend) RPCGasCap() *big.Int {
	return b.eth.config.RPCGasCap
}

func (b *EthAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.eth.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *EthAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.eth.bloomRequests)
	}
}
