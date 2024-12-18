package syncnode

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockChainsDB struct {
	updateLocalSafeFn func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error
}

func (m *mockChainsDB) UpdateLocalSafe(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
	if m.updateLocalSafeFn != nil {
		return m.updateLocalSafeFn(chainID, ref, derived)
	}
	return nil
}

type mockSyncControl struct {
	TryDeriveNextFn func(ctx context.Context, ref eth.BlockRef) (eth.BlockRef, error)
}

func (m *mockSyncControl) TryDeriveNext(ctx context.Context, ref eth.BlockRef) (eth.BlockRef, error) {
	if m.TryDeriveNextFn != nil {
		return m.TryDeriveNextFn(ctx, ref)
	}
	return eth.BlockRef{}, nil
}
func sampleDepSet(t *testing.T) depset.DependencySet {
	depSet, err := depset.NewStaticConfigDependencySet(
		map[types.ChainID]*depset.StaticConfigDependency{
			types.ChainIDFromUInt64(900): {
				ChainIndex:     900,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
			types.ChainIDFromUInt64(901): {
				ChainIndex:     901,
				ActivationTime: 30,
				HistoryMinTime: 20,
			},
		})
	require.NoError(t, err)
	return depSet
}

// TestAttachNodeController tests the AttachNodeController function of the SyncNodesController.
// Only controllers for chains in the dependency set can be attached.
func TestAttachNodeController(t *testing.T) {
	logger := log.New()
	depSet := sampleDepSet(t)
	controller := NewSyncNodesController(logger, depSet, nil)

	require.Zero(t, controller.controllers.Len(), "controllers should be empty to start")

	// Attach a controller for chain 900
	ctrl := mockSyncControl{}
	err := controller.AttachNodeController(types.ChainIDFromUInt64(900), &ctrl)
	require.NoError(t, err)

	require.Equal(t, 1, controller.controllers.Len(), "controllers should have 1 entry")

	// Attach a controller for chain 901
	ctrl2 := mockSyncControl{}
	err = controller.AttachNodeController(types.ChainIDFromUInt64(901), &ctrl2)
	require.NoError(t, err)

	require.Equal(t, 2, controller.controllers.Len(), "controllers should have 2 entries")

	// Attach a controller for chain 902 (which is not in the dependency set)
	ctrl3 := mockSyncControl{}
	err = controller.AttachNodeController(types.ChainIDFromUInt64(902), &ctrl3)
	require.Error(t, err)
	require.Equal(t, 2, controller.controllers.Len(), "controllers should still have 2 entries")
}

// TestDeriveFromL1 tests the DeriveFromL1 function of the SyncNodesController for multiple chains
func TestDeriveFromL1(t *testing.T) {
	logger := log.New()
	depSet := sampleDepSet(t)

	// keep track of the updates for each chain with the mock
	updates := map[types.ChainID][]eth.BlockRef{}
	mockChainsDB := mockChainsDB{}
	updateMu := sync.Mutex{}
	mockChainsDB.updateLocalSafeFn = func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
		updateMu.Lock()
		defer updateMu.Unlock()
		updates[chainID] = append(updates[chainID], derived)
		return nil
	}
	controller := NewSyncNodesController(logger, depSet, &mockChainsDB)

	refA := eth.BlockRef{Number: 1}
	refB := eth.BlockRef{Number: 2}
	refC := eth.BlockRef{Number: 3}
	derived := []eth.BlockRef{refA, refB, refC}

	// Attach a controller for chain 900 with a mock controller function
	ctrl1 := mockSyncControl{}
	ctrl1i := 0
	// the controller will return the next derived block each time TryDeriveNext is called
	ctrl1.TryDeriveNextFn = func(ctx context.Context, ref eth.BlockRef) (eth.BlockRef, error) {
		defer func() { ctrl1i++ }()
		if ctrl1i >= len(derived) {
			return eth.BlockRef{}, nil
		}
		return derived[ctrl1i], nil
	}
	err := controller.AttachNodeController(types.ChainIDFromUInt64(900), &ctrl1)
	require.NoError(t, err)

	// Attach a controller for chain 900 with a mock controller function
	ctrl2 := mockSyncControl{}
	ctrl2i := 0
	// the controller will return the next derived block each time TryDeriveNext is called
	ctrl2.TryDeriveNextFn = func(ctx context.Context, ref eth.BlockRef) (eth.BlockRef, error) {
		defer func() { ctrl2i++ }()
		if ctrl2i >= len(derived) {
			return eth.BlockRef{}, nil
		}
		return derived[ctrl2i], nil
	}
	err = controller.AttachNodeController(types.ChainIDFromUInt64(901), &ctrl2)
	require.NoError(t, err)

	// Derive from L1
	err = controller.DeriveFromL1(refA)
	require.NoError(t, err)

	// Check that the derived blocks were recorded for each chain
	require.Equal(t, []eth.BlockRef{refA, refB, refC}, updates[types.ChainIDFromUInt64(900)])
	require.Equal(t, []eth.BlockRef{refA, refB, refC}, updates[types.ChainIDFromUInt64(901)])

}

// TestDeriveFromL1Error tests that if a chain fails to derive from L1, the derived blocks up to the error are still recorded
// for that chain, and all other chains that derived successfully are also recorded.
func TestDeriveFromL1Error(t *testing.T) {
	logger := log.New()
	depSet := sampleDepSet(t)

	// keep track of the updates for each chain with the mock
	updates := map[types.ChainID][]eth.BlockRef{}
	mockChainsDB := mockChainsDB{}
	updateMu := sync.Mutex{}
	mockChainsDB.updateLocalSafeFn = func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
		updateMu.Lock()
		defer updateMu.Unlock()
		updates[chainID] = append(updates[chainID], derived)
		return nil
	}
	controller := NewSyncNodesController(logger, depSet, &mockChainsDB)

	refA := eth.BlockRef{Number: 1}
	refB := eth.BlockRef{Number: 2}
	refC := eth.BlockRef{Number: 3}
	derived := []eth.BlockRef{refA, refB, refC}

	// Attach a controller for chain 900 with a mock controller function
	ctrl1 := mockSyncControl{}
	ctrl1i := 0
	// the controller will return the next derived block each time TryDeriveNext is called
	ctrl1.TryDeriveNextFn = func(ctx context.Context, ref eth.BlockRef) (eth.BlockRef, error) {
		defer func() { ctrl1i++ }()
		if ctrl1i >= len(derived) {
			return eth.BlockRef{}, nil
		}
		return derived[ctrl1i], nil
	}
	err := controller.AttachNodeController(types.ChainIDFromUInt64(900), &ctrl1)
	require.NoError(t, err)

	// Attach a controller for chain 900 with a mock controller function
	ctrl2 := mockSyncControl{}
	ctrl2i := 0
	// this controller will error on the last derived block
	ctrl2.TryDeriveNextFn = func(ctx context.Context, ref eth.BlockRef) (eth.BlockRef, error) {
		defer func() { ctrl2i++ }()
		if ctrl2i >= len(derived)-1 {
			return eth.BlockRef{}, fmt.Errorf("error")
		}
		return derived[ctrl2i], nil
	}
	err = controller.AttachNodeController(types.ChainIDFromUInt64(901), &ctrl2)
	require.NoError(t, err)

	// Derive from L1
	err = controller.DeriveFromL1(refA)
	require.Error(t, err)

	// Check that the derived blocks were recorded for each chain
	// and in the case of the error, the derived blocks up to the error are recorded
	require.Equal(t, []eth.BlockRef{refA, refB, refC}, updates[types.ChainIDFromUInt64(900)])
	require.Equal(t, []eth.BlockRef{refA, refB}, updates[types.ChainIDFromUInt64(901)])

}
