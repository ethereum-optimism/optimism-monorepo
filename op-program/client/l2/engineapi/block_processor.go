package engineapi

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

var (
	ErrExceedsGasLimit = errors.New("tx gas exceeds block gas limit")
	ErrUsesTooMuchGas  = errors.New("action takes too much gas")
)

type BlockDataProvider interface {
	StateAt(root common.Hash) (*state.StateDB, error)
	GetHeader(common.Hash, uint64) *types.Header
	Engine() consensus.Engine
	GetVMConfig() *vm.Config
	Config() *params.ChainConfig
	consensus.ChainHeaderReader
}

type BlockProcessor struct {
	header       *types.Header
	state        *state.StateDB
	receipts     types.Receipts
	transactions types.Transactions
	gasPool      *core.GasPool
	dataProvider BlockDataProvider
}

func NewBlockProcessorFromPayloadAttributes(provider BlockDataProvider, parent common.Hash, attrs *eth.PayloadAttributes) (*BlockProcessor, error) {
	header := &types.Header{
		ParentHash:       parent,
		Coinbase:         attrs.SuggestedFeeRecipient,
		Difficulty:       common.Big0,
		GasLimit:         uint64(*attrs.GasLimit),
		Time:             uint64(attrs.Timestamp),
		Extra:            nil,
		MixDigest:        common.Hash(attrs.PrevRandao),
		Nonce:            types.EncodeNonce(0),
		ParentBeaconRoot: attrs.ParentBeaconBlockRoot,
	}
	if attrs.EIP1559Params != nil {
		d, e := eip1559.DecodeHolocene1559Params(attrs.EIP1559Params[:])
		if d == 0 {
			d = provider.Config().BaseFeeChangeDenominator(header.Time)
			e = provider.Config().ElasticityMultiplier()
		}
		header.Extra = eip1559.EncodeHoloceneExtraData(d, e)
	}

	return NewBlockProcessorFromHeader(provider, header)
}

func NewBlockProcessorFromHeader(provider BlockDataProvider, h *types.Header) (*BlockProcessor, error) {
	header := types.CopyHeader(h) // Copy to avoid mutating the original header

	if header.GasLimit > params.MaxGasLimit {
		return nil, fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, params.MaxGasLimit)
	}
	parentHeader := provider.GetHeaderByHash(header.ParentHash)
	if header.Time <= parentHeader.Time {
		return nil, errors.New("invalid timestamp")
	}
	statedb, err := provider.StateAt(parentHeader.Root)
	if err != nil {
		return nil, fmt.Errorf("get parent state: %w", err)
	}
	header.Number = new(big.Int).Add(parentHeader.Number, common.Big1)
	header.BaseFee = eip1559.CalcBaseFee(provider.Config(), parentHeader, header.Time)
	header.GasUsed = 0
	gasPool := new(core.GasPool).AddGas(header.GasLimit)
	mkEVM := func() *vm.EVM {
		// Unfortunately this is not part of any Geth environment setup,
		// we just have to apply it, like how the Geth block-builder worker does.
		context := core.NewEVMBlockContext(header, provider, nil, provider.Config(), statedb)
		// NOTE: Unlikely to be needed for the beacon block root, but we setup any precompile overrides anyways for forwards-compatibility
		var precompileOverrides vm.PrecompileOverrides
		if vmConfig := provider.GetVMConfig(); vmConfig != nil && vmConfig.PrecompileOverrides != nil {
			precompileOverrides = vmConfig.PrecompileOverrides
		}
		vmenv := vm.NewEVM(context, vm.TxContext{}, statedb, provider.Config(), vm.Config{PrecompileOverrides: precompileOverrides})
		return vmenv
	}
	if h.ParentBeaconRoot != nil {
		if provider.Config().IsCancun(header.Number, header.Time) {
			// Blob tx not supported on optimism chains but fields must be set when Cancun is active.
			zero := uint64(0)
			header.BlobGasUsed = &zero
			header.ExcessBlobGas = &zero
		}
		vmenv := mkEVM()
		core.ProcessBeaconBlockRoot(*header.ParentBeaconRoot, vmenv, statedb)
	}
	if provider.Config().IsPrague(header.Number, header.Time) {
		vmenv := mkEVM()
		core.ProcessParentBlockHash(header.ParentHash, vmenv, statedb)
	}
	return &BlockProcessor{
		header:       header,
		state:        statedb,
		gasPool:      gasPool,
		dataProvider: provider,
	}, nil
}

func (b *BlockProcessor) CheckTxWithinGasLimit(tx *types.Transaction) error {
	if tx.Gas() > b.header.GasLimit {
		return fmt.Errorf("%w tx gas: %d, block gas limit: %d", ErrExceedsGasLimit, tx.Gas(), b.header.GasLimit)
	}
	if tx.Gas() > b.gasPool.Gas() {
		return fmt.Errorf("%w: %d, only have %d", ErrUsesTooMuchGas, tx.Gas(), b.gasPool.Gas())
	}
	return nil
}

func (b *BlockProcessor) AddTx(tx *types.Transaction) error {
	txIndex := len(b.transactions)
	b.state.SetTxContext(tx.Hash(), txIndex)
	receipt, err := core.ApplyTransaction(b.dataProvider.Config(), b.dataProvider, &b.header.Coinbase,
		b.gasPool, b.state, b.header, tx, &b.header.GasUsed, *b.dataProvider.GetVMConfig())
	if err != nil {
		return fmt.Errorf("failed to apply transaction to L2 block (tx %d): %w", txIndex, err)
	}
	b.receipts = append(b.receipts, receipt)
	b.transactions = append(b.transactions, tx)
	return nil
}

func (b *BlockProcessor) Assemble() (*types.Block, error) {
	body := types.Body{
		Transactions: b.transactions,
	}

	return b.dataProvider.Engine().FinalizeAndAssemble(b.dataProvider, b.header, b.state, &body, b.receipts)
}

func (b *BlockProcessor) Commit() error {
	root, err := b.state.Commit(b.header.Number.Uint64(), b.dataProvider.Config().IsEIP158(b.header.Number))
	if err != nil {
		return fmt.Errorf("state write error: %w", err)
	}
	if err := b.state.Database().TrieDB().Commit(root, false); err != nil {
		return fmt.Errorf("trie write error: %w", err)
	}
	return nil
}
