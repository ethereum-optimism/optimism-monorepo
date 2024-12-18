package syncnode

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
)

type chainsDB interface {
	LocalSafe(types.ChainID) (types.BlockSeal, types.BlockSeal, error)
	UpdateLocalSafe(types.ChainID, eth.BlockRef, eth.BlockRef) error
	UpdateCrossSafe(types.ChainID, eth.BlockRef, eth.BlockRef) error
}

type ManagedNode struct {
	Node SyncControl
	// TODO active subscriptions
}

func (m *ManagedNode) Close() error {
	return nil
}

// SyncNodesController manages a collection of active sync nodes.
// Sync nodes are used to sync the supervisor,
// and subject to the canonical chain view as followed by the supervisor.
type SyncNodesController struct {
	logger log.Logger

	controllers locks.RWMap[*ManagedNode, struct{}]

	db chainsDB

	depSet depset.DependencySet
}

// NewSyncNodesController creates a new SyncNodeController
func NewSyncNodesController(l log.Logger, depset depset.DependencySet, db chainsDB) *SyncNodesController {
	return &SyncNodesController{
		logger: l,
		depSet: depset,
		db:     db,
	}
}

func (snc *SyncNodesController) AttachNodeController(id types.ChainID, ctrl SyncControl) error {
	if !snc.depSet.HasChain(id) {
		return fmt.Errorf("chain %v not in dependency set", id)
	}
	snc.controllers.Set(&ManagedNode{Node: ctrl}, struct{}{})
	snc.maybeInitSafeDB(id, ctrl)
	return nil
}

// maybeInitSafeDB initializes the chain database if it is not already initialized
// it checks if the Local Safe database is empty, and loads it with the Anchor Point if so
func (snc *SyncNodesController) maybeInitSafeDB(id types.ChainID, ctrl SyncControl) {
	_, _, err := snc.db.LocalSafe(id)
	if errors.Is(err, types.ErrFuture) {
		snc.logger.Debug("initializing chain database", "chain", id)
		pair, err := ctrl.AnchorPoint(context.Background())
		if err != nil {
			snc.logger.Warn("failed to get anchor point", "chain", id, "error", err)
			return
		}
		if err := snc.db.UpdateCrossSafe(id, pair.Derived, pair.Derived); err != nil {
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
