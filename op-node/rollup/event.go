package rollup

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// L1TemporaryErrorEvent identifies a temporary issue with the L1 data.
type L1TemporaryErrorEvent struct {
	Err error
}

var _ event.Event = L1TemporaryErrorEvent{}

func (ev L1TemporaryErrorEvent) String() string {
	return "l1-temporary-error"
}

// EngineTemporaryErrorEvent identifies a temporary processing issue.
// It applies to both L1 and L2 data, often inter-related.
// This scope will be reduced over time, to only capture L2-engine specific temporary errors.
// See L1TemporaryErrorEvent for L1 related temporary errors.
type EngineTemporaryErrorEvent struct {
	Err error
}

var _ event.Event = EngineTemporaryErrorEvent{}

func (ev EngineTemporaryErrorEvent) String() string {
	return "engine-temporary-error"
}

type ResetEvent struct {
	Err error
}

var _ event.Event = ResetEvent{}

func (ev ResetEvent) String() string {
	return "reset-event"
}

// ForceResetEvent forces a reset to a specific local-unsafe/local-safe/finalized starting point.
// Resets may override local-unsafe, to reset the very end of the chain.
// Resets may override local-safe, since post-interop we need the local-safe block derivation to continue.
// Pre-interop both local and cross values should be set the same.
type ForceResetEvent struct {
	// LocalUnsafe is optional: the existing chain local-unsafe head will be preserved if this field is zeroed.
	LocalUnsafe eth.L2BlockRef

	CrossUnsafe, LocalSafe, CrossSafe, Finalized eth.L2BlockRef
}

func (ev ForceResetEvent) String() string {
	return "force-reset"
}

// CriticalErrorEvent is an alias for event.CriticalErrorEvent
type CriticalErrorEvent = event.CriticalErrorEvent
