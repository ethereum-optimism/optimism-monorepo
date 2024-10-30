package engine

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type PayloadProcessEvent struct {
	// if payload should be promoted to safe (must also be pending safe, see DerivedFrom)
	IsLastInSpan bool
	// payload is promoted to pending-safe if non-zero
	DerivedFrom eth.L1BlockRef

	Envelope *eth.ExecutionPayloadEnvelope
	Ref      eth.L2BlockRef
}

func (ev PayloadProcessEvent) String() string {
	return "payload-process"
}

func (eq *EngDeriver) onPayloadProcess(ev PayloadProcessEvent) {
	ctx, cancel := context.WithTimeout(eq.ctx, payloadProcessTimeout)
	defer cancel()

	status, err := eq.ec.engine.NewPayload(ctx,
		ev.Envelope.ExecutionPayload, ev.Envelope.ParentBeaconBlockRoot)
	if err != nil {
		eq.emitter.Emit(rollup.EngineTemporaryErrorEvent{
			Err: fmt.Errorf("failed to insert execution payload: %w", err),
		})
		return
	}
	switch status.Status {
	case eth.ExecutionInvalid, eth.ExecutionInvalidBlockHash:
		// TODO: I'm not sure how derivation can reach this point. By my understand, it would
		// already fail earlier during the FCU call.
		if ev.DerivedFrom != (eth.L1BlockRef{}) && eq.cfg.IsHolocene(ev.DerivedFrom.Time) {
			eq.emitDepositsOnlyPayloadAttributesRequest(ev.Ref.ParentID(), ev.DerivedFrom)
			return
		}

		eq.emitter.Emit(PayloadInvalidEvent{
			Envelope: ev.Envelope,
			Err:      eth.NewPayloadErr(ev.Envelope.ExecutionPayload, status),
		})
		return
	case eth.ExecutionValid:
		eq.emitter.Emit(PayloadSuccessEvent(ev))
		return
	default:
		eq.emitter.Emit(rollup.EngineTemporaryErrorEvent{
			Err: eth.NewPayloadErr(ev.Envelope.ExecutionPayload, status),
		})
		return
	}
}
