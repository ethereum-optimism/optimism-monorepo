package sequencing

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/protolambda/ctxlock"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/conductor"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// sealingDuration defines the expected time it takes to seal the block
const sealingDuration = time.Millisecond * 50

var (
	ErrSequencerAlreadyStarted = errors.New("sequencer already running")
	ErrSequencerAlreadyStopped = errors.New("sequencer not running")
)

type L1OriginSelectorIface interface {
	FindL1Origin(ctx context.Context, l2Head eth.L2BlockRef) (eth.L1BlockRef, error)
}

type Metrics interface {
	RecordSequencerInconsistentL1Origin(from eth.BlockID, to eth.BlockID)
	RecordSequencerReset()
	RecordSequencingError()
}

type SequencerStateListener interface {
	SequencerStarted() error
	SequencerStopped() error
}

type AsyncGossiper interface {
	Gossip(payload *eth.ExecutionPayloadEnvelope)
	Get() *eth.ExecutionPayloadEnvelope
	Clear()
	Stop()
	Start()
}

// SequencerActionEvent triggers the sequencer to start/seal a block, if active and ready to act.
// This event is used to prioritize sequencer work over derivation work,
// by emitting it before e.g. a derivation-pipeline step.
// A future sequencer in an async world may manage its own execution.
type SequencerActionEvent struct{}

func (ev SequencerActionEvent) String() string {
	return "sequencer-action"
}

type BuildingState struct {
	Onto eth.L2BlockRef
	Info eth.PayloadInfo

	Started time.Time

	// Set once known
	Ref eth.L2BlockRef
}

// Sequencer implements the sequencing interface of the driver: it starts and completes block building jobs.
type Sequencer struct {
	l ctxlock.Lock

	// closed when driver system closes, to interrupt any ongoing API calls etc.
	ctx context.Context

	log       log.Logger
	rollupCfg *rollup.Config
	spec      *rollup.ChainSpec

	maxSafeLag atomic.Uint64

	// active identifies whether the sequencer is running.
	// This is an atomic value, so it can be read without locking the whole sequencer.
	active atomic.Bool

	// listener for sequencer-state changes. Blocking, may error.
	// May be used to ensure sequencer-state is accurately persisted.
	listener SequencerStateListener

	conductor conductor.SequencerConductor

	asyncGossip AsyncGossiper

	emitter event.Emitter

	attrBuilder      derive.AttributesBuilder
	l1OriginSelector L1OriginSelectorIface

	metrics Metrics

	// timeNow enables sequencer testing to mock the time
	timeNow func() time.Time

	// nextAction is when the next sequencing action should be performed
	nextAction   time.Time
	nextActionOK bool

	latest       BuildingState
	latestSealed eth.L2BlockRef
	latestHead   eth.L2BlockRef

	latestHeadSet chan struct{}

	// toBlockRef converts a payload to a block-ref, and is only configurable for test-purposes
	toBlockRef func(rollupCfg *rollup.Config, payload *eth.ExecutionPayload) (eth.L2BlockRef, error)
}

var _ SequencerIface = (*Sequencer)(nil)

func NewSequencer(driverCtx context.Context, log log.Logger, rollupCfg *rollup.Config,
	attributesBuilder derive.AttributesBuilder,
	l1OriginSelector L1OriginSelectorIface,
	listener SequencerStateListener,
	conductor conductor.SequencerConductor,
	asyncGossip AsyncGossiper,
	metrics Metrics,
) *Sequencer {
	return &Sequencer{
		ctx:              driverCtx,
		log:              log,
		rollupCfg:        rollupCfg,
		spec:             rollup.NewChainSpec(rollupCfg),
		listener:         listener,
		conductor:        conductor,
		asyncGossip:      asyncGossip,
		attrBuilder:      attributesBuilder,
		l1OriginSelector: l1OriginSelector,
		metrics:          metrics,
		timeNow:          time.Now,
		toBlockRef:       derive.PayloadToBlockRef,
	}
}

func (d *Sequencer) AttachEmitter(em event.Emitter) {
	d.emitter = em
}

func (d *Sequencer) OnEvent(ev event.Event) bool {
	d.l.Lock()
	defer d.l.Unlock()

	preTime := d.nextAction
	preOk := d.nextActionOK
	defer func() {
		if d.nextActionOK != preOk || d.nextAction != preTime {
			d.log.Debug("Sequencer action schedule changed",
				"time", d.nextAction, "wait", d.nextAction.Sub(d.timeNow()), "ok", d.nextActionOK, "event", ev)
		}
	}()

	switch x := ev.(type) {
	case engine.BuildStartedEvent:
		d.onBuildStarted(x)
	case engine.InvalidPayloadAttributesEvent:
		d.onInvalidPayloadAttributes(x)
	case engine.BuildSealedEvent:
		d.onBuildSealed(x)
	case engine.PayloadSealInvalidEvent:
		d.onPayloadSealInvalid(x)
	case engine.PayloadSealExpiredErrorEvent:
		d.onPayloadSealExpiredError(x)
	case engine.PayloadInvalidEvent:
		d.onPayloadInvalid(x)
	case engine.PayloadSuccessEvent:
		d.onPayloadSuccess(x)
	case SequencerActionEvent:
		d.onSequencerAction(x)
	case rollup.EngineTemporaryErrorEvent:
		d.onEngineTemporaryError(x)
	case rollup.ResetEvent:
		d.onReset(x)
	case engine.EngineResetConfirmedEvent:
		d.onEngineResetConfirmedEvent(x)
	case engine.ForkchoiceUpdateEvent:
		d.onForkchoiceUpdate(x)
	default:
		return false
	}
	return true
}

func (d *Sequencer) onBuildStarted(x engine.BuildStartedEvent) {
	if x.DerivedFrom != (eth.L1BlockRef{}) {
		// If we are adding new blocks onto the tip of the chain, derived from L1,
		// then don't try to build on top of it immediately, as sequencer.
		d.log.Warn("Detected new block-building from L1 derivation, avoiding sequencing for now.",
			"build_job", x.Info.ID, "build_timestamp", x.Info.Timestamp,
			"parent", x.Parent, "derived_from", x.DerivedFrom)
		d.nextActionOK = false
		return
	}
	if d.latest.Onto != x.Parent {
		d.log.Warn("Canceling stale block-building job that was just started, as target to build onto has changed",
			"stale", x.Parent, "new", d.latest.Onto, "job_id", x.Info.ID, "job_timestamp", x.Info.Timestamp)
		d.emitter.Emit(engine.BuildCancelEvent{
			Info:  x.Info,
			Force: true,
		})
		d.handleInvalid()
		return
	}
	// if not a derived block, then it is work of the sequencer
	d.log.Debug("Sequencer started building new block",
		"payloadID", x.Info.ID, "parent", x.Parent, "parent_time", x.Parent.Time)
	d.latest.Info = x.Info
	d.latest.Started = x.BuildStarted

	d.nextActionOK = d.active.Load()

	// schedule sealing
	now := d.timeNow()
	payloadTime := time.Unix(int64(x.Parent.Time+d.rollupCfg.BlockTime), 0)
	remainingTime := payloadTime.Sub(now)
	if remainingTime < sealingDuration {
		d.nextAction = now // if there's not enough time for sealing, don't wait.
	} else {
		// finish with margin of sealing duration before payloadTime
		d.nextAction = payloadTime.Add(-sealingDuration)
	}
}

func (d *Sequencer) handleInvalid() {
	d.metrics.RecordSequencingError()
	d.latest = BuildingState{}
	d.asyncGossip.Clear()
	// upon error, retry after one block worth of time
	blockTime := time.Duration(d.rollupCfg.BlockTime) * time.Second
	d.nextAction = d.timeNow().Add(blockTime)
	d.nextActionOK = d.active.Load()
}

func (d *Sequencer) onInvalidPayloadAttributes(x engine.InvalidPayloadAttributesEvent) {
	if x.Attributes.DerivedFrom != (eth.L1BlockRef{}) {
		return // not our payload, should be ignored.
	}
	d.log.Error("Cannot sequence invalid payload attributes",
		"attributes_parent", x.Attributes.Parent,
		"timestamp", x.Attributes.Attributes.Timestamp, "err", x.Err)

	d.handleInvalid()
}

func (d *Sequencer) onBuildSealed(x engine.BuildSealedEvent) {
	if d.latest.Info != x.Info {
		return // not our payload, should be ignored.
	}
	d.log.Info("Sequencer sealed block", "payloadID", x.Info.ID,
		"block", x.Envelope.ExecutionPayload.ID(),
		"parent", x.Envelope.ExecutionPayload.ParentID(),
		"txs", len(x.Envelope.ExecutionPayload.Transactions),
		"time", uint64(x.Envelope.ExecutionPayload.Timestamp))

	// generous timeout, the conductor is important
	ctx, cancel := context.WithTimeout(d.ctx, time.Second*30)
	defer cancel()
	if err := d.conductor.CommitUnsafePayload(ctx, x.Envelope); err != nil {
		d.emitter.Emit(rollup.EngineTemporaryErrorEvent{
			Err: fmt.Errorf("failed to commit unsafe payload to conductor: %w", err),
		})
		return
	}

	// begin gossiping as soon as possible
	// asyncGossip.Clear() will be called later if an non-temporary error is found,
	// or if the payload is successfully inserted
	d.asyncGossip.Gossip(x.Envelope)
	// Now after having gossiped the block, try to put it in our own canonical chain
	d.emitter.Emit(engine.PayloadProcessEvent{
		Concluding:  x.Concluding,
		DerivedFrom: x.DerivedFrom,
		Envelope:    x.Envelope,
		Ref:         x.Ref,
	})
	d.latest.Ref = x.Ref
	d.latestSealed = x.Ref
}

func (d *Sequencer) onPayloadSealInvalid(x engine.PayloadSealInvalidEvent) {
	if d.latest.Info != x.Info {
		return // not our payload, should be ignored.
	}
	d.log.Error("Sequencer could not seal block",
		"payloadID", x.Info.ID, "timestamp", x.Info.Timestamp, "err", x.Err)
	d.handleInvalid()
}

func (d *Sequencer) onPayloadSealExpiredError(x engine.PayloadSealExpiredErrorEvent) {
	if d.latest.Info != x.Info {
		return // not our payload, should be ignored.
	}
	d.log.Error("Sequencer temporarily could not seal block",
		"payloadID", x.Info.ID, "timestamp", x.Info.Timestamp, "err", x.Err)
	// Restart building, this way we get a block we should be able to seal
	// (smaller, since we adapt build time).
	d.handleInvalid()
}

func (d *Sequencer) onPayloadInvalid(x engine.PayloadInvalidEvent) {
	if d.latest.Ref.Hash != x.Envelope.ExecutionPayload.BlockHash {
		return // not a payload from the sequencer
	}
	d.log.Error("Sequencer could not insert payload",
		"block", x.Envelope.ExecutionPayload.ID(), "err", x.Err)
	d.handleInvalid()
}

func (d *Sequencer) onPayloadSuccess(x engine.PayloadSuccessEvent) {
	// d.latest as building state may already be empty,
	// if the forkchoice update (that dropped the stale building job) was received before the payload-success.
	if d.latest.Ref != (eth.L2BlockRef{}) && d.latest.Ref.Hash != x.Envelope.ExecutionPayload.BlockHash {
		// Not a payload that was built by this sequencer. We can ignore it, and continue upon forkchoice update.
		return
	}
	d.latest = BuildingState{}
	d.log.Info("Sequencer inserted block",
		"block", x.Ref, "parent", x.Envelope.ExecutionPayload.ParentID())
	// The payload was already published upon sealing.
	// Now that we have processed it ourselves we don't need it anymore.
	d.asyncGossip.Clear()
}

func (d *Sequencer) onSequencerAction(SequencerActionEvent) {
	d.log.Debug("Sequencer action")
	payload := d.asyncGossip.Get()
	if payload != nil {
		if d.latest.Info.ID == (eth.PayloadID{}) {
			d.log.Warn("Found reusable payload from async gossiper, and no block was being built. Reusing payload.",
				"hash", payload.ExecutionPayload.BlockHash,
				"number", uint64(payload.ExecutionPayload.BlockNumber),
				"parent", payload.ExecutionPayload.ParentHash)
		}
		ref, err := d.toBlockRef(d.rollupCfg, payload.ExecutionPayload)
		if err != nil {
			d.log.Error("Payload from async-gossip buffer could not be turned into block-ref", "err", err)
			d.asyncGossip.Clear() // bad payload
			return
		}
		d.log.Info("Resuming sequencing with previously async-gossip confirmed payload",
			"payload", payload.ExecutionPayload.ID())
		// Payload is known, we must have resumed sequencer-actions after a temporary error,
		// meaning that we have seen BuildSealedEvent already.
		// We can retry processing to make it canonical.
		d.emitter.Emit(engine.PayloadProcessEvent{
			Concluding:  false,
			DerivedFrom: eth.L1BlockRef{},
			Envelope:    payload,
			Ref:         ref,
		})
		d.latest.Ref = ref
	} else {
		if d.latest.Info != (eth.PayloadInfo{}) {
			// We should not repeat the seal request.
			d.nextActionOK = false
			// No known payload for block building job,
			// we have to retrieve it first.
			d.emitter.Emit(engine.BuildSealEvent{
				Info:         d.latest.Info,
				BuildStarted: d.latest.Started,
				Concluding:   false,
				DerivedFrom:  eth.L1BlockRef{},
			})
		} else if d.latest == (BuildingState{}) {
			// If we have not started building anything, start building.
			d.startBuildingBlock()
		}
	}
}

func (d *Sequencer) onEngineTemporaryError(x rollup.EngineTemporaryErrorEvent) {
	if d.latest == (BuildingState{}) {
		d.log.Debug("Engine reported temporary error while building state is empty", "err", x.Err)
	}
	d.log.Error("Engine failed temporarily, backing off sequencer", "err", x.Err)
	if errors.Is(x.Err, engine.ErrEngineSyncing) { // if it is syncing we can back off by more
		d.nextAction = d.timeNow().Add(30 * time.Second)
	} else {
		d.nextAction = d.timeNow().Add(time.Second)
	}
	d.nextActionOK = d.active.Load()
	// We don't explicitly cancel block building jobs upon temporary errors: we may still finish the block (if any).
	// Any unfinished block building work eventually times out, and will be cleaned up that way.
	// Note that this only applies to temporary errors upon starting a block-building job.
	// If the engine errors upon sealing, an PayloadSealInvalidEvent will be get it to restart the attributes.

	// If we don't have an ID of a job to resume, then start over.
	// (d.latest.Onto would be set if we emitted BuildStart already)
	if d.latest.Info == (eth.PayloadInfo{}) {
		d.latest = BuildingState{}
	}
}

func (d *Sequencer) onReset(x rollup.ResetEvent) {
	d.log.Error("Sequencer encountered reset signal, aborting work", "err", x.Err)
	d.metrics.RecordSequencerReset()
	// try to cancel any ongoing payload building job
	if d.latest.Info != (eth.PayloadInfo{}) {
		d.emitter.Emit(engine.BuildCancelEvent{Info: d.latest.Info})
	}
	d.latest = BuildingState{}
	// no action to perform until we get a reset-confirmation
	d.nextActionOK = false
}

func (d *Sequencer) onEngineResetConfirmedEvent(engine.EngineResetConfirmedEvent) {
	d.nextActionOK = d.active.Load()
	// Before sequencing we can wait a block,
	// assuming the execution-engine just churned through some work for the reset.
	// This will also prevent any potential reset-loop from running too hot.
	d.nextAction = d.timeNow().Add(time.Second * time.Duration(d.rollupCfg.BlockTime))
	d.log.Info("Engine reset confirmed, sequencer may continue", "next", d.nextActionOK)
}

func (d *Sequencer) onForkchoiceUpdate(x engine.ForkchoiceUpdateEvent) {
	d.log.Debug("Sequencer is processing forkchoice update", "unsafe", x.UnsafeL2Head, "latest", d.latestHead)

	if !d.active.Load() {
		d.setLatestHead(x.UnsafeL2Head)
		return
	}
	// If the safe head has fallen behind by a significant number of blocks, delay creating new blocks
	// until the safe lag is below SequencerMaxSafeLag.
	if maxSafeLag := d.maxSafeLag.Load(); maxSafeLag > 0 && x.SafeL2Head.Number+maxSafeLag <= x.UnsafeL2Head.Number {
		d.log.Warn("sequencer has fallen behind safe head by more than lag, stalling",
			"head", x.UnsafeL2Head, "safe", x.SafeL2Head, "max_lag", maxSafeLag)
		d.nextActionOK = false
	}
	// Drop stale block-building job if the chain has moved past it already.
	if d.latest != (BuildingState{}) && d.latest.Onto.Number < x.UnsafeL2Head.Number {
		d.log.Debug("Dropping stale/completed block-building job",
			"state", d.latest.Onto, "unsafe_head", x.UnsafeL2Head)
		// The cleared state will block further BuildStarted/BuildSealed responses from continuing the stale build job.
		d.latest = BuildingState{}
	}
	if x.UnsafeL2Head.Number > d.latestHead.Number {
		d.nextActionOK = true
		now := d.timeNow()
		blockTime := time.Duration(d.rollupCfg.BlockTime) * time.Second
		payloadTime := time.Unix(int64(x.UnsafeL2Head.Time+d.rollupCfg.BlockTime), 0)
		remainingTime := payloadTime.Sub(now)
		if remainingTime > blockTime {
			// if we have too much time, then wait before starting the build
			d.nextAction = payloadTime.Add(-blockTime)
		} else {
			// otherwise start instantly
			d.nextAction = now
		}
	}
	d.setLatestHead(x.UnsafeL2Head)
}

func (d *Sequencer) setLatestHead(head eth.L2BlockRef) {
	d.latestHead = head
	if d.latestHeadSet != nil {
		close(d.latestHeadSet)
		d.latestHeadSet = nil
	}
}

// StartBuildingBlock initiates a block building job on top of the given L2 head, safe and finalized blocks, and using the provided l1Origin.
func (d *Sequencer) startBuildingBlock() {
	ctx := d.ctx
	l2Head := d.latestHead

	// If we do not have data to know what to build on, then request a forkchoice update
	if l2Head == (eth.L2BlockRef{}) {
		d.emitter.Emit(engine.ForkchoiceRequestEvent{})
		return
	}
	// If we have already started trying to build on top of this block, we can avoid starting over again.
	if d.latest.Onto == l2Head {
		return
	}

	// Figure out which L1 origin block we're going to be building on top of.
	l1Origin, err := d.l1OriginSelector.FindL1Origin(ctx, l2Head)
	if err != nil {
		d.log.Error("Error finding next L1 Origin", "err", err)
		d.emitter.Emit(rollup.L1TemporaryErrorEvent{Err: err})
		return
	}

	if !(l2Head.L1Origin.Hash == l1Origin.ParentHash || l2Head.L1Origin.Hash == l1Origin.Hash) {
		d.metrics.RecordSequencerInconsistentL1Origin(l2Head.L1Origin, l1Origin.ID())
		d.emitter.Emit(rollup.ResetEvent{Err: fmt.Errorf("cannot build new L2 block with L1 origin %s (parent L1 %s) on current L2 head %s with L1 origin %s",
			l1Origin, l1Origin.ParentHash, l2Head, l2Head.L1Origin)})
		return
	}

	d.log.Info("Started sequencing new block", "parent", l2Head, "l1Origin", l1Origin)

	fetchCtx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	attrs, err := d.attrBuilder.PreparePayloadAttributes(fetchCtx, l2Head, l1Origin.ID())
	if err != nil {
		if errors.Is(err, derive.ErrTemporary) {
			d.emitter.Emit(rollup.EngineTemporaryErrorEvent{Err: err})
			return
		} else if errors.Is(err, derive.ErrReset) {
			d.emitter.Emit(rollup.ResetEvent{Err: err})
			return
		} else if errors.Is(err, derive.ErrCritical) {
			d.emitter.Emit(rollup.CriticalErrorEvent{Err: err})
			return
		} else {
			d.emitter.Emit(rollup.CriticalErrorEvent{Err: fmt.Errorf("unexpected attributes-preparation error: %w", err)})
			return
		}
	}

	// If our next L2 block timestamp is beyond the Sequencer drift threshold, then we must produce
	// empty blocks (other than the L1 info deposit and any user deposits). We handle this by
	// setting NoTxPool to true, which will cause the Sequencer to not include any transactions
	// from the transaction pool.
	attrs.NoTxPool = uint64(attrs.Timestamp) > l1Origin.Time+d.spec.MaxSequencerDrift(l1Origin.Time)

	// For the Ecotone activation block we shouldn't include any sequencer transactions.
	if d.rollupCfg.IsEcotoneActivationBlock(uint64(attrs.Timestamp)) {
		attrs.NoTxPool = true
		d.log.Info("Sequencing Ecotone upgrade block")
	}

	// For the Fjord activation block we shouldn't include any sequencer transactions.
	if d.rollupCfg.IsFjordActivationBlock(uint64(attrs.Timestamp)) {
		attrs.NoTxPool = true
		d.log.Info("Sequencing Fjord upgrade block")
	}

	// For the Granite activation block we shouldn't include any sequencer transactions.
	if d.rollupCfg.IsGraniteActivationBlock(uint64(attrs.Timestamp)) {
		d.log.Info("Sequencing Granite upgrade block")
	}

	d.log.Debug("prepared attributes for new block",
		"num", l2Head.Number+1, "time", uint64(attrs.Timestamp),
		"origin", l1Origin, "origin_time", l1Origin.Time, "noTxPool", attrs.NoTxPool)

	// Start a payload building process.
	withParent := &derive.AttributesWithParent{
		Attributes:  attrs,
		Parent:      l2Head,
		Concluding:  false,
		DerivedFrom: eth.L1BlockRef{}, // zero, not going to be pending-safe / safe
	}

	// Don't try to start building a block again, until we have heard back from this attempt
	d.nextActionOK = false

	// Reset building state, and remember what we are building on.
	// If we get a forkchoice update that conflicts, we will have to abort building.
	d.latest = BuildingState{Onto: l2Head}

	d.emitter.Emit(engine.BuildStartEvent{
		Attributes: withParent,
	})
}

func (d *Sequencer) NextAction() (t time.Time, ok bool) {
	d.l.Lock()
	defer d.l.Unlock()
	return d.nextAction, d.nextActionOK
}

func (d *Sequencer) Active() bool {
	return d.active.Load()
}

func (d *Sequencer) Start(ctx context.Context, head common.Hash) error {
	// must be leading to activate
	if isLeader, err := d.conductor.Leader(ctx); err != nil {
		return fmt.Errorf("sequencer leader check failed: %w", err)
	} else if !isLeader {
		return errors.New("sequencer is not the leader, aborting")
	}

	// Note: leader check happens before locking; this is how the Driver used to work,
	// and prevents the event-processing of the sequencer from being stalled due to a potentially slow conductor call.
	if err := d.l.LockCtx(ctx); err != nil {
		return err
	}
	defer d.l.Unlock()

	if d.active.Load() {
		return ErrSequencerAlreadyStarted
	}
	if d.latestHead == (eth.L2BlockRef{}) {
		return fmt.Errorf("no prestate, cannot determine if sequencer start at %s is safe", head)
	}
	if head != d.latestHead.Hash {
		return fmt.Errorf("block hash does not match: head %s, received %s", d.latestHead, head)
	}
	return d.forceStart()
}

func (d *Sequencer) Init(ctx context.Context, active bool) error {
	d.l.Lock()
	defer d.l.Unlock()

	d.asyncGossip.Start()

	// The `latestHead` should be updated, so we can handle start-sequencer requests
	d.emitter.Emit(engine.ForkchoiceRequestEvent{})

	if active {
		return d.forceStart()
	} else {
		if err := d.listener.SequencerStopped(); err != nil {
			return fmt.Errorf("failed to notify sequencer-state listener of initial stopped state: %w", err)
		}
		return nil
	}
}

// forceStart skips all the checks, and just starts the sequencer
func (d *Sequencer) forceStart() error {
	if d.latestHead == (eth.L2BlockRef{}) {
		// This happens if sequencing is activated on op-node startup.
		// The op-conductor check and choice of sequencing with this pre-state already happened before op-node startup.
		d.log.Info("Starting sequencing, without known pre-state")
		d.asyncGossip.Clear() // if we are starting from an unknown pre-state, just clear gossip out of caution.
	} else {
		// This happens when we start sequencing on an already-running node.
		d.log.Info("Starting sequencing on top of known pre-state", "head", d.latestHead)
		if payload := d.asyncGossip.Get(); payload != nil &&
			payload.ExecutionPayload.BlockHash != d.latestHead.Hash {
			d.log.Warn("Cleared old block from async-gossip buffer, sequencing pre-state is different",
				"buffered", payload.ExecutionPayload.ID(), "prestate", d.latestHead)
			d.asyncGossip.Clear()
		}
	}

	if err := d.listener.SequencerStarted(); err != nil {
		return fmt.Errorf("failed to notify sequencer-state listener of start: %w", err)
	}
	// clear the building state; interrupting any existing sequencing job (there should never be one)
	d.latest = BuildingState{}
	d.nextActionOK = true
	d.nextAction = d.timeNow()
	d.active.Store(true)
	d.log.Info("Sequencer has been started", "next action", d.nextAction)
	return nil
}

func (d *Sequencer) Stop(ctx context.Context) (common.Hash, error) {
	if err := d.l.LockCtx(ctx); err != nil {
		return common.Hash{}, err
	}

	if !d.active.Load() {
		d.l.Unlock()
		return common.Hash{}, ErrSequencerAlreadyStopped
	}

	// ensure latestHead has been updated to the latest sealed/gossiped block before stopping the sequencer
	for d.latestHead.Hash != d.latestSealed.Hash {
		latestHeadSet := make(chan struct{})
		d.latestHeadSet = latestHeadSet
		d.l.Unlock()
		select {
		case <-ctx.Done():
			return common.Hash{}, ctx.Err()
		case <-latestHeadSet:
		}
		if err := d.l.LockCtx(ctx); err != nil {
			return common.Hash{}, err
		}
	}
	defer d.l.Unlock()

	// Stop() may have been called twice, so check if we are active after reacquiring the lock
	if !d.active.Load() {
		return common.Hash{}, ErrSequencerAlreadyStopped
	}

	if err := d.listener.SequencerStopped(); err != nil {
		return common.Hash{}, fmt.Errorf("failed to notify sequencer-state listener of stop: %w", err)
	}

	// Cancel any inflight block building. If we don't cancel this, we can resume sequencing an old block
	// even if we've received new unsafe heads in the interim, causing us to introduce a re-org.
	d.latest = BuildingState{} // By wiping this state we cannot continue from it later.

	d.nextActionOK = false
	d.active.Store(false)
	d.log.Info("Sequencer has been stopped")
	return d.latestHead.Hash, nil
}

func (d *Sequencer) SetMaxSafeLag(ctx context.Context, v uint64) error {
	d.maxSafeLag.Store(v)
	return nil
}

func (d *Sequencer) OverrideLeader(ctx context.Context) error {
	return d.conductor.OverrideLeader(ctx)
}

func (d *Sequencer) ConductorEnabled(ctx context.Context) bool {
	return d.conductor.Enabled(ctx)
}

func (d *Sequencer) Close() {
	d.conductor.Close()
	d.asyncGossip.Stop()
}
