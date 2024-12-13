package fromda

import (
	"fmt"
	"io/fs"
	"math/rand" // nosemgrep
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type stubMetrics struct {
	DBDerivedEntryCount int64
}

func (s *stubMetrics) RecordDBDerivedEntryCount(count int64) {
	s.DBDerivedEntryCount = count
}

var _ Metrics = (*stubMetrics)(nil)

type setupFn func(t *testing.T, db *DB, m *stubMetrics)

type assertFn func(t *testing.T, db *DB, m *stubMetrics)

func runDBTest(t *testing.T, setup setupFn, assert assertFn) {
	createDb := func(t *testing.T, dir string) (*DB, *stubMetrics, string) {
		logger := testlog.Logger(t, log.LvlTrace)
		path := filepath.Join(dir, "test.db")
		m := &stubMetrics{}
		db, err := NewFromFile(logger, m, path)
		require.NoError(t, err, "Failed to create database")
		t.Cleanup(func() {
			err := db.Close()
			if err != nil {
				require.ErrorIs(t, err, fs.ErrClosed)
			}
		})
		return db, m, path
	}

	t.Run("New", func(t *testing.T) {
		db, m, _ := createDb(t, t.TempDir())
		setup(t, db, m)
		assert(t, db, m)
	})

	t.Run("Existing", func(t *testing.T) {
		dir := t.TempDir()
		db, m, path := createDb(t, dir)
		setup(t, db, m)
		// Close and recreate the database
		require.NoError(t, db.Close())
		checkDBInvariants(t, path, m)

		db2, m, path := createDb(t, dir)
		assert(t, db2, m)
		checkDBInvariants(t, path, m)
	})
}

func TestEmptyDB(t *testing.T) {
	runDBTest(t,
		func(t *testing.T, db *DB, m *stubMetrics) {},
		func(t *testing.T, db *DB, m *stubMetrics) {
			_, _, err := db.Latest()
			require.ErrorIs(t, err, types.ErrFuture)

			_, _, err = db.First()
			require.ErrorIs(t, err, types.ErrFuture)

			_, err = db.LastDerivedAt(eth.BlockID{})
			require.ErrorIs(t, err, types.ErrFuture)

			_, err = db.DerivedFrom(eth.BlockID{})
			require.ErrorIs(t, err, types.ErrFuture)

			_, err = db.PreviousDerived(eth.BlockID{})
			require.ErrorIs(t, err, types.ErrFuture)

			_, _, err = db.NextDerived(eth.BlockID{})
			require.ErrorIs(t, err, types.ErrFuture)

			_, err = db.PreviousDerivedFrom(eth.BlockID{})
			require.ErrorIs(t, err, types.ErrFuture)

			_, err = db.NextDerivedFrom(eth.BlockID{})
			require.ErrorIs(t, err, types.ErrFuture)

			_, _, err = db.FirstAfter(eth.BlockID{}, eth.BlockID{})
			require.ErrorIs(t, err, types.ErrFuture)
		})
}

func mockL1(i uint64) types.BlockSeal {
	return types.BlockSeal{
		Hash:      crypto.Keccak256Hash([]byte(fmt.Sprintf("L1 block %d", i))),
		Number:    i,
		Timestamp: 1000_000 + i*12,
	}
}

func mockL2(i uint64) types.BlockSeal {
	var h common.Hash
	if i != 0 {
		h = crypto.Keccak256Hash([]byte(fmt.Sprintf("L1 block %d", i)))
	}
	return types.BlockSeal{
		Hash:      h,
		Number:    i,
		Timestamp: 1000_000 + i*12,
	}
}

func toRef(seal types.BlockSeal, parentHash common.Hash) eth.BlockRef {
	return eth.BlockRef{
		Hash:       seal.Hash,
		Number:     seal.Number,
		ParentHash: parentHash,
		Time:       seal.Timestamp,
	}
}

func TestSingleEntryDB(t *testing.T) {
	expectedDerivedFrom := mockL1(0)
	expectedDerived := mockL2(2)
	runDBTest(t,
		func(t *testing.T, db *DB, m *stubMetrics) {
			require.NoError(t, db.AddDerived(toRef(expectedDerivedFrom, mockL1(0).Hash), toRef(expectedDerived, mockL2(0).Hash)))
		},
		func(t *testing.T, db *DB, m *stubMetrics) {
			// First
			derivedFrom, derived, err := db.First()
			require.NoError(t, err)
			require.Equal(t, expectedDerivedFrom, derivedFrom)
			require.Equal(t, expectedDerived, derived)

			// Latest
			derivedFrom, derived, err = db.Latest()
			require.NoError(t, err)
			require.Equal(t, expectedDerivedFrom, derivedFrom)
			require.Equal(t, expectedDerived, derived)

			// FirstAfter Latest
			_, _, err = db.FirstAfter(derivedFrom.ID(), derived.ID())
			require.ErrorIs(t, err, types.ErrFuture)

			// LastDerivedAt
			derived, err = db.LastDerivedAt(expectedDerivedFrom.ID())
			require.NoError(t, err)
			require.Equal(t, expectedDerived, derived)

			// LastDerivedAt with a non-existent block
			_, err = db.LastDerivedAt(eth.BlockID{Hash: common.Hash{0xaa}, Number: expectedDerivedFrom.Number})
			require.ErrorIs(t, err, types.ErrConflict)

			// FirstAfter with a non-existent block (derived and derivedFrom)
			_, _, err = db.FirstAfter(eth.BlockID{Hash: common.Hash{0xaa}, Number: expectedDerivedFrom.Number}, expectedDerived.ID())
			require.ErrorIs(t, err, types.ErrConflict)
			_, _, err = db.FirstAfter(expectedDerivedFrom.ID(), eth.BlockID{Hash: common.Hash{0xaa}, Number: expectedDerived.Number})
			require.ErrorIs(t, err, types.ErrConflict)

			// DerivedFrom
			derivedFrom, err = db.DerivedFrom(expectedDerived.ID())
			require.NoError(t, err)
			require.Equal(t, expectedDerivedFrom, derivedFrom)

			// DerivedFrom with a non-existent block
			_, err = db.DerivedFrom(eth.BlockID{Hash: common.Hash{0xbb}, Number: expectedDerived.Number})
			require.ErrorIs(t, err, types.ErrConflict)

			// PreviousDerived
			prev, err := db.PreviousDerived(expectedDerived.ID())
			require.NoError(t, err)
			require.Equal(t, types.BlockSeal{}, prev, "zeroed seal before first entry")

			// PreviousDerivedFrom
			prev, err = db.PreviousDerivedFrom(expectedDerivedFrom.ID())
			require.NoError(t, err)
			require.Equal(t, types.BlockSeal{}, prev, "zeroed seal before first entry")

			// NextDerived
			_, _, err = db.NextDerived(expectedDerived.ID())
			require.ErrorIs(t, err, types.ErrFuture)

			// NextDerivedFrom
			_, err = db.NextDerivedFrom(expectedDerivedFrom.ID())
			require.ErrorIs(t, err, types.ErrFuture)

			// FirstAfter
			_, _, err = db.FirstAfter(expectedDerivedFrom.ID(), expectedDerived.ID())
			require.ErrorIs(t, err, types.ErrFuture)
		})
}

func TestGap(t *testing.T) {
	// mockL1 starts at block 1 to produce a gap
	expectedDerivedFrom := mockL1(1)
	// mockL2 starts at block 2 to produce a gap
	expectedDerived := mockL2(2)
	runDBTest(t,
		func(t *testing.T, db *DB, m *stubMetrics) {
			require.NoError(t, db.AddDerived(toRef(expectedDerivedFrom, mockL1(0).Hash), toRef(expectedDerived, mockL2(0).Hash)))
		},
		func(t *testing.T, db *DB, m *stubMetrics) {
			_, _, err := db.NextDerived(mockL2(0).ID())
			require.ErrorIs(t, err, types.ErrSkipped)

			_, err = db.NextDerivedFrom(mockL1(0).ID())
			require.ErrorIs(t, err, types.ErrSkipped)
		})
}

func TestThreeEntryDB(t *testing.T) {
	l1Block0 := mockL1(0)
	l1Block1 := mockL1(1)
	l1Block2 := mockL1(2)

	l2Block0 := mockL2(0)
	l2Block1 := mockL2(1)
	l2Block2 := mockL2(2)

	runDBTest(t, func(t *testing.T, db *DB, m *stubMetrics) {
		require.NoError(t, db.AddDerived(toRef(l1Block0, common.Hash{}), toRef(l2Block0, common.Hash{})))
		require.NoError(t, db.AddDerived(toRef(l1Block1, l1Block0.Hash), toRef(l2Block1, l2Block0.Hash)))
		require.NoError(t, db.AddDerived(toRef(l1Block2, l1Block1.Hash), toRef(l2Block2, l2Block1.Hash)))
	}, func(t *testing.T, db *DB, m *stubMetrics) {

		derivedFrom, derived, err := db.Latest()
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)
		require.Equal(t, l2Block2, derived)

		derivedFrom, derived, err = db.First()
		require.NoError(t, err)
		require.Equal(t, l1Block0, derivedFrom)
		require.Equal(t, l2Block0, derived)

		derived, err = db.LastDerivedAt(l1Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block2, derived)

		_, err = db.LastDerivedAt(eth.BlockID{Hash: common.Hash{0xaa}, Number: l1Block2.Number})
		require.ErrorIs(t, err, types.ErrConflict)

		derivedFrom, err = db.DerivedFrom(l2Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)

		_, err = db.DerivedFrom(eth.BlockID{Hash: common.Hash{0xbb}, Number: l2Block2.Number})
		require.ErrorIs(t, err, types.ErrConflict)

		derived, err = db.LastDerivedAt(l1Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block1, derived)

		derivedFrom, err = db.DerivedFrom(l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)

		derived, err = db.LastDerivedAt(l1Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block0, derived)

		derivedFrom, err = db.DerivedFrom(l2Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block0, derivedFrom)

		derived, err = db.PreviousDerived(l2Block0.ID())
		require.NoError(t, err)
		require.Equal(t, types.BlockSeal{}, derived)

		derived, err = db.PreviousDerived(l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block0, derived)

		derived, err = db.PreviousDerived(l2Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block1, derived)

		derivedFrom, derived, err = db.NextDerived(l2Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block1, derived)
		require.Equal(t, l1Block1, derivedFrom)

		derivedFrom, derived, err = db.NextDerived(l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block2, derived)
		require.Equal(t, l1Block2, derivedFrom)

		_, _, err = db.NextDerived(l2Block2.ID())
		require.ErrorIs(t, err, types.ErrFuture)

		derivedFrom, err = db.PreviousDerivedFrom(l1Block0.ID())
		require.NoError(t, err)
		require.Equal(t, types.BlockSeal{}, derivedFrom)

		derivedFrom, err = db.PreviousDerivedFrom(l1Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block0, derivedFrom)

		derivedFrom, err = db.PreviousDerivedFrom(l1Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)

		derivedFrom, err = db.NextDerivedFrom(l1Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)

		derivedFrom, err = db.NextDerivedFrom(l1Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)

		_, err = db.NextDerivedFrom(l1Block2.ID())
		require.ErrorIs(t, err, types.ErrFuture)

		_, _, err = db.FirstAfter(l1Block2.ID(), l2Block2.ID())
		require.ErrorIs(t, err, types.ErrFuture)

		derivedFrom, derived, err = db.FirstAfter(l1Block0.ID(), l2Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		require.Equal(t, l2Block1, derived)

		derivedFrom, derived, err = db.FirstAfter(l1Block1.ID(), l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)
		require.Equal(t, l2Block2, derived)
	})
}

// TestFastL2Batcher tests what happens if we have derived multiple L2 blocks from the same L1 block
func TestFastL2Batcher(t *testing.T) {
	l1Block0 := mockL1(0)
	l1Block1 := mockL1(1)
	l1Block2 := mockL1(2)

	l2Block0 := mockL2(0)
	l2Block1 := mockL2(1)
	l2Block2 := mockL2(2)
	l2Block3 := mockL2(3)
	l2Block4 := mockL2(4)
	l2Block5 := mockL2(5)

	runDBTest(t, func(t *testing.T, db *DB, m *stubMetrics) {
		// L2 genesis derived from L1 genesis
		require.NoError(t, db.AddDerived(toRef(l1Block0, common.Hash{}), toRef(l2Block0, common.Hash{})))
		// Many L2 blocks all derived from same L1 block
		l1Ref1 := toRef(l1Block1, l1Block0.Hash)
		require.NoError(t, db.AddDerived(l1Ref1, toRef(l2Block1, l2Block0.Hash)))
		require.NoError(t, db.AddDerived(l1Ref1, toRef(l2Block2, l2Block1.Hash)))
		require.NoError(t, db.AddDerived(l1Ref1, toRef(l2Block3, l2Block2.Hash)))
		require.NoError(t, db.AddDerived(l1Ref1, toRef(l2Block4, l2Block3.Hash)))
		// Latest L2 block derived from later L1 block
		require.NoError(t, db.AddDerived(toRef(l1Block2, l1Block1.Hash), toRef(l2Block5, l2Block4.Hash)))
	}, func(t *testing.T, db *DB, m *stubMetrics) {

		derivedFrom, derived, err := db.Latest()
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)
		require.Equal(t, l2Block5, derived)

		derived, err = db.LastDerivedAt(l1Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block5, derived)

		// test what tip was derived from
		derivedFrom, err = db.DerivedFrom(l2Block5.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)

		// Multiple L2 blocks all derived from same older L1 block
		for _, b := range []types.BlockSeal{l2Block1, l2Block2, l2Block3, l2Block4} {
			derivedFrom, err = db.DerivedFrom(b.ID())
			require.NoError(t, err)
			require.Equal(t, l1Block1, derivedFrom)
		}

		// test that the latest L2 counts, not the intermediate
		derived, err = db.LastDerivedAt(l1Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block4, derived)

		derived, err = db.PreviousDerived(l2Block5.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block4, derived)
		derived, err = db.PreviousDerived(l2Block4.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block3, derived)
		derived, err = db.PreviousDerived(l2Block3.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block2, derived)
		derived, err = db.PreviousDerived(l2Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block1, derived)
		derived, err = db.PreviousDerived(l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block0, derived)

		derivedFrom, derived, err = db.NextDerived(l2Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		require.Equal(t, l2Block1, derived)
		derivedFrom, derived, err = db.NextDerived(l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		require.Equal(t, l2Block2, derived)
		derivedFrom, derived, err = db.NextDerived(l2Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		require.Equal(t, l2Block3, derived)
		derivedFrom, derived, err = db.NextDerived(l2Block3.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		require.Equal(t, l2Block4, derived)
		derivedFrom, derived, err = db.NextDerived(l2Block4.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom) // derived from later L1 block
		require.Equal(t, l2Block5, derived)
		_, _, err = db.NextDerived(l2Block5.ID())
		require.ErrorIs(t, err, types.ErrFuture)

		derivedFrom, err = db.PreviousDerivedFrom(l1Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		derivedFrom, err = db.PreviousDerivedFrom(l1Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block0, derivedFrom)

		derivedFrom, err = db.NextDerivedFrom(l1Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		derivedFrom, err = db.NextDerivedFrom(l1Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)
		_, err = db.NextDerivedFrom(l1Block2.ID())
		require.ErrorIs(t, err, types.ErrFuture)

		derivedFrom, derived, err = db.FirstAfter(l1Block1.ID(), l2Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom) // no increment in L1 yet, the next after is L2 block 3
		require.Equal(t, l2Block3, derived)
	})
}

// TestSlowL2Batcher tests what happens if we have derived no new L2 blocks in a L1 block
func TestSlowL2Batcher(t *testing.T) {
	l1Block0 := mockL1(0)
	l1Block1 := mockL1(1)
	l1Block2 := mockL1(2)
	l1Block3 := mockL1(3)
	l1Block4 := mockL1(4)
	l1Block5 := mockL1(5)

	l2Block0 := mockL2(0)
	l2Block1 := mockL2(1)
	l2Block2 := mockL2(2)

	runDBTest(t, func(t *testing.T, db *DB, m *stubMetrics) {
		// L2 genesis derived from L1 genesis
		require.NoError(t, db.AddDerived(toRef(l1Block0, common.Hash{}), toRef(l2Block0, common.Hash{})))
		// Many L1 blocks all repeating the same L2 block
		l2Ref1 := toRef(l2Block1, l2Block0.Hash)
		require.NoError(t, db.AddDerived(toRef(l1Block1, l1Block0.Hash), l2Ref1))
		require.NoError(t, db.AddDerived(toRef(l1Block2, l1Block1.Hash), l2Ref1))
		require.NoError(t, db.AddDerived(toRef(l1Block3, l1Block2.Hash), l2Ref1))
		require.NoError(t, db.AddDerived(toRef(l1Block4, l1Block3.Hash), l2Ref1))
		// New L1 block that finally produces a new L2 block
		require.NoError(t, db.AddDerived(toRef(l1Block5, l1Block4.Hash), toRef(l2Block2, l2Block1.Hash)))
	}, func(t *testing.T, db *DB, m *stubMetrics) {

		derivedFrom, derived, err := db.Latest()
		require.NoError(t, err)
		require.Equal(t, l1Block5, derivedFrom)
		require.Equal(t, l2Block2, derived)

		// test what we last derived at the tip
		derived, err = db.LastDerivedAt(l1Block5.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block2, derived)

		// Multiple L1 blocks all copying the last known derived L2 block
		for _, b := range []types.BlockSeal{l1Block1, l1Block2, l1Block3, l1Block4} {
			derived, err = db.LastDerivedAt(b.ID())
			require.NoError(t, err)
			require.Equal(t, l2Block1, derived)
		}

		// test that the first L1 counts, not the ones that repeat the L2 info
		derivedFrom, err = db.DerivedFrom(l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)

		derived, err = db.PreviousDerived(l2Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block1, derived)
		derived, err = db.PreviousDerived(l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l2Block0, derived)

		derivedFrom, derived, err = db.NextDerived(l2Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		require.Equal(t, l2Block1, derived)
		derivedFrom, derived, err = db.NextDerived(l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block5, derivedFrom)
		require.Equal(t, l2Block2, derived)
		_, _, err = db.NextDerived(l2Block2.ID())
		require.ErrorIs(t, err, types.ErrFuture)

		derivedFrom, err = db.PreviousDerivedFrom(l1Block5.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block4, derivedFrom)
		derivedFrom, err = db.PreviousDerivedFrom(l1Block4.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block3, derivedFrom)
		derivedFrom, err = db.PreviousDerivedFrom(l1Block3.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)
		derivedFrom, err = db.PreviousDerivedFrom(l1Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		derivedFrom, err = db.PreviousDerivedFrom(l1Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block0, derivedFrom)

		derivedFrom, err = db.NextDerivedFrom(l1Block0.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		derivedFrom, err = db.NextDerivedFrom(l1Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block2, derivedFrom)
		derivedFrom, err = db.NextDerivedFrom(l1Block2.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block3, derivedFrom)
		derivedFrom, err = db.NextDerivedFrom(l1Block4.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block5, derivedFrom)
		_, err = db.NextDerivedFrom(l1Block5.ID())
		require.ErrorIs(t, err, types.ErrFuture)

		derivedFrom, derived, err = db.FirstAfter(l1Block2.ID(), l2Block1.ID())
		require.NoError(t, err)
		require.Equal(t, l1Block3, derivedFrom)
		require.Equal(t, l2Block1, derived) // no increment in L2 yet, the next after is L1 block 3
	})
}

func TestManyEntryDB(t *testing.T) {
	t.Run("at genesis", func(t *testing.T) {
		testManyEntryDB(t, 0, 0)
	})
	t.Run("start at arbitrary L1 block", func(t *testing.T) {
		testManyEntryDB(t, 30000, 0)
	})
	t.Run("start at arbitrary L2 block", func(t *testing.T) {
		testManyEntryDB(t, 0, 30000)
	})
	t.Run("start at arbitrary L1 and L2 block", func(t *testing.T) {
		testManyEntryDB(t, 2000000, 1000000)
	})
}

func testManyEntryDB(t *testing.T, offsetL1 uint64, offsetL2 uint64) {
	// L2 -> first L1 occurrence
	firstDerivedFrom := make(map[eth.BlockID]types.BlockSeal)
	// L1 -> last L2 occurrence
	lastDerived := make(map[eth.BlockID]types.BlockSeal)

	runDBTest(t, func(t *testing.T, db *DB, m *stubMetrics) {
		// Insert genesis
		require.NoError(t, db.AddDerived(toRef(mockL1(offsetL1), common.Hash{}), toRef(mockL2(offsetL2), common.Hash{})))
		firstDerivedFrom[mockL2(offsetL2).ID()] = mockL1(offsetL1)
		lastDerived[mockL1(offsetL1).ID()] = mockL2(offsetL2)

		rng := rand.New(rand.NewSource(1234))
		// Insert 1000 randomly generated entries, derived at random bumps in L1
		for i := uint64(0); i < 1000; i++ {
			derivedFrom, derived, err := db.Latest()
			require.NoError(t, err)

			switch rng.Intn(3) {
			case 0: // bump L1
				derivedFrom = mockL1(derivedFrom.Number + 1)
			case 1: // bump L2
				derived = mockL2(derived.Number + 1)
			case 2: // bump both
				derivedFrom = mockL1(derivedFrom.Number + 1)
				derived = mockL2(derived.Number + 1)
			}
			derivedFromRef := toRef(derivedFrom, mockL1(derivedFrom.Number-1).Hash)
			derivedRef := toRef(derived, mockL2(derived.Number-1).Hash)
			lastDerived[derivedFromRef.ID()] = derived
			if _, ok := firstDerivedFrom[derivedRef.ID()]; !ok {
				firstDerivedFrom[derivedRef.ID()] = derivedFrom
			}
			require.NoError(t, db.AddDerived(derivedFromRef, derivedRef))
		}
	}, func(t *testing.T, db *DB, m *stubMetrics) {
		// Now assert we can find what they are all derived from, and match the expectations.
		derivedFrom, derived, err := db.Latest()
		require.NoError(t, err)
		require.NotZero(t, derivedFrom.Number-offsetL1)
		require.NotZero(t, derived.Number-offsetL2)

		for i := offsetL1; i <= derivedFrom.Number; i++ {
			l1ID := mockL1(i).ID()
			derived, err := db.LastDerivedAt(l1ID)
			require.NoError(t, err)
			require.Contains(t, lastDerived, l1ID)
			require.Equal(t, lastDerived[l1ID], derived)
		}

		for i := offsetL2; i <= derived.Number; i++ {
			l2ID := mockL2(i).ID()
			derivedFrom, err := db.DerivedFrom(l2ID)
			require.NoError(t, err)
			require.Contains(t, firstDerivedFrom, l2ID)
			require.Equal(t, firstDerivedFrom[l2ID], derivedFrom)
		}

		// if not started at genesis, try to read older data, assert it's unavailable.
		if offsetL1 > 0 {
			_, err := db.LastDerivedAt(mockL1(0).ID())
			require.ErrorIs(t, err, types.ErrSkipped)

			_, err = db.LastDerivedAt(mockL1(offsetL1 - 1).ID())
			require.ErrorIs(t, err, types.ErrSkipped)
		}
		if offsetL2 > 0 {
			_, err := db.DerivedFrom(mockL2(0).ID())
			require.ErrorIs(t, err, types.ErrSkipped)

			_, err = db.DerivedFrom(mockL2(offsetL2 - 1).ID())
			require.ErrorIs(t, err, types.ErrSkipped)
		}
	})
}

// TestRewind tests what happens if we rewind
func TestRewind(t *testing.T) {
	l1Block0 := mockL1(0)
	l1Block1 := mockL1(1)
	l1Block2 := mockL1(2)
	l1Block3 := mockL1(3)
	l1Block4 := mockL1(4)
	l1Block5 := mockL1(5)

	l2Block0 := mockL2(0)
	l2Block1 := mockL2(1)
	l2Block2 := mockL2(2)

	runDBTest(t, func(t *testing.T, db *DB, m *stubMetrics) {
		// L2 genesis derived from L1 genesis
		require.NoError(t, db.AddDerived(toRef(l1Block0, common.Hash{}), toRef(l2Block0, common.Hash{})))
		// Many L1 blocks all repeating the same L2 block
		l2Ref1 := toRef(l2Block1, l2Block0.Hash)
		require.NoError(t, db.AddDerived(toRef(l1Block1, l1Block0.Hash), l2Ref1))
		require.NoError(t, db.AddDerived(toRef(l1Block2, l1Block1.Hash), l2Ref1))
		require.NoError(t, db.AddDerived(toRef(l1Block3, l1Block2.Hash), l2Ref1))
		require.NoError(t, db.AddDerived(toRef(l1Block4, l1Block3.Hash), l2Ref1))
		// New L1 block that finally produces a new L2 block
		require.NoError(t, db.AddDerived(toRef(l1Block5, l1Block4.Hash), toRef(l2Block2, l2Block1.Hash)))
	}, func(t *testing.T, db *DB, m *stubMetrics) {

		derivedFrom, derived, err := db.Latest()
		require.NoError(t, err)
		require.Equal(t, l1Block5, derivedFrom)
		require.Equal(t, l2Block2, derived)

		// Rewind to the future
		require.ErrorIs(t, db.Rewind(6), types.ErrFuture)

		// Rewind to the exact block we're at
		require.NoError(t, db.Rewind(l1Block5.Number))
		derivedFrom, derived, err = db.Latest()
		require.NoError(t, err)
		require.Equal(t, l1Block5, derivedFrom)
		require.Equal(t, l2Block2, derived)

		// Now rewind to L1 block 3 (inclusive).
		require.NoError(t, db.Rewind(l1Block3.Number))

		// See if we find consistent data
		derivedFrom, derived, err = db.Latest()
		require.NoError(t, err)
		require.Equal(t, l1Block3, derivedFrom)
		require.Equal(t, l2Block1, derived)

		// Rewind further to L1 block 1 (inclusive).
		require.NoError(t, db.Rewind(l1Block1.Number))
		derivedFrom, derived, err = db.Latest()
		require.NoError(t, err)
		require.Equal(t, l1Block1, derivedFrom)
		require.Equal(t, l2Block1, derived)

		// Rewind further to L1 block 0 (inclusive).
		require.NoError(t, db.Rewind(l1Block0.Number))
		derivedFrom, derived, err = db.Latest()
		require.NoError(t, err)
		require.Equal(t, l1Block0, derivedFrom)
		require.Equal(t, l2Block0, derived)
	})
}
