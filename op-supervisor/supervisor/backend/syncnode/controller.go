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
	SubscribeCrossSafe(chainID types.ChainID, c chan<- types.DerivedPair) error
	SubscribeFinalized(chainID types.ChainID, c chan<- eth.BlockID) error
}

type ManagedNode struct {
	log                         log.Logger
	Node                        SyncControl
	chainID                     types.ChainID
	CrossSafeUpdateSubscription chan types.DerivedPair
	FinalizedUpdateSubscription chan eth.BlockID
	cancel                      chan struct{}
}

func NewManagedNode(log log.Logger, id types.ChainID, node SyncControl, db chainsDB) *ManagedNode {
	m := &ManagedNode{
		log:     log,
		Node:    node,
		chainID: id,
		cancel:  make(chan struct{}),
	}
	m.SubscribeToDBEvents(id, db)
	return m
}

func (m *ManagedNode) SubscribeToDBEvents(id types.ChainID, db chainsDB) {
	m.CrossSafeUpdateSubscription = make(chan types.DerivedPair, 10)
	m.FinalizedUpdateSubscription = make(chan eth.BlockID, 10)
	db.SubscribeCrossSafe(id, m.CrossSafeUpdateSubscription)
	db.SubscribeFinalized(id, m.FinalizedUpdateSubscription)
}

func (m *ManagedNode) Start() {
	go func() {
		for {
			select {
			case <-m.cancel:
				return
			case pair := <-m.CrossSafeUpdateSubscription:
				m.log.Debug("updating cross safe", "chain", m.chainID, "derived", pair.Derived, "derivedFrom", pair.DerivedFrom)
				m.Node.UpdateCrossSafe(context.Background(), pair.Derived.ID(), pair.DerivedFrom.ID())
			case id := <-m.FinalizedUpdateSubscription:
				m.log.Debug("updating finalized", "chain", m.chainID, "id", id)
				m.Node.UpdateFinalized(context.Background(), id)
			}
		}
	}()
}

func (m *ManagedNode) Close() error {
	m.cancel <- struct{}{}
	return nil
}

// SyncNodesController manages a collection of active sync nodes.
// Sync nodes are used to sync the supervisor,
// and subject to the canonical chain view as followed by the supervisor.
type SyncNodesController struct {
	logger log.Logger

	controllers locks.RWMap[types.ChainID, locks.RWMap[*ManagedNode, struct{}]]

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
	// lazy init the controllers map for this chain
	if !snc.controllers.Has(id) {
		snc.controllers.Set(id, locks.RWMap[*ManagedNode, struct{}]{})
	}
	controllersForChain, _ := snc.controllers.Get(id)
	node := NewManagedNode(snc.logger, id, ctrl, snc.db)
	controllersForChain.Set(node, struct{}{})
	snc.maybeInitSafeDB(id, ctrl)
	node.Start()
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
