package syncnode

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestEventResponse(t *testing.T) {
	chainID := eth.ChainIDFromUInt64(1)
	logger := testlog.Logger(t, log.LvlInfo)
	syncCtrl := &mockSyncControl{}
	backend := &mockBackend{}

	ex := event.NewGlobalSynchronous(context.Background())
	eventSys := event.NewSystem(logger, ex)

	mon := &eventMonitor{}
	eventSys.Register("monitor", mon, event.DefaultRegisterOpts())

	node := NewManagedNode(logger, chainID, syncCtrl, backend, false)
	eventSys.Register("node", node, event.DefaultRegisterOpts())

	emitter := eventSys.Register("test", nil, event.DefaultRegisterOpts())

	crossUnsafe := 0
	crossSafe := 0
	finalized := 0

	nodeExhausted := 0

	// the node will call UpdateCrossUnsafe when a cross-unsafe event is received from the database
	syncCtrl.updateCrossUnsafeFn = func(ctx context.Context, id eth.BlockID) error {
		crossUnsafe++
		return nil
	}
	// the node will call UpdateCrossSafe when a cross-safe event is received from the database
	syncCtrl.updateCrossSafeFn = func(ctx context.Context, derived eth.BlockID, source eth.BlockID) error {
		crossSafe++
		return nil
	}
	// the node will call UpdateFinalized when a finalized event is received from the database
	syncCtrl.updateFinalizedFn = func(ctx context.Context, id eth.BlockID) error {
		finalized++
		return nil
	}

	// the node will call ProvideL1 when the node is exhausted and needs a new L1 derivation source
	syncCtrl.provideL1Fn = func(ctx context.Context, nextL1 eth.BlockRef) error {
		nodeExhausted++
		return nil
	}

	// TODO(#13595): rework node-reset, and include testing for it here

	node.Start()

	// send events and continue to do so until at least one of each type has been received
	require.Eventually(t, func() bool {
		// send in one event of each type
		emitter.Emit(superevents.CrossUnsafeUpdateEvent{ChainID: chainID})
		emitter.Emit(superevents.CrossSafeUpdateEvent{ChainID: chainID})
		emitter.Emit(superevents.FinalizedL2UpdateEvent{ChainID: chainID})

		syncCtrl.subscribeEvents.Send(&types.ManagedEvent{
			UnsafeBlock: &eth.BlockRef{Number: 1}})
		syncCtrl.subscribeEvents.Send(&types.ManagedEvent{
			DerivationUpdate: &types.DerivedBlockRefPair{Source: eth.BlockRef{Number: 1}, Derived: eth.BlockRef{Number: 2}}})
		syncCtrl.subscribeEvents.Send(&types.ManagedEvent{
			ExhaustL1: &types.DerivedBlockRefPair{Source: eth.BlockRef{Number: 1}, Derived: eth.BlockRef{Number: 2}}})
		syncCtrl.subscribeEvents.Send(&types.ManagedEvent{
			DerivationOriginUpdate: &eth.BlockRef{Number: 1}})

		require.NoError(t, ex.Drain())

		return crossUnsafe >= 1 &&
			crossSafe >= 1 &&
			finalized >= 1 &&
			mon.receivedLocalUnsafe >= 1 &&
			mon.localDerived >= 1 &&
			nodeExhausted >= 1 &&
			mon.localDerivedOriginUpdate >= 1
	}, 4*time.Second, 250*time.Millisecond)
}

func TestPrepareReset(t *testing.T) {
	chainID := eth.ChainIDFromUInt64(1)
	logger := testlog.Logger(t, log.LvlInfo)
	syncCtrl := &mockSyncControl{}
	backend := &mockBackend{}

	ex := event.NewGlobalSynchronous(context.Background())
	eventSys := event.NewSystem(logger, ex)

	mon := &eventMonitor{}
	eventSys.Register("monitor", mon, event.DefaultRegisterOpts())

	node := NewManagedNode(logger, chainID, syncCtrl, backend, false)
	eventSys.Register("node", node, event.DefaultRegisterOpts())

	// mock: return a block of the same number as requested
	syncCtrl.blockRefByNumFn = func(ctx context.Context, number uint64) (eth.BlockRef, error) {
		return eth.BlockRef{Number: number}, nil
	}

	// mock: control whether the blocks appear valid or not
	var pivot uint64
	backend.isLocalUnsafeFn = func(ctx context.Context, chainID eth.ChainID, id eth.BlockID) error {
		if id.Number > uint64(pivot) {
			return types.ErrConflict
		}
		return nil
	}

	// mock: record the reset signal given to the node
	var unsafe, safe, finalized eth.BlockID
	syncCtrl.resetFn = func(ctx context.Context, u, s, f eth.BlockID) error {
		unsafe = u
		safe = s
		finalized = f
		return nil
	}

	// test that the bisection finds the correct block,
	// anywhere inside the min-max range
	min, max := uint64(0), uint64(235)
	for i := min; i < max; i++ {
		node.ongoingReset = &ongoingReset{
			a: eth.BlockID{Number: min},
			z: eth.BlockID{Number: max},
		}
		pivot = i
		node.prepareReset()
		require.Equal(t, i, unsafe.Number)
		require.Equal(t, uint64(0), safe.Number)
		require.Equal(t, uint64(0), finalized.Number)
	}

	// mock: return local safe and finalized blocks which are *ahead* of the pivot
	backend.localSafeFn = func(ctx context.Context, chainID eth.ChainID) (types.DerivedIDPair, error) {
		return types.DerivedIDPair{
			Derived: eth.BlockID{Number: pivot + 1},
		}, nil
	}
	backend.finalizedFn = func(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error) {
		return eth.BlockID{Number: pivot + 1}, nil
	}
	// test that the bisection finds the correct block,
	// AND that the safe and finalized blocks are updated to match the unsafe block
	for i := min; i < max; i++ {
		node.ongoingReset = &ongoingReset{
			a: eth.BlockID{Number: min},
			z: eth.BlockID{Number: max},
		}
		pivot = i
		node.prepareReset()
		require.Equal(t, i, unsafe.Number)
		require.Equal(t, i, safe.Number)
		require.Equal(t, i, finalized.Number)
	}
}
