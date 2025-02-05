package cross

import (
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
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (pair types.DerivedBlockRefPair, err error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		csd.checkFn = func(chainID eth.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns no error,
		// no error is returned
		err := CrossSafeUpdate(logger, chainID, csd)
		require.NoError(t, err)
	})
	t.Run("scopedCrossSafeUpdate returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (pair types.DerivedBlockRefPair, err error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, errors.New("some error")
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns an error,
		// (by way of OpenBlock returning an error),
		// the error is returned
		err := CrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
	})
	t.Run("scopedCrossSafeUpdate returns ErrAwaitReplacementBlock", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (pair types.DerivedBlockRefPair, err error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrAwaitReplacementBlock
		}
		csd.deps = mockDependencySet{}
		err := CrossSafeUpdate(logger, chainID, csd)
		require.ErrorIs(t, err, types.ErrAwaitReplacementBlock)
	})
	t.Run("scopedCrossSafeUpdate returns ErrConflict and triggers invalidate-local-safe", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (pair types.DerivedBlockRefPair, err error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrConflict
		}
		invalidated := false
		csd.invalidateLocalSafeFn = func(id eth.ChainID, p types.DerivedBlockRefPair) error {
			require.Equal(t, chainID, id)
			require.Equal(t, candidate, p.Derived)
			require.Equal(t, candidateScope, p.Source)
			invalidated = true
			return nil
		}
		csd.deps = mockDependencySet{}
		err := CrossSafeUpdate(logger, chainID, csd)
		require.NoError(t, err)
		require.True(t, invalidated)
	})
	t.Run("scopedCrossSafeUpdate returns ErrOutOfScope", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrOutOfScope
		}
		newScope := eth.BlockRef{Number: 3}
		csd.nextSourceFn = func(chain eth.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error) {
			return newScope, nil
		}
		currentCrossSafe := types.BlockSeal{Number: 5}
		csd.crossSafeFn = func(chainID eth.ChainID) (pair types.DerivedBlockSealPair, err error) {
			return types.DerivedBlockSealPair{Derived: currentCrossSafe}, nil
		}
		parent := types.BlockSeal{Number: 4}
		csd.previousDerivedFn = func(chain eth.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
			return parent, nil
		}
		csd.deps = mockDependencySet{}
		var updatingChain eth.ChainID
		var updatingCandidateScope eth.BlockRef
		var updatingCandidate eth.BlockRef
		csd.updateCrossSafeFn = func(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
			updatingChain = chain
			updatingCandidateScope = l1View
			updatingCandidate = lastCrossDerived
			return nil
		}
		// when scopedCrossSafeUpdate returns Out of Scope error,
		// CrossSafeUpdate proceeds anyway and calls UpdateCrossSafe
		// the update uses the new scope returned by NextSource
		// and a crossSafeRef made from the current crossSafe and its parent
		err := CrossSafeUpdate(logger, chainID, csd)
		require.NoError(t, err)
		require.Equal(t, chainID, updatingChain)
		require.Equal(t, newScope, updatingCandidateScope)
		crossSafeRef := currentCrossSafe.MustWithParent(parent.ID())
		require.Equal(t, crossSafeRef, updatingCandidate)
	})
	t.Run("NextSource returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrOutOfScope
		}
		csd.nextSourceFn = func(chain eth.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error) {
			return eth.BlockRef{}, errors.New("some error")
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns Out of Scope error,
		// and NextSource returns an error,
		// the error is returned
		err := CrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
	})
	t.Run("PreviousDerived returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrOutOfScope
		}
		csd.previousDerivedFn = func(chain eth.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
			return types.BlockSeal{}, errors.New("some error")
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns Out of Scope error,
		// and PreviousDerived returns an error,
		// the error is returned
		err := CrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
	})
	t.Run("UpdateCrossSafe returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, types.ErrOutOfScope
		}
		csd.updateCrossSafeFn = func(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
			return errors.New("some error")
		}
		csd.deps = mockDependencySet{}
		// when scopedCrossSafeUpdate returns Out of Scope error,
		// and UpdateCrossSafe returns an error,
		// the error is returned
		err := CrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
	})
}

func TestScopedCrossSafeUpdate(t *testing.T) {
	t.Run("CandidateCrossSafe returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{}, errors.New("some error")
		}
		// when CandidateCrossSafe returns an error,
		// the error is returned
		candidate, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.Equal(t, eth.BlockRef{}, candidate.Source)
	})
	t.Run("CandidateCrossSafe returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 0, nil, errors.New("some error")
		}
		// when OpenBlock returns an error,
		// the error is returned
		pair, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.Equal(t, eth.BlockRef{}, pair.Source)
	})
	t.Run("candidate does not match opened block", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  eth.BlockRef{},
				Derived: candidate,
			}, nil
		}
		opened := eth.BlockRef{Number: 2}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 0, nil, nil
		}
		// when OpenBlock and CandidateCrossSafe return different blocks,
		// an ErrConflict is returned
		pair, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorIs(t, err, types.ErrConflict)
		require.Equal(t, eth.BlockRef{}, pair.Source)
	})
	t.Run("CrossSafeHazards returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  eth.BlockRef{},
				Derived: candidate,
			}, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		// cause CrossSafeHazards to return an error by making ChainIDFromIndex return an error
		csd.deps = mockDependencySet{}
		csd.deps.chainIDFromIndexfn = func() (eth.ChainID, error) {
			return eth.ChainID{}, errors.New("some error")
		}
		// when CrossSafeHazards returns an error,
		// the error is returned
		pair, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.ErrorContains(t, err, "dependencies of cross-safe candidate")
		require.Equal(t, eth.BlockRef{}, pair.Source)
	})
	t.Run("HazardSafeFrontierChecks returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  eth.BlockRef{},
				Derived: candidate,
			}, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		csd.checkFn = func(chainID eth.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		count := 0
		csd.deps = mockDependencySet{}
		// cause CrossSafeHazards to return an error by making ChainIDFromIndex return an error
		// but only on the second call (which will be used by HazardSafeFrontierChecks)
		csd.deps.chainIDFromIndexfn = func() (eth.ChainID, error) {
			defer func() { count++ }()
			if count == 0 {
				return eth.ChainID{}, nil
			}
			return eth.ChainID{}, errors.New("some error")
		}
		// when CrossSafeHazards returns an error,
		// the error is returned
		pair, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.ErrorContains(t, err, "frontier")
		require.Equal(t, eth.BlockRef{}, pair.Source)
	})
	t.Run("HazardCycleChecks returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1, Time: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		opened := eth.BlockRef{Number: 1, Time: 1}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1, LogIdx: 2}
		em2 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1, LogIdx: 1}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 3, map[uint32]*types.ExecutingMessage{1: em1, 2: em2}, nil
		}
		csd.checkFn = func(chainID eth.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		csd.deps = mockDependencySet{}

		// HazardCycleChecks returns an error with appropriate wrapping
		pair, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "cycle detected")
		require.ErrorContains(t, err, "failed to verify block")
		require.Equal(t, eth.BlockRef{Number: 2}, pair.Source)
	})
	t.Run("UpdateCrossSafe returns error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		csd.checkFn = func(chainID eth.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		csd.deps = mockDependencySet{}
		csd.updateCrossSafeFn = func(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
			return errors.New("some error")
		}
		// when UpdateCrossSafe returns an error,
		// the error is returned
		pair, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.ErrorContains(t, err, "some error")
		require.ErrorContains(t, err, "failed to update")
		require.Equal(t, eth.BlockRef{Number: 2}, pair.Source)
	})
	t.Run("successful update", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		chainID := eth.ChainIDFromUInt64(0)
		csd := &mockCrossSafeDeps{}
		candidate := eth.BlockRef{Number: 1}
		candidateScope := eth.BlockRef{Number: 2}
		csd.candidateCrossSafeFn = func() (types.DerivedBlockRefPair, error) {
			return types.DerivedBlockRefPair{
				Source:  candidateScope,
				Derived: candidate,
			}, nil
		}
		opened := eth.BlockRef{Number: 1}
		execs := map[uint32]*types.ExecutingMessage{1: {}}
		csd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return opened, 10, execs, nil
		}
		csd.deps = mockDependencySet{}
		var updatingChain eth.ChainID
		var updatingCandidateScope eth.BlockRef
		var updatingCandidate eth.BlockRef
		csd.updateCrossSafeFn = func(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
			updatingChain = chain
			updatingCandidateScope = l1View
			updatingCandidate = lastCrossDerived
			return nil
		}
		// when no errors occur, the update is carried out
		// the used candidate and scope are from CandidateCrossSafe
		// the candidateScope is returned
		csd.checkFn = func(chainID eth.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1, Timestamp: 1}, nil
		}
		pair, err := scopedCrossSafeUpdate(logger, chainID, csd)
		require.Equal(t, chainID, updatingChain)
		require.Equal(t, candidateScope, updatingCandidateScope)
		require.Equal(t, candidate, updatingCandidate)
		require.Equal(t, candidateScope, pair.Source)
		require.NoError(t, err)
	})
}

type mockCrossSafeDeps struct {
	deps                  mockDependencySet
	crossSafeFn           func(chainID eth.ChainID) (pair types.DerivedBlockSealPair, err error)
	candidateCrossSafeFn  func() (candidate types.DerivedBlockRefPair, err error)
	openBlockFn           func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
	updateCrossSafeFn     func(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error
	nextSourceFn          func(chain eth.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error)
	previousDerivedFn     func(chain eth.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error)
	checkFn               func(chainID eth.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (types.BlockSeal, error)
	invalidateLocalSafeFn func(chainID eth.ChainID, candidate types.DerivedBlockRefPair) error
}

var _ CrossSafeDeps = (*mockCrossSafeDeps)(nil)

func (m *mockCrossSafeDeps) CrossSafe(chainID eth.ChainID) (pair types.DerivedBlockSealPair, err error) {
	if m.crossSafeFn != nil {
		return m.crossSafeFn(chainID)
	}
	return types.DerivedBlockSealPair{}, nil
}

func (m *mockCrossSafeDeps) CandidateCrossSafe(chain eth.ChainID) (candidate types.DerivedBlockRefPair, err error) {
	if m.candidateCrossSafeFn != nil {
		return m.candidateCrossSafeFn()
	}
	return types.DerivedBlockRefPair{}, nil
}

func (m *mockCrossSafeDeps) DependencySet() depset.DependencySet {
	return m.deps
}

func (m *mockCrossSafeDeps) CrossDerivedToSource(chainID eth.ChainID, derived eth.BlockID) (derivedFrom types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}

func (m *mockCrossSafeDeps) Contains(chainID eth.ChainID, q types.ContainsQuery) (types.BlockSeal, error) {
	if m.checkFn != nil {
		return m.checkFn(chainID, q.BlockNum, q.LogIdx, q.LogHash)
	}
	return types.BlockSeal{}, nil
}

func (m *mockCrossSafeDeps) NextSource(chain eth.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error) {
	if m.nextSourceFn != nil {
		return m.nextSourceFn(chain, derivedFrom)
	}
	return eth.BlockRef{}, nil
}

func (m *mockCrossSafeDeps) PreviousDerived(chain eth.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
	if m.previousDerivedFn != nil {
		return m.previousDerivedFn(chain, derived)
	}
	return types.BlockSeal{}, nil
}

func (m *mockCrossSafeDeps) OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	if m.openBlockFn != nil {
		return m.openBlockFn(chainID, blockNum)
	}
	return eth.BlockRef{}, 0, nil, nil
}

func (m *mockCrossSafeDeps) UpdateCrossSafe(chain eth.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error {
	if m.updateCrossSafeFn != nil {
		return m.updateCrossSafeFn(chain, l1View, lastCrossDerived)
	}
	return nil
}

func (m *mockCrossSafeDeps) InvalidateLocalSafe(chainID eth.ChainID, candidate types.DerivedBlockRefPair) error {
	if m.invalidateLocalSafeFn != nil {
		return m.invalidateLocalSafeFn(chainID, candidate)
	}
	return nil
}
