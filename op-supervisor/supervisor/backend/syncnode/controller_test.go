package syncnode

import (
	"context"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum"
	gethevent "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockChainsDB struct {
	localSafeFn       func(chainID types.ChainID) (types.DerivedBlockSealPair, error)
	updateLocalSafeFn func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error
	updateCrossSafeFn func(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error
	openBlockFn       func(chainID types.ChainID, i uint64) (seal eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)

	subscribeCrossUnsafe gethevent.FeedOf[types.BlockSeal]
	subscribeCrosSafe    gethevent.FeedOf[types.DerivedBlockSealPair]
	subscribeFinalized   gethevent.FeedOf[types.BlockSeal]
}

func (m *mockChainsDB) OpenBlock(chainID types.ChainID, i uint64) (seal eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	if m.openBlockFn != nil {
		return m.openBlockFn(chainID, i)
	}
	return eth.BlockRef{}, 0, nil, nil
}

func (m *mockChainsDB) UpdateLocalSafe(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
	if m.updateLocalSafeFn != nil {
		return m.updateLocalSafeFn(chainID, ref, derived)
	}
	return nil
}

func (m *mockChainsDB) LocalSafe(chainID types.ChainID) (types.DerivedBlockSealPair, error) {
	if m.localSafeFn != nil {
		return m.localSafeFn(chainID)
	}
	return types.DerivedBlockSealPair{}, nil
}

func (m *mockChainsDB) UpdateCrossSafe(chainID types.ChainID, ref eth.BlockRef, derived eth.BlockRef) error {
	if m.updateCrossSafeFn != nil {
		return m.updateCrossSafeFn(chainID, ref, derived)
	}
	return nil
}

func (m *mockChainsDB) SubscribeCrossUnsafe(chainID types.ChainID, c chan<- types.BlockSeal) (gethevent.Subscription, error) {
	return m.subscribeCrossUnsafe.Subscribe(c), nil
}

func (m *mockChainsDB) SubscribeCrossSafe(chainID types.ChainID, c chan<- types.DerivedBlockSealPair) (gethevent.Subscription, error) {
	return m.subscribeCrosSafe.Subscribe(c), nil
}

func (m *mockChainsDB) SubscribeFinalized(chainID types.ChainID, c chan<- types.BlockSeal) (gethevent.Subscription, error) {
	return m.subscribeFinalized.Subscribe(c), nil
}

var _ chainsDB = (*mockChainsDB)(nil)

type mockSyncControl struct {
	anchorPointFn       func(ctx context.Context) (types.DerivedBlockRefPair, error)
	provideL1Fn         func(ctx context.Context, ref eth.BlockRef) error
	resetFn             func(ctx context.Context, unsafe, safe, finalized eth.BlockID) error
	updateCrossSafeFn   func(ctx context.Context, derived, derivedFrom eth.BlockID) error
	updateCrossUnsafeFn func(ctx context.Context, derived eth.BlockID) error
	updateFinalizedFn   func(ctx context.Context, id eth.BlockID) error
	pullEventFn         func(ctx context.Context) (*types.ManagedEvent, error)

	subscribeEvents gethevent.FeedOf[*types.ManagedEvent]
}

func (m *mockSyncControl) AnchorPoint(ctx context.Context) (types.DerivedBlockRefPair, error) {
	if m.anchorPointFn != nil {
		return m.anchorPointFn(ctx)
	}
	return types.DerivedBlockRefPair{}, nil
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

func (m *mockSyncControl) PullEvent(ctx context.Context) (*types.ManagedEvent, error) {
	if m.pullEventFn != nil {
		return m.pullEventFn(ctx)
	}
	return nil, nil
}

func (m *mockSyncControl) SubscribeEvents(ctx context.Context, ch chan *types.ManagedEvent) (ethereum.Subscription, error) {
	return m.subscribeEvents.Subscribe(ch), nil
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

var _ SyncControl = (*mockSyncControl)(nil)

type mockBackend struct {
	updateLocalUnsafeFn func(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error
	updateLocalSafeFn   func(ctx context.Context, chainID types.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) error
}

func (m *mockBackend) LocalSafe(ctx context.Context, chainID types.ChainID) (pair types.DerivedIDPair, err error) {
	return types.DerivedIDPair{}, nil
}

func (m *mockBackend) LocalUnsafe(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	return eth.BlockID{}, nil
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
	if m.updateLocalSafeFn != nil {
		return m.updateLocalSafeFn(ctx, chainID, derivedFrom, lastDerived)
	}
	return nil
}

func (m *mockBackend) UpdateLocalUnsafe(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error {
	if m.updateLocalUnsafeFn != nil {
		return m.updateLocalUnsafeFn(ctx, chainID, head)
	}
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
	logger := testlog.Logger(t, log.LvlInfo)
	depSet := sampleDepSet(t)
	controller := NewSyncNodesController(logger, depSet, &mockChainsDB{}, &mockBackend{})

	require.Zero(t, controller.controllers.Len(), "controllers should be empty to start")

	// Attach a controller for chain 900
	// make the controller return an anchor point
	ctrl := mockSyncControl{}
	ctrl.anchorPointFn = func(ctx context.Context) (types.DerivedBlockRefPair, error) {
		return types.DerivedBlockRefPair{
			Derived:     eth.BlockRef{Number: 1},
			DerivedFrom: eth.BlockRef{Number: 0},
		}, nil
	}

	// have the local safe return an error to trigger the initialization
	controller.db.(*mockChainsDB).localSafeFn = func(chainID types.ChainID) (types.DerivedBlockSealPair, error) {
		return types.DerivedBlockSealPair{}, types.ErrFuture
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

	// have OpenBlock return an error to trigger the initialization
	controller.db.(*mockChainsDB).openBlockFn = func(chainID types.ChainID, i uint64) (seal eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
		return eth.BlockRef{}, 0, nil, types.ErrFuture
	}
	unsafeCalled := 0
	controller.backend.(*mockBackend).updateLocalUnsafeFn = func(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error {
		unsafeCalled++
		return nil
	}

	// after the first attach, both databases are called for update
	_, err := controller.AttachNodeController(types.ChainIDFromUInt64(900), &ctrl, false)

	require.NoError(t, err)
	require.Equal(t, 1, localCalled, "local safe should have been updated once")
	require.Equal(t, 1, crossCalled, "cross safe should have been updated twice")
	require.Equal(t, 1, unsafeCalled, "local unsafe should have been updated once")

	// reset the local safe function to return no error
	controller.db.(*mockChainsDB).localSafeFn = nil
	// reset the open block function to return no error
	controller.db.(*mockChainsDB).openBlockFn = nil

	// after the second attach, there are no additional updates (no empty signal from the DB)
	ctrl2 := mockSyncControl{}
	_, err = controller.AttachNodeController(types.ChainIDFromUInt64(901), &ctrl2, false)
	require.NoError(t, err)
	require.Equal(t, 1, localCalled, "local safe should have been updated once")
	require.Equal(t, 1, crossCalled, "cross safe should have been updated once")
	require.Equal(t, 1, unsafeCalled, "local unsafe should have been updated once")
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
	_, err := controller.AttachNodeController(types.ChainIDFromUInt64(900), &ctrl, false)
	require.NoError(t, err)

	require.Equal(t, 1, controller.controllers.Len(), "controllers should have 1 entry")

	// Attach a controller for chain 901
	ctrl2 := mockSyncControl{}
	_, err = controller.AttachNodeController(types.ChainIDFromUInt64(901), &ctrl2, false)
	require.NoError(t, err)

	require.Equal(t, 2, controller.controllers.Len(), "controllers should have 2 entries")

	// Attach a controller for chain 902 (which is not in the dependency set)
	ctrl3 := mockSyncControl{}
	_, err = controller.AttachNodeController(types.ChainIDFromUInt64(902), &ctrl3, false)
	require.Error(t, err)
	require.Equal(t, 2, controller.controllers.Len(), "controllers should still have 2 entries")
}
