package finality

import (
	"context"
	"math/rand" // nosemgrep
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

type fakeAltDABackend struct {
	altDAFn   altda.HeadSignalFn
	forwardTo altda.HeadSignalFn
}

func (b *fakeAltDABackend) Finalize(ref eth.L1BlockRef) {
	b.altDAFn(ref)
}

func (b *fakeAltDABackend) OnFinalizedHeadSignal(f altda.HeadSignalFn) {
	b.forwardTo = f
}

var _ AltDABackend = (*fakeAltDABackend)(nil)

func TestAltDAFinalityData(t *testing.T) {
	logger := testlog.Logger(t, log.LevelInfo)
	l1F := &testutils.MockL1Source{}

	rng := rand.New(rand.NewSource(1234))

	refA := testutils.RandomBlockRef(rng)
	refA0 := eth.L2BlockRef{
		Hash:           testutils.RandomHash(rng),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           refA.Time,
		L1Origin:       refA.ID(),
		SequenceNumber: 0,
	}

	cfg := &rollup.Config{
		Genesis: rollup.Genesis{
			L1:     refA.ID(),
			L2:     refA0.ID(),
			L2Time: refA0.Time,
			SystemConfig: eth.SystemConfig{
				BatcherAddr: common.Address{42},
				Overhead:    [32]byte{123},
				Scalar:      [32]byte{42},
				GasLimit:    20_000_000,
			},
		},
		BlockTime:     1,
		SeqWindowSize: 2,
	}
	altDACfg := &rollup.AltDAConfig{
		DAChallengeWindow: 90,
		DAResolveWindow:   90,
	}
	// shoud return l1 finality if altda is not enabled
	require.Equal(t, uint64(defaultFinalityLookback), calcFinalityLookback(cfg))

	cfg.AltDAConfig = altDACfg
	expFinalityLookback := 181
	require.Equal(t, uint64(expFinalityLookback), calcFinalityLookback(cfg))

	refA1 := eth.L2BlockRef{
		Hash:           testutils.RandomHash(rng),
		Number:         refA0.Number + 1,
		ParentHash:     refA0.Hash,
		Time:           refA0.Time + cfg.BlockTime,
		L1Origin:       refA.ID(),
		SequenceNumber: 1,
	}

	// Simulate altda finality by waiting for the finalized-inclusion
	// of a commitment to turn into undisputed finalized data.
	commitmentInclusionFinalized := eth.L1BlockRef{}
	altDABackend := &fakeAltDABackend{
		altDAFn: func(ref eth.L1BlockRef) {
			commitmentInclusionFinalized = ref
		},
		forwardTo: nil,
	}

	emitter := &testutils.MockEmitter{}
	fi := NewAltDAFinalizer(context.Background(), logger, cfg, l1F, altDABackend)
	fi.AttachEmitter(emitter)
	require.NotNil(t, altDABackend.forwardTo, "altda backend must have access to underlying standard finalizer")

	require.Equal(t, expFinalityLookback, cap(fi.finalityData))

	l1parent := refA
	l2parent := refA1

	// advance over 200 l1 origins each time incrementing new l2 safe heads
	// and post processing.
	for i := uint64(0); i < 200; i++ {
		if i == 10 { // finalize a L1 commitment
			fi.OnEvent(FinalizeL1Event{FinalizedL1: l1parent})
			emitter.AssertExpectations(t) // no events emitted upon L1 finality
			require.Equal(t, l1parent, commitmentInclusionFinalized, "altda backend received L1 signal")
		}

		previous := l1parent
		l1parent = eth.L1BlockRef{
			Hash:       testutils.RandomHash(rng),
			Number:     previous.Number + 1,
			ParentHash: previous.Hash,
			Time:       previous.Time + 12,
		}

		for j := uint64(0); j < 2; j++ {
			l2parent = eth.L2BlockRef{
				Hash:           testutils.RandomHash(rng),
				Number:         l2parent.Number + 1,
				ParentHash:     l2parent.Hash,
				Time:           l2parent.Time + cfg.BlockTime,
				L1Origin:       previous.ID(), // reference previous origin, not the block the batch was included in
				SequenceNumber: j,
			}
			fi.OnEvent(engine.SafeDerivedEvent{Safe: l2parent, Source: l1parent})
			emitter.AssertExpectations(t)
		}
		// might trigger finalization attempt, if expired finality delay
		emitter.ExpectMaybeRun(func(ev event.Event) {
			require.IsType(t, TryFinalizeEvent{}, ev)
		})
		fi.OnEvent(derive.DeriverIdleEvent{})
		emitter.AssertExpectations(t)
		// clear expectations
		emitter.Mock.ExpectedCalls = nil

		// no L2 finalize event, as no L1 finality signal has been forwarded by altda backend yet
		fi.OnEvent(TryFinalizeEvent{})
		emitter.AssertExpectations(t)

		// Pretend to be the altda backend,
		// send the original finalization signal to the underlying finalizer,
		// now that we are sure the commitment itself is not just finalized,
		// but the referenced data cannot be disputed anymore.
		altdaFinalization := commitmentInclusionFinalized.Number + cfg.AltDAConfig.DAChallengeWindow
		if commitmentInclusionFinalized != (eth.L1BlockRef{}) && l1parent.Number == altdaFinalization {
			// When the signal is forwarded, a finalization attempt will be scheduled
			emitter.ExpectOnce(TryFinalizeEvent{})
			altDABackend.forwardTo(commitmentInclusionFinalized)
			emitter.AssertExpectations(t)
			require.Equal(t, commitmentInclusionFinalized, fi.finalizedL1, "finality signal now made its way in regular finalizer")

			// As soon as a finalization attempt is made, after the finality signal was triggered by altda backend,
			// we should get an attempt to get a finalized L2 block.
			// In this test the L1 origin of the simulated L2 blocks lags 1 behind the block the L2 block is included in on L1.
			// So to check the L2 finality progress, we check if the next L1 block after the L1 origin
			// of the safe block matches that of the finalized L1 block.
			l1F.ExpectL1BlockRefByNumber(commitmentInclusionFinalized.Number, commitmentInclusionFinalized, nil)
			l1F.ExpectL1BlockRefByNumber(commitmentInclusionFinalized.Number, commitmentInclusionFinalized, nil)
			var finalizedL2 eth.L2BlockRef
			emitter.ExpectOnceRun(func(ev event.Event) {
				if x, ok := ev.(engine.PromoteFinalizedEvent); ok {
					finalizedL2 = x.Ref
				} else {
					t.Fatalf("expected L2 finalization, but got: %s", ev)
				}
			})
			fi.OnEvent(TryFinalizeEvent{})
			l1F.AssertExpectations(t)
			emitter.AssertExpectations(t)
			require.Equal(t, commitmentInclusionFinalized.Number, finalizedL2.L1Origin.Number+1)
			// Confirm finalization, so there will be no repeats of the PromoteFinalizedEvent
			fi.OnEvent(engine.ForkchoiceUpdateEvent{FinalizedL2Head: finalizedL2})
			emitter.AssertExpectations(t)
		}
	}

	// finality data does not go over challenge + resolve windows + 1 capacity
	// (prunes down to 180 then adds the extra 1 each time)
	require.Equal(t, expFinalityLookback, len(fi.finalityData))
}
