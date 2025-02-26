package status

import (
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/finality"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type L1UnsafeEvent struct {
	L1Unsafe eth.L1BlockRef
}

func (ev L1UnsafeEvent) String() string {
	return "l1-unsafe"
}

type L1SafeEvent struct {
	L1Safe eth.L1BlockRef
}

func (ev L1SafeEvent) String() string {
	return "l1-safe"
}

type Metrics interface {
	RecordL1ReorgDepth(d uint64)
	RecordL1Ref(name string, ref eth.L1BlockRef)
}

type StatusTracker struct {
	data eth.SyncStatus

	published atomic.Pointer[eth.SyncStatus]

	log log.Logger

	metrics Metrics

	mu sync.RWMutex
}

func NewStatusTracker(log log.Logger, metrics Metrics) *StatusTracker {
	st := &StatusTracker{
		log:     log,
		metrics: metrics,
	}
	st.data = eth.SyncStatus{}
	st.published.Store(&eth.SyncStatus{})
	return st
}

func (st *StatusTracker) OnEvent(ev event.Event) bool {
	st.mu.Lock()
	defer st.mu.Unlock()

	switch x := ev.(type) {
	case engine.ForkchoiceUpdateEvent:
		st.log.Debug("Forkchoice update", "unsafe", x.UnsafeL2Head, "safe", x.SafeL2Head, "finalized", x.FinalizedL2Head)
		st.data.UnsafeL2 = x.UnsafeL2Head
		st.data.SafeL2 = x.SafeL2Head
		st.data.FinalizedL2 = x.FinalizedL2Head
	case engine.PendingSafeUpdateEvent:
		st.data.UnsafeL2 = x.Unsafe
		st.data.PendingSafeL2 = x.PendingSafe
	case engine.CrossUnsafeUpdateEvent:
		st.log.Debug("Cross unsafe head updated", "cross_unsafe", x.CrossUnsafe, "local_unsafe", x.LocalUnsafe)
		st.data.CrossUnsafeL2 = x.CrossUnsafe
		st.data.UnsafeL2 = x.LocalUnsafe
	case engine.LocalSafeUpdateEvent:
		st.log.Debug("Local safe head updated", "local_safe", x.Ref)
		st.data.LocalSafeL2 = x.Ref
	case engine.CrossSafeUpdateEvent:
		st.log.Debug("Cross safe head updated", "cross_safe", x.CrossSafe, "local_safe", x.LocalSafe)
		st.data.SafeL2 = x.CrossSafe
		st.data.LocalSafeL2 = x.LocalSafe
	case derive.DeriverL1StatusEvent:
		st.data.CurrentL1 = x.Origin
	case L1UnsafeEvent:
		st.metrics.RecordL1Ref("l1_head", x.L1Unsafe)
		// We don't need to do anything if the head hasn't changed.
		if st.data.HeadL1 == (eth.L1BlockRef{}) {
			st.log.Info("Received first L1 head signal", "l1_head", x.L1Unsafe)
		} else if st.data.HeadL1.Hash == x.L1Unsafe.Hash {
			st.log.Trace("Received L1 head signal that is the same as the current head", "l1_head", x.L1Unsafe)
		} else if st.data.HeadL1.Hash == x.L1Unsafe.ParentHash {
			// We got a new L1 block whose parent hash is the same as the current L1 head. Means we're
			// dealing with a linear extension (new block is the immediate child of the old one).
			st.log.Debug("L1 head moved forward", "l1_head", x.L1Unsafe)
		} else {
			if st.data.HeadL1.Number >= x.L1Unsafe.Number {
				st.metrics.RecordL1ReorgDepth(st.data.HeadL1.Number - x.L1Unsafe.Number)
			}
			// New L1 block is not the same as the current head or a single step linear extension.
			// This could either be a long L1 extension, or a reorg, or we simply missed a head update.
			st.log.Warn("L1 head signal indicates a possible L1 re-org",
				"old_l1_head", st.data.HeadL1, "new_l1_head_parent", x.L1Unsafe.ParentHash, "new_l1_head", x.L1Unsafe)
		}
		st.data.HeadL1 = x.L1Unsafe
	case L1SafeEvent:
		st.log.Info("New L1 safe block", "l1_safe", x.L1Safe)
		st.metrics.RecordL1Ref("l1_safe", x.L1Safe)
		st.data.SafeL1 = x.L1Safe
	case finality.FinalizeL1Event:
		st.log.Info("New L1 finalized block", "l1_finalized", x.FinalizedL1)
		st.metrics.RecordL1Ref("l1_finalized", x.FinalizedL1)
		st.data.FinalizedL1 = x.FinalizedL1
		st.data.CurrentL1Finalized = x.FinalizedL1
	case rollup.ResetEvent:
		st.data.UnsafeL2 = eth.L2BlockRef{}
		st.data.SafeL2 = eth.L2BlockRef{}
		st.data.CurrentL1 = eth.L1BlockRef{}
	case engine.EngineResetConfirmedEvent:
		st.data.UnsafeL2 = x.LocalUnsafe
		st.data.CrossUnsafeL2 = x.CrossUnsafe
		st.data.LocalSafeL2 = x.LocalSafe
		st.data.SafeL2 = x.CrossSafe
		st.data.FinalizedL2 = x.Finalized
	default: // other events do not affect the sync status
		return false
	}

	// If anything changes, then copy the state to the published SyncStatus
	// @dev: If this becomes a performance bottleneck during sync (because mem copies onto heap, and 1KB comparisons),
	// we can rate-limit updates of the published data.
	published := *st.published.Load()
	if st.data != published {
		published = st.data
		st.published.Store(&published)
	}
	return true
}

// SyncStatus is thread safe, and reads the latest view of L1 and L2 block labels
func (st *StatusTracker) SyncStatus() *eth.SyncStatus {
	return st.published.Load()
}

// L1Head is a helper function; the L1 head is closely monitored for confirmation-distance logic.
func (st *StatusTracker) L1Head() eth.L1BlockRef {
	return st.SyncStatus().HeadL1
}
