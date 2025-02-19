package syncnode

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// consistencyState is used to track the state of most recent blocks
// given by this node and supervisor. It is used to detect inconsistencies
// to trigger a recovery.
type consistencyState struct {
	lastSupervisorUnsafe    eth.BlockID
	lastSupervisorLocalSafe eth.BlockID

	lastNodeLocalUnsafe eth.BlockID
	lastNodeLocalSafe   eth.BlockID

	resetRangeStart eth.BlockID
	resetRangeEnd   eth.BlockID
}

func (m *ManagedNode) checkConsistencyState() {
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()

	// check if the last unsafe block we saw is consistent with the logs db
	err := m.backend.IsLocalUnsafe(ctx, m.chainID, m.lastState.lastNodeLocalUnsafe)
	if errors.Is(err, types.ErrConflict) {
		m.log.Warn("local unsafe block is inconsistent with logs db. Initiating recovery",
			"lastUnsafeblock", m.lastState.lastNodeLocalUnsafe,
			"err", err)
	}

	// check if the last safe block we saw is consistent with the local safe db
	err = m.backend.IsLocalSafe(ctx, m.chainID, m.lastState.lastNodeLocalSafe)
	if errors.Is(err, types.ErrConflict) {
		m.log.Warn("local safe block is inconsistent with logs db. Initiating recovery",
			"lastSafeblock", m.lastState.lastNodeLocalSafe,
			"err", err)
	}

	// if either of these checks fail, we need to trigger a recovery
	// recovery could range from 0 to the block which is inconsistent
	// we can bisect the range and keep track of a job which identifies the recovery point
	// it would not be good form to attempt a full bisecion search synchronously here.
	// maybe we can make an async job which decides the recovery point and then triggers the recovery itself
}
