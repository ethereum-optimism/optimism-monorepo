package super

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	interopTypes "github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

var (
	gameDepth          = types.Depth(30)
	prestateTimestamp  = uint64(1000)
	poststateTimestamp = uint64(5000)
)

func TestGet(t *testing.T) {
	t.Run("AtPostState", func(t *testing.T) {
		provider, stubSupervisor := createProvider(t)
		response := eth.SuperRootResponse{
			Timestamp: poststateTimestamp,
			SuperRoot: eth.Bytes32{0xaa},
			Chains: []eth.ChainRootInfo{
				{
					ChainID:   eth.ChainIDFromUInt64(1),
					Canonical: eth.Bytes32{0xbb},
					Pending:   []byte{0xcc},
				},
			},
		}
		stubSupervisor.Add(response)
		claim, err := provider.Get(context.Background(), types.RootPosition)
		require.NoError(t, err)
		expected := responseToSuper(response)
		require.Equal(t, common.Hash(eth.SuperRoot(expected)), claim)
	})

	t.Run("AtNewTimestamp", func(t *testing.T) {
		provider, stubSupervisor := createProvider(t)
		response := eth.SuperRootResponse{
			Timestamp: prestateTimestamp + 1,
			SuperRoot: eth.Bytes32{0xaa},
			Chains: []eth.ChainRootInfo{
				{
					ChainID:   eth.ChainIDFromUInt64(1),
					Canonical: eth.Bytes32{0xbb},
					Pending:   []byte{0xcc},
				},
			},
		}
		stubSupervisor.Add(response)
		claim, err := provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(StepsPerTimestamp-1)))
		require.NoError(t, err)
		expected := responseToSuper(response)
		require.Equal(t, common.Hash(eth.SuperRoot(expected)), claim)
	})

	t.Run("FirstTimestamp", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1))
		provider, stubSupervisor := createProvider(t)
		outputA1 := testutils.RandomOutputV0(rng)
		outputA2 := testutils.RandomOutputV0(rng)
		outputB1 := testutils.RandomOutputV0(rng)
		outputB2 := testutils.RandomOutputV0(rng)
		superRoot1 := eth.NewSuperV1(
			prestateTimestamp,
			eth.ChainIDAndOutput{ChainID: 1, Output: eth.OutputRoot(outputA1)},
			eth.ChainIDAndOutput{ChainID: 2, Output: eth.OutputRoot(outputB1)})
		superRoot2 := eth.NewSuperV1(prestateTimestamp+1,
			eth.ChainIDAndOutput{ChainID: 1, Output: eth.OutputRoot(outputA2)},
			eth.ChainIDAndOutput{ChainID: 2, Output: eth.OutputRoot(outputB2)})
		stubSupervisor.Add(eth.SuperRootResponse{
			Timestamp: prestateTimestamp,
			SuperRoot: eth.SuperRoot(superRoot1),
			Chains: []eth.ChainRootInfo{
				{
					ChainID:   eth.ChainIDFromUInt64(1),
					Canonical: eth.OutputRoot(outputA1),
					Pending:   outputA1.Marshal(),
				},
				{
					ChainID:   eth.ChainIDFromUInt64(2),
					Canonical: eth.OutputRoot(outputB1),
					Pending:   outputB1.Marshal(),
				},
			},
		})
		stubSupervisor.Add(eth.SuperRootResponse{
			Timestamp: prestateTimestamp + 1,
			SuperRoot: eth.SuperRoot(superRoot2),
			Chains: []eth.ChainRootInfo{
				{
					ChainID:   eth.ChainIDFromUInt64(1),
					Canonical: eth.OutputRoot(outputA2),
					Pending:   outputA2.Marshal(),
				},
				{
					ChainID:   eth.ChainIDFromUInt64(1),
					Canonical: eth.OutputRoot(outputB2),
					Pending:   outputB2.Marshal(),
				},
			},
		})

		expectedFirstStep := &interopTypes.TransitionState{
			SuperRoot: superRoot1.Marshal(),
			PendingProgress: []interopTypes.OptimisticBlock{
				{BlockHash: outputA2.BlockHash, OutputRoot: eth.OutputRoot(outputA2)},
			},
			Step: 1,
		}
		claim, err := provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(0)))
		require.NoError(t, err)
		require.Equal(t, expectedFirstStep.Hash(), claim)

		expectedSecondStep := &interopTypes.TransitionState{
			SuperRoot: superRoot1.Marshal(),
			PendingProgress: []interopTypes.OptimisticBlock{
				{BlockHash: outputA2.BlockHash, OutputRoot: eth.OutputRoot(outputA2)},
				{BlockHash: outputB2.BlockHash, OutputRoot: eth.OutputRoot(outputB2)},
			},
			Step: 2,
		}
		claim, err = provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(1)))
		require.NoError(t, err)
		require.Equal(t, expectedSecondStep.Hash(), claim)

		for step := uint64(3); step < StepsPerTimestamp; step++ {
			expectedPaddingStep := &interopTypes.TransitionState{
				SuperRoot: superRoot1.Marshal(),
				PendingProgress: []interopTypes.OptimisticBlock{
					{BlockHash: outputA2.BlockHash, OutputRoot: eth.OutputRoot(outputA2)},
					{BlockHash: outputB2.BlockHash, OutputRoot: eth.OutputRoot(outputB2)},
				},
				Step: step,
			}
			claim, err = provider.Get(context.Background(), types.NewPosition(gameDepth, new(big.Int).SetUint64(step-1)))
			require.NoError(t, err)
			require.Equalf(t, expectedPaddingStep.Hash(), claim, "incorrect hash at step %v", step)
		}
	})
}

func TestGetStepDataReturnsError(t *testing.T) {
	provider, _ := createProvider(t)
	_, _, _, err := provider.GetStepData(context.Background(), types.RootPosition)
	require.ErrorIs(t, err, ErrGetStepData)
}

func TestGetL2BlockNumberChallengeReturnsError(t *testing.T) {
	provider, _ := createProvider(t)
	_, err := provider.GetL2BlockNumberChallenge(context.Background())
	require.ErrorIs(t, err, types.ErrL2BlockNumberValid)
}

func TestComputeStep(t *testing.T) {
	t.Run("ErrorWhenTraceIndexTooBig", func(t *testing.T) {
		// Uses a big game depth so the trace index doesn't fit in uint64
		provider := NewSuperTraceProvider(testlog.Logger(t, log.LvlInfo), nil, &stubRootProvider{}, eth.BlockID{}, 65, prestateTimestamp, poststateTimestamp)
		// Left-most position in top game
		_, _, err := provider.ComputeStep(types.RootPosition)
		require.ErrorIs(t, err, ErrIndexTooBig)
	})

	t.Run("FirstTimestampSteps", func(t *testing.T) {
		provider, _ := createProvider(t)
		for i := int64(0); i < StepsPerTimestamp-1; i++ {
			timestamp, step, err := provider.ComputeStep(types.NewPosition(gameDepth, big.NewInt(i)))
			require.NoError(t, err)
			// The prestate must be a super root and is on the timestamp boundary.
			// So the first step has the same timestamp and increments step from 0 to 1.
			require.Equalf(t, prestateTimestamp, timestamp, "Incorrect timestamp at trace index %d", i)
			require.Equalf(t, uint64(i+1), step, "Incorrect step at trace index %d", i)
		}
	})

	t.Run("SecondTimestampSteps", func(t *testing.T) {
		provider, _ := createProvider(t)
		for i := int64(-1); i < StepsPerTimestamp-1; i++ {
			traceIndex := StepsPerTimestamp + i
			timestamp, step, err := provider.ComputeStep(types.NewPosition(gameDepth, big.NewInt(traceIndex)))
			require.NoError(t, err)
			// We should now be iterating through the steps of the second timestamp - 1s after the prestate
			require.Equalf(t, prestateTimestamp+1, timestamp, "Incorrect timestamp at trace index %d", traceIndex)
			require.Equalf(t, uint64(i+1), step, "Incorrect step at trace index %d", traceIndex)
		}
	})

	t.Run("LimitToPoststateTimestamp", func(t *testing.T) {
		provider, _ := createProvider(t)
		timestamp, step, err := provider.ComputeStep(types.RootPosition)
		require.NoError(t, err)
		require.Equal(t, poststateTimestamp, timestamp, "Incorrect timestamp at root position")
		require.Equal(t, uint64(0), step, "Incorrect step at trace index at root position")
	})

	t.Run("StepShouldLoopBackToZero", func(t *testing.T) {
		provider, _ := createProvider(t)
		prevTimestamp := prestateTimestamp
		prevStep := uint64(0) // Absolute prestate is always on a timestamp boundary, so step 0
		for traceIndex := int64(0); traceIndex < 5*StepsPerTimestamp; traceIndex++ {
			timestamp, step, err := provider.ComputeStep(types.NewPosition(gameDepth, big.NewInt(traceIndex)))
			require.NoError(t, err)
			if timestamp == prevTimestamp {
				require.Equal(t, prevStep+1, step, "Incorrect step at trace index %d", traceIndex)
			} else {
				require.Equal(t, prevTimestamp+1, timestamp, "Incorrect timestamp at trace index %d", traceIndex)
				require.Zero(t, step, "Incorrect step at trace index %d", traceIndex)
				require.Equal(t, uint64(1023), prevStep, "Should only loop back to step 0 after the consolidation step")
			}
			prevTimestamp = timestamp
			prevStep = step
		}
	})
}

func createProvider(t *testing.T) (*SuperTraceProvider, *stubRootProvider) {
	logger := testlog.Logger(t, log.LvlInfo)
	stubSupervisor := &stubRootProvider{
		rootsByTimestamp: make(map[uint64]eth.SuperRootResponse),
	}
	return NewSuperTraceProvider(logger, nil, stubSupervisor, eth.BlockID{}, gameDepth, prestateTimestamp, poststateTimestamp), stubSupervisor
}

type stubRootProvider struct {
	rootsByTimestamp map[uint64]eth.SuperRootResponse
}

func (s *stubRootProvider) Add(root eth.SuperRootResponse) {
	if s.rootsByTimestamp == nil {
		s.rootsByTimestamp = make(map[uint64]eth.SuperRootResponse)
	}
	s.rootsByTimestamp[root.Timestamp] = root
}

func (s *stubRootProvider) SuperRootAtTimestamp(_ context.Context, timestamp hexutil.Uint64) (eth.SuperRootResponse, error) {
	root, ok := s.rootsByTimestamp[uint64(timestamp)]
	if !ok {
		return eth.SuperRootResponse{}, fmt.Errorf("timestamp %v %w", uint64(timestamp), ethereum.NotFound)
	}
	return root, nil
}
