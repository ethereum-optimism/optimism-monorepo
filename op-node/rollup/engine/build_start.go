package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type BuildStartEvent struct {
	Attributes *derive.AttributesWithParent
}

func (ev BuildStartEvent) String() string {
	return "build-start"
}

func (eq *EngDeriver) onBuildStart(ev BuildStartEvent) {
	ctx, cancel := context.WithTimeout(eq.ctx, buildStartTimeout)
	defer cancel()

	if ev.Attributes.DerivedFrom != (eth.L1BlockRef{}) &&
		eq.ec.PendingSafeL2Head().Hash != ev.Attributes.Parent.Hash {
		// Warn about small reorgs, happens when pending safe head is getting rolled back
		eq.log.Warn("block-attributes derived from L1 do not build on pending safe head, likely reorg",
			"pending_safe", eq.ec.PendingSafeL2Head(), "attributes_parent", ev.Attributes.Parent)
	}

	fcEvent := ForkchoiceUpdateEvent{
		UnsafeL2Head:    ev.Attributes.Parent,
		SafeL2Head:      eq.ec.safeHead,
		FinalizedL2Head: eq.ec.finalizedHead,
	}
	if fcEvent.UnsafeL2Head.Number < fcEvent.FinalizedL2Head.Number {
		err := fmt.Errorf("invalid block-building pre-state, unsafe head %s is behind finalized head %s", fcEvent.UnsafeL2Head, fcEvent.FinalizedL2Head)
		eq.emitter.Emit(rollup.CriticalErrorEvent{Err: err}) // make the node exit, things are very wrong.
		return
	}
	fc := eth.ForkchoiceState{
		HeadBlockHash:      fcEvent.UnsafeL2Head.Hash,
		SafeBlockHash:      fcEvent.SafeL2Head.Hash,
		FinalizedBlockHash: fcEvent.FinalizedL2Head.Hash,
	}
	buildStartTime := time.Now()
	id, errTyp, err := startPayload(ctx, eq.ec.engine, fc, ev.Attributes.Attributes)
	if err != nil {
		switch errTyp {
		case BlockInsertTemporaryErr:
			// RPC errors are recoverable, we can retry the buffered payload attributes later.
			eq.emitter.Emit(rollup.EngineTemporaryErrorEvent{Err: fmt.Errorf("temporarily cannot insert new safe block: %w", err)})
			return
		case BlockInsertPrestateErr:
			eq.emitter.Emit(rollup.ResetEvent{Err: fmt.Errorf("need reset to resolve pre-state problem: %w", err)})
			return
		case BlockInsertPayloadErr:
			eq.emitter.Emit(BuildInvalidEvent{Attributes: ev.Attributes, Err: err})
			return
		default:
			eq.emitter.Emit(rollup.CriticalErrorEvent{Err: fmt.Errorf("unknown error type %d: %w", errTyp, err)})
			return
		}
	}
	eq.emitter.Emit(fcEvent)

	eq.emitter.Emit(BuildStartedEvent{
		Info:         eth.PayloadInfo{ID: id, Timestamp: uint64(ev.Attributes.Attributes.Timestamp)},
		BuildStarted: buildStartTime,
		Concluding:   ev.Attributes.Concluding,
		DerivedFrom:  ev.Attributes.DerivedFrom,
		Parent:       ev.Attributes.Parent,
	})
}
