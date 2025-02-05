package l2

import (
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-program/client/l2/test"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// Should be an Oracle implementation
var _ Oracle = (*CachingOracle)(nil)

func TestBlockByHash(t *testing.T) {
	chainID := uint64(48294)
	stub, _ := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)

	rng := rand.New(rand.NewSource(1))
	block, _ := testutils.RandomBlock(rng, 1)

	// Initial call retrieves from the stub
	stub.Blocks[block.Hash()] = block
	actual := oracle.BlockByHash(block.Hash(), eth.ChainIDFromUInt64(chainID))
	require.Equal(t, block, actual)

	// Later calls should retrieve from cache (even if chain ID is different)
	delete(stub.Blocks, block.Hash())
	actual = oracle.BlockByHash(block.Hash(), eth.ChainIDFromUInt64(9982))
	require.Equal(t, block, actual)
}

func TestNodeByHash(t *testing.T) {
	stub, stateStub := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)

	node := []byte{12, 3, 4}
	hash := common.Hash{0xaa}

	// Initial call retrieves from the stub
	stateStub.Data[hash] = node
	actual := oracle.NodeByHash(hash, eth.ChainIDFromUInt64(1234))
	require.Equal(t, node, actual)

	// Later calls should retrieve from cache (even if chain ID is different)
	delete(stateStub.Data, hash)
	actual = oracle.NodeByHash(hash, eth.ChainIDFromUInt64(997845))
	require.Equal(t, node, actual)
}

func TestReceiptsByBlockHash(t *testing.T) {
	chainID := eth.ChainIDFromUInt64(48294)
	stub, _ := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)

	rng := rand.New(rand.NewSource(1))
	block, rcpts := testutils.RandomBlock(rng, 1)

	// Initial call retrieves from the stub
	stub.Blocks[block.Hash()] = block
	stub.Receipts[block.Hash()] = rcpts
	actualBlock, actualRcpts := oracle.ReceiptsByBlockHash(block.Hash(), chainID)
	require.EqualValues(t, block, actualBlock)
	require.EqualValues(t, rcpts, actualRcpts)

	// Later calls should retrieve from cache (even if chain ID is different)
	delete(stub.Blocks, block.Hash())
	delete(stub.Receipts, block.Hash())
	actualBlock, actualRcpts = oracle.ReceiptsByBlockHash(block.Hash(), eth.ChainIDFromUInt64(9982))
	require.EqualValues(t, block, actualBlock)
	require.EqualValues(t, rcpts, actualRcpts)
}

func TestCodeByHash(t *testing.T) {
	stub, stateStub := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)

	node := []byte{12, 3, 4}
	hash := common.Hash{0xaa}

	// Initial call retrieves from the stub
	stateStub.Code[hash] = node
	actual := oracle.CodeByHash(hash, eth.ChainIDFromUInt64(342))
	require.Equal(t, node, actual)

	// Later calls should retrieve from cache (even if the chain ID is different)
	delete(stateStub.Code, hash)
	actual = oracle.CodeByHash(hash, eth.ChainIDFromUInt64(986776))
	require.Equal(t, node, actual)
}

func TestOutputByRoot(t *testing.T) {
	stub, _ := test.NewStubOracle(t)
	oracle := NewCachingOracle(stub)

	rng := rand.New(rand.NewSource(1))
	output := testutils.RandomOutputV0(rng)

	// Initial call retrieves from the stub
	root := common.Hash(eth.OutputRoot(output))
	stub.Outputs[root] = output
	actual := oracle.OutputByRoot(root, eth.ChainIDFromUInt64(59284))
	require.Equal(t, output, actual)

	// Later calls should retrieve from cache (even if the chain ID is different)
	delete(stub.Outputs, root)
	actual = oracle.OutputByRoot(root, eth.ChainIDFromUInt64(9193))
	require.Equal(t, output, actual)
}
