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
		l1Source := eth.BlockID{}
		hazards := map[types.ChainIndex]types.BlockSeal{}
		// when there are no hazards,
		// no work is done, and no error is returned
		err := HazardSafeFrontierChecks(sfcd, l1Source, hazards)
		require.NoError(t, err)
	})
	t.Run("unknown chain", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{
			deps: mockDependencySet{
				chainIDFromIndexfn: func() (eth.ChainID, error) {
					return eth.ChainID{}, types.ErrUnknownChain
				},
			},
		}
		l1Source := eth.BlockID{}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {}}
		// when there is one hazard, and ChainIDFromIndex returns ErrUnknownChain,
		// an error is returned as a ErrConflict
		err := HazardSafeFrontierChecks(sfcd, l1Source, hazards)
		require.ErrorIs(t, err, types.ErrConflict)
	})
	t.Run("initSource in scope", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossSourceFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{Number: 1}, nil
		}
		l1Source := eth.BlockID{Number: 2}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {}}
		// when there is one hazard, and CrossSource returns a BlockSeal within scope
		// (ie the hazard's block number is less than or equal to the source block number),
		// no error is returned
		err := HazardSafeFrontierChecks(sfcd, l1Source, hazards)
		require.NoError(t, err)
	})
	t.Run("initSource out of scope", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossSourceFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{Number: 3}, nil
		}
		l1Source := eth.BlockID{Number: 2}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {}}
		// when there is one hazard, and CrossSource returns a BlockSeal out of scope
		// (ie the hazard's block number is greater than the source block number),
		// an error is returned as a ErrOutOfScope
		err := HazardSafeFrontierChecks(sfcd, l1Source, hazards)
		require.ErrorIs(t, err, types.ErrOutOfScope)
	})
	t.Run("errFuture: candidate cross safe failure", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossSourceFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{Number: 3}, types.ErrFuture
		}
		sfcd.candidateCrossSafeFn = func() (candidate types.DerivedBlockRefPair, err error) {
			return types.DerivedBlockRefPair{
					Source:  eth.BlockRef{},
					Derived: eth.BlockRef{Number: 3, Hash: common.BytesToHash([]byte{0x01})}},
				errors.New("some error")
		}
		l1Source := eth.BlockID{}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {}}
		// when there is one hazard, and CrossSource returns an ErrFuture,
		// and CandidateCrossSafe returns an error,
		// the error from CandidateCrossSafe is returned
		err := HazardSafeFrontierChecks(sfcd, l1Source, hazards)
		require.ErrorContains(t, err, "some error")
	})
	t.Run("errFuture: expected block does not match candidate", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossSourceFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{}, types.ErrFuture
		}
		sfcd.candidateCrossSafeFn = func() (candidate types.DerivedBlockRefPair, err error) {
			return types.DerivedBlockRefPair{
				Source:  eth.BlockRef{},
				Derived: eth.BlockRef{Number: 3, Hash: common.BytesToHash([]byte{0x01})},
			}, nil
		}
		l1Source := eth.BlockID{}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {Number: 3, Hash: common.BytesToHash([]byte{0x02})}}
		// when there is one hazard, and CrossSource returns an ErrFuture,
		// and CandidateCrossSafe returns a candidate that does not match the hazard,
		// (ie the candidate's block number is the same as the hazard's block number, but the hashes are different),
		// an error is returned as a ErrConflict
		err := HazardSafeFrontierChecks(sfcd, l1Source, hazards)
		require.ErrorIs(t, err, types.ErrConflict)
	})
	t.Run("errFuture: local-safe hazard out of scope", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossSourceFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{}, types.ErrFuture
		}
		sfcd.candidateCrossSafeFn = func() (candidate types.DerivedBlockRefPair, err error) {
			return types.DerivedBlockRefPair{
					Source:  eth.BlockRef{Number: 9},
					Derived: eth.BlockRef{}},
				nil
		}
		l1Source := eth.BlockID{Number: 8}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {Number: 3, Hash: common.BytesToHash([]byte{0x02})}}
		// when there is one hazard, and CrossSource returns an ErrFuture,
		// and the initSource is out of scope,
		// an error is returned as a ErrOutOfScope
		err := HazardSafeFrontierChecks(sfcd, l1Source, hazards)
		require.ErrorIs(t, err, types.ErrOutOfScope)
	})
	t.Run("CrossSource Error", func(t *testing.T) {
		sfcd := &mockSafeFrontierCheckDeps{}
		sfcd.crossSourceFn = func() (types.BlockSeal, error) {
			return types.BlockSeal{}, errors.New("some error")
		}
		sfcd.candidateCrossSafeFn = func() (candidate types.DerivedBlockRefPair, err error) {
			return types.DerivedBlockRefPair{
				Source:  eth.BlockRef{Number: 9},
				Derived: eth.BlockRef{},
			}, nil
		}
		l1Source := eth.BlockID{Number: 8}
		hazards := map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): {Number: 3, Hash: common.BytesToHash([]byte{0x02})}}
		// when there is one hazard, and CrossSource returns an ErrFuture,
		// and the initSource is out of scope,
		// an error is returned as a ErrOutOfScope
		err := HazardSafeFrontierChecks(sfcd, l1Source, hazards)
		require.ErrorContains(t, err, "some error")
	})
}

type mockSafeFrontierCheckDeps struct {
	deps                 mockDependencySet
	candidateCrossSafeFn func() (candidate types.DerivedBlockRefPair, err error)
	crossSourceFn        func() (source types.BlockSeal, err error)
}

func (m *mockSafeFrontierCheckDeps) CandidateCrossSafe(chain eth.ChainID) (candidate types.DerivedBlockRefPair, err error) {
	if m.candidateCrossSafeFn != nil {
		return m.candidateCrossSafeFn()
	}
	return types.DerivedBlockRefPair{}, nil
}

func (m *mockSafeFrontierCheckDeps) CrossDerivedToSource(chainID eth.ChainID, derived eth.BlockID) (source types.BlockSeal, err error) {
	if m.crossSourceFn != nil {
		return m.crossSourceFn()
	}
	return types.BlockSeal{}, nil
}

func (m *mockSafeFrontierCheckDeps) DependencySet() depset.DependencySet {
	return m.deps
}

type mockDependencySet struct {
	chainIDFromIndexfn func() (eth.ChainID, error)
	canExecuteAtfn     func() (bool, error)
	canInitiateAtfn    func() (bool, error)
}

func (m mockDependencySet) CanExecuteAt(chain eth.ChainID, timestamp uint64) (bool, error) {
	if m.canExecuteAtfn != nil {
		return m.canExecuteAtfn()
	}
	return true, nil
}

func (m mockDependencySet) CanInitiateAt(chain eth.ChainID, timestamp uint64) (bool, error) {
	if m.canInitiateAtfn != nil {
		return m.canInitiateAtfn()
	}
	return true, nil
}

func (m mockDependencySet) ChainIDFromIndex(index types.ChainIndex) (eth.ChainID, error) {
	if m.chainIDFromIndexfn != nil {
		return m.chainIDFromIndexfn()
	}
	id := eth.ChainIDFromUInt64(uint64(index) - 1000)
	return id, nil
}

func (m mockDependencySet) ChainIndexFromID(chain eth.ChainID) (types.ChainIndex, error) {
	v, err := chain.ToUInt32()
	if err != nil {
		return 0, err
	}
	// offset, so we catch improper manual conversion that doesn't apply this offset
	return types.ChainIndex(v + 1000), nil
}

func (m mockDependencySet) Chains() []eth.ChainID {
	return nil
}

func (m mockDependencySet) HasChain(chain eth.ChainID) bool {
	return true
}
