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

func (m *ManagedNode) checkConsistencyWithDB() {
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

	// there is inconsistency. initiate reset
	if z != (eth.BlockID{}) {
		m.resetting.Store(true)
		m.cancelReset.Store(false)
		m.ongoingReset = &ongoingReset{a: eth.BlockID{}, z: z}
		// trigger the reset asynchronously
		go m.prepareReset()
	} else {
		m.log.Debug("no inconsistency found")
		m.resetting.Store(false)
		m.ongoingReset = nil
	}
}

// prepareReset prepares the reset by bisecting the the search range
// until the last consistent block is found. It then identifies the correct
// unsafe, safe, and finalized blocks to target for the reset.
func (m *ManagedNode) prepareReset() {
	internalCtx, iCancel := context.WithTimeout(m.ctx, internalTimeout)
	defer iCancel()
	nodeCtx, nCancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer nCancel()
	// repeatedly bisect the range until the last consistent block is found
	for {
		if m.cancelReset.Load() {
			m.log.Info("reset cancelled")
			m.resetting.Store(false)
			m.ongoingReset = nil
			return
		}
		if m.ongoingReset.a.Number >= m.ongoingReset.z.Number {
			m.log.Error("no reset target found. restarting reset",
				"a", m.ongoingReset.a,
				"z", m.ongoingReset.z)
			// check consistency again once this function returns
			// the reset failed, and may need to be retried
			defer m.checkConsistencyWithDB()
			return
		}
		if m.ongoingReset.a.Number+1 == m.ongoingReset.z.Number {
			break
		}
		m.bisectRecoveryRange(internalCtx, nodeCtx)
	}

	// the bisection is now complete. a is the last consistent block, and z is the first inconsistent block
	target := m.ongoingReset.a
	m.log.Info("reset point has been found", "target", target)
	var unsafe, safe, finalized eth.BlockID

	// the unsafe block is always the last block we found to be consistent
	unsafe = target

	// the safe block is either the last consistent block, or the last safe block, whichever is earlier
	lastSafe, err := m.backend.LocalSafe(internalCtx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last safe block. cancelling reset", "err", err)
		defer m.checkConsistencyWithDB()
		return
	}
	if lastSafe.Derived.Number < target.Number {
		safe = lastSafe.Derived
	} else {
		safe = target
	}

	// the finalized block is also either the last consistent block, or the last finalized block, whichever is earlier
	lastFinalized, err := m.backend.Finalized(internalCtx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last finalized block. cancelling reset", "err", err)
		defer m.checkConsistencyWithDB()
		return
	}
	if lastFinalized.Number < target.Number {
		finalized = lastFinalized
	} else {
		finalized = target
	}

	// trigger the reset
	m.log.Info("triggering reset on node", "unsafe", unsafe, "safe", safe, "finalized", finalized)
	m.OnResetReady(unsafe, safe, finalized)
}

// bisectRecoveryRange halves the search range for the reset point,
// where the reset will target. It bisects the range and constrains either
// the start or the end of the range, based on the consistency of the midpoint
// with the logs db.
func (m *ManagedNode) bisectRecoveryRange(internalCtx, nodeCtx context.Context) error {
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
