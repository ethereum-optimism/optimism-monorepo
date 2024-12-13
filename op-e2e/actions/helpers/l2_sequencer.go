package helpers

import (
	"context"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-node/node/safedb"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/async"
	"github.com/ethereum-optimism/optimism/op-node/rollup/conductor"
	"github.com/ethereum-optimism/optimism/op-node/rollup/confdepth"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/interop"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sequencing"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// MockL1OriginSelector is a shim to override the origin as sequencer, so we can force it to stay on an older origin.
type MockL1OriginSelector struct {
	actual         *sequencing.L1OriginSelector
	originOverride eth.L1BlockRef // override which origin gets picked
}

func (m *MockL1OriginSelector) FindL1Origin(ctx context.Context, l2Head eth.L2BlockRef) (eth.L1BlockRef, error) {
	if m.originOverride != (eth.L1BlockRef{}) {
		return m.originOverride, nil
	}
	return m.actual.FindL1Origin(ctx, l2Head)
}

// L2Sequencer is an actor that functions like a rollup node,
// without the full P2P/API/Node stack, but just the derivation state, and simplified driver with sequencing ability.
type L2Sequencer struct {
	*L2Verifier

	sequencer   *sequencing.Sequencer
	attrBuilder *derive.FetchingAttributesBuilder

	failL2GossipUnsafeBlock error // mock error

	mockL1OriginSelector *MockL1OriginSelector
}

func NewL2Sequencer(t Testing, log log.Logger, l1 derive.L1Fetcher, blobSrc derive.L1BlobsFetcher,
	altDASrc driver.AltDAIface, eng L2API, cfg *rollup.Config, seqConfDepth uint64,
	interopBackend interop.InteropBackend,
) *L2Sequencer {
	ver := NewL2Verifier(t, log, l1, blobSrc, altDASrc, eng, cfg, &sync.Config{}, safedb.Disabled, interopBackend)
	attrBuilder := derive.NewFetchingAttributesBuilder(cfg, l1, eng)
	seqConfDepthL1 := confdepth.NewConfDepth(seqConfDepth, ver.syncStatus.L1Head, l1)
	originSelector := sequencing.NewL1OriginSelector(t.Ctx(), log, cfg, seqConfDepthL1)
	l1OriginSelector := &MockL1OriginSelector{
		actual: originSelector,
	}
	metr := metrics.NoopMetrics
	seqStateListener := node.DisabledConfigPersistence{}
	conduc := &conductor.NoOpConductor{}
	asyncGossip := async.NoOpGossiper{}
	seq := sequencing.NewSequencer(t.Ctx(), log, cfg, attrBuilder, l1OriginSelector,
		seqStateListener, conduc, asyncGossip, metr)
	opts := event.DefaultRegisterOpts()
	opts.Emitter = event.EmitterOpts{
		Limiting: true,
		// TestSyncBatchType/DerivationWithFlakyL1RPC does *a lot* of quick retries
		// TestL2BatcherBatchType/ExtendedTimeWithoutL1Batches as well.
		Rate:  rate.Limit(100_000),
		Burst: 100_000,
		OnLimited: func() {
			log.Warn("Hitting events rate-limit. An events code-path may be hot-looping.")
			t.Fatal("Tests must not hot-loop events")
		},
	}
	ver.eventSys.Register("sequencer", seq, opts)
	ver.eventSys.Register("origin-selector", originSelector, opts)
	require.NoError(t, seq.Init(t.Ctx(), true))
	return &L2Sequencer{
		L2Verifier:              ver,
		sequencer:               seq,
		attrBuilder:             attrBuilder,
		mockL1OriginSelector:    l1OriginSelector,
		failL2GossipUnsafeBlock: nil,
	}
}

// ActL2StartBlock starts building of a new L2 block on top of the head
func (s *L2Sequencer) ActL2StartBlock(t Testing) {
	if !s.L2PipelineIdle {
		t.InvalidAction("cannot start L2 build when derivation is not idle")
		return
	}
	if s.l2Building {
		t.InvalidAction("already started building L2 block")
		return
	}
	s.synchronousEvents.Emit(sequencing.SequencerActionEvent{})
	require.NoError(t, s.drainer.DrainUntil(event.Is[engine.BuildStartedEvent], false),
		"failed to start block building")

	s.l2Building = true
}

// ActL2EndBlock completes a new L2 block and applies it to the L2 chain as new canonical unsafe head
func (s *L2Sequencer) ActL2EndBlock(t Testing) {
	if !s.l2Building {
		t.InvalidAction("cannot end L2 block building when no block is being built")
		return
	}
	s.l2Building = false

	s.synchronousEvents.Emit(sequencing.SequencerActionEvent{})
	require.NoError(t, s.drainer.DrainUntil(event.Is[engine.PayloadSuccessEvent], false),
		"failed to complete block building")

	// After having built a L2 block, make sure to get an engine update processed.
	// This will ensure the sync-status and such reflect the latest changes.
	s.synchronousEvents.Emit(engine.TryUpdateEngineEvent{})
	s.synchronousEvents.Emit(engine.ForkchoiceRequestEvent{})
	require.NoError(t, s.drainer.DrainUntil(func(ev event.Event) bool {
		x, ok := ev.(engine.ForkchoiceUpdateEvent)
		return ok && x.UnsafeL2Head == s.engine.UnsafeL2Head()
	}, false))
	require.Equal(t, s.engine.UnsafeL2Head(), s.syncStatus.SyncStatus().UnsafeL2,
		"sync status must be accurate after block building")
}

func (s *L2Sequencer) ActL2EmptyBlock(t Testing) {
	s.ActL2StartBlock(t)
	s.ActL2EndBlock(t)
}

// ActL2KeepL1Origin makes the sequencer use the current L1 origin, even if the next origin is available.
func (s *L2Sequencer) ActL2KeepL1Origin(t Testing) {
	parent := s.engine.UnsafeL2Head()
	// force old origin
	oldOrigin, err := s.l1.L1BlockRefByHash(t.Ctx(), parent.L1Origin.Hash)
	require.NoError(t, err, "failed to get current origin: %s", parent.L1Origin)
	s.mockL1OriginSelector.originOverride = oldOrigin
}

// ActL2ForceAdvanceL1Origin forces the sequencer to advance the current L1 origin, even if the next origin's timestamp is too new.
func (s *L2Sequencer) ActL2ForceAdvanceL1Origin(t Testing) {
	s.attrBuilder.TestSkipL1OriginCheck() // skip check in attributes builder
	parent := s.engine.UnsafeL2Head()
	// force next origin
	nextNum := parent.L1Origin.Number + 1
	nextOrigin, err := s.l1.L1BlockRefByNumber(t.Ctx(), nextNum)
	require.NoError(t, err, "failed to get next origin by number: %d", nextNum)
	s.mockL1OriginSelector.originOverride = nextOrigin
}

// ActBuildToL1Head builds empty blocks until (incl.) the L1 head becomes the L2 origin
func (s *L2Sequencer) ActBuildToL1Head(t Testing) {
	for s.engine.UnsafeL2Head().L1Origin.Number < s.syncStatus.L1Head().Number {
		s.ActL2PipelineFull(t)
		s.ActL2EmptyBlock(t)
	}
}

// ActBuildToL1HeadUnsafe builds empty blocks until (incl.) the L1 head becomes the L1 origin of the L2 head
func (s *L2Sequencer) ActBuildToL1HeadUnsafe(t Testing) {
	for s.engine.UnsafeL2Head().L1Origin.Number < s.syncStatus.L1Head().Number {
		// Note: the derivation pipeline does not run, we are just sequencing a block on top of the existing L2 chain.
		s.ActL2EmptyBlock(t)
	}
}

// ActBuildToL1HeadExcl builds empty blocks until (excl.) the L1 head becomes the L1 origin of the L2 head
func (s *L2Sequencer) ActBuildToL1HeadExcl(t Testing) {
	for {
		s.ActL2PipelineFull(t)
		nextOrigin, err := s.mockL1OriginSelector.FindL1Origin(t.Ctx(), s.engine.UnsafeL2Head())
		require.NoError(t, err)
		if nextOrigin.Number >= s.syncStatus.L1Head().Number {
			break
		}
		s.ActL2EmptyBlock(t)
	}
}

// ActBuildToL1HeadExclUnsafe builds empty blocks until (excl.) the L1 head becomes the L1 origin of the L2 head, without safe-head progression.
func (s *L2Sequencer) ActBuildToL1HeadExclUnsafe(t Testing) {
	for {
		// Note: the derivation pipeline does not run, we are just sequencing a block on top of the existing L2 chain.
		nextOrigin, err := s.mockL1OriginSelector.FindL1Origin(t.Ctx(), s.engine.UnsafeL2Head())
		require.NoError(t, err)
		if nextOrigin.Number >= s.syncStatus.L1Head().Number {
			break
		}
		s.ActL2EmptyBlock(t)
	}
}

func (s *L2Sequencer) ActBuildL2ToTime(t Testing, target uint64) {
	for s.L2Unsafe().Time < target {
		s.ActL2EmptyBlock(t)
	}
}

func (s *L2Sequencer) ActBuildL2ToEcotone(t Testing) {
	require.NotNil(t, s.RollupCfg.EcotoneTime, "cannot activate Ecotone when it is not scheduled")
	for s.L2Unsafe().Time < *s.RollupCfg.EcotoneTime {
		s.ActL2EmptyBlock(t)
	}
}

func (s *L2Sequencer) ActBuildL2ToFjord(t Testing) {
	require.NotNil(t, s.RollupCfg.FjordTime, "cannot activate FjordTime when it is not scheduled")
	for s.L2Unsafe().Time < *s.RollupCfg.FjordTime {
		s.ActL2EmptyBlock(t)
	}
}

func (s *L2Sequencer) ActBuildL2ToGranite(t Testing) {
	require.NotNil(t, s.RollupCfg.GraniteTime, "cannot activate GraniteTime when it is not scheduled")
	for s.L2Unsafe().Time < *s.RollupCfg.GraniteTime {
		s.ActL2EmptyBlock(t)
	}
}

func (s *L2Sequencer) ActBuildL2ToHolocene(t Testing) {
	require.NotNil(t, s.RollupCfg.HoloceneTime, "cannot activate HoloceneTime when it is not scheduled")
	for s.L2Unsafe().Time < *s.RollupCfg.HoloceneTime {
		s.ActL2EmptyBlock(t)
	}
}
