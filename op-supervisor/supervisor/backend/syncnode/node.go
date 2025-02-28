package syncnode

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/rpc"
	gethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	gethevent "github.com/ethereum/go-ethereum/event"
)

type backend interface {
	LocalUnsafe(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error)
	CrossUnsafe(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error)
	LocalSafe(ctx context.Context, chainID eth.ChainID) (pair types.DerivedIDPair, err error)
	CrossSafe(ctx context.Context, chainID eth.ChainID) (pair types.DerivedIDPair, err error)
	Finalized(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error)

	FindSealedBlock(ctx context.Context, chainID eth.ChainID, number uint64) (eth.BlockID, error)
	IsLocalSafe(ctx context.Context, chainID eth.ChainID, block eth.BlockID) error
	IsCrossSafe(ctx context.Context, chainID eth.ChainID, block eth.BlockID) error
	IsLocalUnsafe(ctx context.Context, chainID eth.ChainID, block eth.BlockID) error
	SafeDerivedAt(ctx context.Context, chainID eth.ChainID, source eth.BlockID) (derived eth.BlockID, err error)
	L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error)
}

const (
	internalTimeout            = time.Second * 30
	nodeTimeout                = time.Second * 10
	maxWalkBackAttempts        = 300
	blockNotFoundRPCErrCode    = -39001
	conflictingBlockRPCErrCode = -39002
)

type ManagedNode struct {
	log     log.Logger
	Node    SyncControl
	chainID eth.ChainID

	backend backend

	// When the node has an update for us
	// Nil when node events are pulled synchronously.
	nodeEvents chan *types.ManagedEvent

	subscriptions []gethevent.Subscription

	emitter event.Emitter

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	lastNodeLocalUnsafe eth.BlockID
	lastNodeLocalSafe   eth.BlockID

	resetTracker *resetTracker
}

var _ event.AttachEmitter = (*ManagedNode)(nil)
var _ event.Deriver = (*ManagedNode)(nil)

func NewManagedNode(log log.Logger, id eth.ChainID, node SyncControl, backend backend, noSubscribe bool) *ManagedNode {
	ctx, cancel := context.WithCancel(context.Background())
	m := &ManagedNode{
		log:     log.New("chain", id),
		backend: backend,
		Node:    node,
		chainID: id,
		ctx:     ctx,
		cancel:  cancel,
	}
	m.resetTracker = &resetTracker{
		managed:     m,
		synchronous: noSubscribe,
		cancelling:  &atomic.Bool{},
		resetting:   &atomic.Bool{},
	}
	if !noSubscribe {
		m.SubscribeToNodeEvents()
	}
	m.WatchSubscriptionErrors()
	return m
}

func (m *ManagedNode) AttachEmitter(em event.Emitter) {
	m.emitter = em
}

func (m *ManagedNode) OnEvent(ev event.Event) bool {
	// if we're resetting, ignore all events
	if m.resetTracker.isResetting() {
		// even if we are resetting, cancel the reset if the L1 rewinds
		if _, ok := ev.(superevents.ChainRewoundEvent); ok {
			m.resetTracker.cancelReset()
		}
		m.log.Debug("Ignoring event during ongoing reset", "event", ev)
		return false
	}
	switch x := ev.(type) {
	case superevents.UpdateLocalSafeFailedEvent:
		if x.ChainID != m.chainID ||
			x.NodeID != m.Node.String() {
			return false
		}
		m.onUpdateLocalSafeFailed(x)
	case superevents.InvalidateLocalSafeEvent:
		if x.ChainID != m.chainID {
			return false
		}
		m.onInvalidateLocalSafe(x.Candidate)
	case superevents.CrossUnsafeUpdateEvent:
		if x.ChainID != m.chainID {
			return false
		}
		m.onCrossUnsafeUpdate(x.NewCrossUnsafe)
	case superevents.CrossSafeUpdateEvent:
		if x.ChainID != m.chainID {
			return false
		}
		m.onCrossSafeUpdate(x.NewCrossSafe)
	case superevents.FinalizedL2UpdateEvent:
		if x.ChainID != m.chainID {
			return false
		}
		m.onFinalizedL2(x.FinalizedL2)
	case superevents.ChainRewoundEvent:
		if x.ChainID != m.chainID {
			return false
		}
	default:
		return false
	}
	return true
}

func (m *ManagedNode) SubscribeToNodeEvents() {
	m.nodeEvents = make(chan *types.ManagedEvent, 10)

	// Resubscribe, since the RPC subscription might fail intermittently.
	// And fall back to polling, if RPC subscriptions are not supported.
	m.subscriptions = append(m.subscriptions, gethevent.ResubscribeErr(time.Second*10,
		func(ctx context.Context, prevErr error) (gethevent.Subscription, error) {
			if prevErr != nil {
				// This is the RPC runtime error, not the setup error we have logging for below.
				m.log.Error("RPC subscription failed, restarting now", "err", prevErr)
			}
			sub, err := m.Node.SubscribeEvents(ctx, m.nodeEvents)
			if err != nil {
				if errors.Is(err, gethrpc.ErrNotificationsUnsupported) {
					m.log.Warn("No RPC notification support detected, falling back to polling")
					// fallback to polling if subscriptions are not supported.
					sub, err := rpc.StreamFallback[types.ManagedEvent](
						m.Node.PullEvent, time.Millisecond*100, m.nodeEvents)
					if err != nil {
						m.log.Error("Failed to start RPC stream fallback", "err", err)
						return nil, err
					}
					return sub, err
				}
				return nil, err
			}
			return sub, nil
		}))
}

func (m *ManagedNode) WatchSubscriptionErrors() {
	watchSub := func(sub ethereum.Subscription) {
		defer m.wg.Done()
		select {
		case err := <-sub.Err():
			m.log.Error("Subscription error", "err", err)
		case <-m.ctx.Done():
			// we're closing, stop watching the subscription
		}
	}
	for _, sub := range m.subscriptions {
		m.wg.Add(1)
		go watchSub(sub)
	}
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
			case ev := <-m.nodeEvents: // nil, indefinitely blocking, if no node-events subscriber is set up.
				m.onNodeEvent(ev)
			}
		}
	}()
}

// PullEvents pulls all events, until there are none left,
// the ctx is canceled, or an error upon event-pulling occurs.
func (m *ManagedNode) PullEvents(ctx context.Context) (pulledAny bool, err error) {
	for {
		ev, err := m.Node.PullEvent(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// no events left
				return pulledAny, nil
			}
			return pulledAny, err
		}
		pulledAny = true
		m.onNodeEvent(ev)
	}
}

func (m *ManagedNode) onNodeEvent(ev *types.ManagedEvent) {
	if m.resetTracker.isResetting() {
		m.log.Debug("Ignoring event during ongoing reset", "event", ev)
		return
	}
	if ev == nil {
		m.log.Warn("Received nil event")
		return
	}
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
	if ev.ReplaceBlock != nil {
		m.onReplaceBlock(*ev.ReplaceBlock)
	}
	if ev.DerivationOriginUpdate != nil {
		m.onDerivationOriginUpdate(*ev.DerivationOriginUpdate)
	}
}

// onResetEvent handles a reset event from the node
func (m *ManagedNode) onResetEvent(errStr string) {
	m.log.Warn("Node sent us a reset error", "err", errStr)
	m.resetFullRange()
}

func (m *ManagedNode) onUpdateLocalSafeFailed(ev superevents.UpdateLocalSafeFailedEvent) {
	switch {
	case errors.Is(ev.Err, types.ErrConflict):
		m.log.Warn("DB indicated a conflict, checking consistency")
		m.resetIfInconsistent()
	case errors.Is(ev.Err, types.ErrFuture):
		m.log.Warn("DB indicated an update is in the future, checking if node is ahead")
		m.resetIfAhead()
	}
}

// OnResetReady handles a reset-ready event from the supervisor
// once the supervisor has determined the reset target by bisecting the search range
func (m *ManagedNode) OnResetReady(lUnsafe, xUnsafe, lSafe, xSafe, finalized eth.BlockID) {
	m.log.Info("Reset ready event received",
		"localUnsafe", lUnsafe,
		"crossUnsafe", xUnsafe,
		"localSafe", lSafe,
		"crossSafe", xSafe,
		"finalized", finalized)
	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	// whether the reset passes or fails, this ongoing reset is done
	m.resetTracker.endReset()
	if err := m.Node.Reset(ctx,
		lUnsafe, xUnsafe,
		lSafe, xSafe,
		finalized); err != nil {
		m.log.Error("Failed to reset node", "err", err)
	}
}

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
}

func (m *ManagedNode) onCrossSafeUpdate(pair types.DerivedBlockSealPair) {
	m.log.Debug("updating cross safe", "derived", pair.Derived, "source", pair.Source)
	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	pairIDs := pair.IDs()
	err := m.Node.UpdateCrossSafe(ctx, pairIDs.Derived, pairIDs.Source)
	if err != nil {
		m.log.Warn("Node failed cross-safe updating", "err", err)
		return
	}
}

func (m *ManagedNode) onFinalizedL2(seal types.BlockSeal) {
	m.log.Info("updating finalized L2", "finalized", seal)
	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	id := seal.ID()
	err := m.Node.UpdateFinalized(ctx, id)
	if err != nil {
		m.log.Warn("Node failed finality updating", "update", seal, "err", err)
		return
	}
}

func (m *ManagedNode) onUnsafeBlock(unsafeRef eth.BlockRef) {
	m.log.Info("Node has new unsafe block", "unsafeBlock", unsafeRef)
	m.emitter.Emit(superevents.LocalUnsafeReceivedEvent{
		ChainID:        m.chainID,
		NewLocalUnsafe: unsafeRef,
	})
	m.lastNodeLocalUnsafe = unsafeRef.ID()
	m.resetIfInconsistent()
}

func (m *ManagedNode) onDerivationUpdate(pair types.DerivedBlockRefPair) {
	m.log.Info("Node derived new block", "derived", pair.Derived,
		"derivedParent", pair.Derived.ParentID(), "source", pair.Source)
	m.emitter.Emit(superevents.LocalDerivedEvent{
		ChainID: m.chainID,
		Derived: pair,
		NodeID:  m.Node.String(),
	})
	m.lastNodeLocalSafe = pair.Derived.ID()
	m.resetIfInconsistent()
}

func (m *ManagedNode) onDerivationOriginUpdate(origin eth.BlockRef) {
	m.log.Info("Node derived new origin", "origin", origin)
	m.emitter.Emit(superevents.LocalDerivedOriginUpdateEvent{
		ChainID: m.chainID,
		Origin:  origin,
	})
}

func (m *ManagedNode) onExhaustL1Event(completed types.DerivedBlockRefPair) {
	m.log.Info("Node completed syncing", "l2", completed.Derived, "l1", completed.Source)

	internalCtx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()
	nextL1, err := m.backend.L1BlockRefByNumber(internalCtx, completed.Source.Number+1)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			m.log.Debug("Next L1 block is not yet available", "l1Block", completed.Source, "err", err)
			return
		}
		m.log.Error("Failed to retrieve next L1 block for node", "l1Block", completed.Source, "err", err)
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

// onInvalidateLocalSafe listens for when a local-safe block is found to be invalid in the cross-safe context
// and needs to be replaced with a deposit only block.
func (m *ManagedNode) onInvalidateLocalSafe(invalidated types.DerivedBlockRefPair) {
	m.log.Warn("Instructing node to replace invalidated local-safe block",
		"invalidated", invalidated.Derived, "scope", invalidated.Source)

	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	// Send instruction to the node to invalidate the block, and build a replacement block.
	if err := m.Node.InvalidateBlock(ctx, types.BlockSealFromRef(invalidated.Derived)); err != nil {
		m.log.Warn("Node is unable to invalidate block",
			"invalidated", invalidated.Derived, "scope", invalidated.Source, "err", err)
	}
}

func (m *ManagedNode) onReplaceBlock(replacement types.BlockReplacement) {
	m.log.Info("Node provided replacement block",
		"ref", replacement.Replacement, "invalidated", replacement.Invalidated)
	m.emitter.Emit(superevents.ReplaceBlockEvent{
		ChainID:     m.chainID,
		Replacement: replacement,
	})
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

// resetIfInconsistent checks if the node is consistent with the logs db
// and initiates a bisection based reset preparation if it is
func (m *ManagedNode) resetIfInconsistent() {
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()

	var last eth.BlockID

	// check if the last unsafe block we saw is consistent with the logs db
	err := m.backend.IsLocalUnsafe(ctx, m.chainID, m.lastNodeLocalUnsafe)
	if errors.Is(err, types.ErrConflict) {
		m.log.Warn("local unsafe block is inconsistent with logs db. Initiating reset",
			"lastUnsafeblock", m.lastNodeLocalUnsafe,
			"err", err)
		last = m.lastNodeLocalUnsafe
	}

	// check if the last safe block we saw is consistent with the local safe db
	err = m.backend.IsLocalSafe(ctx, m.chainID, m.lastNodeLocalSafe)
	if errors.Is(err, types.ErrConflict) {
		m.log.Warn("local safe block is inconsistent with logs db. Initiating reset",
			"lastSafeblock", m.lastNodeLocalSafe,
			"err", err)
		last = m.lastNodeLocalSafe
	}

	// there is inconsistency. begin the reset process
	if last != (eth.BlockID{}) {
		m.resetTracker.beginBisectionReset(last)
	} else {
		m.log.Debug("no inconsistency found")
	}
}

// resetIfAhead checks if the node is ahead of the logs db
// and initiates a bisection based reset preparation if it is
func (m *ManagedNode) resetIfAhead() {
	ctx, cancel := context.WithTimeout(m.ctx, internalTimeout)
	defer cancel()

	// get the last local safe block
	lastDBLocalSafe, err := m.backend.LocalSafe(ctx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last local safe block", "err", err)
		return
	}
	// if the node is ahead of the logs db, initiate a reset
	// with the end of the range being the last safe block in the db
	if m.lastNodeLocalSafe.Number > lastDBLocalSafe.Derived.Number {
		m.log.Warn("local safe block on node is ahead of logs db. Initiating reset",
			"lastNodeLocalSafe", m.lastNodeLocalSafe,
			"lastDBLocalSafe", lastDBLocalSafe.Derived)
		m.resetTracker.beginBisectionReset(lastDBLocalSafe.Derived)
	}
}

// resetFullRange resets the node using the last block in the db
// as the end of the range to search for the last consistent block
func (m *ManagedNode) resetFullRange() {
	internalCtx, iCancel := context.WithTimeout(m.ctx, internalTimeout)
	defer iCancel()
	dbLast, err := m.backend.LocalUnsafe(internalCtx, m.chainID)
	if err != nil {
		m.log.Error("failed to get last local unsafe block", "err", err)
		return
	}
	m.resetTracker.beginBisectionReset(dbLast)
}
