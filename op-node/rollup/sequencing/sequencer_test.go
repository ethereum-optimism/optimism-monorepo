package sequencing

import (
	"context"
	"math/rand" // nosemgrep
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/conductor"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

type FakeAttributesBuilder struct {
	cfg *rollup.Config
	rng *rand.Rand
}

func (m *FakeAttributesBuilder) PreparePayloadAttributes(ctx context.Context,
	l2Parent eth.L2BlockRef, epoch eth.BlockID) (attrs *eth.PayloadAttributes, err error) {
	gasLimit := eth.Uint64Quantity(30_000_000)
	attrs = &eth.PayloadAttributes{
		Timestamp:             eth.Uint64Quantity(l2Parent.Time + m.cfg.BlockTime),
		PrevRandao:            eth.Bytes32(testutils.RandomHash(m.rng)),
		SuggestedFeeRecipient: predeploys.SequencerFeeVaultAddr,
		Withdrawals:           nil,
		ParentBeaconBlockRoot: nil,
		Transactions:          []eth.Data{eth.Data("mock tx")},
		NoTxPool:              false,
		GasLimit:              &gasLimit,
	}
	if m.cfg.IsEcotone(uint64(attrs.Timestamp)) {
		r := testutils.RandomHash(m.rng)
		attrs.ParentBeaconBlockRoot = &r
	}
	return attrs, nil
}

var _ derive.AttributesBuilder = (*FakeAttributesBuilder)(nil)

type FakeL1OriginSelector struct {
	request  eth.L2BlockRef
	l1Origin eth.L1BlockRef
	err      error
}

func (f *FakeL1OriginSelector) FindL1Origin(ctx context.Context, l2Head eth.L2BlockRef) (eth.L1BlockRef, error) {
	f.request = l2Head
	return f.l1Origin, f.err
}

var _ L1OriginSelectorIface = (*FakeL1OriginSelector)(nil)

type BasicSequencerStateListener struct {
	active bool
}

func (b *BasicSequencerStateListener) SequencerStarted() error {
	b.active = true
	return nil
}

func (b *BasicSequencerStateListener) SequencerStopped() error {
	b.active = false
	return nil
}

var _ SequencerStateListener = (*BasicSequencerStateListener)(nil)

// FakeConductor is a no-op conductor that assumes this node is the leader sequencer.
type FakeConductor struct {
	closed    bool
	leader    bool
	committed *eth.ExecutionPayloadEnvelope
}

var _ conductor.SequencerConductor = &FakeConductor{}

func (c *FakeConductor) Leader(ctx context.Context) (bool, error) {
	return c.leader, nil
}

func (c *FakeConductor) CommitUnsafePayload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error {
	c.committed = payload
	return nil
}

func (c *FakeConductor) OverrideLeader(ctx context.Context) error {
	c.leader = true
	return nil
}

func (c *FakeConductor) Close() {
	c.closed = true
}

type FakeAsyncGossip struct {
	payload *eth.ExecutionPayloadEnvelope
	started bool
	stopped bool
}

func (f *FakeAsyncGossip) Gossip(payload *eth.ExecutionPayloadEnvelope) {
	f.payload = payload
}

func (f *FakeAsyncGossip) Get() *eth.ExecutionPayloadEnvelope {
	return f.payload
}

func (f *FakeAsyncGossip) Clear() {
	f.payload = nil
}

func (f *FakeAsyncGossip) Stop() {
	f.stopped = true
}

func (f *FakeAsyncGossip) Start() {
	f.started = true
}

var _ AsyncGossiper = (*FakeAsyncGossip)(nil)

// TestSequencer_StartStop runs through start/stop state back and forth to test state changes.
func TestSequencer_StartStop(t *testing.T) {
	logger := testlog.Logger(t, log.LevelError)
	seq, deps := createSequencer(logger)
	emitter := &testutils.MockEmitter{}
	seq.AttachEmitter(emitter)

	// Allow the sequencer to be the leader.
	// This is checked, since we start sequencing later, after initialization.
	// Also see issue #11121 for context: the conductor is checked by the infra, when initialized in active state.
	deps.conductor.leader = true

	emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
	require.NoError(t, seq.Init(context.Background(), false))
	emitter.AssertExpectations(t)
	require.False(t, deps.conductor.closed, "conductor is ready")
	require.True(t, deps.asyncGossip.started, "async gossip is always started on initialization")
	require.False(t, deps.seqState.active, "sequencer not active yet")

	seq.OnEvent(engine.ForkchoiceUpdateEvent{
		UnsafeL2Head:    eth.L2BlockRef{Hash: common.Hash{0xaa}},
		SafeL2Head:      eth.L2BlockRef{},
		FinalizedL2Head: eth.L2BlockRef{},
	})

	require.False(t, seq.Active())
	// no action scheduled
	_, ok := seq.NextAction()
	require.False(t, ok)

	require.NoError(t, seq.Start(context.Background(), common.Hash{0xaa}))
	require.True(t, seq.Active())
	require.True(t, deps.seqState.active, "sequencer signaled it is active")

	// sequencer is active now, it should schedule work
	_, ok = seq.NextAction()
	require.True(t, ok)

	// can't activate again before stopping
	err := seq.Start(context.Background(), common.Hash{0xaa})
	require.ErrorIs(t, err, ErrSequencerAlreadyStarted)

	head, err := seq.Stop(context.Background())
	require.NoError(t, err)
	require.Equal(t, head, common.Hash{0xaa})
	require.False(t, deps.seqState.active, "sequencer signaled it is no longer active")

	_, err = seq.Stop(context.Background())
	require.ErrorIs(t, err, ErrSequencerAlreadyStopped)

	// need to resume from the last head
	err = seq.Start(context.Background(), common.Hash{0xbb})
	require.ErrorContains(t, err, "block hash does not match")

	// can start again from head that it left
	err = seq.Start(context.Background(), head)
	require.NoError(t, err)
}

func TestSequencerBuild(t *testing.T) {
	logger := testlog.Logger(t, log.LevelError)
	seq, deps := createSequencer(logger)
	testClock := clock.NewSimpleClock()
	seq.timeNow = testClock.Now
	emitter := &testutils.MockEmitter{}
	seq.AttachEmitter(emitter)

	// Init will request a forkchoice update
	emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
	require.NoError(t, seq.Init(context.Background(), true))
	emitter.AssertExpectations(t)
	require.True(t, seq.Active(), "started in active mode")

	// It will request a forkchoice update, it needs the head before being able to build on top of it
	emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
	seq.OnEvent(SequencerActionEvent{})
	emitter.AssertExpectations(t)

	// Now send the forkchoice data, for the sequencer to learn what to build on top of.
	head := eth.L2BlockRef{
		Hash:   common.Hash{0x22},
		Number: 100,
		L1Origin: eth.BlockID{
			Hash:   common.Hash{0x11, 0xa},
			Number: 1000,
		},
		Time: 30000,
	}
	seq.OnEvent(engine.ForkchoiceUpdateEvent{UnsafeL2Head: head})
	emitter.AssertExpectations(t)

	// pretend we progress to the next L1 origin, catching up with the L2 time
	deps.l1OriginSelector.l1Origin = eth.L1BlockRef{
		Hash:       common.Hash{0x11, 0xb},
		ParentHash: common.Hash{0x11, 0xa},
		Number:     1001,
		Time:       29998,
	}
	var sentAttributes *derive.AttributesWithParent
	emitter.ExpectOnceRun(func(ev event.Event) {
		x, ok := ev.(engine.BuildStartEvent)
		require.True(t, ok)
		require.Equal(t, head, x.Attributes.Parent)
		require.Equal(t, head.Time+deps.cfg.BlockTime, uint64(x.Attributes.Attributes.Timestamp))
		require.Equal(t, eth.L1BlockRef{}, x.Attributes.DerivedFrom)
		sentAttributes = x.Attributes
	})
	seq.OnEvent(SequencerActionEvent{})
	emitter.AssertExpectations(t)

	// pretend we are already 150ms into the block-window when starting building
	startedTime := time.Unix(int64(head.Time), 0).Add(time.Millisecond * 150)
	testClock.Set(startedTime)
	payloadInfo := eth.PayloadInfo{
		ID:        eth.PayloadID{0x42},
		Timestamp: head.Time + deps.cfg.BlockTime,
	}
	seq.OnEvent(engine.BuildStartedEvent{
		Info:         payloadInfo,
		BuildStarted: startedTime,
		Parent:       head,
		IsLastInSpan: false,
		DerivedFrom:  eth.L1BlockRef{},
	})
	// The sealing should now be scheduled as next action.
	// We expect to seal just before the block-time boundary, leaving enough time for the sealing itself.
	sealTargetTime, ok := seq.NextAction()
	require.True(t, ok)
	buildDuration := sealTargetTime.Sub(time.Unix(int64(head.Time), 0))
	require.Equal(t, (time.Duration(deps.cfg.BlockTime)*time.Second)-sealingDuration, buildDuration)

	// Now trigger the sequencer to start sealing
	emitter.ExpectOnce(engine.BuildSealEvent{
		Info:         payloadInfo,
		BuildStarted: startedTime,
		IsLastInSpan: false,
		DerivedFrom:  eth.L1BlockRef{},
	})
	seq.OnEvent(SequencerActionEvent{})
	emitter.AssertExpectations(t)
	_, ok = seq.NextAction()
	require.False(t, ok, "cannot act until sealing completes/fails")

	payloadEnvelope := &eth.ExecutionPayloadEnvelope{
		ParentBeaconBlockRoot: sentAttributes.Attributes.ParentBeaconBlockRoot,
		ExecutionPayload: &eth.ExecutionPayload{
			ParentHash:   head.Hash,
			FeeRecipient: sentAttributes.Attributes.SuggestedFeeRecipient,
			BlockNumber:  eth.Uint64Quantity(sentAttributes.Parent.Number + 1),
			BlockHash:    common.Hash{0x12, 0x34},
			// Not all attributes matter to sequencer. We can leave these nil.
		},
	}
	payloadRef := eth.L2BlockRef{
		Hash:           payloadEnvelope.ExecutionPayload.BlockHash,
		Number:         uint64(payloadEnvelope.ExecutionPayload.BlockNumber),
		ParentHash:     payloadEnvelope.ExecutionPayload.ParentHash,
		Time:           uint64(payloadEnvelope.ExecutionPayload.Timestamp),
		L1Origin:       deps.l1OriginSelector.l1Origin.ID(),
		SequenceNumber: 0,
	}
	emitter.ExpectOnce(engine.PayloadProcessEvent{
		IsLastInSpan: false,
		DerivedFrom:  eth.L1BlockRef{},
		Envelope:     payloadEnvelope,
		Ref:          payloadRef,
	})
	// And report back the sealing result to the engine
	seq.OnEvent(engine.BuildSealedEvent{
		IsLastInSpan: false,
		DerivedFrom:  eth.L1BlockRef{},
		Info:         payloadInfo,
		Envelope:     payloadEnvelope,
		Ref:          payloadRef,
	})
	// The sequencer should start processing the payload
	emitter.AssertExpectations(t)
	// But also optimistically give it to the conductor and the async gossip
	require.Equal(t, payloadEnvelope, deps.conductor.committed, "must commit to conductor")
	require.Equal(t, payloadEnvelope, deps.asyncGossip.payload, "must send to async gossip")
	_, ok = seq.NextAction()
	require.False(t, ok, "optimistically published, but not ready to sequence next, until local processing completes")

	// Mock that the processing was successful
	seq.OnEvent(engine.PayloadSuccessEvent{
		IsLastInSpan: false,
		DerivedFrom:  eth.L1BlockRef{},
		Envelope:     payloadEnvelope,
		Ref:          payloadRef,
	})
	require.Nil(t, deps.asyncGossip.payload, "async gossip should have cleared,"+
		" after previous publishing and now having persisted the block ourselves")
	_, ok = seq.NextAction()
	require.False(t, ok, "published and processed, but not canonical yet. Cannot proceed until then.")

	// Once the forkchoice update identifies the processed block
	// as canonical we can proceed to the next sequencer cycle iteration.
	// Pretend we only completed processing the block 120 ms into the next block time window.
	// (This is why we publish optimistically)
	testClock.Set(time.Unix(int64(payloadRef.Time), 0).Add(time.Millisecond * 120))
	seq.OnEvent(engine.ForkchoiceUpdateEvent{
		UnsafeL2Head:    payloadRef,
		SafeL2Head:      eth.L2BlockRef{},
		FinalizedL2Head: eth.L2BlockRef{},
	})
	nextTime, ok := seq.NextAction()
	require.True(t, ok, "ready to build next block")
	require.Equal(t, testClock.Now(), nextTime, "start asap on the next block")
}

type sequencerTestDeps struct {
	cfg              *rollup.Config
	attribBuilder    *FakeAttributesBuilder
	l1OriginSelector *FakeL1OriginSelector
	seqState         *BasicSequencerStateListener
	conductor        *FakeConductor
	asyncGossip      *FakeAsyncGossip
}

func createSequencer(log log.Logger) (*Sequencer, *sequencerTestDeps) {
	cfg := chaincfg.Mainnet
	rng := rand.New(rand.NewSource(123))
	deps := &sequencerTestDeps{
		cfg:              cfg,
		attribBuilder:    &FakeAttributesBuilder{cfg: cfg, rng: rng},
		l1OriginSelector: &FakeL1OriginSelector{},
		seqState:         &BasicSequencerStateListener{},
		conductor:        &FakeConductor{},
		asyncGossip:      &FakeAsyncGossip{},
	}
	return NewSequencer(context.Background(), log, cfg, deps.attribBuilder,
		deps.l1OriginSelector, deps.seqState, deps.conductor,
		deps.asyncGossip, metrics.NoopMetrics), deps
}
