package syncnode

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// SyncNodesController manages a collection of active sync nodes.
// Sync nodes are used to sync the supervisor,
// and subject to the canonical chain view as followed by the supervisor.
type SyncNodesController struct {
	logger log.Logger

	controllers locks.RWMap[types.ChainID, *locks.RWMap[*ManagedNode, struct{}]]

	backend backend
	db      chainsDB

	depSet depset.DependencySet
}

// NewSyncNodesController creates a new SyncNodeController
func NewSyncNodesController(l log.Logger, depset depset.DependencySet, db chainsDB, backend backend) *SyncNodesController {
	return &SyncNodesController{
		logger:  l,
		depSet:  depset,
		db:      db,
		backend: backend,
	}
}

// AttachNodeController attaches a node to be managed by the supervisor.
// If noSubscribe, the node is not actively polled/subscribed to, and requires manual ManagedNode.PullEvents calls.
func (snc *SyncNodesController) AttachNodeController(id types.ChainID, ctrl SyncControl, noSubscribe bool) (Node, error) {
	if !snc.depSet.HasChain(id) {
		return nil, fmt.Errorf("chain %v not in dependency set: %w", id, types.ErrUnknownChain)
	}
	// lazy init the controllers map for this chain
	if !snc.controllers.Has(id) {
		snc.controllers.Set(id, &locks.RWMap[*ManagedNode, struct{}]{})
	}
	controllersForChain, _ := snc.controllers.Get(id)
	node := NewManagedNode(snc.logger, id, ctrl, snc.db, snc.backend, noSubscribe)
	controllersForChain.Set(node, struct{}{})
	snc.maybeInitSafeDB(id, ctrl)
	node.Start()
	return node, nil
}

// maybeInitSafeDB initializes the chain database if it is not already initialized
// it checks if the Local Safe database is empty, and loads it with the Anchor Point if so
func (snc *SyncNodesController) maybeInitSafeDB(id types.ChainID, ctrl SyncControl) {
	_, err := snc.db.LocalSafe(id)
	if errors.Is(err, types.ErrFuture) {
		snc.logger.Debug("initializing chain database", "chain", id)
		pair, err := ctrl.AnchorPoint(context.Background())
		if err != nil {
			snc.logger.Warn("failed to get anchor point", "chain", id, "error", err)
			return
		}
		if err := snc.db.UpdateCrossSafe(id, pair.DerivedFrom, pair.Derived); err != nil {
			snc.logger.Warn("failed to initialize cross safe", "chain", id, "error", err)
		}
		if err := snc.db.UpdateLocalSafe(id, pair.DerivedFrom, pair.Derived); err != nil {
			snc.logger.Warn("failed to initialize local safe", "chain", id, "error", err)
		}
		snc.logger.Debug("initialized chain database", "chain", id, "anchor", pair)
	} else {
		snc.logger.Debug("chain database already initialized", "chain", id)
	}
}
