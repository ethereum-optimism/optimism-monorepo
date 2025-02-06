package db

import (
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockDerivationStorage struct {
	lastFn      func() (pair types.DerivedBlockSealPair, err error)
	sourceNumFn func(sourceNum uint64) (source types.BlockSeal, err error)
}

func (m *mockDerivationStorage) First() (pair types.DerivedBlockSealPair, err error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) Last() (pair types.DerivedBlockSealPair, err error) {
	if m.lastFn != nil {
		return m.lastFn()
	}
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) Invalidated() (pair types.DerivedBlockSealPair, err error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) AddDerived(derivedFrom eth.BlockRef, derived eth.BlockRef) error {
	return nil
}
func (m *mockDerivationStorage) ReplaceInvalidatedBlock(replacementDerived eth.BlockRef, invalidated common.Hash) (types.DerivedBlockSealPair, error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) RewindAndInvalidate(invalidated types.DerivedBlockRefPair) error {
	return nil
}
func (m *mockDerivationStorage) SourceToLastDerived(source eth.BlockID) (derived types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) ContainsDerived(derived eth.BlockID) error {
	return nil
}
func (m *mockDerivationStorage) DerivedToFirstSource(derived eth.BlockID) (source types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) Next(pair types.DerivedIDPair) (next types.DerivedBlockSealPair, err error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) NextSource(source eth.BlockID) (nextSource types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) NextDerived(derived eth.BlockID) (next types.DerivedBlockSealPair, err error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) PreviousSource(source eth.BlockID) (prevSource types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) PreviousDerived(derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) RewindToScope(scope eth.BlockID) error {
	return nil
}
func (m *mockDerivationStorage) RewindToFirstDerived(derived eth.BlockID) error {
	return nil
}
func (m *mockDerivationStorage) SourceNumToSource(sourceNum uint64) (source types.BlockSeal, err error) {
	if m.sourceNumFn != nil {
		return m.sourceNumFn(sourceNum)
	}
	return types.BlockSeal{}, nil
}

func sampleDepSet(t *testing.T) depset.DependencySet {
	depSet, err := depset.NewStaticConfigDependencySet(
		map[eth.ChainID]*depset.StaticConfigDependency{
			eth.ChainIDFromUInt64(900): {
				ChainIndex:     900,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
			eth.ChainIDFromUInt64(901): {
				ChainIndex:     901,
				ActivationTime: 30,
				HistoryMinTime: 20,
			},
			eth.ChainIDFromUInt64(902): {
				ChainIndex:     902,
				ActivationTime: 30,
				HistoryMinTime: 20,
			},
		})
	require.NoError(t, err)
	return depSet
}

func TestCommonL1UnknownChain(t *testing.T) {
	m1 := &mockDerivationStorage{}
	m2 := &mockDerivationStorage{}
	logger := testlog.Logger(t, log.LevelDebug)
	chainDB := NewChainsDB(logger, sampleDepSet(t), metrics.NoopMetrics)

	// add a mock local derived-from storage to drive the test
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(900), m1)
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(901), m2)
	// don't attach a mock for chain 902

	_, err := chainDB.LastCommonL1(false)
	require.ErrorIs(t, err, types.ErrUnknownChain)
}

func TestCommonL1(t *testing.T) {
	m1 := &mockDerivationStorage{}
	m2 := &mockDerivationStorage{}
	m3 := &mockDerivationStorage{}
	logger := testlog.Logger(t, log.LevelDebug)
	chainDB := NewChainsDB(logger, sampleDepSet(t), metrics.NoopMetrics)

	// add a mock local derived-from storage to drive the test
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(900), m1)
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(901), m2)
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(902), m3)

	// lastN is a helper function which creates a Latest Function for the test
	lastN := func(n uint64) func() (pair types.DerivedBlockSealPair, err error) {
		return func() (pair types.DerivedBlockSealPair, err error) {
			return types.DerivedBlockSealPair{
				Source: types.BlockSeal{
					Number: n,
					Hash:   common.Hash{1},
				},
			}, nil
		}
	}
	// sourceN is a helper function which creates a SourceNumToSource Function for the test
	sourceN := func(n uint64, h common.Hash) func(sourceNum uint64) (source types.BlockSeal, err error) {
		return func(sourceNum uint64) (source types.BlockSeal, err error) {
			require.Equal(t, n, sourceNum)
			return types.BlockSeal{
				Number: n,
				Hash:   h,
			}, nil
		}
	}
	t.Run("pattern 1", func(t *testing.T) {
		m1.lastFn = lastN(1)
		m2.lastFn = lastN(2)
		m3.lastFn = lastN(3)
		m1.sourceNumFn = sourceN(1, common.Hash{1})
		m2.sourceNumFn = sourceN(1, common.Hash{1})
		m3.sourceNumFn = sourceN(1, common.Hash{1})

		last, err := chainDB.LastCommonL1(false)
		require.NoError(t, err)
		require.Equal(t, uint64(1), last.Number)
	})
	t.Run("pattern 1 parent", func(t *testing.T) {
		m1.lastFn = lastN(1)
		m2.lastFn = lastN(2)
		m3.lastFn = lastN(3)
		m1.sourceNumFn = sourceN(0, common.Hash{1})
		m2.sourceNumFn = sourceN(0, common.Hash{1})
		m3.sourceNumFn = sourceN(0, common.Hash{1})

		last, err := chainDB.LastCommonL1(true)
		require.NoError(t, err)
		require.Equal(t, uint64(0), last.Number)
	})
	t.Run("pattern 2", func(t *testing.T) {
		m1.lastFn = lastN(3)
		m2.lastFn = lastN(2)
		m3.lastFn = lastN(1)
		m1.sourceNumFn = sourceN(1, common.Hash{1})
		m2.sourceNumFn = sourceN(1, common.Hash{1})
		m3.sourceNumFn = sourceN(1, common.Hash{1})

		last, err := chainDB.LastCommonL1(false)
		require.NoError(t, err)
		require.Equal(t, uint64(1), last.Number)
	})
	t.Run("pattern 2 parent", func(t *testing.T) {
		m1.lastFn = lastN(3)
		m2.lastFn = lastN(2)
		m3.lastFn = lastN(1)
		m1.sourceNumFn = sourceN(0, common.Hash{1})
		m2.sourceNumFn = sourceN(0, common.Hash{1})
		m3.sourceNumFn = sourceN(0, common.Hash{1})

		last, err := chainDB.LastCommonL1(true)
		require.NoError(t, err)
		require.Equal(t, uint64(0), last.Number)
	})
	t.Run("pattern 3", func(t *testing.T) {
		m1.lastFn = lastN(99)
		m2.lastFn = lastN(1)
		m3.lastFn = lastN(98)
		m1.sourceNumFn = sourceN(1, common.Hash{1})
		m2.sourceNumFn = sourceN(1, common.Hash{1})
		m3.sourceNumFn = sourceN(1, common.Hash{1})

		last, err := chainDB.LastCommonL1(false)
		require.NoError(t, err)
		require.Equal(t, uint64(1), last.Number)
	})
	t.Run("pattern 3 parent", func(t *testing.T) {
		m1.lastFn = lastN(99)
		m2.lastFn = lastN(1)
		m3.lastFn = lastN(98)
		m1.sourceNumFn = sourceN(0, common.Hash{1})
		m2.sourceNumFn = sourceN(0, common.Hash{1})
		m3.sourceNumFn = sourceN(0, common.Hash{1})

		last, err := chainDB.LastCommonL1(true)
		require.NoError(t, err)
		require.Equal(t, uint64(0), last.Number)
	})
	t.Run("error", func(t *testing.T) {
		m1.lastFn = lastN(99)
		m2.lastFn = lastN(1)
		m3.lastFn = func() (pair types.DerivedBlockSealPair, err error) {
			return types.DerivedBlockSealPair{}, fmt.Errorf("error")
		}
		last, err := chainDB.LastCommonL1(false)
		require.Error(t, err)
		require.Equal(t, types.BlockSeal{}, last)
	})
	t.Run("inconsistent", func(t *testing.T) {
		m1.lastFn = lastN(99)
		m2.lastFn = lastN(1)
		m3.lastFn = lastN(2)
		m1.sourceNumFn = sourceN(1, common.Hash{1})
		m2.sourceNumFn = sourceN(1, common.Hash{2}) // m2 has a different hash
		m3.sourceNumFn = sourceN(1, common.Hash{1})
		last, err := chainDB.LastCommonL1(false)
		require.Error(t, err)
		require.Equal(t, types.BlockSeal{}, last)
	})
	t.Run("inconsistent parent", func(t *testing.T) {
		m1.lastFn = lastN(99)
		m2.lastFn = lastN(1)
		m3.lastFn = lastN(2)
		m1.sourceNumFn = sourceN(0, common.Hash{1})
		m2.sourceNumFn = sourceN(0, common.Hash{2}) // m2 has a different hash
		m3.sourceNumFn = sourceN(0, common.Hash{1})
		last, err := chainDB.LastCommonL1(true)
		require.Error(t, err)
		require.Equal(t, types.BlockSeal{}, last)
	})
}
