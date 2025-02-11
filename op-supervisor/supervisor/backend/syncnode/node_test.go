package syncnode

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
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
	syncCtrl.updateCrossSafeFn = func(ctx context.Context, derived eth.BlockID, derivedFrom eth.BlockID) error {
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

func TestResetConflict(t *testing.T) {
	chainID := eth.ChainIDFromUInt64(1)
	logger := testlog.Logger(t, log.LvlDebug)

	tests := []struct {
		name           string
		resetErrors    []error
		expectAttempts int
		expectError    bool
		l1RefNum       uint64
		finalizedNum   uint64
	}{
		{
			name:           "succeeds_first_try",
			resetErrors:    []error{nil},
			expectAttempts: 1,
			expectError:    false,
			l1RefNum:       100,
			finalizedNum:   50,
		},
		{
			name: "walks_back_on_block_not_found",
			resetErrors: []error{
				&gethrpc.JsonError{Code: blockNotFoundRPCErrCode},
				&gethrpc.JsonError{Code: blockNotFoundRPCErrCode},
				nil,
			},
			expectAttempts: 3,
			expectError:    false,
			l1RefNum:       100,
			finalizedNum:   50,
		},
		{
			name: "handles_finalized_boundary",
			resetErrors: []error{
				&gethrpc.JsonError{Code: blockNotFoundRPCErrCode},
			},
			expectAttempts: 1,
			expectError:    true,
			l1RefNum:       100,
			finalizedNum:   99,
		},
		{
			name: "stops_after_max_attempts_exceeded",
			resetErrors: func() []error {
				// Generate more errors than we allow attempts for
				errors := make([]error, maxWalkBackAttempts+100)
				for i := range errors {
					errors[i] = &gethrpc.JsonError{Code: blockNotFoundRPCErrCode}
				}
				return errors
			}(),
			// We expect the max number of attempts to be made, plus one for the initial attempt
			expectAttempts: maxWalkBackAttempts + 1,
			expectError:    true,
			l1RefNum:       1000,
			finalizedNum:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetAttempts := 0
			ctrl := &mockSyncControl{
				resetFn: func(ctx context.Context, unsafe, safe, finalized eth.BlockID) error {
					resetAttempts++
					if resetAttempts > len(tc.resetErrors) {
						return fmt.Errorf("unexpected reset attempt %d", resetAttempts)
					}
					return tc.resetErrors[resetAttempts-1]
				},
			}
			backend := &mockBackend{
				safeDerivedAtFn: func(ctx context.Context, chainID eth.ChainID, derivedFrom eth.BlockID) (eth.BlockID, error) {
					return eth.BlockID{Number: derivedFrom.Number}, nil
				},
			}

			node := NewManagedNode(logger, chainID, ctrl, backend, true)
			l1Ref := eth.BlockRef{Number: tc.l1RefNum}
			unsafe := eth.BlockID{Number: tc.l1RefNum + 100}
			finalized := eth.BlockID{Number: tc.finalizedNum}

			err := node.resolveConflict(context.Background(), l1Ref, unsafe, finalized)

			require.Equal(t, tc.expectAttempts, resetAttempts, "incorrect number of reset attempts")
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
