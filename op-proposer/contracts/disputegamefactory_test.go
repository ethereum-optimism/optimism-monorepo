package contracts

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	batchingTest "github.com/ethereum-optimism/optimism/op-service/sources/batching/test"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

var factoryAddr = common.Address{0xff, 0xff}
var proposerAddr = common.Address{0xaa, 0xbb}

func TestHasProposedSince(t *testing.T) {
	cutOffTime := time.Unix(1000, 0)

	t.Run("NoProposals", func(t *testing.T) {
		stubRpc, factory := setupDisputeGameFactoryTest(t)
		withClaims(stubRpc)

		proposed, proposalTime, claim, err := factory.HasProposedSince(context.Background(), proposerAddr, cutOffTime, 0)
		require.NoError(t, err)
		require.False(t, proposed)
		require.Equal(t, time.Time{}, proposalTime)
		require.Equal(t, common.Hash{}, claim)
	})

	t.Run("NoMatchingProposal", func(t *testing.T) {
		stubRpc, factory := setupDisputeGameFactoryTest(t)
		withClaims(
			stubRpc,
			gameMetadata{
				GameType:  0,
				Timestamp: time.Unix(1600, 0),
				Address:   common.Address{0x22},
				Proposer:  common.Address{0xee}, // Wrong proposer
			},
			gameMetadata{
				GameType:  1, // Wrong game type
				Timestamp: time.Unix(1700, 0),
				Address:   common.Address{0x33},
				Proposer:  proposerAddr,
			},
		)

		proposed, proposalTime, claim, err := factory.HasProposedSince(context.Background(), proposerAddr, cutOffTime, 0)
		require.NoError(t, err)
		require.False(t, proposed)
		require.Equal(t, time.Time{}, proposalTime)
		require.Equal(t, common.Hash{}, claim)
	})

	t.Run("MatchingProposalBeforeCutOff", func(t *testing.T) {
		stubRpc, factory := setupDisputeGameFactoryTest(t)
		withClaims(
			stubRpc,
			gameMetadata{
				GameType:  0,
				Timestamp: time.Unix(999, 0),
				Address:   common.Address{0x11},
				Proposer:  proposerAddr,
			},
			gameMetadata{
				GameType:  0,
				Timestamp: time.Unix(1600, 0),
				Address:   common.Address{0x22},
				Proposer:  common.Address{0xee}, // Wrong proposer
			},
			gameMetadata{
				GameType:  1, // Wrong game type
				Timestamp: time.Unix(1700, 0),
				Address:   common.Address{0x33},
				Proposer:  proposerAddr,
			},
		)

		proposed, proposalTime, claim, err := factory.HasProposedSince(context.Background(), proposerAddr, cutOffTime, 0)
		require.NoError(t, err)
		require.False(t, proposed)
		require.Equal(t, time.Time{}, proposalTime)
		require.Equal(t, common.Hash{}, claim)
	})

	t.Run("MatchingProposalAtCutOff", func(t *testing.T) {
		stubRpc, factory := setupDisputeGameFactoryTest(t)
		withClaims(
			stubRpc,
			gameMetadata{
				GameType:  0,
				Timestamp: cutOffTime,
				Address:   common.Address{0x11},
				Proposer:  proposerAddr,
			},
			gameMetadata{
				GameType:  0,
				Timestamp: time.Unix(1600, 0),
				Address:   common.Address{0x22},
				Proposer:  common.Address{0xee}, // Wrong proposer
			},
			gameMetadata{
				GameType:  1, // Wrong game type
				Timestamp: time.Unix(1700, 0),
				Address:   common.Address{0x33},
				Proposer:  proposerAddr,
			},
		)

		proposed, proposalTime, claim, err := factory.HasProposedSince(context.Background(), proposerAddr, cutOffTime, 0)
		require.NoError(t, err)
		require.True(t, proposed)
		require.Equal(t, cutOffTime, proposalTime)
		require.Equal(t, common.Hash{0xdd}, claim)
	})

	t.Run("MatchingProposalAfterCutOff", func(t *testing.T) {
		stubRpc, factory := setupDisputeGameFactoryTest(t)
		expectedProposalTime := time.Unix(1100, 0)
		withClaims(
			stubRpc,
			gameMetadata{
				GameType:  0,
				Timestamp: expectedProposalTime,
				Address:   common.Address{0x11},
				Proposer:  proposerAddr,
			},
			gameMetadata{
				GameType:  0,
				Timestamp: time.Unix(1600, 0),
				Address:   common.Address{0x22},
				Proposer:  common.Address{0xee}, // Wrong proposer
			},
			gameMetadata{
				GameType:  1, // Wrong game type
				Timestamp: time.Unix(1700, 0),
				Address:   common.Address{0x33},
				Proposer:  proposerAddr,
			},
		)

		proposed, proposalTime, claim, err := factory.HasProposedSince(context.Background(), proposerAddr, cutOffTime, 0)
		require.NoError(t, err)
		require.True(t, proposed)
		require.Equal(t, expectedProposalTime, proposalTime)
		require.Equal(t, common.Hash{0xdd}, claim)
	})

	t.Run("MultipleMatchingProposalAfterCutOff", func(t *testing.T) {
		stubRpc, factory := setupDisputeGameFactoryTest(t)
		expectedProposalTime := time.Unix(1600, 0)
		withClaims(
			stubRpc,
			gameMetadata{
				GameType:  0,
				Timestamp: time.Unix(1400, 0),
				Address:   common.Address{0x11},
				Proposer:  proposerAddr,
			},
			gameMetadata{
				GameType:  0,
				Timestamp: time.Unix(1500, 0),
				Address:   common.Address{0x22},
				Proposer:  proposerAddr,
			},
			gameMetadata{
				GameType:  0,
				Timestamp: expectedProposalTime,
				Address:   common.Address{0x33},
				Proposer:  proposerAddr,
			},
		)

		proposed, proposalTime, claim, err := factory.HasProposedSince(context.Background(), proposerAddr, cutOffTime, 0)
		require.NoError(t, err)
		require.True(t, proposed)
		// Should find the most recent proposal
		require.Equal(t, expectedProposalTime, proposalTime)
		require.Equal(t, common.Hash{0xdd}, claim)
	})
}

func TestProposalTx(t *testing.T) {
	stubRpc, factory := setupDisputeGameFactoryTest(t)
	traceType := uint32(123)
	outputRoot := common.Hash{0x01}
	l2BlockNum := common.BigToHash(big.NewInt(456)).Bytes()
	bond := big.NewInt(49284294829)
	stubRpc.SetResponse(factoryAddr, methodInitBonds, rpcblock.Latest, []interface{}{traceType}, []interface{}{bond})
	stubRpc.SetResponse(factoryAddr, methodCreateGame, rpcblock.Latest, []interface{}{traceType, outputRoot, l2BlockNum}, nil)
	tx, err := factory.ProposalTx(context.Background(), traceType, outputRoot, uint64(456))
	require.NoError(t, err)
	stubRpc.VerifyTxCandidate(tx)
	require.NotNil(t, tx.Value)
	require.Truef(t, bond.Cmp(tx.Value) == 0, "Expected bond %v but was %v", bond, tx.Value)
}

func withClaims(stubRpc *batchingTest.AbiBasedRpc, games ...gameMetadata) {
	gameAbi := snapshots.LoadFaultDisputeGameABI()
	stubRpc.SetResponse(factoryAddr, methodGameCount, rpcblock.Latest, nil, []interface{}{big.NewInt(int64(len(games)))})
	for i, game := range games {
		stubRpc.SetResponse(factoryAddr, methodGameAtIndex, rpcblock.Latest, []interface{}{big.NewInt(int64(i))}, []interface{}{
			game.GameType,
			uint64(game.Timestamp.Unix()),
			game.Address,
		})
		stubRpc.AddContract(game.Address, gameAbi)
		stubRpc.SetResponse(game.Address, methodClaim, rpcblock.Latest, []interface{}{big.NewInt(0)}, []interface{}{
			uint32(math.MaxUint32), // Parent address (none for root claim)
			common.Address{},       // Countered by
			game.Proposer,          // Claimant
			big.NewInt(1000),       // Bond
			common.Hash{0xdd},      // Claim
			big.NewInt(1),          // Position (gindex 1 for root position)
			big.NewInt(100),        // Clock
		})
	}
}

func setupDisputeGameFactoryTest(t *testing.T) (*batchingTest.AbiBasedRpc, *DisputeGameFactory) {
	fdgAbi := snapshots.LoadDisputeGameFactoryABI()

	stubRpc := batchingTest.NewAbiBasedRpc(t, factoryAddr, fdgAbi)
	caller := batching.NewMultiCaller(stubRpc, batching.DefaultBatchSize)
	factory := NewDisputeGameFactory(factoryAddr, caller, time.Minute)
	return stubRpc, factory
}
