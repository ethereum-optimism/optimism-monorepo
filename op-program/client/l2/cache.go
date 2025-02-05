package l2

import (
	interopTypes "github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// blockCacheSize should be set large enough to handle the pipeline reset process of walking back from L2 head to find
// the L1 origin that is old enough to start buffering channel data from.
const blockCacheSize = 3_000
const nodeCacheSize = 100_000
const codeCacheSize = 10_000
const receiptsCacheSize = 100

type CachingOracle struct {
	oracle  Oracle
	blocks  *simplelru.LRU[common.Hash, *types.Block]
	nodes   *simplelru.LRU[common.Hash, []byte]
	rcpts   *simplelru.LRU[common.Hash, types.Receipts]
	codes   *simplelru.LRU[common.Hash, []byte]
	outputs *simplelru.LRU[common.Hash, eth.Output]
}

func NewCachingOracle(oracle Oracle) *CachingOracle {
	blockLRU, _ := simplelru.NewLRU[common.Hash, *types.Block](blockCacheSize, nil)
	nodeLRU, _ := simplelru.NewLRU[common.Hash, []byte](nodeCacheSize, nil)
	rcptsLRU, _ := simplelru.NewLRU[common.Hash, types.Receipts](receiptsCacheSize, nil)
	codeLRU, _ := simplelru.NewLRU[common.Hash, []byte](codeCacheSize, nil)
	outputLRU, _ := simplelru.NewLRU[common.Hash, eth.Output](codeCacheSize, nil)
	return &CachingOracle{
		oracle:  oracle,
		blocks:  blockLRU,
		rcpts:   rcptsLRU,
		nodes:   nodeLRU,
		codes:   codeLRU,
		outputs: outputLRU,
	}
}

func (o *CachingOracle) NodeByHash(nodeHash common.Hash, chainID eth.ChainID) []byte {
	node, ok := o.nodes.Get(nodeHash)
	if ok {
		return node
	}
	node = o.oracle.NodeByHash(nodeHash, chainID)
	o.nodes.Add(nodeHash, node)
	return node
}

func (o *CachingOracle) ReceiptsByBlockHash(blockHash common.Hash, chainID eth.ChainID) (*types.Block, types.Receipts) {
	rcpts, ok := o.rcpts.Get(blockHash)
	if ok {
		return o.BlockByHash(blockHash, chainID), rcpts
	}
	block, rcpts := o.oracle.ReceiptsByBlockHash(blockHash, chainID)
	o.blocks.Add(blockHash, block)
	o.rcpts.Add(blockHash, rcpts)
	return block, rcpts
}

func (o *CachingOracle) CodeByHash(codeHash common.Hash, chainID eth.ChainID) []byte {
	code, ok := o.codes.Get(codeHash)
	if ok {
		return code
	}
	code = o.oracle.CodeByHash(codeHash, chainID)
	o.codes.Add(codeHash, code)
	return code
}

func (o *CachingOracle) BlockByHash(blockHash common.Hash, chainID eth.ChainID) *types.Block {
	block, ok := o.blocks.Get(blockHash)
	if ok {
		return block
	}
	block = o.oracle.BlockByHash(blockHash, chainID)
	o.blocks.Add(blockHash, block)
	return block
}

func (o *CachingOracle) OutputByRoot(root common.Hash, chainID eth.ChainID) eth.Output {
	output, ok := o.outputs.Get(root)
	if ok {
		return output
	}
	output = o.oracle.OutputByRoot(root, chainID)
	o.outputs.Add(root, output)
	return output
}

func (o *CachingOracle) BlockDataByHash(agreedBlockHash, blockHash common.Hash, chainID eth.ChainID) *types.Block {
	// Always request from the oracle even on cache hit. as we want the effects of the host oracle hinting
	block := o.oracle.BlockDataByHash(agreedBlockHash, blockHash, chainID)
	o.blocks.Add(blockHash, block)
	return block
}

func (o *CachingOracle) TransitionStateByRoot(root common.Hash) *interopTypes.TransitionState {
	// Don't bother caching as this is only requested once as part of the bootstrap process
	return o.oracle.TransitionStateByRoot(root)
}
