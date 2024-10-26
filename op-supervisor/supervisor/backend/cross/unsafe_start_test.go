package cross

import (
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestCrossUnsafeHazards(t *testing.T) {
	t.Run("empty execMsgs", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{}
		execMsgs := []*types.ExecutingMessage{}
		// when there are no execMsgs,
		// no work is done, and no error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.NoError(t, err)
		require.Empty(t, hazards)
	})
	t.Run("CanExecuteAt returns false", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			canExecuteAtfn: func() (bool, error) {
				return false, nil
			},
		}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{}
		execMsgs := []*types.ExecutingMessage{{}}
		// when there is one execMsg, and CanExecuteAt returns false,
		// no work is done and an error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorIs(t, err, types.ErrConflict)
		require.Empty(t, hazards)
	})
	t.Run("CanExecuteAt returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			canExecuteAtfn: func() (bool, error) {
				return false, errors.New("some error")
			},
		}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{}
		execMsgs := []*types.ExecutingMessage{{}}
		// when there is one execMsg, and CanExecuteAt returns false,
		// no work is done and an error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("unknown chain", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			chainIDFromIndexfn: func() (types.ChainID, error) {
				return types.ChainID{}, types.ErrUnknownChain
			},
		}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{}
		execMsgs := []*types.ExecutingMessage{{}}
		// when there is one execMsg, and ChainIDFromIndex returns ErrUnknownChain,
		// an error is returned as a ErrConflict
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorIs(t, err, types.ErrConflict)
		require.Empty(t, hazards)
	})
	t.Run("ChainIDFromUInt64 returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			chainIDFromIndexfn: func() (types.ChainID, error) {
				return types.ChainID{}, errors.New("some error")
			},
		}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{}
		execMsgs := []*types.ExecutingMessage{{}}
		// when there is one execMsg, and ChainIDFromIndex returns some other error,
		// the error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("CanInitiateAt returns false", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			canInitiateAtfn: func() (bool, error) {
				return false, nil
			},
		}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{}
		execMsgs := []*types.ExecutingMessage{{}}
		// when there is one execMsg, and CanInitiateAt returns false,
		// the error is returned as a ErrConflict
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorIs(t, err, types.ErrConflict)
		require.Empty(t, hazards)
	})
	t.Run("CanInitiateAt returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			canInitiateAtfn: func() (bool, error) {
				return false, errors.New("some error")
			},
		}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{}
		execMsgs := []*types.ExecutingMessage{{}}
		// when there is one execMsg, and CanInitiateAt returns an error,
		// the error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is greater than candidate", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 10}
		execMsgs := []*types.ExecutingMessage{em1}
		// when there is one execMsg, and the timestamp is greater than the candidate,
		// an error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorContains(t, err, "breaks timestamp invariant")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is equal, Check returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return types.BlockSeal{}, errors.New("some error")
		}
		usd.deps = mockDependencySet{}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		execMsgs := []*types.ExecutingMessage{em1}
		// when there is one execMsg, and the timetamp is equal to the candidate,
		// and check returns an error,
		// that error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is equal, same hazard twice", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		sampleBlockSeal := types.BlockSeal{Number: 3, Hash: common.BytesToHash([]byte{0x02})}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return sampleBlockSeal, nil
		}
		usd.deps = mockDependencySet{}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		em2 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		execMsgs := []*types.ExecutingMessage{em1, em2}
		// when there are two execMsgs, and both are equal time to the candidate,
		// and check returns the same includedIn for both
		// they load the hazards once, and return no error
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.NoError(t, err)
		require.Equal(t, hazards, map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): sampleBlockSeal})
	})
	t.Run("timestamp is equal, different hazards", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		// set the check function to return a different BlockSeal for the second call
		sampleBlockSeal := types.BlockSeal{Number: 3, Hash: common.BytesToHash([]byte{0x02})}
		sampleBlockSeal2 := types.BlockSeal{Number: 333, Hash: common.BytesToHash([]byte{0x22})}
		calls := 0
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			defer func() { calls++ }()
			if calls == 0 {
				return sampleBlockSeal, nil
			}
			return sampleBlockSeal2, nil
		}
		usd.deps = mockDependencySet{}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		em2 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		execMsgs := []*types.ExecutingMessage{em1, em2}
		// when there are two execMsgs, and both are equal time to the candidate,
		// and check returns different includedIn for the two,
		// an error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorContains(t, err, "but already depend on")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is less, check returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return types.BlockSeal{}, errors.New("some error")
		}
		usd.deps = mockDependencySet{}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1}
		execMsgs := []*types.ExecutingMessage{em1}
		// when there is one execMsg, and the timestamp is less than the candidate,
		// and check returns an error,
		// that error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is less, IsCrossUnsafe returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		sampleBlockSeal := types.BlockSeal{Number: 3, Hash: common.BytesToHash([]byte{0x02})}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return sampleBlockSeal, nil
		}
		usd.isCrossUnsafeFn = func() error {
			return errors.New("some error")
		}
		usd.deps = mockDependencySet{}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1}
		execMsgs := []*types.ExecutingMessage{em1}
		// when there is one execMsg, and the timestamp is less than the candidate,
		// and IsCrossUnsafe returns an error,
		// that error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is less, IsCrossUnsafe", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		sampleBlockSeal := types.BlockSeal{Number: 3, Hash: common.BytesToHash([]byte{0x02})}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return sampleBlockSeal, nil
		}
		usd.isCrossUnsafeFn = func() error {
			return nil
		}
		usd.deps = mockDependencySet{}
		chainID := types.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1}
		execMsgs := []*types.ExecutingMessage{em1}
		// when there is one execMsg, and the timestamp is less than the candidate,
		// and IsCrossUnsafe returns no error,
		// no error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate, execMsgs)
		require.NoError(t, err)
		require.Empty(t, hazards)
	})
}

type mockUnsafeStartDeps struct {
	deps            mockDependencySet
	checkFn         func() (includedIn types.BlockSeal, err error)
	isCrossUnsafeFn func() error
}

func (m *mockUnsafeStartDeps) Check(chain types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (includedIn types.BlockSeal, err error) {
	if m.checkFn != nil {
		return m.checkFn()
	}
	return types.BlockSeal{}, nil
}

func (m *mockUnsafeStartDeps) IsCrossUnsafe(chainID types.ChainID, derived eth.BlockID) error {
	if m.isCrossUnsafeFn != nil {
		return m.isCrossUnsafeFn()
	}
	return nil
}

func (m *mockUnsafeStartDeps) DependencySet() depset.DependencySet {
	return m.deps
}
