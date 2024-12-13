package cross

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestCrossSafeUpdate(t *testing.T) {
	t.Run("scopedCrossSafeUpdate passes", func(t *testing.T) {
		ctx := context.Background()
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		csd.checkFn = func(chainID types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns no error,
		// no error is returned
		err := CrossSafeUpdate(ctx, logger, chainID, csd)
		require.NoError(t, err)
	})
	t.Run("scopedCrossSafeUpdate reuturns error", func(t *testing.T) {
		ctx := context.Background()
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, errors.New("some error")
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns an error,
		// (by way of OpenBlock returning an error),
		// the error is returned
		err := CrossSafeUpdate(ctx, logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
	})
	t.Run("scopedCrossSafeUpdate reuturns ErrOutOfScope", func(t *testing.T) {
		ctx := context.Background()
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrOutOfScope
		}
		newScope := eth.BlockRef{Number: 3}
		csd.nextDerivedFromFn = func(chain types.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error) {
			return newScope, nil
		}
		currentCrossSafe := types.BlockSeal{Number: 5}
		csd.crossSafeFn = func(chainID types.ChainID) (derivedFrom types.BlockSeal, derived types.BlockSeal, err error) {
			return types.BlockSeal{}, currentCrossSafe, nil
		}
		parent := types.BlockSeal{Number: 4}
		csd.previousDerivedFn = func(chain types.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
			return parent, nil
		}
		csd.deps = mockDependencySet{}
		var updatingChain types.ChainID
		var updatingCandidateScope eth.BlockRef
		var updatingCandidate eth.BlockRef
		csd.updateCrossSafeFn = func(chain types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
			updatingChain = chain
			updatingCandidateScope = l1View
			updatingCandidate = lastCrossDerived
			return nil
		}
		// when scopedCrossSafeUpdate returns Out of Scope error,
		// CrossSafeUpdate proceeds anyway and calls UpdateCrossSafe
		// the update uses the new scope returned by NextDerivedFrom
		// and a crossSafeRef made from the current crossSafe and its parent
		err := CrossSafeUpdate(ctx, logger, chainID, csd)
		require.NoError(t, err)
		require.Equal(t, chainID, updatingChain)
		require.Equal(t, newScope, updatingCandidateScope)
		crossSafeRef := currentCrossSafe.MustWithParent(parent.ID())
		require.Equal(t, crossSafeRef, updatingCandidate)
	})
	t.Run("NextDerivedFrom returns error", func(t *testing.T) {
		ctx := context.Background()
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrOutOfScope
		}
		csd.nextDerivedFromFn = func(chain types.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error) {
			return eth.BlockRef{}, errors.New("some error")
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns Out of Scope error,
		// and NextDerivedFrom returns an error,
		// the error is returned
		err := CrossSafeUpdate(ctx, logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
	})
	t.Run("PreviousDerived returns error", func(t *testing.T) {
		ctx := context.Background()
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrOutOfScope
		}
		csd.previousDerivedFn = func(chain types.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
			return types.BlockSeal{}, errors.New("some error")
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns Out of Scope error,
		// and PreviousDerived returns an error,
		// the error is returned
		err := CrossSafeUpdate(ctx, logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
	})
	t.Run("UpdateCrossSafe returns error", func(t *testing.T) {
		ctx := context.Background()
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrOutOfScope
		}
		csd.updateCrossSafeFn = func(chain types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
			return errors.New("some error")
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns Out of Scope error,
		// and UpdateCrossSafe returns an error,
		// the error is returned
		err := CrossSafeUpdate(ctx, logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
	})
}

func TestScopedCrossSafeUpdate(t *testing.T) {
	t.Run("CandidateCrossSafe returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return eth.BlockRef{}, eth.BlockRef{}, errors.New("some error")
		}
		// when CandidateCrossSafe returns an error,
		// the error is returned
		blockRef, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.Equal(t, eth.BlockRef{}, blockRef)
	})
	t.Run("CandidateCrossSafe returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, errors.New("some error")
		}
		// when OpenBlock returns an error,
		// the error is returned
		blockRef, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.Equal(t, eth.BlockRef{}, blockRef)
	})
	t.Run("candidate does not match opened block", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return eth.BlockRef{}, candidate, nil
		}
		opened := eth.BlockRef{Number: 2}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 0, nil, nil
		}
		// when OpenBlock and CandidateCrossSafe return different blocks,
		// an ErrConflict is returned
		blockRef, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorIs(t, err, types.ErrConflict)
		require.Equal(t, eth.BlockRef{}, blockRef)
	})
	t.Run("CrossSafeHazards returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return eth.BlockRef{}, candidate, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		// cause CrossSafeHazards to return an error by making ChainIDFromIndex return an error
		csd.deps = mockDependencySet{}
		csd.deps.chainIDFromIndexfn = func() (types.ChainID, error) {
			return types.ChainID{}, errors.New("some error")
		}
		// when CrossSafeHazards returns an error,
		// the error is returned
		blockRef, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.ErrorContains(t, err, "dependencies of cross-safe candidate")
		require.Equal(t, eth.BlockRef{}, blockRef)
	})
	t.Run("HazardSafeFrontierChecks returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return eth.BlockRef{}, candidate, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		csd.checkFn = func(chainID types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		count := 0
		csd.deps = mockDependencySet{}
		// cause CrossSafeHazards to return an error by making ChainIDFromIndex return an error
		// but only on the second call (which will be used by HazardSafeFrontierChecks)
		csd.deps.chainIDFromIndexfn = func() (types.ChainID, error) {
			defer func() { count++ }()
			if count == 0 {
				return types.ChainID{}, nil
			}
			return types.ChainID{}, errors.New("some error")
		}
		// when CrossSafeHazards returns an error,
		// the error is returned
		blockRef, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.ErrorContains(t, err, "frontier")
		require.Equal(t, eth.BlockRef{}, blockRef)
	})
	t.Run("HazardCycleChecks returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1, Time: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		opened := eth.BlockRef{Number: 1, Time: 1}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1, LogIdx: 2}
		em2 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1, LogIdx: 1}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 3, map[uint32]*types.ExecutingMessage{1: em1, 2: em2}, nil
		}
		csd.checkFn = func(chainID types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		csd.deps = mockDependencySet{}

		// HazardCycleChecks returns an error with appropriate wrapping
		blockRef, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "cycle detected")
		require.ErrorContains(t, err, "failed to verify block")
		require.Equal(t, eth.BlockRef{Number: 2}, blockRef)
	})
	t.Run("UpdateCrossSafe returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		csd.checkFn = func(chainID types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		csd.deps = mockDependencySet{}
		csd.updateCrossSafeFn = func(chain types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
			return errors.New("some error")
		}
		// when UpdateCrossSafe returns an error,
		// the error is returned
		blockRef, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.ErrorContains(t, err, "failed to update")
		require.Equal(t, eth.BlockRef{Number: 2}, blockRef)
	})
	t.Run("successful update", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := types.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return candidateScope, candidate, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		csd.deps = mockDependencySet{}
		var updatingChain types.ChainID
		var updatingCandidateScope eth.BlockRef
		var updatingCandidate eth.BlockRef
		csd.updateCrossSafeFn = func(chain types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
			updatingChain = chain
			updatingCandidateScope = l1View
			updatingCandidate = lastCrossDerived
			return nil
		}
		// when no errors occur, the update is carried out
		// the used candidate and scope are from CandidateCrossSafe
		// the candidateScope is returned
		csd.checkFn = func(chainID types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		blockRef, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.Equal(t, chainID, updatingChain)
		require.Equal(t, candidateScope, updatingCandidateScope)
		require.Equal(t, candidate, updatingCandidate)
		require.Equal(t, candidateScope, blockRef)
		require.NoError(t, err)
	})
}

type mockCrossSafeDeps struct {
	deps                 mockDependencySet
	crossSafeFn          func(chainID types.ChainID) (derivedFrom types.BlockSeal, derived types.BlockSeal, err error)
	candidateCrossSafeFn func() (derivedFromScope, crossSafe eth.BlockRef, err error)
	openBlockFn          func(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
	updateCrossSafeFn    func(chain types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error
	nextDerivedFromFn    func(chain types.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error)
	previousDerivedFn    func(chain types.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error)
	checkFn              func(chainID types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error)
}

func (m *mockCrossSafeDeps) CrossSafe(chainID types.ChainID) (derivedFrom types.BlockSeal, derived types.BlockSeal, err error) {
	if m.crossSafeFn != nil {
		return m.crossSafeFn(chainID)
	}
	return types.BlockSeal{}, types.BlockSeal{}, nil
}

func (m *mockCrossSafeDeps) CandidateCrossSafe(chain types.ChainID) (derivedFromScope, crossSafe eth.BlockRef, err error) {
	if m.candidateCrossSafeFn != nil {
		return m.candidateCrossSafeFn()
	}
	return eth.BlockRef{}, eth.BlockRef{}, nil
}

func (m *mockCrossSafeDeps) DependencySet() depset.DependencySet {
	return m.deps
}

func (m *mockCrossSafeDeps) CrossDerivedFrom(chainID types.ChainID, derived eth.BlockID) (derivedFrom types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}

func (m *mockCrossSafeDeps) Check(chainID types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
	if m.checkFn != nil {
		return m.checkFn(chainID, blockNum, logIdx, logHash)
	}
	return types.BlockSeal{}, nil
}

func (m *mockCrossSafeDeps) NextDerivedFrom(chain types.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error) {
	if m.nextDerivedFromFn != nil {
		return m.nextDerivedFromFn(chain, derivedFrom)
	}
	return eth.BlockRef{}, nil
}

func (m *mockCrossSafeDeps) PreviousDerived(chain types.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
	if m.previousDerivedFn != nil {
		return m.previousDerivedFn(chain, derived)
	}
	return types.BlockSeal{}, nil
}

func (m *mockCrossSafeDeps) OpenBlock(chainID types.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	if m.openBlockFn != nil {
		return m.openBlockFn(chainID, blockNum)
	}
	return eth.BlockRef{}, 0, nil, nil
}

func (m *mockCrossSafeDeps) UpdateCrossSafe(chain types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
	if m.updateCrossSafeFn != nil {
		return m.updateCrossSafeFn(chain, l1View, lastCrossDerived)
	}
	return nil
}
