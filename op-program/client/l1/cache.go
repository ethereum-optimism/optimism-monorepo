package l1

import (
	"encoding/binary"

	"github.com/hashicorp/golang-lru/v2/simplelru"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// Cache size is quite high as retrieving data from the pre-image oracle can be quite expensive
const cacheSize = 2000

// CachingOracle is an implementation of Oracle that delegates to another implementation, adding caching of all results
type CachingOracle struct {
	oracle Oracle
	blocks *simplelru.LRU[common.Hash, eth.BlockInfo]
	txs    *simplelru.LRU[common.Hash, []eth.GenericTx]
	rcpts  *simplelru.LRU[common.Hash, types.Receipts]
	blobs  *simplelru.LRU[common.Hash, *eth.Blob]
	pcmps  *simplelru.LRU[common.Hash, precompileResult]
}

type precompileResult struct {
	result []byte
	ok     bool
}

func NewCachingOracle(oracle Oracle) *CachingOracle {
	blockLRU, _ := simplelru.NewLRU[common.Hash, eth.BlockInfo](cacheSize, nil)
	txsLRU, _ := simplelru.NewLRU[common.Hash, []eth.GenericTx](cacheSize, nil)
	rcptsLRU, _ := simplelru.NewLRU[common.Hash, types.Receipts](cacheSize, nil)
	blobsLRU, _ := simplelru.NewLRU[common.Hash, *eth.Blob](cacheSize, nil)
	pcmps, _ := simplelru.NewLRU[common.Hash, precompileResult](cacheSize, nil)
	return &CachingOracle{
		oracle: oracle,
		blocks: blockLRU,
		txs:    txsLRU,
		rcpts:  rcptsLRU,
		blobs:  blobsLRU,
		pcmps:  pcmps,
	}
}

func (o *CachingOracle) HeaderByBlockHash(blockHash common.Hash) eth.BlockInfo {
	block, ok := o.blocks.Get(blockHash)
	if ok {
		return block
	}
	block = o.oracle.HeaderByBlockHash(blockHash)
	o.blocks.Add(blockHash, block)
	return block
}

func (o *CachingOracle) TransactionsByBlockHash(blockHash common.Hash) (eth.BlockInfo, []eth.GenericTx) {
	txs, ok := o.txs.Get(blockHash)
	if ok {
		return o.HeaderByBlockHash(blockHash), txs
	}
	block, txs := o.oracle.TransactionsByBlockHash(blockHash)
	o.blocks.Add(blockHash, block)
	o.txs.Add(blockHash, txs)
	return block, txs
}

func (o *CachingOracle) ReceiptsByBlockHash(blockHash common.Hash) (eth.BlockInfo, types.Receipts) {
	rcpts, ok := o.rcpts.Get(blockHash)
	if ok {
		return o.HeaderByBlockHash(blockHash), rcpts
	}
	block, rcpts := o.oracle.ReceiptsByBlockHash(blockHash)
	o.blocks.Add(blockHash, block)
	o.rcpts.Add(blockHash, rcpts)
	return block, rcpts
}

func (o *CachingOracle) GetBlob(ref eth.L1BlockRef, blobHash eth.IndexedBlobHash) *eth.Blob {
	// Create a 32 byte hash key by hashing `blobHash.Hash ++ ref.Time ++ blobHash.Index`
	hashBuf := make([]byte, 48)
	copy(hashBuf[0:32], blobHash.Hash[:])
	binary.BigEndian.PutUint64(hashBuf[32:], ref.Time)
	binary.BigEndian.PutUint64(hashBuf[40:], blobHash.Index)
	cacheKey := crypto.Keccak256Hash(hashBuf)

	blob, ok := o.blobs.Get(cacheKey)
	if ok {
		return blob
	}
	blob = o.oracle.GetBlob(ref, blobHash)
	o.blobs.Add(cacheKey, blob)
	return blob
}

func (o *CachingOracle) Precompile(address common.Address, input []byte, requiredGas uint64) ([]byte, bool) {
	cacheKey := crypto.Keccak256Hash(append(address.Bytes(), input...))
	if val, ok := o.pcmps.Get(cacheKey); ok {
		return val.result, val.ok
	}
	res, ok := o.oracle.Precompile(address, input, requiredGas)
	o.pcmps.Add(cacheKey, precompileResult{res, ok})
	return res, ok
}
