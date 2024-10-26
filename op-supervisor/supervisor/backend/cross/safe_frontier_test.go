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

func TestHazardSafeFrontierChecks(t *testing.T) {
	t.Run("empty hazards", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		l1DerivedFrom := eth.BlockID{}
		hazards := map[types.ChainIndex]types.BlockSeal{}
		// when there are no hazards,
		// no work is done, and no error is returned
		err := HazardSafeFrontierChecks(sfcd, l1DerivedFrom, hazards)
		require.NoError(t, err)
	})
	t.Run("unknown chain", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{
			deps: mockDependencySet{
				chainIDFromIndexfn: func() (types.ChainID, error) {
					return types.ChainID{}, types.ErrUnknownChain
				},
			},
		}
		l1DerivedFrom := eth.BlockID{}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {}}
		// when there is one hazard, and ChainIDFromIndex returns ErrUnknownChain,
		// an error is returned as a ErrConflict
		err := HazardSafeFrontierChecks(sfcd, l1DerivedFrom, hazards)
		require.ErrorIs(t, err, types.ErrConflict)
	})
	t.Run("initDerivedFrom in scope", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossDerivedFromFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1}, nil
		}
		l1DerivedFrom := eth.BlockID{Number: 2}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {}}
		// when there is one hazard, and CrossDerivedFrom returns a BlockSeal within scope
		// (ie the hazard's block number is less than or equal to the derivedFrom block number),
		// no error is returned
		err := HazardSafeFrontierChecks(sfcd, l1DerivedFrom, hazards)
		require.NoError(t, err)
	})
	t.Run("initDerivedFrom out of scope", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossDerivedFromFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{Number: 3}, nil
		}
		l1DerivedFrom := eth.BlockID{Number: 2}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {}}
		// when there is one hazard, and CrossDerivedFrom returns a BlockSeal out of scope
		// (ie the hazard's block number is greater than the derivedFrom block number),
		// an error is returned as a ErrOutOfScope
		err := HazardSafeFrontierChecks(sfcd, l1DerivedFrom, hazards)
		require.ErrorIs(t, err, types.ErrOutOfScope)
	})
	t.Run("errFuture: candidate cross safe failure", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossDerivedFromFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{Number: 3}, types.ErrFuture
		}
		sfcd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return eth.BlockRef{},
				eth.BlockRef{Number: 3, Hash: common.BytesToHash([]byte{0x01})},
				errors.New("some error")
		}
		l1DerivedFrom := eth.BlockID{}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {}}
		// when there is one hazard, and CrossDerivedFrom returns an ErrFuture,
		// and CandidateCrossSafe returns an error,
		// the error from CandidateCrossSafe is returned
		err := HazardSafeFrontierChecks(sfcd, l1DerivedFrom, hazards)
		require.ErrorContains(t, err, "some error")
	})
	t.Run("errFuture: expected block does not match candidate", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossDerivedFromFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{}, types.ErrFuture
		}
		sfcd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return eth.BlockRef{},
				eth.BlockRef{Number: 3, Hash: common.BytesToHash([]byte{0x01})},
				nil
		}
		l1DerivedFrom := eth.BlockID{}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {Number: 3, Hash: common.BytesToHash([]byte{0x02})}}
		// when there is one hazard, and CrossDerivedFrom returns an ErrFuture,
		// and CandidateCrossSafe returns a candidate that does not match the hazard,
		// (ie the candidate's block number is the same as the hazard's block number, but the hashes are different),
		// an error is returned as a ErrConflict
		err := HazardSafeFrontierChecks(sfcd, l1DerivedFrom, hazards)
		require.ErrorIs(t, err, types.ErrConflict)
	})
	t.Run("errFuture: local-safe hazard out of scope", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossDerivedFromFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{}, types.ErrFuture
		}
		sfcd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return eth.BlockRef{Number: 9},
				eth.BlockRef{},
				nil
		}
		l1DerivedFrom := eth.BlockID{Number: 8}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {Number: 3, Hash: common.BytesToHash([]byte{0x02})}}
		// when there is one hazard, and CrossDerivedFrom returns an ErrFuture,
		// and the initDerivedFrom is out of scope,
		// an error is returned as a ErrOutOfScope
		err := HazardSafeFrontierChecks(sfcd, l1DerivedFrom, hazards)
		require.ErrorIs(t, err, types.ErrOutOfScope)
	})
	t.Run("CrossDerivedFrom Error", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossDerivedFromFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{}, errors.New("some error")
		}
		sfcd.candidateCrossSafeFn = func() (derivedFromScope, crossSafe eth.BlockRef, err error) {
			return eth.BlockRef{Number: 9},
				eth.BlockRef{},
				nil
		}
		l1DerivedFrom := eth.BlockID{Number: 8}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {Number: 3, Hash: common.BytesToHash([]byte{0x02})}}
		// when there is one hazard, and CrossDerivedFrom returns an ErrFuture,
		// and the initDerivedFrom is out of scope,
		// an error is returned as a ErrOutOfScope
		err := HazardSafeFrontierChecks(sfcd, l1DerivedFrom, hazards)
		require.ErrorContains(t, err, "some error")
	})
}

type mockSafeFrontierCheckDeps struct {
	deps                 mockDependencySet
	candidateCrossSafeFn func() (derivedFromScope, crossSafe eth.BlockRef, err error)
	crossDerivedFromFn   func() (derivedFrom types.BlockSeal, err error)
}

func (m *mockSafeFrontierCheckDeps) CandidateCrossSafe(chain types.ChainID) (derivedFromScope, crossSafe eth.BlockRef, err error) {
	if m.candidateCrossSafeFn != nil {
		return m.candidateCrossSafeFn()
	}
	return eth.BlockRef{}, eth.BlockRef{}, nil
}

func (m *mockSafeFrontierCheckDeps) CrossDerivedFrom(chainID types.ChainID, derived eth.BlockID) (derivedFrom types.BlockSeal, err error) {
	if m.crossDerivedFromFn != nil {
		return m.crossDerivedFromFn()
	}
	return types.BlockSeal{}, nil
}

func (m *mockSafeFrontierCheckDeps) DependencySet() depset.DependencySet {
	return m.deps
}

type mockDependencySet struct {
	chainIDFromIndexfn func() (types.ChainID, error)
	canExecuteAtfn     func() (bool, error)
	canInitiateAtfn    func() (bool, error)
}

func (m mockDependencySet) CanExecuteAt(chain types.ChainID, timestamp uint64) (bool, error) {
	if m.canExecuteAtfn != nil {
		return m.canExecuteAtfn()
	}
	return true, nil
}

func (m mockDependencySet) CanInitiateAt(chain types.ChainID, timestamp uint64) (bool, error) {
	if m.canInitiateAtfn != nil {
		return m.canInitiateAtfn()
	}
	return true, nil
}

func (m mockDependencySet) ChainIDFromIndex(index types.ChainIndex) (types.ChainID, error) {
	if m.chainIDFromIndexfn != nil {
		return m.chainIDFromIndexfn()
	}
	return types.ChainID{}, nil
}

func (m mockDependencySet) ChainIndexFromID(chain types.ChainID) (types.ChainIndex, error) {
	return types.ChainIndex(0), nil
}

func (m mockDependencySet) Chains() []types.ChainID {
	return nil
}

func (m mockDependencySet) HasChain(chain types.ChainID) bool {
	return true
}
