package l2

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-program/client/l2/engineapi"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
)

type OracleBackedL2Chain struct {
	log        log.Logger
	oracle     Oracle
	chainCfg   *params.ChainConfig
	engine     consensus.Engine
	oracleHead *types.Header
	head       *types.Header
	safe       *types.Header
	finalized  *types.Header
	vmCfg      vm.Config

	// Block by number cache
	hashByNum            map[uint64]common.Hash
	earliestIndexedBlock *types.Header

	// Inserted blocks
	blocks map[common.Hash]*types.Block
	db     ethdb.KeyValueStore
}

// Must implement CachingEngineBackend, not just EngineBackend to ensure that blocks are stored when they are created
// and don't need to be re-executed when sent back via execution_newPayload.
var _ engineapi.CachingEngineBackend = (*OracleBackedL2Chain)(nil)

func NewOracleBackedL2Chain(logger log.Logger, oracle Oracle, precompileOracle engineapi.PrecompileOracle, chainCfg *params.ChainConfig, l2OutputRoot common.Hash) (*OracleBackedL2Chain, error) {
	output := oracle.OutputByRoot(l2OutputRoot)
	outputV0, ok := output.(*eth.OutputV0)
	if !ok {
		return nil, fmt.Errorf("unsupported L2 output version: %d", output.Version())
	}
	head := oracle.BlockByHash(outputV0.BlockHash)
	logger.Info("Loaded L2 head", "hash", head.Hash(), "number", head.Number())
	return &OracleBackedL2Chain{
		log:      logger,
		oracle:   oracle,
		chainCfg: chainCfg,
		engine:   beacon.New(nil),

		hashByNum: map[uint64]common.Hash{
			head.NumberU64(): head.Hash(),
		},
		earliestIndexedBlock: head.Header(),

		// Treat the agreed starting head as finalized - nothing before it can be disputed
		head:       head.Header(),
		safe:       head.Header(),
		finalized:  head.Header(),
		oracleHead: head.Header(),
		blocks:     make(map[common.Hash]*types.Block),
		db:         NewOracleBackedDB(oracle),
		vmCfg: vm.Config{
			PrecompileOverrides: engineapi.CreatePrecompileOverrides(precompileOracle),
		},
	}, nil
}

func (o *OracleBackedL2Chain) CurrentHeader() *types.Header {
	return o.head
}

func (o *OracleBackedL2Chain) GetHeaderByNumber(n uint64) *types.Header {
	if o.head.Number.Uint64() < n {
		return nil
	}
	hash, ok := o.hashByNum[n]
	if ok {
		return o.GetHeaderByHash(hash)
	}
	// Walk back from current head to the requested block number
	h := o.head
	for h.Number.Uint64() > n {
		h = o.GetHeaderByHash(h.ParentHash)
		o.hashByNum[h.Number.Uint64()] = h.Hash()
	}
	o.earliestIndexedBlock = h
	return h
}

func (o *OracleBackedL2Chain) GetTd(hash common.Hash, number uint64) *big.Int {
	// Difficulty is always 0 post-merge and bedrock starts post-merge so total difficulty also always 0
	return common.Big0
}

func (o *OracleBackedL2Chain) CurrentSafeBlock() *types.Header {
	return o.safe
}

func (o *OracleBackedL2Chain) CurrentFinalBlock() *types.Header {
	return o.finalized
}

func (o *OracleBackedL2Chain) GetHeaderByHash(hash common.Hash) *types.Header {
	return o.GetBlockByHash(hash).Header()
}

func (o *OracleBackedL2Chain) GetBlockByHash(hash common.Hash) *types.Block {
	// Check inserted blocks
	block, ok := o.blocks[hash]
	if ok {
		return block
	}
	// Retrieve from the oracle
	return o.oracle.BlockByHash(hash)
}

func (o *OracleBackedL2Chain) GetBlock(hash common.Hash, number uint64) *types.Block {
	var block *types.Block
	if o.oracleHead.Number.Uint64() < number {
		// For blocks above the chain head, only consider newly built blocks
		// Avoids requesting an unknown block from the oracle which would panic.
		block = o.blocks[hash]
	} else {
		block = o.GetBlockByHash(hash)
	}
	if block == nil {
		return nil
	}
	if block.NumberU64() != number {
		return nil
	}
	return block
}

func (o *OracleBackedL2Chain) GetHeader(hash common.Hash, u uint64) *types.Header {
	block := o.GetBlock(hash, u)
	return block.Header()
}

func (o *OracleBackedL2Chain) HasBlockAndState(hash common.Hash, number uint64) bool {
	block := o.GetBlock(hash, number)
	return block != nil
}

func (o *OracleBackedL2Chain) GetCanonicalHash(n uint64) common.Hash {
	header := o.GetHeaderByNumber(n)
	if header == nil {
		return common.Hash{}
	}
	return header.Hash()
}

func (o *OracleBackedL2Chain) GetVMConfig() *vm.Config {
	return &o.vmCfg
}

func (o *OracleBackedL2Chain) Config() *params.ChainConfig {
	return o.chainCfg
}

func (o *OracleBackedL2Chain) Engine() consensus.Engine {
	return o.engine
}

func (o *OracleBackedL2Chain) StateAt(root common.Hash) (*state.StateDB, error) {
	stateDB, err := state.New(root, state.NewDatabase(triedb.NewDatabase(rawdb.NewDatabase(o.db), nil), nil))
	if err != nil {
		return nil, err
	}
	stateDB.MakeSinglethreaded()
	return stateDB, nil
}

func (o *OracleBackedL2Chain) InsertBlockWithoutSetHead(block *types.Block, makeWitness bool) (*stateless.Witness, error) {
	processor, err := engineapi.NewBlockProcessorFromHeader(o, block.Header())
	if err != nil {
		return nil, err
	}
	for i, tx := range block.Transactions() {
		err = processor.AddTx(tx)
		if err != nil {
			return nil, fmt.Errorf("invalid transaction (%d): %w", i, err)
		}
	}
	expected, err := o.AssembleAndInsertBlockWithoutSetHead(processor)
	if err != nil {
		return nil, fmt.Errorf("invalid block: %w", err)
	}
	if expected.Hash() != block.Hash() {
		return nil, fmt.Errorf("block root mismatch, expected: %v, actual: %v", expected.Hash(), block.Hash())
	}
	return nil, nil
}

func (o *OracleBackedL2Chain) AssembleAndInsertBlockWithoutSetHead(processor *engineapi.BlockProcessor) (*types.Block, error) {
	block, err := processor.Assemble()
	if err != nil {
		return nil, fmt.Errorf("invalid block: %w", err)
	}
	err = processor.Commit()
	if err != nil {
		return nil, fmt.Errorf("commit block: %w", err)
	}
	o.blocks[block.Hash()] = block
	return block, nil
}

func (o *OracleBackedL2Chain) SetCanonical(head *types.Block) (common.Hash, error) {
	oldHead := o.head
	o.head = head.Header()

	// Remove canonical hashes after the new header
	for n := head.NumberU64() + 1; n <= oldHead.Number.Uint64(); n++ {
		delete(o.hashByNum, n)
	}

	// Add new canonical blocks to the block by number cache
	// Since the original head is added to the block number cache and acts as the finalized block,
	// at some point we must reach the existing canonical chain and can stop updating.
	h := o.head
	for {
		newHash := h.Hash()
		prevHash, ok := o.hashByNum[h.Number.Uint64()]
		if ok && prevHash == newHash {
			// Connected with the existing canonical chain so stop updating
			break
		}
		o.hashByNum[h.Number.Uint64()] = newHash
		h = o.GetHeaderByHash(h.ParentHash)
	}
	return head.Hash(), nil
}

func (o *OracleBackedL2Chain) SetFinalized(header *types.Header) {
	o.finalized = header
}

func (o *OracleBackedL2Chain) SetSafe(header *types.Header) {
	o.safe = header
}
