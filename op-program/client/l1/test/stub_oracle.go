package test

import (
	"encoding/binary"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type StubOracle struct {
	t *testing.T

	// Blocks maps block hash to eth.BlockInfo
	Blocks map[common.Hash]eth.BlockInfo

	// Txs maps block hash to transactions
	Txs map[common.Hash][]eth.GenericTx

	// Rcpts maps Block hash to receipts
	Rcpts map[common.Hash]types.Receipts

	// Blobs maps indexed blob hash to l1 block ref to blob
	Blobs map[eth.L1BlockRef]map[eth.IndexedBlobHash]*eth.Blob

	// PcmpResults maps hashed input to the results of precompile calls
	PcmpResults map[common.Hash][]byte
}

func NewStubOracle(t *testing.T) *StubOracle {
	return &StubOracle{
		t:           t,
		Blocks:      make(map[common.Hash]eth.BlockInfo),
		Txs:         make(map[common.Hash]([]eth.GenericTx)),
		Rcpts:       make(map[common.Hash]types.Receipts),
		Blobs:       make(map[eth.L1BlockRef]map[eth.IndexedBlobHash]*eth.Blob),
		PcmpResults: make(map[common.Hash][]byte),
	}
}

func (o StubOracle) HeaderByBlockHash(blockHash common.Hash) eth.BlockInfo {
	info, ok := o.Blocks[blockHash]
	if !ok {
		o.t.Fatalf("unknown block %s", blockHash)
	}
	return info
}

func (o StubOracle) TransactionsByBlockHash(blockHash common.Hash) (eth.BlockInfo, []eth.GenericTx) {
	txs, ok := o.Txs[blockHash]
	if !ok {
		o.t.Fatalf("unknown txs %s", blockHash)
	}
	return o.HeaderByBlockHash(blockHash), txs
}

func (o StubOracle) ReceiptsByBlockHash(blockHash common.Hash) (eth.BlockInfo, types.Receipts) {
	rcpts, ok := o.Rcpts[blockHash]
	if !ok {
		o.t.Fatalf("unknown rcpts %s", blockHash)
	}
	return o.HeaderByBlockHash(blockHash), rcpts
}

func (o StubOracle) GetBlob(ref eth.L1BlockRef, blobHash eth.IndexedBlobHash) *eth.Blob {
	blobMap, ok := o.Blobs[ref]
	if !ok {
		o.t.Fatalf("unknown blob ref %s", ref)
	}
	blob, ok := blobMap[blobHash]
	if !ok {
		o.t.Fatalf("unknown blob hash %s %d", blobHash.Hash, blobHash.Index)
	}
	return blob
}

func (o StubOracle) Precompile(addr common.Address, input []byte, requiredGas uint64) ([]byte, bool) {
	arg := append(addr.Bytes(), binary.BigEndian.AppendUint64(nil, requiredGas)...)
	arg = append(arg, input...)
	result, ok := o.PcmpResults[crypto.Keccak256Hash(arg)]
	if !ok {
		o.t.Fatalf("unknown kzg point evaluation %x", input)
	}
	return result, true
}
