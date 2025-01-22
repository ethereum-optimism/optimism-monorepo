package engine

import (
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type PayloadSuccessEvent struct {
	// if payload should be promoted to (local) safe (must also be pending safe, see DerivedFrom)
	Concluding bool
	// payload is promoted to pending-safe if non-zero
	DerivedFrom   eth.L1BlockRef
	BuildStarted  time.Time
	InsertStarted time.Time

	Envelope *eth.ExecutionPayloadEnvelope
	Ref      eth.L2BlockRef
}

func (ev PayloadSuccessEvent) String() string {
	return "payload-success"
}

func (eq *EngDeriver) onPayloadSuccess(ev PayloadSuccessEvent) {
	if ev.DerivedFrom == ReplaceBlockDerivedFrom {
		eq.log.Warn("Successfully built replacement block, resetting chain to continue now", "replacement", ev.Ref)
		// Change the engine state to make the replacement block the cross-safe head of the chain
		ForceEngineReset(eq.ec, ForceEngineResetEvent{
			Unsafe:    ev.Ref,
			Safe:      ev.Ref,
			Finalized: eq.ec.Finalized(),
		})
		eq.emitter.Emit(InteropReplacedBlockEvent{
			Envelope: ev.Envelope,
			Ref:      ev.Ref.BlockRef(),
		})
		// Apply it to the execution engine
		eq.emitter.Emit(TryUpdateEngineEvent{})
		// Reset the rest of the system to continue from here.
		eq.emitter.Emit(rollup.ResetEvent{Err: fmt.Errorf("replaced invalidated block with %s, chain may continue after reset", ev.Ref)})
		return
	}

	eq.emitter.Emit(PromoteUnsafeEvent{Ref: ev.Ref})

	// If derived from L1, then it can be considered (pending) safe
	if ev.DerivedFrom != (eth.L1BlockRef{}) {
		eq.emitter.Emit(PromotePendingSafeEvent{
			Ref:         ev.Ref,
			Concluding:  ev.Concluding,
			DerivedFrom: ev.DerivedFrom,
		})
	}

	eq.emitter.Emit(TryUpdateEngineEvent{
		BuildStarted:  ev.BuildStarted,
		InsertStarted: ev.InsertStarted,
		Envelope:      ev.Envelope,
	})
}
