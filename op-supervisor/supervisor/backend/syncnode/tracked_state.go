package syncnode

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// trackedState is used to track the state of most recent blocks
// from the node, in order to determine if the node is in sync with the logs db
type trackedState struct {
	lastNodeLocalUnsafe eth.BlockID
	lastNodeLocalSafe   eth.BlockID

	// signals for handling the recovery bisection and reset
	ongoingReset *ongoingReset
	cancelReset  bool
}

// ongoingReset represents a bisection,
// where a is the earliest in the range,
// and z is the latest in the range.
type ongoingReset struct {
	a eth.BlockID
	z eth.BlockID
}

func (m *ManagedNode) checkConsistencyWithDB() {
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()

	var z eth.BlockID

	// check if the last unsafe block we saw is consistent with the logs db
	err := m.backend.IsLocalUnsafe(ctx, m.chainID, m.lastState.lastNodeLocalUnsafe)
	if errors.Is(err, types.ErrConflict) {
		m.log.Warn("local unsafe block is inconsistent with logs db. Initiating recovery",
			"lastUnsafeblock", m.lastState.lastNodeLocalUnsafe,
			"err", err)
		z = m.lastState.lastNodeLocalUnsafe
	}

	// check if the last safe block we saw is consistent with the local safe db
	err = m.backend.IsLocalSafe(ctx, m.chainID, m.lastState.lastNodeLocalSafe)
	if errors.Is(err, types.ErrConflict) {
		m.log.Warn("local safe block is inconsistent with logs db. Initiating recovery",
			"lastSafeblock", m.lastState.lastNodeLocalSafe,
			"err", err)
		z = m.lastState.lastNodeLocalSafe
	}

	if z != (eth.BlockID{}) {
		m.lastState.cancelReset = false
		m.lastState.ongoingReset = &ongoingReset{a: eth.BlockID{}, z: z}
		m.prepareReset()
	}
}

// prepareReset identifies the heads needed for a node reset
// it manages the ongoingReset state until the reset is ready to be triggered
func (m *ManagedNode) prepareReset() {
	internalCtx, iCancel := context.WithTimeout(m.ctx, internalTimeout)
	defer iCancel()
	nodeCtx, nCancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer nCancel()
	for {
		if m.lastState.cancelReset {
			m.log.Info("reset was cancelled")
			return
		}
		if m.lastState.ongoingReset.a.Number >= m.lastState.ongoingReset.z.Number {
			m.log.Error("no recovery target found. cancelling reset",
				"a", m.lastState.ongoingReset.a,
				"z", m.lastState.ongoingReset.z)
			return
		}
		if m.lastState.ongoingReset.a.Number+1 == m.lastState.ongoingReset.z.Number {
			m.log.Info("reset is prepared", "target", m.lastState.ongoingReset.a)
			break
		}
		m.bisectRecoveryRange(internalCtx, nodeCtx)
	}

	// the bisection is now complete. a is the last consistent block, and z is the first inconsistent block
	target := m.lastState.ongoingReset.a
	var unsafe, safe, finalized eth.BlockID

	// the unsafe block is always the last block we found to be consistent
	unsafe = target

	// the safe block is either the last consistent block, or the last safe block, whichever is earlier
	lastSafe, err := m.backend.LocalSafe(internalCtx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last safe block. cancelling reset", "err", err)
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
		return
	}
	if lastFinalized.Number < target.Number {
		finalized = lastFinalized
	} else {
		finalized = target
	}

	// trigger the reset
	m.log.Info("triggering reset on node", "unsafe", unsafe, "safe", safe, "finalized", finalized)
	m.Node.Reset(nodeCtx, unsafe, safe, finalized)
}

// bisectRecoveryRange halves the search range for the recovery point,
// where the reset will target. It bisects the range and constrains either
// the start or the end of the range, based on the consistency of the midpoint
// with the logs db.
func (m *ManagedNode) bisectRecoveryRange(internalCtx, nodeCtx context.Context) error {
	if m.lastState.ongoingReset == nil {
		m.log.Error("can't progress bisection if there is no ongoing reset")
	}

	// get or initialize the range
	a := m.lastState.ongoingReset.a
	z := m.lastState.ongoingReset.z
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
		m.lastState.ongoingReset.z = nodeI
	} else {
		m.lastState.ongoingReset.a = nodeI
	}

	return nil
}
