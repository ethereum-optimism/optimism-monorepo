package syncnode

import (
	"context"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum"
	gethevent "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockChainsDB struct {
	localSafeFn       func(chainID types.ChainID) (types.BlockSeal, types.BlockSeal, error)
	updateLocalSafeFn func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error
	updateCrossSafeFn func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error
}

func (m *mockChainsDB) UpdateLocalSafe(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
	if m.updateLocalSafeFn != nil {
		return m.updateLocalSafeFn(chainID, ref, derived)
	}
	return nil
}

func (m *mockChainsDB) LocalSafe(chainID types.ChainID) (types.BlockSeal, types.BlockSeal, error) {
	if m.localSafeFn != nil {
		return m.localSafeFn(chainID)
	}
	return types.BlockSeal{}, types.BlockSeal{}, nil
}

func (m *mockChainsDB) UpdateCrossSafe(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
	if m.updateCrossSafeFn != nil {
		return m.updateCrossSafeFn(chainID, ref, derived)
	}
	return nil
}

func (m *mockChainsDB) SubscribeCrossSafe(chainID types.ChainID, c chan<- types.DerivedPair) (gethevent.Subscription, error) {
	return nil, nil
}

func (m *mockChainsDB) SubscribeFinalized(chainID types.ChainID, c chan<- eth.BlockID) (gethevent.Subscription, error) {
	return nil, nil
}

type mockSyncControl struct {
	anchorPointFn       func(ctx context.Context) (types.DerivedPair, error)
	provideL1Fn         func(ctx context.Context, ref eth.BlockRef) error
	resetFn             func(ctx context.Context, unsafe, safe, finalized eth.BlockID) error
	updateCrossSafeFn   func(ctx context.Context, derived, derivedFrom eth.BlockID) error
	updateCrossUnsafeFn func(ctx context.Context, derived eth.BlockID) error
	updateFinalizedFn   func(ctx context.Context, id eth.BlockID) error

	subscribeDerivationUpdates gethevent.FeedOf[types.DerivedPair]
	subscribeExhaustL1Events   gethevent.FeedOf[types.DerivedPair]
	subscribeUnsafeBlocks      gethevent.FeedOf[eth.L1BlockRef]
	subscribeResetEvents       gethevent.FeedOf[string]
}

func (m *mockSyncControl) AnchorPoint(ctx context.Context) (types.DerivedPair, error) {
	if m.anchorPointFn != nil {
		return m.anchorPointFn(ctx)
	}
	return types.DerivedPair{}, nil
}

func (m *mockSyncControl) ProvideL1(ctx context.Context, ref eth.BlockRef) error {
	if m.provideL1Fn != nil {
		return m.provideL1Fn(ctx, ref)
	}
	return nil
}

func (m *mockSyncControl) Reset(ctx context.Context, unsafe, safe, finalized eth.BlockID) error {
	if m.resetFn != nil {
		return m.resetFn(ctx, unsafe, safe, finalized)
	}
	return nil
}

func (m *mockSyncControl) SubscribeDerivationUpdates(ctx context.Context, c chan types.DerivedPair) (ethereum.Subscription, error) {
	return m.subscribeDerivationUpdates.Subscribe(c), nil
}

func (m *mockSyncControl) SubscribeExhaustL1Events(ctx context.Context, c chan types.DerivedPair) (ethereum.Subscription, error) {
	return m.subscribeExhaustL1Events.Subscribe(c), nil
}

func (m *mockSyncControl) SubscribeUnsafeBlocks(ctx context.Context, c chan eth.L1BlockRef) (ethereum.Subscription, error) {
	return m.subscribeUnsafeBlocks.Subscribe(c), nil
}

func (m *mockSyncControl) SubscribeResetEvents(ctx context.Context, c chan string) (ethereum.Subscription, error) {
	return m.subscribeResetEvents.Subscribe(c), nil
}

func (m *mockSyncControl) UpdateCrossSafe(ctx context.Context, derived eth.BlockID, derivedFrom eth.BlockID) error {
	if m.updateCrossSafeFn != nil {
		return m.updateCrossSafeFn(ctx, derived, derivedFrom)
	}
	return nil
}

func (m *mockSyncControl) UpdateCrossUnsafe(ctx context.Context, derived eth.BlockID) error {
	if m.updateCrossUnsafeFn != nil {
		return m.updateCrossUnsafeFn(ctx, derived)
	}
	return nil
}

func (m *mockSyncControl) UpdateFinalized(ctx context.Context, id eth.BlockID) error {
	if m.updateFinalizedFn != nil {
		return m.updateFinalizedFn(ctx, id)
	}
	return nil
}

type mockBackend struct {
}

func (m *mockBackend) LocalSafe(ctx context.Context, chainID types.ChainID) (derivedFrom eth.BlockID, derived eth.BlockID, err error) {
	return eth.BlockID{}, eth.BlockID{}, nil
}

func (m *mockBackend) LatestUnsafe(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	return eth.BlockID{}, nil
}

func (m *mockBackend) SafeDerivedAt(ctx context.Context, chainID types.ChainID, derivedFrom eth.BlockID) (derived eth.BlockID, err error) {
	return eth.BlockID{}, nil
}

func (m *mockBackend) Finalized(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	return eth.BlockID{}, nil
}

func (m *mockBackend) UpdateLocalSafe(ctx context.Context, chainID types.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) error {
	return nil
}

func (m *mockBackend) UpdateLocalUnsafe(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error {
	return nil
}

func (m *mockBackend) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	return eth.L1BlockRef{}, nil
}

var _ backend = (*mockBackend)(nil)

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

// TestInitFromAnchorPoint tests that the SyncNodesController uses the Anchor Point to initialize databases
func TestInitFromAnchorPoint(t *testing.T) {
	logger := log.New()
	depSet := sampleDepSet(t)
	controller := NewSyncNodesController(logger, depSet, &mockChainsDB{}, &mockBackend{})

	require.Zero(t, controller.controllers.Len(), "controllers should be empty to start")

	// Attach a controller for chain 900
	// make the controller return an anchor point
	ctrl := mockSyncControl{}
	ctrl.anchorPointFn = func(ctx context.Context) (types.DerivedPair, error) {
		return types.DerivedPair{
			Derived:     eth.BlockRef{Number: 1},
			DerivedFrom: eth.BlockRef{Number: 0},
		}, nil
	}

	// have the local safe return an error to trigger the initialization
	controller.db.(*mockChainsDB).localSafeFn = func(chainID types.ChainID) (types.BlockSeal, types.BlockSeal, error) {
		return types.BlockSeal{}, types.BlockSeal{}, types.ErrFuture
	}
	// record when the updateLocalSafe function is called
	localCalled := 0
	controller.db.(*mockChainsDB).updateLocalSafeFn = func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
		localCalled++
		return nil
	}
	// record when the updateCrossSafe function is called
	crossCalled := 0
	controller.db.(*mockChainsDB).updateCrossSafeFn = func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
		crossCalled++
		return nil
	}

	// after the first attach, both databases are called for update
	err := controller.AttachNodeController(types.ChainIDFromUInt64(900), &ctrl)
	require.NoError(t, err)
	require.Equal(t, 1, localCalled, "local safe should have been updated once")
	require.Equal(t, 1, crossCalled, "cross safe should have been updated twice")

	// reset the local safe function to return no error
	controller.db.(*mockChainsDB).localSafeFn = nil

	// after the second attach, there are no additional updates (no empty signal from the DB)
	ctrl2 := mockSyncControl{}
	err = controller.AttachNodeController(types.ChainIDFromUInt64(901), &ctrl2)
	require.NoError(t, err)
	require.Equal(t, 1, localCalled, "local safe should have been updated once")
	require.Equal(t, 1, crossCalled, "cross safe should have been updated twice")
}

// TestAttachNodeController tests the AttachNodeController function of the SyncNodesController.
// Only controllers for chains in the dependency set can be attached.
func TestAttachNodeController(t *testing.T) {
	logger := log.New()
	depSet := sampleDepSet(t)
	controller := NewSyncNodesController(logger, depSet, &mockChainsDB{}, &mockBackend{})

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
