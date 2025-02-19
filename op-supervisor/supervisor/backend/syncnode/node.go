package syncnode

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
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
	LocalSafe(ctx context.Context, chainID eth.ChainID) (pair types.DerivedIDPair, err error)
	LocalUnsafe(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error)
	IsLocalSafe(ctx context.Context, chainID eth.ChainID, block eth.BlockID) error
	IsLocalUnsafe(ctx context.Context, chainID eth.ChainID, block eth.BlockID) error
	CrossSafe(ctx context.Context, chainID eth.ChainID) (pair types.DerivedIDPair, err error)
	SafeDerivedAt(ctx context.Context, chainID eth.ChainID, source eth.BlockID) (derived eth.BlockID, err error)
	Finalized(ctx context.Context, chainID eth.ChainID) (eth.BlockID, error)
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

	lastState consistencyState
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
	switch x := ev.(type) {
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
		m.recovery()
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

func (m *ManagedNode) onResetEvent(errStr string) {
	m.log.Warn("Node sent us a reset error", "err", errStr)
	if strings.Contains(errStr, "cannot continue derivation until Engine has been reset") {
		// TODO
		return
	}

	m.recovery()
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
	// this new unsafe block may conflict with the supervisor's unsafe block
	m.lastState.lastNodeLocalUnsafe = unsafeRef.ID()
	m.checkConsistencyState()
}

func (m *ManagedNode) onDerivationUpdate(pair types.DerivedBlockRefPair) {
	m.log.Info("Node derived new block", "derived", pair.Derived,
		"derivedParent", pair.Derived.ParentID(), "source", pair.Source)
	m.emitter.Emit(superevents.LocalDerivedEvent{
		ChainID: m.chainID,
		Derived: pair,
	})
	m.lastState.lastNodeLocalSafe = pair.Derived.ID()
	m.checkConsistencyState()
}

func (m *ManagedNode) onDerivationOriginUpdate(origin eth.BlockRef) {
	m.log.Info("Node derived new origin", "origin", origin)
	m.emitter.Emit(superevents.LocalDerivedOriginUpdateEvent{
		ChainID: m.chainID,
		Origin:  origin,
	})
}

func (m *ManagedNode) maybeRecover() {
	m.recovery()
}

func (m *ManagedNode) recovery() {
	// TODO: trigger event to continue reducing of search space,
	//  which then triggers eventual reset.
	// -- I'm not sure triggering events is a good idea here, since the OnEvent handlers
	// -- still need to filter down to which host is being referenced. Maybe this can just be
	// -- an async job that is triggered and then sets a recovery flag when ready
}

func (m *ManagedNode) onNodeLocalSafeUpdate(update eth.BlockRef) {
	// TODO  like below

	// TODO also check DB
	// if mismatch, start recovery
}

func (m *ManagedNode) onNodeLocalUnsafeUpdate(update eth.BlockRef) {
	// TODO  like below

	// TODO also check DB
	// if mismatch, start recovery
}
func (m *ManagedNode) onSupervisorLocalSafeUpdate(update eth.BlockRef) {
	if m.lastSupervisorLocalSafe.Hash != update.Hash && m.lastSupervisorLocalSafe.Hash != update.ParentHash {
		// reorg may have continued!
		m.recovery()
	}
}

func (m *ManagedNode) onSupervisorLocalUnsafeUpdate(update eth.BlockRef) {
	if m.lastSupervisorUnsafe.Hash != update.Hash && m.lastSupervisorUnsafe.Hash != update.ParentHash {
		// reorg may have continued!
		m.recovery()
	}
}

func (m *ManagedNode) reduceSearchSpace() {
	hi := m.lastConflictingBlock.Number
	lo := m.lastCommonBlock.Number
	if hi == lo {
		// found the matching tip of the chain, time to reset
		m.resetTo(hi)
	}
	if hi < lo {
		// abort
		return
	}
	pivot := (lo + hi) / 2
	// TODO load pivot from both op-node and supervisor

	// TODO check if pivot matches
	// if it matches, set lastCommonBlock to pivot
	// if it does not, set lastConflictingBlock to pivot

	// TODO: trigger event to continue reducing of search space
}

// resetTo prepares a reset to the given block,
// with safety levels adjusted to match the supervisor
func (m *ManagedNode) resetTo(blockNum uint64) {
	var unsafe, safe, finalized eth.BlockID
	// TODO: determine safety levels, given the part of the chain that matches

	// if supervisor has conflicting data after common point:
	//  limit the unsafe block
	// if supervisor has no conflicting data after common point:
	//  no-op the unsafe block, we don't want to rewind the node.

	ctx, cancel := context.WithTimeout(m.ctx, nodeTimeout)
	defer cancel()
	if err := m.Node.Reset(ctx, unsafe, safe, finalized); err != nil {

	}
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
