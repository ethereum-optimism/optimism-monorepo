package interop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup/interop"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

var _ interop.InteropBackend = (*testutils.FakeInteropBackend)(nil)

func TestInteropVerifier(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	// Temporary work-around: interop needs to be active, for cross-safety to not be instant.
	// The state genesis in this test is pre-interop however.
	sd.RollupCfg.InteropTime = new(uint64)
	logger := testlog.Logger(t, log.LevelDebug)
	seqMockBackend := &testutils.FakeInteropBackend{}
	l1Miner, seqEng, seq := helpers.SetupSequencerTest(t, sd, logger,
		helpers.WithVerifierOpts(helpers.WithInteropBackend(seqMockBackend)))

	batcher := helpers.NewL2Batcher(logger, sd.RollupCfg, helpers.DefaultBatcherCfg(dp),
		seq.RollupClient(), l1Miner.EthClient(), seqEng.EthClient(), seqEng.EngineClient(t, sd.RollupCfg))

	verMockBackend := &testutils.FakeInteropBackend{}
	_, ver := helpers.SetupVerifier(t, sd, logger,
		l1Miner.L1Client(t, sd.RollupCfg), l1Miner.BlobStore(), &sync.Config{},
		helpers.WithInteropBackend(verMockBackend))

	// Genesis block will be registered as local-safe when we first trigger the derivation pipeline
	seqMockBackend.UpdateLocalSafeFn = func(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.L2BlockRef) error {
		require.Equal(t, sd.RollupCfg.Genesis.L1, derivedFrom.ID())
		require.Equal(t, sd.RollupCfg.Genesis.L2, lastDerived.ID())
		return nil
	}
	verMockBackend.UpdateLocalSafeFn = func(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.L2BlockRef) error {
		require.Equal(t, sd.RollupCfg.Genesis.L1, derivedFrom.ID())
		require.Equal(t, sd.RollupCfg.Genesis.L2, lastDerived.ID())
		return nil
	}

	seq.ActL2PipelineFull(t)
	ver.ActL2PipelineFull(t)

	l2ChainID := types.ChainIDFromBig(sd.RollupCfg.L2ChainID)
	seqMockBackend.UpdateLocalUnsafeFn = func(ctx context.Context, chainID types.ChainID, head eth.L2BlockRef) error {
		require.Equal(t, chainID, l2ChainID)
		require.Equal(t, uint64(1), head.Number)
		return nil
	}
	seqMockBackend.UnsafeViewFn = func(ctx context.Context, chainID types.ChainID, unsafe types.ReferenceView) (types.ReferenceView, error) {
		require.Equal(t, chainID, l2ChainID)
		require.Equal(t, uint64(1), unsafe.Local.Number)
		require.Equal(t, uint64(0), unsafe.Cross.Number)
		return unsafe, nil
	}
	// create an unsafe L2 block
	seq.ActL2StartBlock(t)
	seq.ActL2EndBlock(t)
	seq.ActL2PipelineFull(t)
	status := seq.SyncStatus()
	require.Equal(t, uint64(1), status.UnsafeL2.Number)
	require.Equal(t, uint64(0), status.CrossUnsafeL2.Number)
	require.Equal(t, uint64(0), status.LocalSafeL2.Number)
	require.Equal(t, uint64(0), status.SafeL2.Number)

	// promote it to cross-unsafe in the backend
	// and see if the node picks up on it
	seqMockBackend.UnsafeViewFn = func(ctx context.Context, chainID types.ChainID, unsafe types.ReferenceView) (types.ReferenceView, error) {
		require.Equal(t, chainID, l2ChainID)
		require.Equal(t, uint64(1), unsafe.Local.Number)
		require.Equal(t, uint64(0), unsafe.Cross.Number)
		out := unsafe
		out.Cross = unsafe.Local
		return out, nil
	}
	seq.ActInteropBackendCheck(t)
	seq.ActL2PipelineFull(t)
	status = seq.SyncStatus()
	require.Equal(t, uint64(1), status.UnsafeL2.Number)
	require.Equal(t, uint64(1), status.CrossUnsafeL2.Number, "cross unsafe now")
	require.Equal(t, uint64(0), status.LocalSafeL2.Number)
	require.Equal(t, uint64(0), status.SafeL2.Number)

	// submit all new L2 blocks
	batcher.ActSubmitAll(t)
	// new L1 block with L2 batch
	l1Miner.ActL1StartBlock(12)(t)
	l1Miner.ActL1IncludeTx(sd.RollupCfg.Genesis.SystemConfig.BatcherAddr)(t)
	l1Miner.ActL1EndBlock(t)

	// Sync the L1 block, to verify the L2 block as local-safe.
	seqMockBackend.UpdateLocalUnsafeFn = nil
	nextL2 := uint64(0)
	seqMockBackend.UpdateLocalSafeFn = func(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.L2BlockRef) error {
		require.Equal(t, nextL2, lastDerived.Number)
		nextL2 += 1
		return nil
	}
	seqMockBackend.SafeViewFn = func(ctx context.Context, chainID types.ChainID, safe types.ReferenceView) (types.ReferenceView, error) {
		require.Equal(t, chainID, l2ChainID)
		require.Equal(t, uint64(1), safe.Local.Number)
		require.Equal(t, uint64(0), safe.Cross.Number)
		return safe, nil
	}
	seq.ActL1HeadSignal(t)
	l1Head := seq.SyncStatus().HeadL1
	seq.ActL2PipelineFull(t)
	require.Equal(t, uint64(2), nextL2)

	status = seq.SyncStatus()
	require.Equal(t, uint64(1), status.UnsafeL2.Number)
	require.Equal(t, uint64(1), status.CrossUnsafeL2.Number)
	require.Equal(t, uint64(1), status.LocalSafeL2.Number, "local safe changed")
	require.Equal(t, uint64(0), status.SafeL2.Number)

	// Now mark it as cross-safe
	seqMockBackend.SafeViewFn = func(ctx context.Context, chainID types.ChainID, request types.ReferenceView) (types.ReferenceView, error) {
		require.Equal(t, chainID, l2ChainID)
		require.Equal(t, uint64(1), request.Local.Number)
		require.Equal(t, uint64(0), request.Cross.Number)
		out := request
		out.Cross = request.Local
		return out, nil
	}
	seqMockBackend.DerivedFromFn = func(ctx context.Context, chainID types.ChainID, blockHash common.Hash, blockNumber uint64) (eth.L1BlockRef, error) {
		require.Equal(t, uint64(1), blockNumber)
		return l1Head, nil
	}
	seqMockBackend.FinalizedFn = func(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
		return seq.RollupCfg.Genesis.L1, nil
	}
	seq.ActInteropBackendCheck(t)
	seq.ActL2PipelineFull(t)

	status = seq.SyncStatus()
	require.Equal(t, uint64(1), status.UnsafeL2.Number)
	require.Equal(t, uint64(1), status.CrossUnsafeL2.Number)
	require.Equal(t, uint64(1), status.LocalSafeL2.Number)
	require.Equal(t, uint64(1), status.SafeL2.Number, "cross-safe reached")
	require.Equal(t, uint64(0), status.FinalizedL2.Number)

	verMockBackend.UpdateLocalUnsafeFn = func(ctx context.Context, chainID types.ChainID, head eth.L2BlockRef) error {
		require.Equal(t, uint64(1), head.Number)
		return nil
	}
	nextL2 = 0
	verMockBackend.UpdateLocalSafeFn = func(ctx context.Context, chainID types.ChainID, derivedFrom eth.L1BlockRef, lastDerived eth.L2BlockRef) error {
		require.Equal(t, nextL2, lastDerived.Number)
		nextL2 += 1
		require.Equal(t, l1Head.ID(), derivedFrom.ID())
		return nil
	}
	// The verifier might not see the L2 block that was just derived from L1 as cross-verified yet.
	verMockBackend.UnsafeViewFn = func(ctx context.Context, chainID types.ChainID, request types.ReferenceView) (types.ReferenceView, error) {
		require.Equal(t, uint64(1), request.Local.Number)
		require.Equal(t, uint64(0), request.Cross.Number)
		// Don't promote the Cross value yet
		return request, nil
	}
	verMockBackend.SafeViewFn = func(ctx context.Context, chainID types.ChainID, request types.ReferenceView) (types.ReferenceView, error) {
		require.Equal(t, uint64(1), request.Local.Number)
		require.Equal(t, uint64(0), request.Cross.Number)
		// Don't promote the Cross value yet
		return request, nil
	}
	ver.ActL1HeadSignal(t)
	ver.ActL2PipelineFull(t)
	require.Equal(t, uint64(2), nextL2)
	status = ver.SyncStatus()
	require.Equal(t, uint64(1), status.UnsafeL2.Number, "synced the block")
	require.Equal(t, uint64(0), status.CrossUnsafeL2.Number, "not cross-verified yet")
	require.Equal(t, uint64(1), status.LocalSafeL2.Number, "derived from L1, thus local-safe")
	require.Equal(t, uint64(0), status.SafeL2.Number, "not yet cross-safe")
	require.Equal(t, uint64(0), status.FinalizedL2.Number)

	seqMockBackend.UpdateFinalizedL1Fn = func(ctx context.Context, chainID types.ChainID, finalized eth.L1BlockRef) error {
		require.Equal(t, l1Head, finalized)
		return nil
	}
	// signal that L1 finalized; the cross-safe block we have should get finalized too
	l1Miner.ActL1SafeNext(t)
	l1Miner.ActL1FinalizeNext(t)
	seq.ActL1SafeSignal(t)
	seq.ActL1FinalizedSignal(t)
	seq.ActL2PipelineFull(t)

	status = seq.SyncStatus()
	require.Equal(t, uint64(1), status.FinalizedL2.Number, "finalized the block")
}
