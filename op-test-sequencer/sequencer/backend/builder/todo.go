package builder

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/forking"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// TxResult is a tx that has been processed,
// and kept track of, while the block is being built.
type TxResult struct {
	Tx                *types.Transaction
	TxOrigin          common.Address
	Failed            bool
	Nonce             uint64
	UsedGas           uint64
	EffectiveGasPrice *big.Int
	// Logs can be retrieved from the state-db that holds on to them till end of block.
}

// Note: log-events are tracked in the State-DB because reverts can undo logs.
// We access the current accessible logs by tx-hash.

func (r *TxResult) Status() uint64 {
	if r.Failed {
		return types.ReceiptStatusFailed
	} else {
		return types.ReceiptStatusSuccessful
	}
}
func (r *TxResult) ContractAddress() common.Address {
	if r.Tx.To() == nil {
		return crypto.CreateAddress(r.TxOrigin, r.Nonce)
	}
	return common.Address{}
}

type Worker struct {
	log log.Logger

	chainCfg *params.ChainConfig
	env      *vm.EVM

	state     *forking.ForkableState
	baseState *state.StateDB

	parent     *types.Header
	parentHash common.Hash

	signer  types.Signer
	gasPool *core.GasPool

	attrs *eth.PayloadAttributes

	results []TxResult
}

// TODO init worker

func (b *Worker) Build() error {
	// TODO: prepare attributes
	return nil
}

func (b *Worker) setParent(h common.Hash, hdr *types.Header) {
	b.parent = hdr
	b.parentHash = h
}

func (b *Worker) prepareEnv() {
	baseFee := eip1559.CalcBaseFee(b.chainCfg, b.parent, uint64(b.attrs.Timestamp))
	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     b.getHash,
		L1CostFunc:  types.NewL1CostFunc(b.chainCfg, b.state),
		Coinbase:    b.attrs.SuggestedFeeRecipient,
		GasLimit:    0,
		BlockNumber: new(big.Int).Add(b.parent.Number, big.NewInt(1)),
		Time:        uint64(b.attrs.Timestamp),
		Difficulty:  big.NewInt(0),
		BaseFee:     baseFee,
		BlobBaseFee: eip4844.CalcBlobFee(0),
		Random:      (*common.Hash)(&b.attrs.PrevRandao),
	}
	b.signer = types.MakeSigner(b.chainCfg, blockCtx.BlockNumber, blockCtx.Time)
	b.gasPool.SetGas(blockCtx.GasLimit)
	vmCfg := vm.Config{
		Tracer: &tracing.Hooks{
			OnLog: b.onLog,
		},
		NoBaseFee:               false,
		EnablePreimageRecording: false,
		ExtraEips:               nil,
		StatelessSelfValidation: false,
		PrecompileOverrides:     nil,
		NoMaxCodeSize:           false,
		CallerOverride:          nil,
	}
	b.env = vm.NewEVM(blockCtx, b.state, b.chainCfg, vmCfg)
}

func (b *Worker) onLog(log *types.Log) {
	// Note: logs may be reverted, so we have to be really careful with persistent state and streaming.
}

func (b *Worker) getHash(num uint64) common.Hash {
	// TODO
	return common.Hash{}
}

func (b *Worker) blockPrefix() {
	b.env.TxContext = vm.TxContext{}
	// Note: the below calls perform their own EVM env.Reset() calls
	if b.attrs.ParentBeaconBlockRoot != nil {
		core.ProcessBeaconBlockRoot(*b.attrs.ParentBeaconBlockRoot, b.env)
	}
	if b.chainCfg.IsPrague(b.env.Context.BlockNumber, b.env.Context.Time) {
		core.ProcessParentBlockHash(b.parentHash, b.env)
	}

	misc.EnsureCreate2Deployer(b.chainCfg, b.env.Context.Time, b.state)
}

func (b *Worker) processForceTxs() error {
	for _, txData := range b.attrs.Transactions {
		var tx types.Transaction // TODO: this is silly, we should prepare attributes in deserialized form
		if err := tx.UnmarshalBinary(txData); err != nil {
			return err
		}

		if err := b.applyTransaction(&tx); err != nil {
			return err
		}
	}
	return nil
}

// applyTransaction is like ApplyTransactionWithEVM but more minimal, no bloat.
func (b *Worker) applyTransaction(tx *types.Transaction) error {
	// TODO:
	// check gas pool
	// check DA limit
	// SetTxContext
	// check tx-conditional

	var (
		preStateSnap = b.state.Snapshot()
		preStateGas  = b.gasPool.Gas()
	)

	msg, err := core.TransactionToMessage(tx, b.signer, b.env.Context.BaseFee)
	if err != nil {
		return fmt.Errorf("could not prepare EVM message [%s]: %w", tx.Hash(), err)
	}

	txIndex := len(b.results)
	b.baseState.SetTxContext(tx.Hash(), txIndex)

	// Note: we don't run Tracer.OnTxStart and Tracer.OnTxEnd,
	// because we already have control over tx execution and don't produce the full receipt immediately.

	// Create a new context to be used in the EVM environment.
	txContext := core.NewEVMTxContext(msg)
	b.env.SetTxContext(txContext)

	nonce := tx.Nonce()
	if msg.IsDepositTx && b.chainCfg.IsOptimismRegolith(b.env.Context.Time) {
		nonce = b.state.GetNonce(msg.From)
	}

	// Apply the transaction to the current state (included in the env).
	result, err := core.ApplyMessage(b.env, msg, b.gasPool)
	if err != nil {
		b.gasPool.SetGas(preStateGas)
		b.state.RevertToSnapshot(preStateSnap)
		return err
	}

	b.state.Finalise(true)

	// core.MakeReceipt during block-building is bloated.
	// Just log the result. Logs can be collected later. Keep it simple.
	txResult := TxResult{
		Tx:                tx,
		TxOrigin:          b.env.TxContext.Origin,
		Failed:            result.Failed(),
		Nonce:             nonce,
		UsedGas:           result.UsedGas,
		EffectiveGasPrice: msg.GasPrice,
	}
	b.results = append(b.results, txResult)

	return nil
}

// makeReceipts is the core.MakeReceipt equivalent, but not as bloated.
// Produces all receipts of the block at once.
func (b *Worker) makeReceipts() types.Receipts {
	out := make(types.Receipts, len(b.results))
	cumulativeGasUsed := uint64(0)
	for i, txResult := range b.results {
		cumulativeGasUsed += txResult.UsedGas
		// TODO: better logs access
		logs := b.baseState.GetLogs(txResult.Tx.Hash(), 0, common.Hash{})
		bloom := types.CreateBloom(types.Receipts{{Logs: logs}})

		rec := &types.Receipt{
			// Consensus fields
			Type:              txResult.Tx.Type(),
			PostState:         nil, // never set since Byzantium
			Status:            txResult.Status(),
			CumulativeGasUsed: cumulativeGasUsed,
			Bloom:             bloom,
			Logs:              logs,
			// Implementation fields
			TxHash:                txResult.Tx.Hash(),
			ContractAddress:       txResult.ContractAddress(),
			GasUsed:               txResult.UsedGas,
			EffectiveGasPrice:     txResult.EffectiveGasPrice,
			BlobGasUsed:           0,
			BlobGasPrice:          nil,
			DepositNonce:          nil,
			DepositReceiptVersion: nil,
			// Inclusion information (Optional).
			BlockHash:        common.Hash{}, // not sealed yet
			BlockNumber:      b.env.Context.BlockNumber,
			TransactionIndex: uint(i),
			// Optimism extension  // TODO: probably not need these here, but could set them
			L1GasPrice:          nil,
			L1BlobBaseFee:       nil,
			L1GasUsed:           nil,
			L1Fee:               nil,
			FeeScalar:           nil,
			L1BaseFeeScalar:     nil,
			L1BlobBaseFeeScalar: nil,
		}
		if txResult.Tx.Type() == types.DepositTxType {
			nonce := txResult.Nonce
			rec.DepositNonce = &nonce
			if b.chainCfg.IsOptimismCanyon(b.env.Context.Time) {
				v := types.CanyonDepositReceiptVersion
				rec.DepositReceiptVersion = &v
			}
		}

		out[i] = rec
	}
	return out
}

func (b *Worker) seal() (*types.Header, types.Transactions, types.Receipts) {
	blockCtx := &b.env.Context
	// consensus interface FinalizeAndAssemble equivalent, without the bloat
	stateRoot := b.baseState.IntermediateRoot(true)
	withdrawalsRoot := types.EmptyWithdrawalsHash
	if b.chainCfg.IsOptimismIsthmus(blockCtx.Time) {
		// State-root has just been computed, we can get an accurate storage-root now.
		withdrawalsRoot = b.state.GetStorageRoot(params.OptimismL2ToL1MessagePasser)
	}
	hasher := trie.NewStackTrie(nil)
	txs := make(types.Transactions, len(b.results))
	for i, result := range b.results {
		txs[i] = result.Tx
	}
	txsRoot := types.DeriveSha(txs, hasher)
	receipts := b.makeReceipts()
	receiptsRoot := types.DeriveSha(receipts, hasher)
	bloom := types.CreateBloom(receipts)
	hdr := &types.Header{
		ParentHash:       b.parentHash,
		UncleHash:        types.EmptyUncleHash,
		Coinbase:         blockCtx.Coinbase,
		Root:             stateRoot,
		TxHash:           txsRoot,
		ReceiptHash:      receiptsRoot,
		Bloom:            bloom,
		Difficulty:       blockCtx.Difficulty,
		Number:           blockCtx.BlockNumber,
		GasLimit:         blockCtx.GasLimit,
		GasUsed:          blockCtx.GasLimit - b.gasPool.Gas(),
		Time:             blockCtx.Time,
		Extra:            nil, // set below (Holocene 1559 data)
		MixDigest:        *blockCtx.Random,
		Nonce:            types.BlockNonce{},
		BaseFee:          blockCtx.BaseFee,
		WithdrawalsHash:  &withdrawalsRoot,
		BlobGasUsed:      new(uint64),
		ExcessBlobGas:    new(uint64),
		ParentBeaconRoot: b.attrs.ParentBeaconBlockRoot,
		RequestsHash:     nil, // only in Prague
	}
	if b.attrs.EIP1559Params != nil {
		// If this is a holocene block and the params are 0, we must convert them to their previous
		// constants in the header.
		d, e := eip1559.DecodeHolocene1559Params(b.attrs.EIP1559Params[:])
		if d == 0 {
			d = b.chainCfg.BaseFeeChangeDenominator(hdr.Time)
			e = b.chainCfg.ElasticityMultiplier()
		}
		hdr.Extra = eip1559.EncodeHoloceneExtraData(d, e)
	}
	if b.chainCfg.IsPrague(blockCtx.BlockNumber, blockCtx.Time) {
		h := types.EmptyRequestsHash
		hdr.RequestsHash = &h
	}

	blockHash := hdr.Hash()
	for _, rec := range receipts {
		rec.BlockHash = blockHash
	}

	return hdr, txs, receipts
}

type CrossBuilder struct {
	byChain map[eth.ChainID]*Worker
}

func (c *CrossBuilder) Build() {
	// TODO two classes of state:
	// 1. the canonical block building state
	// 2. worker forks on top of the canonical building state,
	//    using a state-fork over uncommited state

	// get tx
	// get chain ID
	// simulate tx
	//   halt on emit event
	//   check interop-gas, enforce priority fee.
	//   if exec:
	//  	check if emitted by other chain
	//      if same chain, must already be in buffer
	//      if wildcard, use solver to create tx
	//   if init:
	//    append to event buffer, usable by others.
	//    unblock any prior exec of other chain
	//

}
