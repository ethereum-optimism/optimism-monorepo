package status

import (
	"sync"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
)

type StatusTracker struct {
	statuses map[eth.ChainID]*eth.SupervisorChainStatus
	mu       sync.RWMutex
}

func NewStatusTracker() *StatusTracker {
	return &StatusTracker{
		statuses: make(map[eth.ChainID]*eth.SupervisorChainStatus),
	}
}

func (su *StatusTracker) OnEvent(ev event.Event) bool {
	su.mu.Lock()
	defer su.mu.Unlock()

	loadStatus := func(chainID eth.ChainID) *eth.SupervisorChainStatus {
		v := su.statuses[chainID]
		if v == nil {
			v = &eth.SupervisorChainStatus{}
		}
		return v
	}
	switch x := ev.(type) {
	case superevents.LocalDerivedEvent:
		v := loadStatus(x.ChainID)
		v.LocalDerived = x.Derived.Derived
		v.LocalDerivedFrom = x.Derived.DerivedFrom
		su.statuses[x.ChainID] = v
	case superevents.LocalUnsafeUpdateEvent:
		v := loadStatus(x.ChainID)
		v.LocalUnsafe = x.NewLocalUnsafe
		su.statuses[x.ChainID] = v
	default:
		return false
	}
	return true
}

func (su *StatusTracker) SyncStatus() map[eth.ChainID]*eth.SupervisorChainStatus {
	su.mu.RLock()
	defer su.mu.RUnlock()
	ret := make(map[eth.ChainID]*eth.SupervisorChainStatus)
	for chainID, status := range su.statuses {
		clone := new(eth.SupervisorChainStatus)
		*clone = *status
		ret[chainID] = clone
	}
	return ret
}
