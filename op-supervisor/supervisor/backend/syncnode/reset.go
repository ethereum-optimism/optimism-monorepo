package syncnode

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// ongoingReset represents a bisection,
// where a is the earliest in the range,
// and z is the latest in the range.
// when a and z are directly adjacent, the
// bisection is complete. a is the last consistent
// block, and z is the first inconsistent block.
type ongoingReset struct {
	a eth.BlockID
	z eth.BlockID
}

func (m *ManagedNode) resetIfAhead() {
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()

	// get the last local safe block
	lastDBLocalSafe, err := m.backend.LocalSafe(ctx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last local safe block", "err", err)
		return
	}
	// if the node is ahead of the logs db, initiate a reset
	// with the end of the range being the last safe block in the db
	if m.lastNodeLocalSafe.Number > lastDBLocalSafe.Derived.Number {
		m.log.Warn("local safe block on node is ahead of logs db. Initiating reset",
			"lastNodeLocalSafe", m.lastNodeLocalSafe,
			"lastDBLocalSafe", lastDBLocalSafe.Derived)
		m.ongoingReset = &ongoingReset{z: lastDBLocalSafe.Derived}
		m.beginReset()
	}
}

func (m *ManagedNode) resetIfInconsistent() {
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()

	var z eth.BlockID

	// check if the last unsafe block we saw is consistent with the logs db
	err := m.backend.IsLocalUnsafe(ctx, m.chainID, m.lastNodeLocalUnsafe)
	if errors.Is(err, types.ErrConflict) {
		m.log.Warn("local unsafe block is inconsistent with logs db. Initiating reset",
			"lastUnsafeblock", m.lastNodeLocalUnsafe,
			"err", err)
		z = m.lastNodeLocalUnsafe
	}

	// check if the last safe block we saw is consistent with the local safe db
	err = m.backend.IsLocalSafe(ctx, m.chainID, m.lastNodeLocalSafe)
	if errors.Is(err, types.ErrConflict) {
		m.log.Warn("local safe block is inconsistent with logs db. Initiating reset",
			"lastSafeblock", m.lastNodeLocalSafe,
			"err", err)
		z = m.lastNodeLocalSafe
	}

	// there is inconsistency. begin the reset process
	if z != (eth.BlockID{}) {
		m.ongoingReset = &ongoingReset{z: z}
		m.beginReset()
	} else {
		m.log.Debug("no inconsistency found")
	}
}

func (m *ManagedNode) resetIfNotAlready() {
	if m.resetting.Load() {
		return
	}
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()
	last, err := m.backend.LocalUnsafe(ctx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last local unsafe block", "err", err)
		return
	}
	m.ongoingReset = &ongoingReset{z: last}
	m.resetting.Store(true)

	// action tests may prefer to run the managed node totally synchronously
	if m.syncReset {
		m.prepareResetRequest()
	} else {
		go m.prepareResetRequest()
	}
}

func (m *ManagedNode) beginReset() {
	m.resetting.Store(true)
	m.cancelReset.Store(false)
	// action tests may prefer to run the managed node totally synchronously
	if m.syncReset {
		m.prepareResetRequest()
	} else {
		go m.prepareResetRequest()
	}
}

func (m *ManagedNode) endReset() {
	m.resetting.Store(false)
	m.cancelReset.Store(false)
	m.ongoingReset = nil
}

// prepareResetRequest prepares the reset by bisecting the search range
// until the last consistent block is found. It then identifies the correct
// unsafe, safe, and finalized blocks to target for the reset.
func (m *ManagedNode) prepareResetRequest() {
	nodeCtx, nCancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer nCancel()

	// before starting bisection, check if z is already consistent (i.e. the node is ahead but otherwise consistent)
	nodeZ, err := m.Node.BlockRefByNumber(nodeCtx, m.ongoingReset.z.Number)
	if err != nil {
		m.log.Error("failed to get block at end of range. cannot reset node", "err", err)
		defer m.endReset()
		return
	}
	// if z is already consistent, we can skip the bisection
	if nodeZ.ID() == m.ongoingReset.z {
		m.resetHeadsFromTarget(m.ongoingReset.z)
		return
	}

	// before starting bisection, check if a is inconsistent (i.e. the node has no common reference point)
	nodeA, err := m.Node.BlockRefByNumber(nodeCtx, m.ongoingReset.a.Number)
	if err != nil {
		m.log.Error("failed to get block at start of range. cannot reset node", "err", err)
		defer m.endReset()
		return
	}
	if nodeA.ID() != m.ongoingReset.a {
		m.log.Error("start of range is inconsistent with logs db. cannot reset node",
			"a", m.ongoingReset.a,
			"block", nodeA.ID())
		defer m.endReset()
		return
	}

	// repeatedly bisect the range until the last consistent block is found
	for {
		if m.cancelReset.Load() {
			m.log.Debug("reset cancelled")
			defer m.endReset()
			return
		}
		if m.ongoingReset.a.Number >= m.ongoingReset.z.Number {
			m.log.Error("no reset target found. cannot reset node",
				"a", m.ongoingReset.a,
				"z", m.ongoingReset.z)
			defer m.endReset()
			return
		}
		if m.ongoingReset.a.Number+1 == m.ongoingReset.z.Number {
			break
		}
		err := m.bisectReset()
		if err != nil {
			m.log.Error("failed to bisect recovery range. cannot reset node", "err", err)
			defer m.endReset()
			return
		}
	}
	// the bisection is now complete. a is the last consistent block, and z is the first inconsistent block
	m.resetHeadsFromTarget(m.ongoingReset.a)
}

// resetHeadsFromTarget takes a target block and identifies the correct
// unsafe, safe, and finalized blocks to target for the reset.
// It then triggers the reset on the node.
func (m *ManagedNode) resetHeadsFromTarget(target eth.BlockID) {
	internalCtx, iCancel := context.WithTimeout(m.ctx, internalTimeout)
	defer iCancel()

	// if the target is empty, no reset can be done
	if target == (eth.BlockID{}) {
		m.log.Error("no reset target found. cannot reset node")
		defer m.endReset()
		return
	}

	m.log.Info("reset target identified", "target", target)
	var lUnsafe, xUnsafe, lSafe, xSafe, finalized eth.BlockID

	// the unsafe block is always the last block we found to be consistent
	lUnsafe = target

	// all other blocks are either the last consistent block, or the last block in the db, whichever is earlier
	// cross unsafe
	lastXUnsafe, err := m.backend.CrossUnsafe(internalCtx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last cross unsafe block. cancelling reset", "err", err)
		defer m.endReset()
		return
	}
	if lastXUnsafe.Number < target.Number {
		xUnsafe = lastXUnsafe
	} else {
		xUnsafe = target
	}
	// local safe
	lastLSafe, err := m.backend.LocalSafe(internalCtx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last safe block. cancelling reset", "err", err)
		defer m.endReset()
		return
	}
	if lastLSafe.Derived.Number < target.Number {
		lSafe = lastLSafe.Derived
	} else {
		lSafe = target
	}
	// cross safe
	lastXSafe, err := m.backend.CrossSafe(internalCtx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last cross safe block. cancelling reset", "err", err)
		defer m.endReset()
		return
	}
	if lastXSafe.Derived.Number < target.Number {
		xSafe = lastXSafe.Derived
	} else {
		xSafe = target
	}
	// finalized
	lastFinalized, err := m.backend.Finalized(internalCtx, m.chainID)
	if errors.Is(err, types.ErrFuture) {
		m.log.Warn("finalized block is not yet known", "err", err)
		lastFinalized = eth.BlockID{}
	} else if err != nil {
		m.log.Error("failed to get last finalized block. cancelling reset", "err", err)
		defer m.endReset()
		return
	}
	if lastFinalized.Number < target.Number {
		finalized = lastFinalized
	} else {
		finalized = target
	}

	// trigger the reset
	m.log.Info("triggering reset on node",
		"localUnsafe", lUnsafe,
		"crossUnsafe", xUnsafe,
		"localSafe", lSafe,
		"crossSafe", xSafe,
		"finalized", finalized)
	m.OnResetReady(lUnsafe, xUnsafe, lSafe, xSafe, finalized)
}

// bisectReset halves the search range of the ongoing reset to narrow down
// where the reset will target. It bisects the range and constrains either
// the start or the end of the range, based on the consistency of the midpoint
// with the logs db.
func (m *ManagedNode) bisectReset() error {
	internalCtx, iCancel := context.WithTimeout(m.ctx, internalTimeout)
	defer iCancel()
	nodeCtx, nCancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer nCancel()
	if m.ongoingReset == nil {
		m.log.Error("can't progress bisection if there is no ongoing reset")
	}

	// get or initialize the range
	a := m.ongoingReset.a
	z := m.ongoingReset.z
	if a == (eth.BlockID{}) {
		m.log.Debug("start of range is empty, finding the first block")
		var err error
		a, err = m.backend.FindSealedBlock(internalCtx, m.chainID, 0)
		if err != nil {
			return err
		}
	}
	if z == (eth.BlockID{}) {
		m.log.Debug("end of range is empty, finding the last block")
		var err error
		z, err = m.backend.LocalUnsafe(internalCtx, m.chainID)
		if err != nil {
			return err
		}
	}

	// i is the midpoint between a and z
	i := (a.Number + z.Number) / 2

	// get the block at i
	nodeIRef, err := m.Node.BlockRefByNumber(nodeCtx, i)
	if err != nil {
		return err
	}
	nodeI := nodeIRef.ID()

	// check if the block at i is consistent with the logs db
	// and update the search range accordingly
	err = m.backend.IsLocalUnsafe(internalCtx, m.chainID, nodeI)
	if errors.Is(err, types.ErrConflict) {
		m.ongoingReset.z = nodeI
	} else if err != nil {
		m.log.Warn("failed to check consistency with logs db", "block", nodeI, "err", err)
		return err
	} else {
		m.ongoingReset.a = nodeI
	}

	return nil
}
