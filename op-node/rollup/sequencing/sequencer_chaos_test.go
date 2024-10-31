package sequencing

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand" // nosemgrep
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

// ChaoticEngine simulates what the Engine deriver would do, upon events from the sequencer.
// But does so with repeated errors and bad time delays.
// It is up to the sequencer code to recover from the errors and keep the
// onchain time accurate to the simulated offchain time.
type ChaoticEngine struct {
	t *testing.T

	rng *rand.Rand

	emitter event.Emitter

	clock interface {
		Now() time.Time
		Set(t time.Time)
	}

	deps *sequencerTestDeps

	currentPayloadInfo eth.PayloadInfo
	currentAttributes  *derive.AttributesWithParent

	unsafe, safe, finalized eth.L2BlockRef
}

func (c *ChaoticEngine) clockRandomIncrement(minIncr, maxIncr time.Duration) {
	require.LessOrEqual(c.t, minIncr, maxIncr, "sanity check time duration range")
	incr := minIncr + time.Duration(c.rng.Int63n(int64(maxIncr-minIncr)))
	c.clock.Set(c.clock.Now().Add(incr))
}

func (c *ChaoticEngine) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case engine.BuildStartEvent:
		c.currentPayloadInfo = eth.PayloadInfo{}
		// init new payload building ID
		_, err := c.rng.Read(c.currentPayloadInfo.ID[:])
		require.NoError(c.t, err)
		c.currentPayloadInfo.Timestamp = uint64(x.Attributes.Attributes.Timestamp)

		// Move forward time, to simulate time consumption
		c.clockRandomIncrement(0, time.Millisecond*300)
		if c.rng.Intn(10) == 0 { // 10% chance the block start is slow
			c.clockRandomIncrement(0, time.Second*2)
		}

		p := c.rng.Float32()
		switch {
		case p < 0.05: // 5%
			c.emitter.Emit(engine.BuildInvalidEvent{
				Attributes: x.Attributes,
				Err:        errors.New("mock start invalid error"),
			})
		case p < 0.07: // 2 %
			c.emitter.Emit(rollup.ResetEvent{
				Err: errors.New("mock reset on start error"),
			})
		case p < 0.12: // 5%
			c.emitter.Emit(rollup.EngineTemporaryErrorEvent{
				Err: errors.New("mock temp start error"),
			})
		default:
			c.currentAttributes = x.Attributes
			c.emitter.Emit(engine.BuildStartedEvent{
				Info:         c.currentPayloadInfo,
				BuildStarted: c.clock.Now(),
				Parent:       x.Attributes.Parent,
				Concluding:   false,
				DerivedFrom:  eth.L1BlockRef{},
			})
		}
	case rollup.EngineTemporaryErrorEvent:
		c.clockRandomIncrement(0, time.Millisecond*100)
		c.currentPayloadInfo = eth.PayloadInfo{}
		c.currentAttributes = nil
	case rollup.ResetEvent:
		// In real-world the reset may take even longer,
		// but then there are also less random errors and delays thrown from the engine after.
		// Here we keep the delay relatively small, to keep possible random diff between chain and wallclock smaller.
		c.clockRandomIncrement(0, time.Second*4)
		c.currentPayloadInfo = eth.PayloadInfo{}
		c.currentAttributes = nil
		c.emitter.Emit(engine.EngineResetConfirmedEvent{
			Unsafe:    c.unsafe,
			Safe:      c.safe,
			Finalized: c.finalized,
		})
	case engine.BuildInvalidEvent:
		// Engine translates the internal BuildInvalidEvent event
		// to the external sequencer-handled InvalidPayloadAttributesEvent.
		c.clockRandomIncrement(0, time.Millisecond*50)
		c.currentPayloadInfo = eth.PayloadInfo{}
		c.currentAttributes = nil
		c.emitter.Emit(engine.InvalidPayloadAttributesEvent(x))
	case engine.BuildSealEvent:
		// Move forward time, to simulate time consumption on sealing
		c.clockRandomIncrement(0, time.Millisecond*300)

		if c.currentPayloadInfo == (eth.PayloadInfo{}) {
			c.emitter.Emit(engine.PayloadSealExpiredErrorEvent{
				Info:        x.Info,
				Err:         errors.New("job was cancelled"),
				Concluding:  false,
				DerivedFrom: eth.L1BlockRef{},
			})
			return true
		}
		require.Equal(c.t, c.currentPayloadInfo, x.Info, "seal the current payload")
		require.NotNil(c.t, c.currentAttributes, "must have started building")

		if c.rng.Intn(20) == 0 { // 5% chance of terribly slow block building hiccup
			c.clockRandomIncrement(0, time.Second*3)
		}

		p := c.rng.Float32()
		switch {
		case p < 0.03: // 3%
			c.emitter.Emit(engine.PayloadSealInvalidEvent{
				Info:        x.Info,
				Err:         errors.New("mock invalid seal"),
				Concluding:  x.Concluding,
				DerivedFrom: x.DerivedFrom,
			})
		case p < 0.08: // 5%
			c.emitter.Emit(engine.PayloadSealExpiredErrorEvent{
				Info:        x.Info,
				Err:         errors.New("mock temp engine error"),
				Concluding:  x.Concluding,
				DerivedFrom: x.DerivedFrom,
			})
		default:
			payloadEnvelope := &eth.ExecutionPayloadEnvelope{
				ParentBeaconBlockRoot: c.currentAttributes.Attributes.ParentBeaconBlockRoot,
				ExecutionPayload: &eth.ExecutionPayload{
					ParentHash:   c.currentAttributes.Parent.Hash,
					FeeRecipient: c.currentAttributes.Attributes.SuggestedFeeRecipient,
					BlockNumber:  eth.Uint64Quantity(c.currentAttributes.Parent.Number + 1),
					BlockHash:    testutils.RandomHash(c.rng),
					Timestamp:    c.currentAttributes.Attributes.Timestamp,
					Transactions: c.currentAttributes.Attributes.Transactions,
					// Not all attributes matter to sequencer. We can leave these nil.
				},
			}
			// We encode the L1 origin as block-ID in tx[0] for testing.
			l1Origin := decodeID(c.currentAttributes.Attributes.Transactions[0])
			payloadRef := eth.L2BlockRef{
				Hash:           payloadEnvelope.ExecutionPayload.BlockHash,
				Number:         uint64(payloadEnvelope.ExecutionPayload.BlockNumber),
				ParentHash:     payloadEnvelope.ExecutionPayload.ParentHash,
				Time:           uint64(payloadEnvelope.ExecutionPayload.Timestamp),
				L1Origin:       l1Origin,
				SequenceNumber: 0, // ignored
			}
			c.emitter.Emit(engine.BuildSealedEvent{
				Info:        x.Info,
				Envelope:    payloadEnvelope,
				Ref:         payloadRef,
				Concluding:  x.Concluding,
				DerivedFrom: x.DerivedFrom,
			})
		}
		c.currentPayloadInfo = eth.PayloadInfo{}
		c.currentAttributes = nil
	case engine.BuildCancelEvent:
		c.currentPayloadInfo = eth.PayloadInfo{}
		c.currentAttributes = nil
	case engine.ForkchoiceRequestEvent:
		c.emitter.Emit(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    c.unsafe,
			SafeL2Head:      c.safe,
			FinalizedL2Head: c.finalized,
		})
	case engine.PayloadProcessEvent:
		// Move forward time, to simulate time consumption
		c.clockRandomIncrement(0, time.Millisecond*500)

		p := c.rng.Float32()
		switch {
		case p < 0.05: // 5%
			c.emitter.Emit(rollup.EngineTemporaryErrorEvent{
				Err: errors.New("mock temp engine error"),
			})
		case p < 0.08: // 3%
			c.emitter.Emit(engine.PayloadInvalidEvent{
				Envelope: x.Envelope,
				Err:      errors.New("mock invalid payload"),
			})
		default:
			if p < 0.13 { // 5% chance it is an extra slow block
				c.clockRandomIncrement(0, time.Second*3)
			}
			c.unsafe = x.Ref
			c.emitter.Emit(engine.PayloadSuccessEvent(x))
			// With event delay, the engine would update and signal the new forkchoice.
			c.emitter.Emit(engine.ForkchoiceRequestEvent{})
		}
	default:
		return false
	}
	return true
}

func (c *ChaoticEngine) AttachEmitter(em event.Emitter) {
	c.emitter = em
}

var _ event.Deriver = (*ChaoticEngine)(nil)

// TestSequencerChaos runs the sequencer with a simulated engine,
// mocking different kinds of errors and timing issues.
func TestSequencerChaos(t *testing.T) {
	for i := int64(1); i < 100; i++ {
		t.Run(fmt.Sprintf("simulation-%d", i), func(t *testing.T) {
			testSequencerChaosWithSeed(t, i)
		})
	}
}

func testSequencerChaosWithSeed(t *testing.T, seed int64) {
	// Lower the log level to inspect the mocked errors and event-traces.
	logger := testlog.Logger(t, log.LevelCrit)
	seq, deps := createSequencer(logger)
	testClock := clock.NewSimpleClock()
	testClock.SetTime(deps.cfg.Genesis.L2Time)
	seq.timeNow = testClock.Now
	emitter := &testutils.MockEmitter{}
	seq.AttachEmitter(emitter)
	ex := event.NewGlobalSynchronous(context.Background())
	sys := event.NewSystem(logger, ex)
	sys.AddTracer(event.NewLogTracer(logger, log.LevelInfo))

	opts := &event.RegisterOpts{
		Executor: event.ExecutorOpts{
			Capacity: 200,
		},
		Emitter: event.EmitterOpts{
			Limiting: false, // We're rapidly simulating with fake clock, so don't rate-limit
		},
	}
	sys.Register("sequencer", seq, opts)

	rng := rand.New(rand.NewSource(seed))
	genesisRef := eth.L2BlockRef{
		Hash:           deps.cfg.Genesis.L2.Hash,
		Number:         deps.cfg.Genesis.L2.Number,
		ParentHash:     common.Hash{},
		Time:           deps.cfg.Genesis.L2Time,
		L1Origin:       deps.cfg.Genesis.L1,
		SequenceNumber: 0,
	}
	var l1OriginSelectErr error
	l1BlockHash := func(num uint64) (out common.Hash) {
		out[0] = 1
		binary.BigEndian.PutUint64(out[32-8:], num)
		return
	}
	deps.l1OriginSelector.l1OriginFn = func(l2Head eth.L2BlockRef) (eth.L1BlockRef, error) {
		if l1OriginSelectErr != nil {
			return eth.L1BlockRef{}, l1OriginSelectErr
		}
		if l2Head.Number == genesisRef.Number {
			return eth.L1BlockRef{
				Hash:       genesisRef.L1Origin.Hash,
				Number:     genesisRef.L1Origin.Number,
				Time:       genesisRef.Time,
				ParentHash: common.Hash{},
			}, nil
		}
		origin := eth.L1BlockRef{
			Hash:       l2Head.L1Origin.Hash,
			Number:     l2Head.L1Origin.Number,
			ParentHash: l1BlockHash(l2Head.L1Origin.Number - 1),
			Time:       genesisRef.Time + (l2Head.L1Origin.Number-genesisRef.L1Origin.Number)*12,
		}
		// Handle sequencer time drift, by proceeding to the next L1 origin when we run out of valid time
		if l2Head.Time+deps.cfg.BlockTime > origin.Time+deps.cfg.MaxSequencerDrift {
			origin.Number += 1
			origin.ParentHash = origin.Hash
			origin.Hash = l1BlockHash(origin.Number)
			origin.Time += 12
		}
		return origin, nil
	}
	eng := &ChaoticEngine{
		t:         t,
		rng:       rng,
		clock:     testClock,
		deps:      deps,
		finalized: genesisRef,
		safe:      genesisRef,
		unsafe:    genesisRef,
	}
	sys.Register("engine", eng, opts)
	testEm := sys.Register("test", nil, opts)

	// Init sequencer, as active
	require.NoError(t, seq.Init(context.Background(), true))
	require.NoError(t, ex.Drain(), "initial forkchoice update etc. completes")

	genesisTime := time.Unix(int64(deps.cfg.Genesis.L2Time), 0)

	i := 0
	// If we can't sequence 100 blocks in 1k simulation steps, something is wrong.
	sanityCap := 1000
	targetBlocks := uint64(100)
	// sequence a lot of blocks, against the chaos engine
	for eng.unsafe.Number < deps.cfg.Genesis.L2.Number+targetBlocks && i < sanityCap {
		simPast := eng.clock.Now().Sub(genesisTime)
		onchainPast := time.Unix(int64(eng.unsafe.Time), 0).Sub(genesisTime)
		logger.Info("Simulation step", "i", i, "sim_time", simPast,
			"onchain_time", onchainPast,
			"relative", simPast-onchainPast, "blocks", eng.unsafe.Number-deps.cfg.Genesis.L2.Number)

		eng.clockRandomIncrement(0, time.Millisecond*10)

		// Consume a random amount of events. Take a 10% chance to stop at an event without continuing draining (!!!).
		// If using a synchronous executor it would be completely drained during regular operation,
		// but once we use a parallel executor in the actual op-node Driver,
		// then there may be unprocessed events before checking the next scheduled sequencing action.
		// What makes this difficult for the sequencer is that it may decide to emit a sequencer-action,
		// while previous emitted events are not processed yet. This helps identify bad state dependency assumptions.
		drainErr := ex.DrainUntil(func(ev event.Event) bool {
			return rng.Intn(10) == 0
		}, false)

		nextTime, ok := seq.NextAction()
		if drainErr == io.EOF && !ok {
			t.Fatalf("No action scheduled, but also no events to change inputs left")
		}
		if ok && testClock.Now().After(nextTime) {
			testEm.Emit(SequencerActionEvent{})
		} else {
			waitTime := nextTime.Sub(eng.clock.Now())
			if drainErr == io.EOF {
				logger.Info("No events left, skipping forward to next sequencing action", "wait", waitTime)
				// if no events are left, then we can deterministically skip forward to where we are ready
				// to process sequencing actions again. With some noise, to not skip exactly to the perfect time.
				eng.clockRandomIncrement(waitTime, waitTime+time.Millisecond*10)
			} else {
				logger.Info("Not sequencing time yet, processing more events first", "wait", waitTime)
			}
		}

		i += 1
	}

	blocksSinceGenesis := eng.unsafe.Number - deps.cfg.Genesis.L2.Number
	if i >= sanityCap {
		t.Fatalf("Sequenced %d blocks, ran out of simulation steps", blocksSinceGenesis)
	}
	require.Equal(t, targetBlocks, blocksSinceGenesis)

	now := testClock.Now()
	timeSinceGenesis := now.Sub(genesisTime)
	idealTimeSinceGenesis := time.Duration(blocksSinceGenesis*deps.cfg.BlockTime) * time.Second
	diff := timeSinceGenesis - idealTimeSinceGenesis
	// If timing keeps adjusting, even with many errors over time, it should stay close to target.
	if diff.Abs() > time.Second*20 {
		t.Fatalf("Failed to maintain target time. Spent %s, but target was %s",
			timeSinceGenesis, idealTimeSinceGenesis)
	}
}
