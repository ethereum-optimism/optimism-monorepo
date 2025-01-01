package syncnode

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	gethevent "github.com/ethereum/go-ethereum/event"
)

type chainsDB interface {
	LocalSafe(chainID types.ChainID) (types.DerivedBlockSealPair, error)
	UpdateLocalSafe(chainID types.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) error
	UpdateCrossSafe(chainID types.ChainID, l1View eth.BlockRef, lastCrossDerived eth.BlockRef) error
	SubscribeCrossUnsafe(chainID types.ChainID, c chan<- types.BlockSeal) (gethevent.Subscription, error)
	SubscribeCrossSafe(chainID types.ChainID, c chan<- types.DerivedBlockSealPair) (gethevent.Subscription, error)
	SubscribeFinalized(chainID types.ChainID, c chan<- types.BlockSeal) (gethevent.Subscription, error)
}

type backend interface {
	UpdateLocalSafe(ctx context.Context, chainID types.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) error
	UpdateLocalUnsafe(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error
	LocalSafe(ctx context.Context, chainID types.ChainID) (pair types.DerivedIDPair, err error)
	LocalUnsafe(ctx context.Context, chainID types.ChainID) (eth.BlockID, error)
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

	lastSentCrossUnsafe locks.Watch[eth.BlockID]
	lastSentCrossSafe   locks.Watch[types.DerivedIDPair]
	lastSentFinalized   locks.Watch[eth.BlockID]

	// when the supervisor has a cross-safe update for the node
	crossSafeUpdateChan chan types.DerivedBlockSealPair
	// when the supervisor has a cross-unsafe update for the node
	crossUnsafeUpdateChan chan types.BlockSeal
	// when the supervisor has a finality update for the node
	finalizedUpdateChan chan types.BlockSeal

	// when the node has an update for us
	nodeEvents chan *types.ManagedEvent

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
	m.crossUnsafeUpdateChan = make(chan types.BlockSeal, 10)
	m.crossSafeUpdateChan = make(chan types.DerivedBlockSealPair, 10)
	m.finalizedUpdateChan = make(chan types.BlockSeal, 10)
	if sub, err := db.SubscribeCrossUnsafe(m.chainID, m.crossUnsafeUpdateChan); err != nil {
		m.log.Warn("failed to subscribe to cross unsafe", "err", err)
	} else {
		m.subscriptions = append(m.subscriptions, sub)
	}
	if sub, err := db.SubscribeCrossSafe(m.chainID, m.crossSafeUpdateChan); err != nil {
		m.log.Warn("failed to subscribe to cross safe", "err", err)
	} else {
		m.subscriptions = append(m.subscriptions, sub)
	}
	if sub, err := db.SubscribeFinalized(m.chainID, m.finalizedUpdateChan); err != nil {
		m.log.Warn("failed to subscribe to finalized", "err", err)
	} else {
		m.subscriptions = append(m.subscriptions, sub)
	}
}

func (m *ManagedNode) SubscribeToNodeEvents() {
	m.nodeEvents = make(chan *types.ManagedEvent, 10)

	// For each of these, we want to resubscribe on error. Since the RPC subscription might fail intermittently.
	m.subscriptions = append(m.subscriptions, gethevent.ResubscribeErr(time.Second*10,
		func(ctx context.Context, err error) (gethevent.Subscription, error) {
			return m.Node.SubscribeEvents(ctx, m.nodeEvents)
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
			case seal := <-m.crossUnsafeUpdateChan:
				m.onCrossUnsafeUpdate(seal)
			case pair := <-m.crossSafeUpdateChan:
				m.onCrossSafeUpdate(pair)
			case seal := <-m.finalizedUpdateChan:
				m.onFinalizedL2(seal)
			case ev := <-m.nodeEvents:
				m.onNodeEvent(ev)
			}
		}
	}()
}

// PollEvents pulls all events, until there are none left,
// the ctx is canceled, or an error upon event-pulling occurs.
func (m *ManagedNode) PollEvents(ctx context.Context) error {
	for {
		ev, err := m.Node.PullEvent(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// no events left
				return nil
			}
			return err
		}
		m.onNodeEvent(ev)
	}
}

func (m *ManagedNode) onNodeEvent(ev *types.ManagedEvent) {
	if ev.Reset != nil {
		m.onResetEvent(*ev.Reset)
	}
	if ev.UnsafeBlock != nil {
		m.onUnsafeBlock(*ev.UnsafeBlock)
	}
	if ev.DerivationUpdate != nil {
		m.onDerivationUpdate(*ev.DerivationUpdate)
	}
	if ev.ExhaustL1 != nil {
		m.onExhaustL1Event(*ev.ExhaustL1)
	}
}

func (m *ManagedNode) onResetEvent(errStr string) {
	m.log.Warn("Node sent us a reset error", "err", errStr)
	if strings.Contains(errStr, "cannot continue derivation until Engine has been reset") {
		// TODO
		return
	}
	// Try and restore the safe head of the op-supervisor.
	// The node will abort the reset until we find a block that is known.
	m.resetSignal(types.ErrFuture, eth.L1BlockRef{})
}

// in chain: "failed to update cross-safe head
//		to 0x2d904e68aa2a8a9dbc1ba023b4b04d485cc88885747d1d5999548ce8e3c956b6:1,
//		derived from scope 0x0fd8b875f8564ab0287c0180b905c75c3441ef491db57743964db0a261fee89b:1:
//		cannot add block 0x2d904e68aa2a8a9dbc1ba023b4b04d485cc88885747d1d5999548ce8e3c956b6:1
//		as derived from 0x0fd8b875f8564ab0287c0180b905c75c3441ef491db57743964db0a261fee89b:1
//		(parent 0x5b340cb5db27e84cb11f63f5e8329f1d31a2518e8064b52ed262b7105c8d0f21)
//		derived on top of BlockSeal(
//			hash:0x0bfcdc12076a6a4eab82afc6133f09afd37ff85f135f628c983a43079f21ad0f, number:0, time:1735057355):
//				conflicting data"
// 	"cannot add block 0x2d904e68aa2a8a9dbc1ba023b4b04d485cc88885747d1d5999548ce8e3c956b6:1
//		as derived from 0x0fd8b875f8564ab0287c0180b905c75c3441ef491db57743964db0a261fee89b:1
//		(parent 0x5b340cb5db27e84cb11f63f5e8329f1d31a2518e8064b52ed262b7105c8d0f21)
//		derived on top of BlockSeal(
//			hash:0x0bfcdc12076a6a4eab82afc6133f09afd37ff85f135f628c983a43079f21ad0f, number:0, time:1735057355): conflicting data"
//        	            		"conflicting data"

func (m *ManagedNode) onCrossUnsafeUpdate(seal types.BlockSeal) {
	m.log.Debug("updating cross unsafe", "crossUnsafe", seal)
	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	id := seal.ID()
	err := m.Node.UpdateCrossUnsafe(ctx, id)
	if err != nil {
		m.log.Warn("Node failed cross-unsafe updating", "err", err)
		return
	}
	m.lastSentCrossUnsafe.Set(id)
}

func (m *ManagedNode) onCrossSafeUpdate(pair types.DerivedBlockSealPair) {
	m.log.Debug("updating cross safe", "derived", pair.Derived, "derivedFrom", pair.DerivedFrom)
	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	pairIDs := pair.IDs()
	err := m.Node.UpdateCrossSafe(ctx, pairIDs.Derived, pairIDs.DerivedFrom)
	if err != nil {
		m.log.Warn("Node failed cross-safe updating", "err", err)
		return
	}
	m.lastSentCrossSafe.Set(pairIDs)
}

func (m *ManagedNode) onFinalizedL2(seal types.BlockSeal) {
	m.log.Debug("updating finalized L2", "finalized", seal)
	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	id := seal.ID()
	err := m.Node.UpdateFinalized(ctx, id)
	if err != nil {
		m.log.Warn("Node failed finality updating", "err", err)
		return
	}
	m.lastSentFinalized.Set(id)
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

func (m *ManagedNode) onDerivationUpdate(pair types.DerivedBlockRefPair) {
	m.log.Info("Node derived new block", "derived", pair.Derived,
		"derivedParent", pair.Derived.ParentID(), "derivedFrom", pair.DerivedFrom)
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
	u, err := m.backend.LocalUnsafe(ctx, m.chainID)
	if err != nil {
		m.log.Warn("Failed to retrieve local-unsafe", "err", err)
		return
	}
	f, err := m.backend.Finalized(ctx, m.chainID)
	if err != nil {
		m.log.Warn("Failed to retrieve finalized", "err", err)
		return
	}

	// fix finalized to point to a L2 block that the L2 node knows about
	// Conceptually: track the last known block by the node (based on unsafe block updates), as upper bound for resets.
	// Then when reset fails, lower the last known block
	// (and prevent it from changing by subscription, until success with reset), and rinse and repeat.

	// TODO: this is very very broken

	// TODO: errors.As switch
	switch errSignal {
	case types.ErrConflict:
		s, err := m.backend.SafeDerivedAt(ctx, m.chainID, l1Ref.ID())
		if err != nil {
			m.log.Warn("Failed to retrieve cross-safe", "err", err)
			return
		}
		log.Debug("Node detected conflict, resetting", "unsafe", u, "safe", s, "finalized", f)
		err = m.Node.Reset(ctx, u, s, f)
		if err != nil {
			m.log.Warn("Node failed to reset", "err", err)
		}
	case types.ErrFuture:
		s, err := m.backend.LocalSafe(ctx, m.chainID)
		if err != nil {
			m.log.Warn("Failed to retrieve local-safe", "err", err)
		}
		log.Debug("Node detected future block, resetting", "unsafe", u, "safe", s, "finalized", f)
		err = m.Node.Reset(ctx, u, s.Derived, f)
		if err != nil {
			m.log.Warn("Node failed to reset", "err", err)
		}
	case types.ErrOutOfOrder:
		m.log.Warn("Node detected out of order block", "unsafe", u, "finalized", f)
	}
}

func (m *ManagedNode) onExhaustL1Event(completed types.DerivedBlockRefPair) {
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

func (m *ManagedNode) AwaitSentCrossUnsafeUpdate(ctx context.Context, minNum uint64) error {
	_, err := m.lastSentCrossUnsafe.Catch(ctx, func(id eth.BlockID) bool {
		return id.Number >= minNum
	})
	return err
}

func (m *ManagedNode) AwaitSentCrossSafeUpdate(ctx context.Context, minNum uint64) error {
	_, err := m.lastSentCrossSafe.Catch(ctx, func(pair types.DerivedIDPair) bool {
		return pair.Derived.Number >= minNum
	})
	return err
}

func (m *ManagedNode) AwaitSentFinalizedUpdate(ctx context.Context, minNum uint64) error {
	_, err := m.lastSentFinalized.Catch(ctx, func(id eth.BlockID) bool {
		return id.Number >= minNum
	})
	return err
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
