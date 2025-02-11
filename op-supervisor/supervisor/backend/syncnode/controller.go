package syncnode

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// SyncNodesController manages a collection of active sync nodes.
// Sync nodes are used to sync the supervisor,
// and subject to the canonical chain view as followed by the supervisor.
type SyncNodesController struct {
	logger log.Logger

	id          atomic.Uint64
	controllers locks.RWMap[eth.ChainID, *locks.RWMap[*ManagedNode, struct{}]]

	eventSys event.System

	emitter event.Emitter

	backend backend

	depSet depset.DependencySet
}

var _ event.AttachEmitter = (*SyncNodesController)(nil)

// NewSyncNodesController creates a new SyncNodeController
func NewSyncNodesController(l log.Logger, depset depset.DependencySet, eventSys event.System, backend backend) *SyncNodesController {
	return &SyncNodesController{
		logger:   l,
		depSet:   depset,
		eventSys: eventSys,
		backend:  backend,
	}
}

func (snc *SyncNodesController) AttachEmitter(em event.Emitter) {
	snc.emitter = em
}

func (snc *SyncNodesController) OnEvent(ev event.Event) bool {
	return false
}

func (snc *SyncNodesController) Close() error {
	snc.controllers.Range(func(chainID eth.ChainID, controllers *locks.RWMap[*ManagedNode, struct{}]) bool {
		controllers.Range(func(node *ManagedNode, _ struct{}) bool {
			node.Close()
			return true
		})
		return true
	})
	return nil
}

// AttachNodeController attaches a node to be managed by the supervisor.
// If noSubscribe, the node is not actively polled/subscribed to, and requires manual ManagedNode.PullEvents calls.
func (snc *SyncNodesController) AttachNodeController(chainID eth.ChainID, ctrl SyncControl, noSubscribe bool) (Node, error) {
	if !snc.depSet.HasChain(chainID) {
		return nil, fmt.Errorf("chain %v not in dependency set: %w", chainID, types.ErrUnknownChain)
	}
	// lazy init the controllers map for this chain
	snc.controllers.Default(chainID, func() *locks.RWMap[*ManagedNode, struct{}] {
		return &locks.RWMap[*ManagedNode, struct{}]{}
	})
	controllersForChain, _ := snc.controllers.Get(chainID)

	nodeID := snc.id.Add(1)
	name := fmt.Sprintf("syncnode-%s-%d", chainID, nodeID)
	logger := snc.logger.New("syncnode", name, "endpoint", ctrl.String())

	logger.Info("Attaching node", "chain", chainID, "passive", noSubscribe)

	node := NewManagedNode(logger, chainID, ctrl, snc.backend, noSubscribe)
	snc.eventSys.Register(name, node, event.DefaultRegisterOpts())

	controllersForChain.Set(node, struct{}{})
	anchor, err := ctrl.AnchorPoint(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get anchor point: %w", err)
	}
	snc.emitter.Emit(superevents.AnchorEvent{
		ChainID: chainID,
		Anchor:  anchor,
	})
	node.Start()
	return node, nil
}
