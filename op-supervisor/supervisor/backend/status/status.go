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
		status.CurrentL1 = x.Derived.DerivedFrom
	case superevents.LocalUnsafeUpdateEvent:
		status := loadStatusRef(x.ChainID)
		status.LocalUnsafe = x.NewLocalUnsafe
	default:
		return false
	}
	return true
}

func (su *StatusTracker) SyncStatus() eth.SupervisorStatus {
	su.mu.RLock()
	defer su.mu.RUnlock()

	var supervisorStatus eth.SupervisorStatus
	for _, nodeStatus := range su.statuses {
		if supervisorStatus.MinSyncedL1 == (eth.L1BlockRef{}) || supervisorStatus.MinSyncedL1.Number < nodeStatus.CurrentL1.Number {
			supervisorStatus.MinSyncedL1 = nodeStatus.CurrentL1
		}
	}
	supervisorStatus.Chains = make(map[eth.ChainID]*eth.SupervisorChainStatus)
	for chainID, nodeStatus := range su.statuses {
		supervisorStatus.Chains[chainID] = &eth.SupervisorChainStatus{
			LocalUnsafe: nodeStatus.LocalUnsafe,
		}
	}
	return supervisorStatus
}
