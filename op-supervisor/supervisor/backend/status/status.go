package status

import (
	"sync"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
)

type StatusTracker struct {
	statuses map[eth.ChainID]*NodeSyncStatus
	mu       sync.RWMutex
}

type NodeSyncStatus struct {
	CurrentL1   eth.L1BlockRef
	LocalUnsafe eth.BlockRef
}

func NewStatusTracker() *StatusTracker {
	return &StatusTracker{
		statuses: make(map[eth.ChainID]*NodeSyncStatus),
	}
}

func (su *StatusTracker) OnEvent(ev event.Event) bool {
	su.mu.Lock()
	defer su.mu.Unlock()

	loadStatusRef := func(chainID eth.ChainID) *NodeSyncStatus {
		v := su.statuses[chainID]
		if v == nil {
			v = &NodeSyncStatus{}
			su.statuses[chainID] = v
		}
		return v
	}
	switch x := ev.(type) {
	case superevents.LocalDerivedOriginUpdateEvent:
		status := loadStatusRef(x.ChainID)
		status.CurrentL1 = x.Origin
	case superevents.LocalUnsafeUpdateEvent:
		status := loadStatusRef(x.ChainID)
		status.LocalUnsafe = x.NewLocalUnsafe
	default:
		return false
	}
	return true
}

func (su *StatusTracker) SyncStatus() eth.SupervisorSyncStatus {
	su.mu.RLock()
	defer su.mu.RUnlock()

	var supervisorStatus eth.SupervisorSyncStatus
	// to collect the min synced L1, we need to iterate over all nodes
	// and compare the current L1 block they each reported.
	for _, nodeStatus := range su.statuses {
		// if the min synced L1 is not set, or the node's current L1 is lower than the min synced L1,
		if supervisorStatus.MinSyncedL1 == (eth.L1BlockRef{}) || supervisorStatus.MinSyncedL1.Number > nodeStatus.CurrentL1.Number {
			supervisorStatus.MinSyncedL1 = nodeStatus.CurrentL1
		}
		// if the height is equal, we need to compare the hash
		if supervisorStatus.MinSyncedL1.Number == nodeStatus.CurrentL1.Number &&
			supervisorStatus.MinSyncedL1.Hash != nodeStatus.CurrentL1.Hash {
			// if the hashes are not equal, return an empty status
			return eth.SupervisorSyncStatus{}
		}
		// if the node's current L1 is higher than the min synced L1, we can skip it,
		// because we already know a different node isn't synced to it yet
	}
	supervisorStatus.Chains = make(map[eth.ChainID]*eth.SupervisorChainSyncStatus)
	for chainID, nodeStatus := range su.statuses {
		supervisorStatus.Chains[chainID] = &eth.SupervisorChainSyncStatus{
			LocalUnsafe: nodeStatus.LocalUnsafe,
		}
	}
	return supervisorStatus
}
