package interop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/finality"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

const rpcTimeout = time.Second * 10

type InteropBackend interface {
	UnsafeView(ctx context.Context, chainID types.ChainID, unsafe types.ReferenceView) (types.ReferenceView, error)
	SafeView(ctx context.Context, chainID types.ChainID, safe types.ReferenceView) (types.ReferenceView, error)
	Finalized(ctx context.Context, chainID types.ChainID) (eth.BlockID, error)

	CrossDerivedFrom(ctx context.Context, chainID types.ChainID, derived eth.BlockID) (eth.L1BlockRef, error)

	UpdateLocalUnsafe(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error
	UpdateLocalSafe(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.BlockRef) error
	UpdateFinalizedL1(ctx context.Context, chainID types.ChainID, finalized eth.L1BlockRef) error
}

// For testing usage, the backend of the supervisor implements the interface, no need for RPC.
var _ InteropBackend = (*backend.SupervisorBackend)(nil)

// For RPC usage, the supervisor client implements the interop backend.
var _ InteropBackend = (*sources.SupervisorClient)(nil)

type L2Source interface {
	L2BlockRefByNumber(context.Context, uint64) (eth.L2BlockRef, error)
	L2BlockRefByHash(ctx context.Context, l2Hash common.Hash) (eth.L2BlockRef, error)
}

// InteropDeriver watches for update events (either real changes to block safety,
// or updates published upon request), checks if there is some local data to cross-verify,
// and then checks with the interop-backend, to try to promote to cross-verified safety.
type InteropDeriver struct {
	log log.Logger
	cfg *rollup.Config

	// we cache the chainID,
	// to not continuously convert from the type in the rollup-config to this type.
	chainID types.ChainID

	driverCtx context.Context

	backend InteropBackend
	l2      L2Source

	emitter event.Emitter

	mu sync.Mutex
}

var _ event.Deriver = (*InteropDeriver)(nil)
var _ event.AttachEmitter = (*InteropDeriver)(nil)

func NewInteropDeriver(log log.Logger, cfg *rollup.Config,
	driverCtx context.Context, backend InteropBackend, l2 L2Source) *InteropDeriver {
	return &InteropDeriver{
		log:       log,
		cfg:       cfg,
		chainID:   types.ChainIDFromBig(cfg.L2ChainID),
		driverCtx: driverCtx,
		backend:   backend,
		l2:        l2,
	}
}

func (d *InteropDeriver) AttachEmitter(em event.Emitter) {
	d.emitter = em
}

func (d *InteropDeriver) OnEvent(ev event.Event) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	switch x := ev.(type) {
	case engine.UnsafeUpdateEvent:
		d.onLocalUnsafeUpdate(x)
	case engine.InteropPendingSafeChangedEvent:
		d.onInteropPendingSafeChangedEvent(x)
	case finality.FinalizeL1Event:
		d.onFinalizedL1(x)
	case derive.DeriverL1StatusEvent:
		d.log.Debug("deriver L1 traversal event", "l1", x.Origin, "l2", x.LastL2)
		// Register traversal of L1, repeat the last local-safe L2
		d.onInteropPendingSafeChangedEvent(engine.InteropPendingSafeChangedEvent{
			Ref:         x.LastL2,
			DerivedFrom: x.Origin,
		})
	case engine.CrossUnsafeUpdateEvent:
		if err := d.onCrossUnsafe(x); err != nil {
			d.log.Error("Failed to process cross-unsafe update", "err", err)
		}
	case engine.CrossSafeUpdateEvent:
		if err := d.onCrossSafeUpdateEvent(x); err != nil {
			d.log.Error("Failed to process cross-safe update", "err", err)
		}
	case engine.FinalizedUpdateEvent:
		if err := d.onFinalizedUpdate(x); err != nil {
			d.log.Error("Failed to process finalized update", "err", err)
		}
	default:
		return false
	}
	return true
}

func (d *InteropDeriver) onLocalUnsafeUpdate(x engine.UnsafeUpdateEvent) {
	d.log.Debug("Signaling unsafe L2 head update to interop backend", "head", x.Ref)
	ctx, cancel := context.WithTimeout(d.driverCtx, rpcTimeout)
	defer cancel()
	if err := d.backend.UpdateLocalUnsafe(ctx, d.chainID, x.Ref.BlockRef()); err != nil {
		d.log.Warn("Failed to signal unsafe L2 head to interop backend", "head", x.Ref, "err", err)
		// still continue to try and do a cross-unsafe update
	}
	// Now that the op-supervisor is aware of the new local-unsafe block, we want to check if cross-unsafe changed.
	d.emitter.Emit(engine.RequestCrossUnsafeEvent{})
}

func (d *InteropDeriver) onInteropPendingSafeChangedEvent(x engine.InteropPendingSafeChangedEvent) {
	d.log.Debug("Signaling derived-from update to interop backend", "derivedFrom", x.DerivedFrom, "block", x.Ref)
	ctx, cancel := context.WithTimeout(d.driverCtx, rpcTimeout)
	defer cancel()
	if err := d.backend.UpdateLocalSafe(ctx, d.chainID, x.DerivedFrom, x.Ref.BlockRef()); err != nil {
		d.log.Debug("Failed to signal derived-from update to interop backend", "derivedFrom", x.DerivedFrom, "block", x.Ref)
		// still continue to try and do a cross-safe update
	}
	// Now that the op-supervisor is aware of the new local-safe block, we want to check if cross-safe changed.
	d.emitter.Emit(engine.RequestCrossSafeEvent{})
}

func (d *InteropDeriver) onFinalizedL1(x finality.FinalizeL1Event) {
	if !d.cfg.IsInterop(x.FinalizedL1.Time) {
		return
	}
	d.log.Debug("Signaling finalized L1 update to interop backend", "finalized", x.FinalizedL1)
	ctx, cancel := context.WithTimeout(d.driverCtx, rpcTimeout)
	defer cancel()
	if err := d.backend.UpdateFinalizedL1(ctx, d.chainID, x.FinalizedL1); err != nil {
		d.log.Warn("Failed to signal finalized L1 block to interop backend", "finalized", x.FinalizedL1, "err", err)
	}
	// New L2 blocks may be ready to finalize now that the backend knows of new L1 finalized info.
	d.emitter.Emit(engine.RequestFinalizedUpdateEvent{})
}

func (d *InteropDeriver) onCrossUnsafe(x engine.CrossUnsafeUpdateEvent) error {
	if x.CrossUnsafe.Number >= x.LocalUnsafe.Number {
		return nil // nothing left to promote
	}

	// Pre-interop the engine itself handles promotion to cross-unsafe.
	// Start checking cross-unsafe once the local-unsafe block is in the interop update.
	if !d.cfg.IsInterop(x.LocalUnsafe.Time) {
		return nil
	}
	ctx, cancel := context.WithTimeout(d.driverCtx, rpcTimeout)
	defer cancel()
	view := types.ReferenceView{
		Local: x.LocalUnsafe.ID(),
		Cross: x.CrossUnsafe.ID(),
	}
	result, err := d.backend.UnsafeView(ctx, d.chainID, view)
	if err != nil {
		return fmt.Errorf("failed to check unsafe-level view: %w", err)
	}
	if result.Cross.Number == x.CrossUnsafe.Number {
		// supervisor is in sync with op-node
		return nil
	}
	if result.Cross.Number < x.CrossUnsafe.Number {
		d.log.Warn("op-supervisor is behind known cross-unsafe block", "supervisor", result.Cross, "known", x.CrossUnsafe)
		return nil
	}
	d.log.Info("New cross-unsafe block", "block", result.Cross.Number)
	// Note: in the future we want to do reorg-checks,
	// and initiate a reorg, if found to be on a conflicting chain.
	ref, err := d.l2.L2BlockRefByHash(ctx, result.Cross.Hash)
	if err != nil {
		return fmt.Errorf("failed to get cross-unsafe block info of %s: %w", result.Cross, err)
	}
	d.emitter.Emit(engine.PromoteCrossUnsafeEvent{Ref: ref})

	return nil
}

func (d *InteropDeriver) onCrossSafeUpdateEvent(x engine.CrossSafeUpdateEvent) error {
	if x.CrossSafe.Number >= x.LocalSafe.Number {
		return nil // nothing left to promote
	}
	// Pre-interop the engine itself handles promotion to cross-safe.
	// Start checking cross-safe once the local-safe block is in the interop update.
	if !d.cfg.IsInterop(x.LocalSafe.Time) {
		return nil
	}
	ctx, cancel := context.WithTimeout(d.driverCtx, rpcTimeout)
	defer cancel()
	view := types.ReferenceView{
		Local: x.LocalSafe.ID(),
		Cross: x.CrossSafe.ID(),
	}
	result, err := d.backend.SafeView(ctx, d.chainID, view)
	if err != nil {
		return fmt.Errorf("failed to check safe-level view: %w", err)
	}
	if result.Cross.Number == x.CrossSafe.Number {
		// supervisor is in sync with op-node
		return nil
	}
	if result.Cross.Number < x.CrossSafe.Number {
		d.log.Warn("op-supervisor is behind known cross-safe block", "supervisor", result.Cross, "known", x.CrossSafe)
		// TODO: we may want to force set the cross-safe block in the engine,
		//  and then reset derivation, so this op-node can help get the supervisor back in sync.
		return nil
	}
	derived := eth.BlockID{
		Hash:   result.Cross.Hash,
		Number: result.Cross.Number,
	}
	derivedFrom, err := d.backend.CrossDerivedFrom(ctx, d.chainID, derived)
	if err != nil {
		return fmt.Errorf("failed to get derived-from of %s: %w", result.Cross, err)
	}
	d.log.Info("New cross-safe block", "block", result.Cross.Number)
	ref, err := d.l2.L2BlockRefByHash(ctx, result.Cross.Hash)
	if err != nil {
		return fmt.Errorf("failed to get block ref of %s: %w", result.Cross, err)
	}
	d.emitter.Emit(engine.PromoteSafeEvent{
		Ref:         ref,
		DerivedFrom: derivedFrom,
	})
	d.emitter.Emit(engine.RequestFinalizedUpdateEvent{})
	return nil
}

func (d *InteropDeriver) onFinalizedUpdate(x engine.FinalizedUpdateEvent) error {
	// Note: we have to check interop fork, but finality may be pre-fork activation until we update.
	// We may want to change this to only start checking finality once the local head is past the activation.

	ctx, cancel := context.WithTimeout(d.driverCtx, rpcTimeout)
	defer cancel()

	finalized, err := d.backend.Finalized(ctx, d.chainID)
	if err != nil {
		return fmt.Errorf("failed to retrieve finalized L2 block from supervisor: %w", err)
	}
	// Check if we can finalize something new
	if finalized.Number == x.Ref.Number {
		// supervisor is in sync with op-node
		return nil
	}
	if finalized.Number < x.Ref.Number {
		d.log.Warn("op-supervisor is behind known finalized block", "supervisor", finalized, "known", x.Ref)
		return nil
	}
	ref, err := d.l2.L2BlockRefByHash(ctx, finalized.Hash)
	if err != nil {
		return fmt.Errorf("failed to get block ref of %s: %w", finalized, err)
	}
	d.log.Info("New finalized block from supervisor", "block", finalized.Number)
	d.emitter.Emit(engine.PromoteFinalizedEvent{
		Ref: ref,
	})
	return nil
}
