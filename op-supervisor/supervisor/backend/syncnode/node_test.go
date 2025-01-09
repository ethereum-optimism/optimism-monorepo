package syncnode

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestEventResponse(t *testing.T) {
	chainID := types.ChainIDFromUInt64(1)
	logger := testlog.Logger(t, log.LvlInfo)
	syncCtrl := &mockSyncControl{}
	db := &mockChainsDB{}
	backend := &mockBackend{}

	node := NewManagedNode(logger, chainID, syncCtrl, db, backend, false)

	crossUnsafe := 0
	crossSafe := 0
	finalized := 0

	nodeUnsafe := 0
	nodeDerivation := 0
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

	// track events from the node
	// the node will call UpdateLocalUnsafe when a new unsafe block is received
	backend.updateLocalUnsafeFn = func(ctx context.Context, chID types.ChainID, unsafe eth.BlockRef) error {
		nodeUnsafe++
		return nil
	}
	// the node will call UpdateLocalSafe when a new safe and L1 derivation source is received
	backend.updateLocalSafeFn = func(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.L1BlockRef) error {
		nodeDerivation++
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
		db.subscribeCrossUnsafe.Send(types.BlockSeal{})
		db.subscribeCrosSafe.Send(types.DerivedBlockSealPair{})
		db.subscribeFinalized.Send(types.BlockSeal{})
		syncCtrl.subscribeEvents.Send(&types.ManagedEvent{
			UnsafeBlock: &eth.BlockRef{Number: 1}})
		syncCtrl.subscribeEvents.Send(&types.ManagedEvent{
			DerivationUpdate: &types.DerivedBlockRefPair{DerivedFrom: eth.BlockRef{Number: 1}, Derived: eth.BlockRef{Number: 2}}})
		syncCtrl.subscribeEvents.Send(&types.ManagedEvent{
			ExhaustL1: &types.DerivedBlockRefPair{DerivedFrom: eth.BlockRef{Number: 1}, Derived: eth.BlockRef{Number: 2}}})

		return crossUnsafe >= 1 &&
			crossSafe >= 1 &&
			finalized >= 1 &&
			nodeUnsafe >= 1 &&
			nodeDerivation >= 1 &&
			nodeExhausted >= 1
	}, 4*time.Second, 250*time.Millisecond)
}
