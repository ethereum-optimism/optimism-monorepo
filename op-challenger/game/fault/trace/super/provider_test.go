package super

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
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
		provider, stubSupervisor, l1Head, _ := createProvider(t)
		response := eth.SuperRootResponse{
			CrossSafeDerivedFrom: l1Head,
			Timestamp:            poststateTimestamp,
			SuperRoot:            eth.Bytes32{0xaa},
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
		provider, stubSupervisor, l1Head, _ := createProvider(t)
		response := eth.SuperRootResponse{
			CrossSafeDerivedFrom: l1Head,
			Timestamp:            prestateTimestamp + 1,
			SuperRoot:            eth.Bytes32{0xaa},
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

	t.Run("ValidTransitionBetweenFirstTwoSuperRoots", func(t *testing.T) {
		provider, stubSupervisor, l1Head, _ := createProvider(t)
		prev, next := createValidSuperRoots(l1Head)
		stubSupervisor.Add(prev.response)
		stubSupervisor.Add(next.response)

		expectValidTransition(t, provider, prev, next)
	})

	t.Run("Step0SuperRootIsSafeBeforeGameL1Head", func(t *testing.T) {
		provider, stubSupervisor, l1Head, _ := createProvider(t)
		response := eth.SuperRootResponse{
			CrossSafeDerivedFrom: eth.BlockID{Number: l1Head.Number - 10, Hash: common.Hash{0xcc}},
			Timestamp:            poststateTimestamp,
			SuperRoot:            eth.Bytes32{0xaa},
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

	t.Run("Step0SuperRootNotSafeAtGameL1Head", func(t *testing.T) {
		provider, stubSupervisor, l1Head, _ := createProvider(t)
		response := eth.SuperRootResponse{
			CrossSafeDerivedFrom: eth.BlockID{Number: l1Head.Number + 1, Hash: common.Hash{0xaa}},
			Timestamp:            poststateTimestamp,
			SuperRoot:            eth.Bytes32{0xaa},
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
		require.Equal(t, InvalidTransitionHash, claim)
	})

	t.Run("NextSuperRootSafeBeforeGameL1Head", func(t *testing.T) {
		provider, stubSupervisor, l1Head, _ := createProvider(t)
		prev, next := createValidSuperRoots(l1Head)
		// Make super roots be safe earlier
		prev.response.CrossSafeDerivedFrom = eth.BlockID{Number: l1Head.Number - 10, Hash: common.Hash{0xaa}}
		next.response.CrossSafeDerivedFrom = eth.BlockID{Number: l1Head.Number - 5, Hash: common.Hash{0xbb}}
		stubSupervisor.Add(prev.response)
		stubSupervisor.Add(next.response)
		expectValidTransition(t, provider, prev, next)
	})

	t.Run("PreviousSuperRootNotSafeAtGameL1Head", func(t *testing.T) {
		provider, stubSupervisor, l1Head, _ := createProvider(t)
		prev, next := createValidSuperRoots(l1Head)
		// Make super roots be safe only after L1 head
		prev.response.CrossSafeDerivedFrom = eth.BlockID{Number: l1Head.Number + 1, Hash: common.Hash{0xaa}}
		next.response.CrossSafeDerivedFrom = eth.BlockID{Number: l1Head.Number + 2, Hash: common.Hash{0xbb}}
		stubSupervisor.Add(prev.response)
		stubSupervisor.Add(next.response)

		// All steps should be the invalid transition hash.
		for i := int64(0); i < StepsPerTimestamp+1; i++ {
			claim, err := provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(i)))
			require.NoError(t, err)
			require.Equalf(t, InvalidTransitionHash, claim, "incorrect claim at index %d", i)
		}
	})

	t.Run("FirstChainUnsafe", func(t *testing.T) {
		provider, stubSupervisor, l1Head, rollupCfgs := createProvider(t)
		prev, next := createValidSuperRoots(l1Head)
		// Make super roots be safe only after L1 head
		prev.response.CrossSafeDerivedFrom = eth.BlockID{Number: l1Head.Number, Hash: common.Hash{0xaa}}
		next.response.CrossSafeDerivedFrom = eth.BlockID{Number: l1Head.Number + 1, Hash: common.Hash{0xbb}}
		stubSupervisor.Add(prev.response)
		stubSupervisor.Add(next.response)

		chain1Cfg, ok := rollupCfgs.Get(eth.ChainIDFromUInt64(1))
		require.True(t, ok)
		chain2Cfg, ok := rollupCfgs.Get(eth.ChainIDFromUInt64(2))
		require.True(t, ok)
		chain1RequiredBlock, err := chain1Cfg.TargetBlockNumber(prestateTimestamp + 1)
		require.NoError(t, err)
		chain2RequiredBlock, err := chain2Cfg.TargetBlockNumber(prestateTimestamp + 1)
		require.NoError(t, err)
		stubSupervisor.SetAllSafeDerivedAt(l1Head, map[eth.ChainID]eth.BlockID{
			eth.ChainIDFromUInt64(1): {Number: chain1RequiredBlock - 1, Hash: common.Hash{0xcc}},
			eth.ChainIDFromUInt64(2): {Number: chain2RequiredBlock, Hash: common.Hash{0xcc}},
		})

		// All steps should be the invalid transition hash.
		for i := int64(0); i < StepsPerTimestamp+1; i++ {
			claim, err := provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(i)))
			require.NoError(t, err)
			require.Equalf(t, InvalidTransitionHash, claim, "incorrect claim at index %d", i)
		}
	})
	t.Run("SecondChainUnsafe", func(t *testing.T) {
		provider, stubSupervisor, l1Head, rollupCfgs := createProvider(t)
		prev, next := createValidSuperRoots(l1Head)
		// Make super roots be safe only after L1 head
		prev.response.CrossSafeDerivedFrom = eth.BlockID{Number: l1Head.Number, Hash: common.Hash{0xaa}}
		next.response.CrossSafeDerivedFrom = eth.BlockID{Number: l1Head.Number + 1, Hash: common.Hash{0xbb}}
		stubSupervisor.Add(prev.response)
		stubSupervisor.Add(next.response)

		chain1Cfg, ok := rollupCfgs.Get(eth.ChainIDFromUInt64(1))
		require.True(t, ok)
		chain2Cfg, ok := rollupCfgs.Get(eth.ChainIDFromUInt64(2))
		require.True(t, ok)
		chain1RequiredBlock, err := chain1Cfg.TargetBlockNumber(prestateTimestamp + 1)
		require.NoError(t, err)
		chain2RequiredBlock, err := chain2Cfg.TargetBlockNumber(prestateTimestamp + 1)
		require.NoError(t, err)
		stubSupervisor.SetAllSafeDerivedAt(l1Head, map[eth.ChainID]eth.BlockID{
			eth.ChainIDFromUInt64(1): {Number: chain1RequiredBlock, Hash: common.Hash{0xcc}},
			eth.ChainIDFromUInt64(2): {Number: chain2RequiredBlock - 1, Hash: common.Hash{0xcc}},
		})

		// First step should be valid because we can reach the required block on chain 1
		claim, err := provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(0)))
		require.NoError(t, err)
		require.NotEqual(t, InvalidTransitionHash, claim, "incorrect claim at index 0")

		// Remaining steps should be the invalid transition hash.
		for i := int64(1); i < StepsPerTimestamp+1; i++ {
			claim, err := provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(i)))
			require.NoError(t, err)
			require.Equalf(t, InvalidTransitionHash, claim, "incorrect claim at index %d", i)
		}
	})
}

func TestGetStepDataReturnsError(t *testing.T) {
	provider, _, _, _ := createProvider(t)
	_, _, _, err := provider.GetStepData(context.Background(), types.RootPosition)
	require.ErrorIs(t, err, ErrGetStepData)
}

func TestGetL2BlockNumberChallengeReturnsError(t *testing.T) {
	provider, _, _, _ := createProvider(t)
	_, err := provider.GetL2BlockNumberChallenge(context.Background())
	require.ErrorIs(t, err, types.ErrL2BlockNumberValid)
}

func TestComputeStep(t *testing.T) {
	t.Run("ErrorWhenTraceIndexTooBig", func(t *testing.T) {
		rollupCfgs, err := NewRollupConfigs(vm.Config{})
		require.NoError(t, err)
		// Uses a big game depth so the trace index doesn't fit in uint64
		provider := NewSuperTraceProvider(testlog.Logger(t, log.LvlInfo), rollupCfgs, nil, &stubRootProvider{}, eth.BlockID{}, 65, prestateTimestamp, poststateTimestamp)
		// Left-most position in top game
		_, _, err = provider.ComputeStep(types.RootPosition)
		require.ErrorIs(t, err, ErrIndexTooBig)
	})

	t.Run("FirstTimestampSteps", func(t *testing.T) {
		provider, _, _, _ := createProvider(t)
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
		provider, _, _, _ := createProvider(t)
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
		provider, _, _, _ := createProvider(t)
		timestamp, step, err := provider.ComputeStep(types.RootPosition)
		require.NoError(t, err)
		require.Equal(t, poststateTimestamp, timestamp, "Incorrect timestamp at root position")
		require.Equal(t, uint64(0), step, "Incorrect step at trace index at root position")
	})

	t.Run("StepShouldLoopBackToZero", func(t *testing.T) {
		provider, _, _, _ := createProvider(t)
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

func createProvider(t *testing.T) (*SuperTraceProvider, *stubRootProvider, eth.BlockID, *RollupConfigs) {
	logger := testlog.Logger(t, log.LvlInfo)
	l1Head := eth.BlockID{Number: 23542, Hash: common.Hash{0xab, 0xcd}}
	stubSupervisor := &stubRootProvider{
		rootsByTimestamp: make(map[uint64]eth.SuperRootResponse),
	}
	chain1Cfg := &rollup.Config{
		L2ChainID: big.NewInt(1),
		Genesis: rollup.Genesis{
			L2Time: 500,
		},
		BlockTime: 1,
	}
	chain2Cfg := &rollup.Config{
		L2ChainID: big.NewInt(2),
		Genesis: rollup.Genesis{
			L2Time: 500,
		},
		BlockTime: 1,
	}
	rollupCfgs, err := NewRollupConfigsFromParsed(chain1Cfg, chain2Cfg)
	require.NoError(t, err)
	provider := NewSuperTraceProvider(logger, rollupCfgs, nil, stubSupervisor, l1Head, gameDepth, prestateTimestamp, poststateTimestamp)
	return provider, stubSupervisor, l1Head, rollupCfgs
}

type superRootData struct {
	response  eth.SuperRootResponse
	super     *eth.SuperV1
	canonical []*eth.OutputV0
	pending   []*eth.OutputV0
}

func createValidSuperRoots(l1Head eth.BlockID) (superRootData, superRootData) {
	rng := rand.New(rand.NewSource(1))
	outputA1 := testutils.RandomOutputV0(rng)
	outputA2 := testutils.RandomOutputV0(rng)
	outputB1 := testutils.RandomOutputV0(rng)
	outputB2 := testutils.RandomOutputV0(rng)
	prevSuper := eth.NewSuperV1(
		prestateTimestamp,
		eth.ChainIDAndOutput{ChainID: eth.ChainIDFromUInt64(1), Output: eth.OutputRoot(outputA1)},
		eth.ChainIDAndOutput{ChainID: eth.ChainIDFromUInt64(2), Output: eth.OutputRoot(outputB1)})
	nextSuper := eth.NewSuperV1(prestateTimestamp+1,
		eth.ChainIDAndOutput{ChainID: eth.ChainIDFromUInt64(1), Output: eth.OutputRoot(outputA2)},
		eth.ChainIDAndOutput{ChainID: eth.ChainIDFromUInt64(2), Output: eth.OutputRoot(outputB2)})
	prevResponse := eth.SuperRootResponse{
		CrossSafeDerivedFrom: l1Head,
		Timestamp:            prestateTimestamp,
		SuperRoot:            eth.SuperRoot(prevSuper),
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
	}
	nextResponse := eth.SuperRootResponse{
		CrossSafeDerivedFrom: l1Head,
		Timestamp:            prestateTimestamp + 1,
		SuperRoot:            eth.SuperRoot(nextSuper),
		Chains: []eth.ChainRootInfo{
			{
				ChainID:   eth.ChainIDFromUInt64(1),
				Canonical: eth.OutputRoot(outputA2),
				Pending:   outputA2.Marshal(),
			},
			{
				ChainID:   eth.ChainIDFromUInt64(2),
				Canonical: eth.OutputRoot(outputB2),
				Pending:   outputB2.Marshal(),
			},
		},
	}
	prev := superRootData{
		response:  prevResponse,
		super:     prevSuper,
		canonical: []*eth.OutputV0{outputA1, outputB1},
		pending:   []*eth.OutputV0{outputA1, outputB1},
	}
	next := superRootData{
		response:  nextResponse,
		super:     nextSuper,
		canonical: []*eth.OutputV0{outputA2, outputB2},
		pending:   []*eth.OutputV0{outputA2, outputB2},
	}
	return prev, next
}

func expectValidTransition(t *testing.T, provider *SuperTraceProvider, prev superRootData, next superRootData) {
	expectedFirstStep := &interopTypes.TransitionState{
		SuperRoot: prev.super.Marshal(),
		PendingProgress: []interopTypes.OptimisticBlock{
			{BlockHash: next.pending[0].BlockHash, OutputRoot: eth.OutputRoot(next.pending[0])},
		},
		Step: 1,
	}
	claim, err := provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(0)))
	require.NoError(t, err)
	require.Equal(t, expectedFirstStep.Hash(), claim)

	expectedSecondStep := &interopTypes.TransitionState{
		SuperRoot: prev.super.Marshal(),
		PendingProgress: []interopTypes.OptimisticBlock{
			{BlockHash: next.pending[0].BlockHash, OutputRoot: eth.OutputRoot(next.pending[0])},
			{BlockHash: next.pending[1].BlockHash, OutputRoot: eth.OutputRoot(next.pending[1])},
		},
		Step: 2,
	}
	claim, err = provider.Get(context.Background(), types.NewPosition(gameDepth, big.NewInt(1)))
	require.NoError(t, err)
	require.Equal(t, expectedSecondStep.Hash(), claim)

	for step := uint64(3); step < StepsPerTimestamp; step++ {
		expectedPaddingStep := &interopTypes.TransitionState{
			SuperRoot: prev.super.Marshal(),
			PendingProgress: []interopTypes.OptimisticBlock{
				{BlockHash: next.pending[0].BlockHash, OutputRoot: eth.OutputRoot(next.pending[0])},
				{BlockHash: next.pending[1].BlockHash, OutputRoot: eth.OutputRoot(next.pending[1])},
			},
			Step: step,
		}
		claim, err = provider.Get(context.Background(), types.NewPosition(gameDepth, new(big.Int).SetUint64(step-1)))
		require.NoError(t, err)
		require.Equalf(t, expectedPaddingStep.Hash(), claim, "incorrect hash at step %v", step)
	}
}

type stubRootProvider struct {
	rootsByTimestamp map[uint64]eth.SuperRootResponse
	allSafeDerivedAt map[eth.BlockID]map[eth.ChainID]eth.BlockID
}

func (s *stubRootProvider) Add(root eth.SuperRootResponse) {
	if s.rootsByTimestamp == nil {
		s.rootsByTimestamp = make(map[uint64]eth.SuperRootResponse)
	}
	s.rootsByTimestamp[root.Timestamp] = root
}

func (s *stubRootProvider) SetAllSafeDerivedAt(derivedFrom eth.BlockID, safeHeads map[eth.ChainID]eth.BlockID) {
	if s.allSafeDerivedAt == nil {
		s.allSafeDerivedAt = make(map[eth.BlockID]map[eth.ChainID]eth.BlockID)
	}
	s.allSafeDerivedAt[derivedFrom] = safeHeads
}

func (s *stubRootProvider) AllSafeDerivedAt(_ context.Context, derivedFrom eth.BlockID) (map[eth.ChainID]eth.BlockID, error) {
	heads, ok := s.allSafeDerivedAt[derivedFrom]
	if !ok {
		return nil, fmt.Errorf("no heads found for block %d", derivedFrom)
	}
	return heads, nil
}

func (s *stubRootProvider) SuperRootAtTimestamp(_ context.Context, timestamp hexutil.Uint64) (eth.SuperRootResponse, error) {
	root, ok := s.rootsByTimestamp[uint64(timestamp)]
	if !ok {
		return eth.SuperRootResponse{}, fmt.Errorf("timestamp %v %w", uint64(timestamp), ethereum.NotFound)
	}
	return root, nil
}
