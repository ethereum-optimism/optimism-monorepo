package fromda

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type testCase struct {
	name     string
	setupFn  setupFn
	assertFn assertFn
}

func TestBadUpdates(t *testing.T) {
	aSource := mockL1(1)
	aDerived := mockL2(201)
	bSource := mockL1(2)
	bDerived := mockL2(202)
	cSource := mockL1(3)
	cDerived := mockL2(203)
	dSource := mockL1(4)
	dAltSource := mockL1(4)
	dAltSource.Hash = crypto.Keccak256Hash([]byte("bad alternative block d"))
	dDerived := mockL2(204)
	eSource := mockL1(5)
	eDerived := mockL2(205)
	fSource := mockL1(6)
	fDerived := mockL2(206)

	noChange := assertFn(func(t *testing.T, db *DB, m *stubMetrics) {
		pair, err := db.Last()
		require.NoError(t, err)
		require.Equal(t, dSource, pair.Source)
		require.Equal(t, dDerived, pair.Derived)
	})

	testCases := []testCase{
		{
			name: "add on old source before DB start",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(bSource, aSource.Hash), // b is before c
					toRef(dDerived, cDerived.Hash)), types.ErrSkipped)
			},
			assertFn: noChange,
		},
		{
			name: "repeat second latest",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(cSource, bSource.Hash),
					toRef(cDerived, bDerived.Hash),
				), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "repeat latest",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.AddDerived(
					toRef(dSource, cSource.Hash),
					toRef(dDerived, cDerived.Hash),
				))
			},
			assertFn: noChange,
		},
		{
			name: "repeat latest derived, old source",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.AddDerived(
					toRef(eSource, dSource.Hash),   // new L1 block
					toRef(dDerived, cDerived.Hash), // same L2 block
				))
				require.ErrorIs(t, db.AddDerived(
					toRef(dSource, cSource.Hash),   // d is old, but was canonically linked like this before
					toRef(dDerived, cDerived.Hash), // same L2 block
				), types.ErrIneffective)
			},
			assertFn: func(t *testing.T, db *DB, m *stubMetrics) {
				pair, err := db.Last()
				require.NoError(t, err)
				require.Equal(t, eSource, pair.Source)
				require.Equal(t, dDerived, pair.Derived)
			},
		},
		{
			name: "repeat latest derived, conflicting old source",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.AddDerived(
					toRef(eSource, dSource.Hash),   // new L1 block
					toRef(dDerived, cDerived.Hash), // same L2 block
				))
				require.ErrorIs(t, db.AddDerived(
					toRef(dAltSource, cSource.Hash), // conflicting old block
					toRef(dDerived, cDerived.Hash),  // same L2 block
				), types.ErrConflict)
			},
			assertFn: func(t *testing.T, db *DB, m *stubMetrics) {
				pair, err := db.Last()
				require.NoError(t, err)
				require.Equal(t, eSource, pair.Source)
				require.Equal(t, dDerived, pair.Derived)
			},
		},
		{
			name: "new derived, old source",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.AddDerived(
					toRef(eSource, dSource.Hash),   // new L1 block
					toRef(dDerived, cDerived.Hash), // same L2 block
				))
				require.ErrorIs(t, db.AddDerived(
					toRef(dSource, cSource.Hash),   // old L1 block
					toRef(eDerived, dDerived.Hash), // new L2 block
				), types.ErrOutOfOrder)
			},
			assertFn: func(t *testing.T, db *DB, m *stubMetrics) {
				pair, err := db.Last()
				require.NoError(t, err)
				require.Equal(t, eSource, pair.Source)
				require.Equal(t, dDerived, pair.Derived)
			},
		},
		{
			name: "add on conflicting source, same height. And new derived value",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(types.BlockSeal{
						Hash:      common.Hash{0xba, 0xd},
						Number:    dSource.Number,
						Timestamp: dSource.Timestamp,
					}, cSource.Hash),
					toRef(eDerived, dDerived.Hash)), types.ErrConflict)
			},
			assertFn: noChange,
		},
		{
			name: "CrossSource with conflicting parent root, same L1 height, new L2: accepted, L1 parent-hash is used only on L1 increments.",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.AddDerived(
					toRef(dSource, common.Hash{0x42}),
					toRef(eDerived, dDerived.Hash)), types.ErrConflict)
			},
			assertFn: func(t *testing.T, db *DB, m *stubMetrics) {
				pair, err := db.Last()
				require.NoError(t, err)
				require.Equal(t, dSource, pair.Source)
				require.Equal(t, eDerived, pair.Derived)
			},
		},
		{
			name: "Conflicting source parent root, new L1 height, same L2",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t,
					db.AddDerived(
						toRef(eSource, common.Hash{0x42}),
						toRef(dDerived, cDerived.Hash)), types.ErrConflict)
			},
			assertFn: noChange,
		},
		{
			name: "add on too new source (even if parent-hash looks correct)",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t,
					db.AddDerived(toRef(fSource, dSource.Hash),
						toRef(eDerived, dDerived.Hash)), types.ErrFuture)
			},
			assertFn: noChange,
		},
		{
			name: "add on old source (even if parent-hash looks correct)",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(cSource, bSource.Hash),
					toRef(cDerived, dDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "add on even older source",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(bSource, aSource.Hash),
					toRef(dDerived, cDerived.Hash)), types.ErrSkipped)
			},
			assertFn: noChange,
		},
		{
			name: "add on conflicting derived, same L2 height, new L1 block",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(eSource, dSource.Hash),
					toRef(types.BlockSeal{
						Hash:      common.Hash{0x42},
						Number:    dDerived.Number,
						Timestamp: dDerived.Timestamp,
					}, cDerived.Hash)), types.ErrConflict)
			},
			assertFn: noChange,
		},
		{
			name: "add derived with conflicting parent hash, new L1 height, same L2 height: accepted, L2 parent-hash is only checked on L2 increments.",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.AddDerived(
					toRef(eSource, dSource.Hash),
					toRef(dDerived, common.Hash{0x42})), types.ErrConflict)
			},
			assertFn: func(t *testing.T, db *DB, m *stubMetrics) {
				pair, err := db.Last()
				require.NoError(t, err)
				require.Equal(t, eSource, pair.Source)
				require.Equal(t, dDerived, pair.Derived)
			},
		},
		{
			name: "add derived with conflicting parent hash, same L1 height, new L2 height",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(dSource, cSource.Hash),
					toRef(eDerived, common.Hash{0x42})), types.ErrConflict)
			},
			assertFn: noChange,
		},
		{
			name: "add on too new derived (even if parent-hash looks correct)",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(dSource, cSource.Hash),
					toRef(fDerived, dDerived.Hash)), types.ErrFuture)
			},
			assertFn: noChange,
		},
		{
			name: "add on old derived (even if parent-hash looks correct)",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(dSource, cSource.Hash),
					toRef(cDerived, bDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "add on even older derived",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(
					toRef(dSource, cSource.Hash),
					toRef(bDerived, aDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "repeat self, silent no-op",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				pre := m.DBDerivedEntryCount
				require.NoError(t, db.AddDerived(
					toRef(dSource, cSource.Hash),
					toRef(dDerived, cDerived.Hash)), types.ErrOutOfOrder)
				require.Equal(t, pre, m.DBDerivedEntryCount)
			},
			assertFn: noChange,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runDBTest(t,
				func(t *testing.T, db *DB, m *stubMetrics) {
					// Good first entry
					require.NoError(t, db.AddDerived(
						toRef(dSource, cSource.Hash),
						toRef(dDerived, cDerived.Hash)))
					// apply the test-case setup
					tc.setupFn(t, db, m)
				},
				tc.assertFn)
		})
	}
}
