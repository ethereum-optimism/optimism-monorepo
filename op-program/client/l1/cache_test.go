package l1

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-program/client/l1/test"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// Should implement Oracle
var _ Oracle = (*CachingOracle)(nil)

func TestCachingOracle_HeaderByBlockHash(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	stub := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)
	block := testutils.RandomBlockInfo(rng)

	// Initial call retrieves from the stub
	stub.Blocks[block.Hash()] = block
	result := oracle.HeaderByBlockHash(block.Hash())
	require.Equal(t, block, result)

	// Later calls should retrieve from cache
	delete(stub.Blocks, block.Hash())
	result = oracle.HeaderByBlockHash(block.Hash())
	require.Equal(t, block, result)
}

func TestCachingOracle_TransactionsByBlockHash(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	stub := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)
	block, _ := testutils.RandomBlock(rng, 3)

	// Initial call retrieves from the stub
	stub.Blocks[block.Hash()] = eth.BlockToInfo(block)
	stub.Txs[block.Hash()] = eth.MustToGenericTxSlice(block.Transactions())
	actualBlock, actualTxs := oracle.TransactionsByBlockHash(block.Hash())
	require.Equal(t, eth.BlockToInfo(block), actualBlock)
	require.Equal(t, eth.MustToGenericTxSlice(block.Transactions()), actualTxs)

	// Later calls should retrieve from cache
	delete(stub.Blocks, block.Hash())
	delete(stub.Txs, block.Hash())
	actualBlock, actualTxs = oracle.TransactionsByBlockHash(block.Hash())
	require.Equal(t, eth.BlockToInfo(block), actualBlock)
	require.Equal(t, eth.MustToGenericTxSlice(block.Transactions()), actualTxs)
}

func TestCachingOracle_ReceiptsByBlockHash(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	stub := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)
	block, rcpts := testutils.RandomBlock(rng, 3)

	// Initial call retrieves from the stub
	stub.Blocks[block.Hash()] = eth.BlockToInfo(block)
	stub.Rcpts[block.Hash()] = rcpts
	actualBlock, actualRcpts := oracle.ReceiptsByBlockHash(block.Hash())
	require.Equal(t, eth.BlockToInfo(block), actualBlock)
	require.EqualValues(t, rcpts, actualRcpts)

	// Later calls should retrieve from cache
	delete(stub.Blocks, block.Hash())
	delete(stub.Rcpts, block.Hash())
	actualBlock, actualRcpts = oracle.ReceiptsByBlockHash(block.Hash())
	require.Equal(t, eth.BlockToInfo(block), actualBlock)
	require.EqualValues(t, rcpts, actualRcpts)
}

func TestCachingOracle_GetBlobs(t *testing.T) {
	stub := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)

	l1BlockRef := eth.L1BlockRef{Time: 0}
	indexedBlobHash := eth.IndexedBlobHash{Hash: [32]byte{0xFA, 0xCA, 0xDE}, Index: 0}
	blob := eth.Blob{0xFF}

	// Initial call retrieves from the stub
	stub.Blobs[l1BlockRef] = make(map[eth.IndexedBlobHash]*eth.Blob)
	stub.Blobs[l1BlockRef][indexedBlobHash] = &blob
	actualBlob := oracle.GetBlob(l1BlockRef, indexedBlobHash)
	require.Equal(t, &blob, actualBlob)

	// Later calls should retrieve from cache
	delete(stub.Blobs[l1BlockRef], indexedBlobHash)
	actualBlob = oracle.GetBlob(l1BlockRef, indexedBlobHash)
	require.Equal(t, &blob, actualBlob)
}

func TestCachingOracle_Precompile(t *testing.T) {
	stub := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)

	input := []byte{0x01, 0x02, 0x03, 0x04}
	requiredGas := uint64(100)
	output := []byte{0x0a, 0x0b, 0x0c, 0x0d}
	addr := common.Address{0x1}

	key := crypto.Keccak256Hash(append(append(addr.Bytes(), binary.BigEndian.AppendUint64(nil, requiredGas)...), input...))

	// Initial call retrieves from the stub
	stub.PcmpResults[key] = output
	actualResult, actualStatus := oracle.Precompile(addr, input, requiredGas)
	require.True(t, actualStatus)
	require.EqualValues(t, output, actualResult)

	// Later calls should retrieve from cache
	delete(stub.PcmpResults, key)
	actualResult, actualStatus = oracle.Precompile(addr, input, requiredGas)
	require.True(t, actualStatus)
	require.EqualValues(t, output, actualResult)
}
