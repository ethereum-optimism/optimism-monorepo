package syncnode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	gethevent "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type mockSyncControl struct {
	anchorPointFn       func(ctx context.Context) (types.DerivedBlockRefPair, error)
	provideL1Fn         func(ctx context.Context, ref eth.BlockRef) error
	resetFn             func(ctx context.Context, unsafe, safe, finalized eth.BlockID) error
	updateCrossSafeFn   func(ctx context.Context, derived, source eth.BlockID) error
	updateCrossUnsafeFn func(ctx context.Context, derived eth.BlockID) error
	updateFinalizedFn   func(ctx context.Context, id eth.BlockID) error
	pullEventFn         func(ctx context.Context) (*types.ManagedEvent, error)

	subscribeEvents gethevent.FeedOf[*types.ManagedEvent]
}

func (m *mockSyncControl) InvalidateBlock(ctx context.Context, seal types.BlockSeal) error {
	return nil
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

func (m *mockSyncControl) UpdateCrossSafe(ctx context.Context, derived eth.BlockID, source eth.BlockID) error {
	if m.updateCrossSafeFn != nil {
		return m.updateCrossSafeFn(ctx, derived, source)
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

func (m *mockSyncControl) String() string {
	return "mock"
}

var _ SyncControl = (*mockSyncControl)(nil)

type mockBackend struct {
	safeDerivedAtFn func(ctx context.Context, chainID eth.ChainID, source eth.BlockID) (eth.BlockID, error)
}

func (m *mockBackend) LocalSafe(ctx context.Context, chainID eth.ChainID) (pair types.DerivedIDPair, err error) {
	return types.DerivedIDPair{}, nil
}

func (m *mockBackend) CrossSafe(ctx context.Context, chainID eth.ChainID) (types.DerivedIDPair, error) {
	return types.DerivedIDPair{}, nil
}

func (m *mockBackend) LocalUnsafe(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error) {
	return eth.BlockID{}, nil
}

func (m *mockBackend) SafeDerivedAt(ctx context.Context, chainID eth.ChainID, source eth.BlockID) (derived eth.BlockID, err error) {
	if m.safeDerivedAtFn != nil {
		return m.safeDerivedAtFn(ctx, chainID, source)
	}
	return eth.BlockID{}, nil
}

func (m *mockBackend) Finalized(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error) {
	return eth.BlockID{}, nil
}

func (m *mockBackend) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	return eth.L1BlockRef{}, nil
}

var _ backend = (*mockBackend)(nil)

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
		})
	require.NoError(t, err)
	return depSet
}

type eventMonitor struct {
	anchorCalled             int
	localDerived             int
	receivedLocalUnsafe      int
	localDerivedOriginUpdate int
}

func (m *eventMonitor) OnEvent(ev event.Event) bool {
	switch ev.(type) {
	case superevents.AnchorEvent:
		m.anchorCalled += 1
	case superevents.LocalDerivedEvent:
		m.localDerived += 1
	case superevents.LocalUnsafeReceivedEvent:
		m.receivedLocalUnsafe += 1
	case superevents.LocalDerivedOriginUpdateEvent:
		m.localDerivedOriginUpdate += 1
	default:
		return false
	}
	return true
}

// TestAttachNodeController tests the AttachNodeController function of the SyncNodesController.
// Only controllers for chains in the dependency set can be attached.
func TestAttachNodeController(t *testing.T) {
	logger := log.New()
	depSet := sampleDepSet(t)
	ex := event.NewGlobalSynchronous(context.Background())
	eventSys := event.NewSystem(logger, ex)
	controller := NewSyncNodesController(logger, depSet, eventSys, &mockBackend{})
	eventSys.Register("controller", controller, event.DefaultRegisterOpts())
	require.Zero(t, controller.controllers.Len(), "controllers should be empty to start")

	// Attach a controller for chain 900
	ctrl := mockSyncControl{}
	_, err := controller.AttachNodeController(eth.ChainIDFromUInt64(900), &ctrl, false)
	require.NoError(t, err)

	require.Equal(t, 1, controller.controllers.Len(), "controllers should have 1 entry")

	// Attach a controller for chain 901
	ctrl2 := mockSyncControl{}
	_, err = controller.AttachNodeController(eth.ChainIDFromUInt64(901), &ctrl2, false)
	require.NoError(t, err)

	require.Equal(t, 2, controller.controllers.Len(), "controllers should have 2 entries")

	// Attach a controller for chain 902 (which is not in the dependency set)
	ctrl3 := mockSyncControl{}
	_, err = controller.AttachNodeController(eth.ChainIDFromUInt64(902), &ctrl3, false)
	require.Error(t, err)
	require.Equal(t, 2, controller.controllers.Len(), "controllers should still have 2 entries")
}
