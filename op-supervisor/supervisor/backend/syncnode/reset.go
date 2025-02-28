package syncnode

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum"
)

// resetTracker manages a bisection
// between consistent and inconsistent blocks
// and is used to prepare a reset request
// which is sent to the managed node
type resetTracker struct {
	a eth.BlockID
	z eth.BlockID

	synchronous bool
	resetting   *atomic.Bool
	cancelling  *atomic.Bool

	managed *ManagedNode
}

// init initializes the reset tracker with
// empty start and end of range, and no reset in progress
func (t *resetTracker) init() {
	t.resetting.Store(true)
	t.cancelling.Store(false)
	t.a = eth.BlockID{}
	t.z = eth.BlockID{}
}

// beginBisectionReset initializes the reset tracker
// and starts the bisection process at the given block
// which will lead to a reset request
func (t *resetTracker) beginBisectionReset(z eth.BlockID) {
	t.managed.log.Info("beginning reset", "endOfRange", z)
	// only one reset can be in progress at a time
	if t.resetting.Load() {
		return
	}
	// initialize the reset tracker
	t.init()
	t.z = z
	// action tests may prefer to run the managed node totally synchronously
	if t.synchronous {
		t.bisectToTarget()
	} else {
		go t.bisectToTarget()
	}
}

// endReset signals that the reset is over
func (t *resetTracker) endReset() {
	t.resetting.Store(false)
	t.cancelling.Store(false)
}

// isResetting returns true if a reset is in progress
func (t *resetTracker) isResetting() bool {
	return t.resetting.Load()
}

// cancelReset signals that the ongoing reset should be cancelled
// it is not guaranteed that the reset will be cancelled immediately
func (t *resetTracker) cancelReset() {
	t.cancelling.Store(true)
}

// bisectToTarget prepares the reset by bisecting the search range until the last consistent block is found.
// it then calls resetHeadsFromTarget to trigger the reset on the node.
func (t *resetTracker) bisectToTarget() {
	nodeCtx, nCancel := context.WithTimeout(t.managed.ctx, nodeTimeout)
	defer nCancel()
	internalCtx, iCancel := context.WithTimeout(t.managed.ctx, internalTimeout)
	defer iCancel()

	// initialize the start of the range if it is empty
	if t.a == (eth.BlockID{}) {
		t.managed.log.Debug("start of range is empty, finding the first block")
		var err error
		t.a, err = t.managed.backend.FindSealedBlock(internalCtx, t.managed.chainID, 0)
		if err != nil {
			t.managed.log.Error("failed to initialize start of bisection range", "err", err)
			t.endReset()
			return
		}
	}

	// before starting bisection, check if z is already consistent (i.e. the node is ahead but otherwise consistent)
	nodeZ, err := t.managed.Node.BlockRefByNumber(nodeCtx, t.z.Number)
	// if z is already consistent, we can skip the bisection
	// and move straight to a targeted reset
	if err == nil && nodeZ.ID() == t.z {
		t.resetHeadsFromTarget(t.z)
		return
	}

	// before starting bisection, check if a is inconsistent (i.e. the node has no common reference point)
	// if the first block in the range can't be found or is inconsistent, we can't do a reset
	nodeA, err := t.managed.Node.BlockRefByNumber(nodeCtx, t.a.Number)
	if err != nil {
		t.managed.log.Error("failed to get block at start of range. cannot reset node", "err", err)
		t.endReset()
		return
	}
	if nodeA.ID() != t.a {
		t.managed.log.Error("start of range is inconsistent with logs db. cannot reset node",
			"a", t.a,
			"block", nodeA.ID())
		t.endReset()
		return
	}

	// repeatedly bisect the range until the last consistent block is found
	for {
		if t.cancelling.Load() {
			t.managed.log.Debug("reset cancelled")
			t.endReset()
			return
		}
		if t.a.Number >= t.z.Number {
			t.managed.log.Debug("reset target converged. Resetting to start of range", "a", t.a, "z", t.z)
			t.resetHeadsFromTarget(t.a)
			return
		}
		if t.a.Number+1 == t.z.Number {
			break
		}
		err := t.bisect()
		if err != nil {
			t.managed.log.Error("failed to bisect recovery range. cannot reset node", "err", err)
			t.endReset()
			return
		}
	}
	// the bisection is now complete. a is the last consistent block, and z is the first inconsistent block
	t.resetHeadsFromTarget(t.a)
}

// bisect halves the search range of the ongoing reset to narrow down
// where the reset will target. It bisects the range and constrains either
// the start or the end of the range, based on the consistency of the midpoint
// with the logs db.
func (t *resetTracker) bisect() error {
	internalCtx, iCancel := context.WithTimeout(t.managed.ctx, internalTimeout)
	defer iCancel()
	nodeCtx, nCancel := context.WithTimeout(t.managed.ctx, nodeTimeout)
	defer nCancel()

	// attempt to get the block at the midpoint of the range
	i := (t.a.Number + t.z.Number) / 2
	nodeIRef, err := t.managed.Node.BlockRefByNumber(nodeCtx, i)
	if err != nil {
		// if the block is not known to the node, it is defacto inconsistent
		if errors.Is(err, ethereum.NotFound) {
			t.managed.log.Trace("midpoint of range is not known to node. pulling back end of range", "i", i)
			t.z = eth.BlockID{Number: i}
			return nil
		} else {
			t.managed.log.Error("failed to get block at midpoint of range. cannot reset node", "err", err)
		}
	}

	// check if the block at i is consistent with the logs db
	// and update the search range accordingly
	nodeI := nodeIRef.ID()
	err = t.managed.backend.IsLocalUnsafe(internalCtx, t.managed.chainID, nodeI)
	if err != nil {
		t.managed.log.Trace("midpoint of range is inconsistent with logs db. pulling back end of range", "i", i)
		t.z = nodeI
	} else {
		t.managed.log.Trace("midpoint of range is consistent with logs db. pushing up start of range", "i", i)
		t.a = nodeI
	}
	return nil
}

// resetHeadsFromTarget takes a target block and identifies the correct
// unsafe, safe, and finalized blocks to target for the reset.
// It then triggers the reset on the node.
func (t *resetTracker) resetHeadsFromTarget(target eth.BlockID) {
	internalCtx, iCancel := context.WithTimeout(t.managed.ctx, internalTimeout)
	defer iCancel()

	// if the target is empty, no reset can be done
	if target == (eth.BlockID{}) {
		t.managed.log.Error("no reset target found. cannot reset node")
		t.endReset()
		return
	}

	t.managed.log.Info("reset target identified", "target", target)
	var lUnsafe, xUnsafe, lSafe, xSafe, finalized eth.BlockID

	// the unsafe block is always the last block we found to be consistent
	lUnsafe = target

	// all other blocks are either the last consistent block, or the last block in the db, whichever is earlier
	// cross unsafe
	lastXUnsafe, err := t.managed.backend.CrossUnsafe(internalCtx, t.managed.chainID)
	if err != nil {
		t.managed.log.Error("failed to get last cross unsafe block. cancelling reset", "err", err)
		t.endReset()
		return
	}
	if lastXUnsafe.Number < target.Number {
		xUnsafe = lastXUnsafe
	} else {
		xUnsafe = target
	}
	// local safe
	lastLSafe, err := t.managed.backend.LocalSafe(internalCtx, t.managed.chainID)
	if err != nil {
		t.managed.log.Error("failed to get last safe block. cancelling reset", "err", err)
		t.endReset()
		return
	}
	if lastLSafe.Derived.Number < target.Number {
		lSafe = lastLSafe.Derived
	} else {
		lSafe = target
	}
	// cross safe
	lastXSafe, err := t.managed.backend.CrossSafe(internalCtx, t.managed.chainID)
	if err != nil {
		t.managed.log.Error("failed to get last cross safe block. cancelling reset", "err", err)
		t.endReset()
		return
	}
	if lastXSafe.Derived.Number < target.Number {
		xSafe = lastXSafe.Derived
	} else {
		xSafe = target
	}
	// finalized
	lastFinalized, err := t.managed.backend.Finalized(internalCtx, t.managed.chainID)
	if errors.Is(err, types.ErrFuture) {
		t.managed.log.Warn("finalized block is not yet known", "err", err)
		lastFinalized = eth.BlockID{}
	} else if err != nil {
		t.managed.log.Error("failed to get last finalized block. cancelling reset", "err", err)
		t.endReset()
		return
	}
	if lastFinalized.Number < target.Number {
		finalized = lastFinalized
	} else {
		finalized = target
	}

	// trigger the reset
	t.managed.log.Info("triggering reset on node",
		"localUnsafe", lUnsafe,
		"crossUnsafe", xUnsafe,
		"localSafe", lSafe,
		"crossSafe", xSafe,
		"finalized", finalized)
	t.managed.OnResetReady(lUnsafe, xUnsafe, lSafe, xSafe, finalized)
}
