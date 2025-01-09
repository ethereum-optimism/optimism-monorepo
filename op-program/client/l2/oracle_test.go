package l2

import (
	"fmt"
	"math/rand"
	"testing"

	types2 "github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/mpt"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func mockPreimageOracle(t *testing.T) (po *PreimageOracle, hintsMock *mock.Mock, preimages map[common.Hash][]byte) {
	// Prepare the pre-images
	preimages = make(map[common.Hash][]byte)

	hintsMock = new(mock.Mock)

	po = &PreimageOracle{
		oracle: preimage.OracleFn(func(key preimage.Key) []byte {
			v, ok := preimages[key.PreimageKey()]
			require.True(t, ok, "preimage must exist")
			return v
		}),
		hint: preimage.HinterFn(func(v preimage.Hint) {
			hintsMock.MethodCalled("hint", v.Hint())
		}),
	}

	return po, hintsMock, preimages
}

// testBlock tests that the given block can be passed through the preimage oracle.
func testBlock(t *testing.T, block *types.Block) {
	po, hints, preimages := mockPreimageOracle(t)

	hdrBytes, err := rlp.EncodeToBytes(block.Header())
	require.NoError(t, err)
	preimages[preimage.Keccak256Key(block.Hash()).PreimageKey()] = hdrBytes

	opaqueTxs, err := eth.EncodeTransactions(block.Transactions())
	require.NoError(t, err)
	_, txsNodes := mpt.WriteTrie(opaqueTxs)
	for _, p := range txsNodes {
		preimages[preimage.Keccak256Key(crypto.Keccak256Hash(p)).PreimageKey()] = p
	}

	// Prepare a raw mock pre-image oracle that will serve the pre-image data and handle hints

	// Check if blocks with txs work
	hints.On("hint", BlockHeaderHint(block.Hash()).Hint()).Once().Return()
	hints.On("hint", TransactionsHint(block.Hash()).Hint()).Once().Return()
	gotBlock := po.BlockByHash(block.Hash())
	hints.AssertExpectations(t)

	require.Equal(t, gotBlock.Hash(), block.Hash())
	expectedTxs := block.Transactions()
	require.Equal(t, len(expectedTxs), len(gotBlock.Transactions()), "expecting equal tx list length")
	for i, tx := range gotBlock.Transactions() {
		require.Equalf(t, tx.Hash(), expectedTxs[i].Hash(), "expecting tx %d to match", i)
	}
}

func TestPreimageOracleBlockByHash(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	for i := 0; i < 10; i++ {
		block, _ := testutils.RandomBlock(rng, 10)
		t.Run(fmt.Sprintf("block_%d", i), func(t *testing.T) {
			testBlock(t, block)
		})
	}
}

func TestPreimageOracleNodeByHash(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("node_%d", i), func(t *testing.T) {
			po, hints, preimages := mockPreimageOracle(t)

			node := make([]byte, 123)
			rng.Read(node)

			h := crypto.Keccak256Hash(node)
			preimages[preimage.Keccak256Key(h).PreimageKey()] = node

			hints.On("hint", StateNodeHint(h).Hint()).Once().Return()
			gotNode := po.NodeByHash(h)
			hints.AssertExpectations(t)
			require.Equal(t, hexutil.Bytes(node), hexutil.Bytes(gotNode), "node matches")
		})
	}
}

func TestPreimageOracleCodeByHash(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("code_%d", i), func(t *testing.T) {
			po, hints, preimages := mockPreimageOracle(t)

			node := make([]byte, 123)
			rng.Read(node)

			h := crypto.Keccak256Hash(node)
			preimages[preimage.Keccak256Key(h).PreimageKey()] = node

			hints.On("hint", CodeHint(h).Hint()).Once().Return()
			gotNode := po.CodeByHash(h)
			hints.AssertExpectations(t)
			require.Equal(t, hexutil.Bytes(node), hexutil.Bytes(gotNode), "code matches")
		})
	}
}

func TestPreimageOracleOutputByRoot(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("output_%d", i), func(t *testing.T) {
			po, hints, preimages := mockPreimageOracle(t)
			output := testutils.RandomOutputV0(rng)

			h := common.Hash(eth.OutputRoot(output))
			preimages[preimage.Keccak256Key(h).PreimageKey()] = output.Marshal()
			hints.On("hint", L2OutputHint(h).Hint()).Once().Return()
			gotOutput := po.OutputByRoot(h)
			hints.AssertExpectations(t)
			require.Equal(t, hexutil.Bytes(output.Marshal()), hexutil.Bytes(gotOutput.Marshal()), "output matches")
		})
	}
}

func TestPreimageOracleTransitionStateByRoot(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("super_root_%d", i), func(t *testing.T) {
			po, hints, preimages := mockPreimageOracle(t)
			output := testutils.RandomSuperV1(rng)
			transitionState := types2.TransitionState{SuperRoot: output.Marshal()}
			expected, err := transitionState.Marshal()
			require.NoError(t, err)

			h := common.Hash(eth.SuperRoot(output))
			preimages[preimage.Keccak256Key(h).PreimageKey()] = output.Marshal()
			hints.On("hint", AgreedPrestateHint(h).Hint()).Once().Return()
			gotOutput := po.TransitionStateByRoot(h)
			hints.AssertExpectations(t)
			actual, err := gotOutput.Marshal()
			require.NoError(t, err)
			require.Equal(t, hexutil.Bytes(expected), hexutil.Bytes(actual), "output matches")
		})

		t.Run(fmt.Sprintf("state_%d", i), func(t *testing.T) {
			po, hints, preimages := mockPreimageOracle(t)
			output := testutils.RandomSuperV1(rng)
			transitionState := types2.TransitionState{
				SuperRoot: output.Marshal(),
				PendingProgress: []types2.OptimisticBlock{
					{
						BlockHash:  testutils.RandomHash(rng),
						OutputRoot: eth.Bytes32(testutils.RandomHash(rng)),
					},
					{
						BlockHash:  testutils.RandomHash(rng),
						OutputRoot: eth.Bytes32(testutils.RandomHash(rng)),
					},
				},
				Step: 2,
			}
			expected, err := transitionState.Marshal()
			require.NoError(t, err)

			h := crypto.Keccak256Hash(expected)
			preimages[preimage.Keccak256Key(h).PreimageKey()] = expected
			hints.On("hint", AgreedPrestateHint(h).Hint()).Once().Return()
			gotOutput := po.TransitionStateByRoot(h)
			hints.AssertExpectations(t)
			actual, err := gotOutput.Marshal()
			require.NoError(t, err)
			require.Equal(t, hexutil.Bytes(expected), hexutil.Bytes(actual), "output matches")
		})
	}
}
