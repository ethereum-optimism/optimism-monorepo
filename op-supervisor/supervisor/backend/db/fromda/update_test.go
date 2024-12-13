package fromda

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type testCase struct {
	name     string
	setupFn  setupFn
	assertFn assertFn
}

func TestBadUpdates(t *testing.T) {
	aDerivedFrom := mockL1(1)
	aDerived := mockL2(201)
	bDerivedFrom := mockL1(2)
	bDerived := mockL2(202)
	cDerivedFrom := mockL1(3)
	cDerived := mockL2(203)
	dDerivedFrom := mockL1(4)
	dDerived := mockL2(204)
	eDerivedFrom := mockL1(5)
	eDerived := mockL2(205)
	fDerivedFrom := mockL1(6)
	fDerived := mockL2(206)

	noChange := assertFn(func(t *testing.T, db *DB, m *stubMetrics) {
		derivedFrom, derived, err := db.Latest()
		require.NoError(t, err)
		require.Equal(t, dDerivedFrom, derivedFrom)
		require.Equal(t, dDerived, derived)
	})

	testCases := []testCase{
		{
			name: "add on old derivedFrom",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(bDerivedFrom, aDerivedFrom.Hash), toRef(dDerived, cDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "repeat parent derivedFrom",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(cDerivedFrom, bDerivedFrom.Hash), toRef(dDerived, cDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "add on conflicting derivedFrom, same height. And new derived value",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(types.BlockSeal{
					Hash:      common.Hash{0xba, 0xd},
					Number:    dDerivedFrom.Number,
					Timestamp: dDerivedFrom.Timestamp,
				}, cDerivedFrom.Hash), toRef(eDerived, dDerived.Hash)), types.ErrConflict)
			},
			assertFn: noChange,
		},
		{
			name: "CrossDerivedFrom with conflicting parent root, same L1 height, new L2: accepted, L1 parent-hash is used only on L1 increments.",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.AddDerived(toRef(dDerivedFrom, common.Hash{0x42}), toRef(eDerived, dDerived.Hash)), types.ErrConflict)
			},
			assertFn: func(t *testing.T, db *DB, m *stubMetrics) {
				derivedFrom, derived, err := db.Latest()
				require.NoError(t, err)
				require.Equal(t, dDerivedFrom, derivedFrom)
				require.Equal(t, eDerived, derived)
			},
		},
		{
			name: "Conflicting derivedFrom parent root, new L1 height, same L2",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(eDerivedFrom, common.Hash{0x42}), toRef(dDerived, cDerived.Hash)), types.ErrConflict)
			},
			assertFn: noChange,
		},
		{
			name: "add on too new derivedFrom (even if parent-hash looks correct)",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(fDerivedFrom, dDerivedFrom.Hash), toRef(eDerived, dDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "add on old derivedFrom (even if parent-hash looks correct)",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(cDerivedFrom, bDerivedFrom.Hash), toRef(cDerived, dDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "add on even older derivedFrom",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(bDerivedFrom, aDerivedFrom.Hash), toRef(dDerived, cDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "add on conflicting derived, same L2 height, new L1 block",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(eDerivedFrom, dDerivedFrom.Hash), toRef(types.BlockSeal{
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
				require.NoError(t, db.AddDerived(toRef(eDerivedFrom, dDerivedFrom.Hash), toRef(dDerived, common.Hash{0x42})), types.ErrConflict)
			},
			assertFn: func(t *testing.T, db *DB, m *stubMetrics) {
				derivedFrom, derived, err := db.Latest()
				require.NoError(t, err)
				require.Equal(t, eDerivedFrom, derivedFrom)
				require.Equal(t, dDerived, derived)
			},
		},
		{
			name: "add derived with conflicting parent hash, same L1 height, new L2 height",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(dDerivedFrom, cDerivedFrom.Hash), toRef(eDerived, common.Hash{0x42})), types.ErrConflict)
			},
			assertFn: noChange,
		},
		{
			name: "add on too new derived (even if parent-hash looks correct)",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(dDerivedFrom, cDerivedFrom.Hash), toRef(fDerived, dDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "add on old derived (even if parent-hash looks correct)",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(dDerivedFrom, cDerivedFrom.Hash), toRef(cDerived, bDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "add on even older derived",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.AddDerived(toRef(dDerivedFrom, cDerivedFrom.Hash), toRef(bDerived, aDerived.Hash)), types.ErrOutOfOrder)
			},
			assertFn: noChange,
		},
		{
			name: "repeat self, silent no-op",
			setupFn: func(t *testing.T, db *DB, m *stubMetrics) {
				pre := m.DBDerivedEntryCount
				require.NoError(t, db.AddDerived(toRef(dDerivedFrom, cDerivedFrom.Hash), toRef(dDerived, cDerived.Hash)), types.ErrOutOfOrder)
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
					require.NoError(t, db.AddDerived(toRef(dDerivedFrom, cDerivedFrom.Hash), toRef(dDerived, cDerived.Hash)))
					// apply the test-case setup
					tc.setupFn(t, db, m)
				},
				tc.assertFn)
		})
	}
}
