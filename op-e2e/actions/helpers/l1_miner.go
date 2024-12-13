package helpers

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-program/host/prefetcher"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// L1Miner wraps a L1Replica with instrumented block building ability.
type L1Miner struct {
	L1Replica

	blobStore *e2eutils.BlobsStore

	// L1 block building preferences
	prefCoinbase common.Address

	// L1 block building data
	l1BuildingHeader *types.Header             // block header that we add txs to for block building
	l1BuildingState  *state.StateDB            // state used for block building
	L1GasPool        *core.GasPool             // track gas used of ongoing building
	pendingIndices   map[common.Address]uint64 // per account, how many txs from the pool were already included in the block, since the pool is lagging behind block mining.
	L1Transactions   []*types.Transaction      // collects txs that were successfully included into current block build
	l1Receipts       []*types.Receipt          // collect receipts of ongoing building
	l1Building       bool
	l1TxFailed       []*types.Transaction // log of failed transactions which could not be included
	// sidecars that come with the transactions
	l1BuildingBlobSidecars []*types.BlobTxSidecar
}

// NewL1Miner creates a new L1Replica that can also build blocks.
func NewL1Miner(t Testing, log log.Logger, genesis *core.Genesis) *L1Miner {
	rep := NewL1Replica(t, log, genesis)
	return &L1Miner{
		L1Replica: *rep,
		blobStore: e2eutils.NewBlobStore(),
	}
}

func (s *L1Miner) BlobSource() prefetcher.L1BlobSource {
	return s.blobStore
}

func (s *L1Miner) BlobStore() *e2eutils.BlobsStore {
	return s.blobStore
}

// ActL1StartBlock returns an action to build a new L1 block on top of the head block,
// with timeDelta added to the head block time.
func (s *L1Miner) ActL1StartBlock(timeDelta uint64) Action {
	return func(t Testing) {
		if s.l1Building {
			t.InvalidAction("not valid if we already started building a block")
		}
		if timeDelta == 0 {
			t.Fatalf("invalid time delta: %d", timeDelta)
		}

		parent := s.l1Chain.CurrentHeader()
		parentHash := parent.Hash()
		statedb, err := state.New(parent.Root, state.NewDatabase(triedb.NewDatabase(s.l1Database, nil), nil))
		if err != nil {
			t.Fatalf("failed to init state db around block %s (state %s): %w", parentHash, parent.Root, err)
		}
		header := &types.Header{
			ParentHash: parentHash,
			Coinbase:   s.prefCoinbase,
			Difficulty: common.Big0,
			Number:     new(big.Int).Add(parent.Number, common.Big1),
			GasLimit:   parent.GasLimit,
			Time:       parent.Time + timeDelta,
			Extra:      []byte("L1 was here"),
			MixDigest:  common.Hash{}, // TODO: maybe randomize this (prev-randao value)
		}

		if s.l1Cfg.Config.IsLondon(header.Number) {
			header.BaseFee = eip1559.CalcBaseFee(s.l1Cfg.Config, parent, header.Time)
			// At the transition, double the gas limit so the gas target is equal to the old gas limit.
			if !s.l1Cfg.Config.IsLondon(parent.Number) {
				header.GasLimit = parent.GasLimit * s.l1Cfg.Config.ElasticityMultiplier()
			}
		}

		if s.l1Cfg.Config.IsShanghai(header.Number, header.Time) {
			header.WithdrawalsHash = &types.EmptyWithdrawalsHash
		}

		if s.l1Cfg.Config.IsCancun(header.Number, header.Time) {
			header.BlobGasUsed = new(uint64)
			header.ExcessBlobGas = new(uint64)
			root := crypto.Keccak256Hash([]byte("fake-beacon-block-root"), header.Number.Bytes())
			header.ParentBeaconRoot = &root

			// Copied from op-program/client/l2/engineapi/block_processor.go
			// TODO(client-pod#826)
			// Unfortunately this is not part of any Geth environment setup,
			// we just have to apply it, like how the Geth block-builder worker does.
			context := core.NewEVMBlockContext(header, s.l1Chain, nil, s.l1Chain.Config(), statedb)
			// NOTE: Unlikely to be needed for the beacon block root, but we setup any precompile overrides anyways for forwards-compatibility
			var precompileOverrides vm.PrecompileOverrides
			if vmConfig := s.l1Chain.GetVMConfig(); vmConfig != nil && vmConfig.PrecompileOverrides != nil {
				precompileOverrides = vmConfig.PrecompileOverrides
			}
			vmenv := vm.NewEVM(context, vm.TxContext{}, statedb, s.l1Chain.Config(), vm.Config{PrecompileOverrides: precompileOverrides})
			core.ProcessBeaconBlockRoot(*header.ParentBeaconRoot, vmenv, statedb)
		}

		s.l1Building = true
		s.l1BuildingHeader = header
		s.l1BuildingState = statedb
		s.l1Receipts = make([]*types.Receipt, 0)
		s.L1Transactions = make([]*types.Transaction, 0)
		s.pendingIndices = make(map[common.Address]uint64)
		s.l1BuildingBlobSidecars = make([]*types.BlobTxSidecar, 0)

		s.L1GasPool = new(core.GasPool).AddGas(header.GasLimit)
	}
}

// ActL1IncludeTx includes the next tx from L1 tx pool from the given account
func (s *L1Miner) ActL1IncludeTx(from common.Address) Action {
	return func(t Testing) {
		if !s.l1Building {
			t.InvalidAction("no tx inclusion when not building l1 block")
			return
		}
		getPendingIndex := func(from common.Address) uint64 {
			return s.pendingIndices[from]
		}
		tx := firstValidTx(t, from, getPendingIndex, s.Eth.TxPool().ContentFrom, s.EthClient().NonceAt)
		s.IncludeTx(t, tx)
		s.pendingIndices[from] = s.pendingIndices[from] + 1 // won't retry the tx
	}
}

// ActL1IncludeTxByHash tries to include a tx by tx-hash.
func (s *L1Miner) ActL1IncludeTxByHash(txHash common.Hash) Action {
	return func(t Testing) {
		if !s.l1Building {
			t.InvalidAction("no tx inclusion when not building l1 block")
			return
		}
		tx := s.Eth.TxPool().Get(txHash)
		require.NotNil(t, tx, "cannot find tx %s", txHash)
		s.IncludeTx(t, tx)
		from, err := s.l1Signer.Sender(tx)
		require.NoError(t, err)
		s.pendingIndices[from] = s.pendingIndices[from] + 1 // won't retry the tx
	}
}

func (s *L1Miner) IncludeTx(t Testing, tx *types.Transaction) {
	from, err := s.l1Signer.Sender(tx)
	require.NoError(t, err)
	s.log.Info("including tx", "nonce", tx.Nonce(), "from", from, "to", tx.To())
	if tx.Gas() > s.l1BuildingHeader.GasLimit {
		t.Fatalf("tx consumes %d gas, more than available in L1 block %d", tx.Gas(), s.l1BuildingHeader.GasLimit)
	}
	if tx.Gas() > uint64(*s.L1GasPool) {
		t.InvalidAction("action takes too much gas: %d, only have %d", tx.Gas(), uint64(*s.L1GasPool))
		return
	}
	s.l1BuildingState.SetTxContext(tx.Hash(), len(s.L1Transactions))
	receipt, err := core.ApplyTransaction(s.l1Cfg.Config, s.l1Chain, &s.l1BuildingHeader.Coinbase,
		s.L1GasPool, s.l1BuildingState, s.l1BuildingHeader, tx.WithoutBlobTxSidecar(), &s.l1BuildingHeader.GasUsed, *s.l1Chain.GetVMConfig())
	if err != nil {
		s.l1TxFailed = append(s.l1TxFailed, tx)
		t.Fatalf("failed to apply transaction to L1 block (tx %d): %v", len(s.L1Transactions), err)
	}
	s.l1Receipts = append(s.l1Receipts, receipt)
	s.L1Transactions = append(s.L1Transactions, tx.WithoutBlobTxSidecar())
	if tx.Type() == types.BlobTxType {
		require.True(t, s.l1Cfg.Config.IsCancun(s.l1BuildingHeader.Number, s.l1BuildingHeader.Time), "L1 must be cancun to process blob tx")
		sidecar := tx.BlobTxSidecar()
		if sidecar != nil {
			s.l1BuildingBlobSidecars = append(s.l1BuildingBlobSidecars, sidecar)
		}
		*s.l1BuildingHeader.BlobGasUsed += receipt.BlobGasUsed
	}
}

func (s *L1Miner) ActL1SetFeeRecipient(coinbase common.Address) {
	s.prefCoinbase = coinbase
	if s.l1Building {
		s.l1BuildingHeader.Coinbase = coinbase
	}
}

// ActL1EndBlock finishes the new L1 block, and applies it to the chain as unsafe block
func (s *L1Miner) ActL1EndBlock(t Testing) *types.Block {
	if !s.l1Building {
		t.InvalidAction("cannot end L1 block when not building block")
		return nil
	}

	s.l1Building = false
	s.l1BuildingHeader.GasUsed = s.l1BuildingHeader.GasLimit - uint64(*s.L1GasPool)
	s.l1BuildingHeader.Root = s.l1BuildingState.IntermediateRoot(s.l1Cfg.Config.IsEIP158(s.l1BuildingHeader.Number))

	var withdrawals []*types.Withdrawal
	if s.l1Cfg.Config.IsShanghai(s.l1BuildingHeader.Number, s.l1BuildingHeader.Time) {
		withdrawals = make([]*types.Withdrawal, 0)
	}

	block := types.NewBlock(s.l1BuildingHeader, &types.Body{Transactions: s.L1Transactions, Withdrawals: withdrawals}, s.l1Receipts, trie.NewStackTrie(nil))
	if s.l1Cfg.Config.IsCancun(s.l1BuildingHeader.Number, s.l1BuildingHeader.Time) {
		parent := s.l1Chain.GetHeaderByHash(s.l1BuildingHeader.ParentHash)
		var (
			parentExcessBlobGas uint64
			parentBlobGasUsed   uint64
		)
		if parent.ExcessBlobGas != nil {
			parentExcessBlobGas = *parent.ExcessBlobGas
			parentBlobGasUsed = *parent.BlobGasUsed
		}
		excessBlobGas := eip4844.CalcExcessBlobGas(parentExcessBlobGas, parentBlobGasUsed)
		s.l1BuildingHeader.ExcessBlobGas = &excessBlobGas
	}

	// Write state changes to db
	root, err := s.l1BuildingState.Commit(s.l1BuildingHeader.Number.Uint64(), s.l1Cfg.Config.IsEIP158(s.l1BuildingHeader.Number))
	if err != nil {
		t.Fatalf("l1 state write error: %v", err)
	}
	if err := s.l1BuildingState.Database().TrieDB().Commit(root, false); err != nil {
		t.Fatalf("l1 trie write error: %v", err)
	}
	// now that the blob txs are in a canonical block, flush them to the blob store
	for _, sidecar := range s.l1BuildingBlobSidecars {
		for i, h := range sidecar.BlobHashes() {
			blob := (*eth.Blob)(&sidecar.Blobs[i])
			indexedHash := eth.IndexedBlobHash{Index: uint64(i), Hash: h}
			s.blobStore.StoreBlob(block.Time(), indexedHash, blob)
		}
	}
	_, err = s.l1Chain.InsertChain(types.Blocks{block})
	if err != nil {
		t.Fatalf("failed to insert block into l1 chain")
	}
	return block
}

func (s *L1Miner) ActEmptyBlock(t Testing) *types.Block {
	s.ActL1StartBlock(12)(t)
	return s.ActL1EndBlock(t)
}

func (s *L1Miner) Close() error {
	return s.L1Replica.Close()
}
