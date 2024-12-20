package syncnode

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	gethevent "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type chainsDB interface {
	LocalSafe(chainID types.ChainID) (derivedFrom types.BlockSeal, derived types.BlockSeal, err error)
	UpdateLocalSafe(chainID types.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) error
	UpdateCrossSafe(chainID types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error
	SubscribeCrossSafe(chainID types.ChainID, c chan<- types.DerivedPair) (gethevent.Subscription, error)
	SubscribeFinalized(chainID types.ChainID, c chan<- eth.BlockID) (gethevent.Subscription, error)
}

type backend interface {
	UpdateLocalSafe(ctx context.Context, chainID types.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) error
	UpdateLocalUnsafe(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error
	LocalSafe(ctx context.Context, chainID types.ChainID) (derivedFrom eth.BlockID, derived eth.BlockID, err error)
	LatestUnsafe(ctx context.Context, chainID types.ChainID) (eth.BlockID, error)
	SafeDerivedAt(ctx context.Context, chainID types.ChainID, derivedFrom eth.BlockID) (derived eth.BlockID, err error)
	Finalized(ctx context.Context, chainID types.ChainID) (eth.BlockID, error)
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
}

const (
	internalTimeout = time.Second * 30
	nodeTimeout     = time.Second * 10
)

type ManagedNode struct {
	log     log.Logger
	Node    SyncControl
	chainID types.ChainID

	backend backend

	// when the supervisor has a cross-safe update for the node
	crossSafeUpdateChan chan types.DerivedPair
	// when the supervisor has a finality update for the node
	finalizedUpdateChan chan eth.BlockID

	// when the node says a reset is necessary, on any sync inconsistency
	resetEventsChan chan string
	// new L2 blocks from the node
	unsafeBlocks chan eth.BlockRef
	// new local-safe L2 blocks from the node
	derivationUpdates chan types.DerivedPair
	// when the node needs new L1 blocks
	exhaustL1Events chan types.DerivedPair

	subscriptions []gethevent.Subscription

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewManagedNode(log log.Logger, id types.ChainID, node SyncControl, db chainsDB, backend backend) *ManagedNode {
	ctx, cancel := context.WithCancel(context.Background())
	m := &ManagedNode{
		log:     log.New("chain", id),
		backend: backend,
		Node:    node,
		chainID: id,
		ctx:     ctx,
		cancel:  cancel,
	}
	m.SubscribeToDBEvents(db)
	m.SubscribeToNodeEvents()
	return m
}

func (m *ManagedNode) SubscribeToDBEvents(db chainsDB) {
	m.crossSafeUpdateChan = make(chan types.DerivedPair, 100)
	m.finalizedUpdateChan = make(chan eth.BlockID, 100)
	sub, err := db.SubscribeCrossSafe(m.chainID, m.crossSafeUpdateChan)
	if err != nil {
		m.log.Warn("failed to subscribe to cross safe", "err", err)
	} else {
		m.subscriptions = append(m.subscriptions, sub)
	}
	if err != nil {
		m.log.Warn("failed to subscribe to finalized", "err", err)
	} else {
		m.subscriptions = append(m.subscriptions, sub)
	}
}

func (m *ManagedNode) SubscribeToNodeEvents() {
	m.unsafeBlocks = make(chan eth.BlockRef)
	m.derivationUpdates = make(chan types.DerivedPair)
	m.exhaustL1Events = make(chan types.DerivedPair)

	// For each of these, we want to resubscribe on error. Since the RPC subscription might fail intermittently.
	m.subscriptions = append(m.subscriptions, gethevent.ResubscribeErr(time.Second*10,
		func(ctx context.Context, err error) (gethevent.Subscription, error) {
			return m.Node.SubscribeResetEvents(ctx, m.resetEventsChan)
		}))
	m.subscriptions = append(m.subscriptions, gethevent.ResubscribeErr(time.Second*10,
		func(ctx context.Context, err error) (gethevent.Subscription, error) {
			return m.Node.SubscribeUnsafeBlocks(ctx, m.unsafeBlocks)
		}))
	m.subscriptions = append(m.subscriptions, gethevent.ResubscribeErr(time.Second*10,
		func(ctx context.Context, err error) (gethevent.Subscription, error) {
			return m.Node.SubscribeDerivationUpdates(ctx, m.derivationUpdates)
		}))
	m.subscriptions = append(m.subscriptions, gethevent.ResubscribeErr(time.Second*10,
		func(ctx context.Context, err error) (gethevent.Subscription, error) {
			return m.Node.SubscribeExhaustL1Events(ctx, m.exhaustL1Events)
		}))
}

func (m *ManagedNode) Start() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		for {
			select {
			case <-m.ctx.Done():
				m.log.Info("Exiting node syncing")
				return
			case errStr := <-m.resetEventsChan:
				m.onResetEvent(errStr)
			case pair := <-m.crossSafeUpdateChan:
				m.onCrossSafeUpdate(pair)
			case id := <-m.finalizedUpdateChan:
				m.onFinalizedL1(id)
			case unsafeRef := <-m.unsafeBlocks:
				m.onUnsafeBlock(unsafeRef)
			case pair := <-m.derivationUpdates:
				m.onDerivationUpdate(pair)
			case completed := <-m.exhaustL1Events:
				m.onExhaustL1Event(completed)
			}
		}
	}()
}

func (m *ManagedNode) onResetEvent(errStr string) {
	m.log.Warn("Node sent us a reset error", "err", errStr)
	// Try and restore the safe head of the op-supervisor.
	// The node will abort the reset until we find a block that is known.
	m.resetSignal(types.ErrFuture, eth.L1BlockRef{})
}

func (m *ManagedNode) onCrossSafeUpdate(pair types.DerivedPair) {
	m.log.Debug("updating cross safe", "derived", pair.Derived, "derivedFrom", pair.DerivedFrom)
	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	err := m.Node.UpdateCrossSafe(ctx, pair.Derived.ID(), pair.DerivedFrom.ID())
	if err != nil {
		m.log.Warn("Node failed cross-safe updating", "err", err)
	}
}

func (m *ManagedNode) onFinalizedL1(id eth.BlockID) {
	m.log.Debug("updating finalized", "finalized", id)
	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	err := m.Node.UpdateFinalized(ctx, id)
	if err != nil {
		m.log.Warn("Node failed finality updating", "err", err)
	}
}

func (m *ManagedNode) onUnsafeBlock(unsafeRef eth.BlockRef) {
	m.log.Info("Node has new unsafe block", "unsafeBlock", unsafeRef)
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()
	if err := m.backend.UpdateLocalUnsafe(ctx, m.chainID, unsafeRef); err != nil {
		m.log.Warn("Backend failed to pick up on new unsafe block", "unsafeBlock", unsafeRef, "err", err)
		// TODO: if conflict error -> send reset to drop
		// TODO: if future error -> send reset to rewind
		// TODO: if out of order -> warn, just old data
	}
}

func (m *ManagedNode) onDerivationUpdate(pair types.DerivedPair) {
	m.log.Info("Node derived new block", "derived", pair.Derived, "derivedFrom", pair.DerivedFrom)
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()
	if err := m.backend.UpdateLocalSafe(ctx, m.chainID, pair.DerivedFrom, pair.Derived); err != nil {
		m.log.Warn("Backend failed to process local-safe update",
			"derived", pair.Derived, "derivedFrom", pair.DerivedFrom, "err", err)
		m.resetSignal(err, pair.DerivedFrom)
	}
}

func (m *ManagedNode) resetSignal(errSignal error, l1Ref eth.BlockRef) {
	// if conflict error -> send reset to drop
	// if future error -> send reset to rewind
	// if out of order -> warn, just old data
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()
	u, err := m.backend.LatestUnsafe(ctx, m.chainID)
	if err != nil {
		m.log.Warn("Node failed to reset", "err", err)
	}
	f, err := m.backend.Finalized(ctx, m.chainID)
	if err != nil {
		m.log.Warn("Node failed to reset", "err", err)
	}

	// fix finalized to point to a L2 block that the L2 node knows about
	// Conceptually: track the last known block by the node (based on unsafe block updates), as upper bound for resets.
	// Then when reset fails, lower the last known block
	// (and prevent it from changing by subscription, until success with reset), and rinse and repeat.

	// TODO: errors.As switch
	switch errSignal {
	case types.ErrConflict:
		s, err := m.backend.SafeDerivedAt(ctx, m.chainID, l1Ref.ID())
		if err != nil {
			m.log.Warn("Node failed to reset", "err", err)
		}
		log.Debug("Node detected conflict, resetting", "unsafe", u, "safe", s, "finalized", f)
		err = m.Node.Reset(ctx, u, s, f)
		if err != nil {
			m.log.Warn("Node failed to reset", "err", err)
		}
	case types.ErrFuture:
		_, s, err := m.backend.LocalSafe(ctx, m.chainID)
		if err != nil {
			m.log.Warn("Node failed to reset", "err", err)
		}
		log.Debug("Node detected future block, resetting", "unsafe", u, "safe", s, "finalized", f)
		err = m.Node.Reset(ctx, u, s, f)
		if err != nil {
			m.log.Warn("Node failed to reset", "err", err)
		}
	case types.ErrOutOfOrder:
		m.log.Warn("Node detected out of order block", "unsafe", u, "finalized", f)
	}
}

func (m *ManagedNode) onExhaustL1Event(completed types.DerivedPair) {
	m.log.Info("Node completed syncing", "l2", completed.Derived, "l1", completed.DerivedFrom)

	internalCtx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()
	nextL1, err := m.backend.L1BlockRefByNumber(internalCtx, completed.DerivedFrom.Number+1)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			m.log.Debug("Next L1 block is not yet available", "l1Block", completed.DerivedFrom, "err", err)
			return
		}
		m.log.Error("Failed to retrieve next L1 block for node", "l1Block", completed.DerivedFrom, "err", err)
		return
	}

	nodeCtx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	if err := m.Node.ProvideL1(nodeCtx, nextL1); err != nil {
		m.log.Warn("Failed to provide next L1 block to node", "err", err)
		// We will reset the node if we receive a reset-event from it,
		// which is fired if the provided L1 block was received successfully,
		// but does not fit on the derivation state.
		return
	}
}

func (m *ManagedNode) Close() error {
	m.cancel()
	m.wg.Wait() // wait for work to complete

	// Now close all subscriptions, since we don't use them anymore.
	for _, sub := range m.subscriptions {
		sub.Unsubscribe()
	}
	return nil
}

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

func (snc *SyncNodesController) AttachNodeController(id types.ChainID, ctrl SyncControl) error {
	if !snc.depSet.HasChain(id) {
		return fmt.Errorf("chain %v not in dependency set", id)
	}
	// lazy init the controllers map for this chain
	if !snc.controllers.Has(id) {
		snc.controllers.Set(id, &locks.RWMap[*ManagedNode, struct{}]{})
	}
	controllersForChain, _ := snc.controllers.Get(id)
	node := NewManagedNode(snc.logger, id, ctrl, snc.db, snc.backend)
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
