package sync

import (
	"errors"
	"math/big"
	"math/rand"
	"strings"
	"testing"
	"time"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	upgradesHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/upgrades/helpers"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	gethengine "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	engine2 "github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func newSpanChannelOut(t actionsHelpers.StatefulTesting, e e2eutils.SetupData) derive.ChannelOut {
	channelOut, err := derive.NewSpanChannelOut(128_000, derive.Zlib, rollup.NewChainSpec(e.RollupCfg))
	require.NoError(t, err)
	return channelOut
}

// TestSyncBatchType run each sync test case in singular batch mode and span batch mode.
func TestSyncBatchType(t *testing.T) {
	tests := []struct {
		name string
		f    func(gt *testing.T, deltaTimeOffset *hexutil.Uint64)
	}{
		{"DerivationWithFlakyL1RPC", DerivationWithFlakyL1RPC},
		{"FinalizeWhileSyncing", FinalizeWhileSyncing},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name+"_SingularBatch", func(t *testing.T) {
			test.f(t, nil)
		})
	}

	deltaTimeOffset := hexutil.Uint64(0)
	for _, test := range tests {
		test := test
		t.Run(test.name+"_SpanBatch", func(t *testing.T) {
			test.f(t, &deltaTimeOffset)
		})
	}
}

func DerivationWithFlakyL1RPC(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	upgradesHelpers.ApplyDeltaTimeOffset(dp, deltaTimeOffset)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelError) // mute all the temporary derivation errors that we forcefully create
	_, _, miner, sequencer, _, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)

	rng := rand.New(rand.NewSource(1234))
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// build a L1 chain with 20 blocks and matching L2 chain and batches to test some derivation work
	miner.ActEmptyBlock(t)
	for i := 0; i < 20; i++ {
		sequencer.ActL1HeadSignal(t)
		sequencer.ActL2PipelineFull(t)
		sequencer.ActBuildToL1Head(t)
		batcher.ActSubmitAll(t)
		miner.ActL1StartBlock(12)(t)
		miner.ActL1IncludeTx(batcher.BatcherAddr)(t)
		miner.ActL1EndBlock(t)
	}
	// Make verifier aware of head
	verifier.ActL1HeadSignal(t)

	// Now make the L1 RPC very flaky: requests will randomly fail with 50% chance
	miner.MockL1RPCErrors(func() error {
		if rng.Intn(2) == 0 {
			return errors.New("mock rpc error")
		}
		return nil
	})

	// And sync the verifier
	verifier.ActL2PipelineFull(t)
	// Verifier should be synced, even though it hit lots of temporary L1 RPC errors
	require.Equal(t, sequencer.L2Unsafe(), verifier.L2Safe(), "verifier is synced")
}

func FinalizeWhileSyncing(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	upgradesHelpers.ApplyDeltaTimeOffset(dp, deltaTimeOffset)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelError) // mute all the temporary derivation errors that we forcefully create
	_, _, miner, sequencer, _, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	verifierStartStatus := verifier.SyncStatus()

	// Build an L1 chain with 64 + 1 blocks, containing batches of L2 chain.
	// Enough to go past the finalityDelay of the engine queue,
	// to make the verifier finalize while it syncs.
	miner.ActEmptyBlock(t)
	for i := 0; i < 64+1; i++ {
		sequencer.ActL1HeadSignal(t)
		sequencer.ActL2PipelineFull(t)
		sequencer.ActBuildToL1Head(t)
		batcher.ActSubmitAll(t)
		miner.ActL1StartBlock(12)(t)
		miner.ActL1IncludeTx(batcher.BatcherAddr)(t)
		miner.ActL1EndBlock(t)
	}
	l1Head := miner.L1Chain().CurrentHeader()
	// finalize all of L1
	miner.ActL1Safe(t, l1Head.Number.Uint64())
	miner.ActL1Finalize(t, l1Head.Number.Uint64())

	// Now signal L1 finality to the verifier, while the verifier is not synced.
	verifier.ActL1HeadSignal(t)
	verifier.ActL1SafeSignal(t)
	verifier.ActL1FinalizedSignal(t)

	// Now sync the verifier, without repeating the signal.
	// While it's syncing, it should finalize on interval now, based on the future L1 finalized block it remembered.
	verifier.ActL2PipelineFull(t)

	// Verify the verifier finalized something new
	result := verifier.SyncStatus()
	require.Less(t, verifierStartStatus.FinalizedL2.Number, result.FinalizedL2.Number, "verifier finalized L2 blocks during sync")
}

// TestUnsafeSync tests that a verifier properly imports unsafe blocks via gossip.
func TestUnsafeSync(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelInfo)

	sd, _, _, sequencer, seqEng, verifier, _, _ := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)
	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	for i := 0; i < 10; i++ {
		// Build a L2 block
		sequencer.ActL2StartBlock(t)
		sequencer.ActL2EndBlock(t)
		// Notify new L2 block to verifier by unsafe gossip
		seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
		require.NoError(t, err)
		verifier.ActL2UnsafeGossipReceive(seqHead)(t)
		// Handle unsafe payload
		verifier.ActL2PipelineFull(t)
		// Verifier must advance its unsafe head.
		require.Equal(t, sequencer.L2Unsafe().Hash, verifier.L2Unsafe().Hash)
	}
}

func TestBackupUnsafe(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	minTs := hexutil.Uint64(0)
	// Activate Delta hardfork
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	dp.DeployConfig.L2BlockTime = 2
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LvlInfo)
	_, dp, miner, sequencer, seqEng, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)
	l2Cl := seqEng.EthClient()
	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	rng := rand.New(rand.NewSource(1234))
	signer := types.LatestSigner(sd.L2Cfg.Config)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Create block A1 ~ A5
	for i := 0; i < 5; i++ {
		// Build a L2 block
		sequencer.ActL2StartBlock(t)
		sequencer.ActL2EndBlock(t)

		// Notify new L2 block to verifier by unsafe gossip
		seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
		require.NoError(t, err)
		verifier.ActL2UnsafeGossipReceive(seqHead)(t)
	}

	seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	// eventually correct hash for A5
	targetUnsafeHeadHash := seqHead.ExecutionPayload.BlockHash

	// only advance unsafe head to A5
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(5))
	require.Equal(t, sequencer.L2Safe().Number, uint64(0))

	// Handle unsafe payload
	verifier.ActL2PipelineFull(t)
	// only advance unsafe head to A5
	require.Equal(t, verifier.L2Unsafe().Number, uint64(5))
	require.Equal(t, verifier.L2Safe().Number, uint64(0))

	channelOut := newSpanChannelOut(t, *sd)

	for i := uint64(1); i <= sequencer.L2Unsafe().Number; i++ {
		block, err := l2Cl.BlockByNumber(t.Ctx(), new(big.Int).SetUint64(i))
		require.NoError(t, err)
		if i == 2 {
			// Make block B2 as an valid block different with unsafe block
			// Alice makes a L2 tx
			n, err := l2Cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
			require.NoError(t, err)
			validTx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
				ChainID:   sd.L2Cfg.Config.ChainID,
				Nonce:     n,
				GasTipCap: big.NewInt(2 * params.GWei),
				GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
				Gas:       params.TxGas,
				To:        &dp.Addresses.Bob,
				Value:     e2eutils.Ether(2),
			})
			block = block.WithBody(types.Body{Transactions: []*types.Transaction{block.Transactions()[0], validTx}})
		}
		if i == 3 {
			// Make block B3 as an invalid block
			invalidTx := testutils.RandomTx(rng, big.NewInt(100), signer)
			block = block.WithBody(types.Body{Transactions: []*types.Transaction{block.Transactions()[0], invalidTx}})
		}
		// Add A1, B2, B3, B4, B5 into the channel
		_, err = channelOut.AddBlock(sd.RollupCfg, block)
		require.NoError(t, err)
	}

	// Submit span batch(A1, B2, invalid B3, B4, B5)
	batcher.L2ChannelOut = channelOut
	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)

	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// let sequencer process invalid span batch
	sequencer.ActL1HeadSignal(t)
	// before stepping, make sure backupUnsafe is empty
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())
	// pendingSafe must not be advanced as well
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(0))
	// Run until we consume A1 from batch
	sequencer.ActL2EventsUntilPending(t, 1)
	// A1 is valid original block so pendingSafe is advanced
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(1))
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(5))
	// backupUnsafe is still empty
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())

	// Process B2
	// Run until we consume B2 from batch
	sequencer.ActL2EventsUntilPending(t, 2)
	// B2 is valid different block, triggering unsafe chain reorg
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(2))
	// B2 is valid different block, triggering unsafe block backup
	require.Equal(t, targetUnsafeHeadHash, sequencer.L2BackupUnsafe().Hash)
	// B2 is valid different block, so pendingSafe is advanced
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(2))
	// try to process invalid leftovers: B3, B4, B5
	sequencer.ActL2PipelineFull(t)
	// backupUnsafe is used because A3 is invalid. Check backupUnsafe is emptied after used
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())

	// check pendingSafe is reset
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(0))
	// check backupUnsafe is applied
	require.Equal(t, sequencer.L2Unsafe().Hash, targetUnsafeHeadHash)
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(5))
	// safe head cannot be advanced because batch contained invalid blocks
	require.Equal(t, sequencer.L2Safe().Number, uint64(0))

	// let verifier process invalid span batch
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	// safe head cannot be advanced, while unsafe head not changed
	require.Equal(t, verifier.L2Unsafe().Number, uint64(5))
	require.Equal(t, verifier.L2Safe().Number, uint64(0))
	require.Equal(t, verifier.L2Unsafe().Hash, targetUnsafeHeadHash)

	// Build and submit a span batch with A1 ~ A5
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// let sequencer process valid span batch
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)

	// safe/unsafe head must be advanced
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(5))
	require.Equal(t, sequencer.L2Safe().Number, uint64(5))
	require.Equal(t, sequencer.L2Safe().Hash, targetUnsafeHeadHash)
	// check backupUnsafe is emptied after consolidation
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())

	// let verifier process valid span batch
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	// safe and unsafe head must be advanced
	require.Equal(t, verifier.L2Unsafe().Number, uint64(5))
	require.Equal(t, verifier.L2Safe().Number, uint64(5))
	require.Equal(t, verifier.L2Safe().Hash, targetUnsafeHeadHash)
	// check backupUnsafe is emptied after consolidation
	require.Equal(t, eth.L2BlockRef{}, verifier.L2BackupUnsafe())
}

func TestBackupUnsafeReorgForkChoiceInputError(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	minTs := hexutil.Uint64(0)
	// Activate Delta hardfork
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	dp.DeployConfig.L2BlockTime = 2
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LvlInfo)
	_, dp, miner, sequencer, seqEng, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)
	l2Cl := seqEng.EthClient()
	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	rng := rand.New(rand.NewSource(1234))
	signer := types.LatestSigner(sd.L2Cfg.Config)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Create block A1 ~ A5
	for i := 0; i < 5; i++ {
		// Build a L2 block
		sequencer.ActL2StartBlock(t)
		sequencer.ActL2EndBlock(t)

		// Notify new L2 block to verifier by unsafe gossip
		seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
		require.NoError(t, err)
		verifier.ActL2UnsafeGossipReceive(seqHead)(t)
	}

	seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	// eventually correct hash for A5
	targetUnsafeHeadHash := seqHead.ExecutionPayload.BlockHash

	// only advance unsafe head to A5
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(5))
	require.Equal(t, sequencer.L2Safe().Number, uint64(0))

	// Handle unsafe payload
	verifier.ActL2PipelineFull(t)
	// only advance unsafe head to A5
	require.Equal(t, verifier.L2Unsafe().Number, uint64(5))
	require.Equal(t, verifier.L2Safe().Number, uint64(0))

	channelOut := newSpanChannelOut(t, *sd)

	for i := uint64(1); i <= sequencer.L2Unsafe().Number; i++ {
		block, err := l2Cl.BlockByNumber(t.Ctx(), new(big.Int).SetUint64(i))
		require.NoError(t, err)
		if i == 2 {
			// Make block B2 as an valid block different with unsafe block
			// Alice makes a L2 tx
			n, err := l2Cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
			require.NoError(t, err)
			validTx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
				ChainID:   sd.L2Cfg.Config.ChainID,
				Nonce:     n,
				GasTipCap: big.NewInt(2 * params.GWei),
				GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
				Gas:       params.TxGas,
				To:        &dp.Addresses.Bob,
				Value:     e2eutils.Ether(2),
			})
			block = block.WithBody(types.Body{Transactions: []*types.Transaction{block.Transactions()[0], validTx}})
		}
		if i == 3 {
			// Make block B3 as an invalid block
			invalidTx := testutils.RandomTx(rng, big.NewInt(100), signer)
			block = block.WithBody(types.Body{Transactions: []*types.Transaction{block.Transactions()[0], invalidTx}})
		}
		// Add A1, B2, B3, B4, B5 into the channel
		_, err = channelOut.AddBlock(sd.RollupCfg, block)
		require.NoError(t, err)
	}

	// Submit span batch(A1, B2, invalid B3, B4, B5)
	batcher.L2ChannelOut = channelOut
	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)

	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// let sequencer process invalid span batch
	sequencer.ActL1HeadSignal(t)
	// before stepping, make sure backupUnsafe is empty
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())
	// pendingSafe must not be advanced as well
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(0))
	// Run till we consumed A1 from batch
	sequencer.ActL2EventsUntilPending(t, 1)
	// A1 is valid original block so pendingSafe is advanced
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(1))
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(5))
	// backupUnsafe is still empty
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())

	// Process B2
	sequencer.ActL2EventsUntilPending(t, 2)
	// B2 is valid different block, triggering unsafe chain reorg
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(2))
	// B2 is valid different block, triggering unsafe block backup
	require.Equal(t, targetUnsafeHeadHash, sequencer.L2BackupUnsafe().Hash)
	// B2 is valid different block, so pendingSafe is advanced
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(2))

	// B3 is invalid block
	// NextAttributes is called
	sequencer.ActL2EventsUntil(t, event.Is[engine2.BuildStartEvent], 100, true)
	// mock forkChoiceUpdate error while restoring previous unsafe chain using backupUnsafe.
	seqEng.ActL2RPCFail(t, eth.InputError{Inner: errors.New("mock L2 RPC error"), Code: eth.InvalidForkchoiceState})

	// The backup-unsafe rewind is applied

	// try to process invalid leftovers: B4, B5
	sequencer.ActL2PipelineFull(t)

	// backupUnsafe is not used because forkChoiceUpdate returned an error.
	// Check backupUnsafe is emptied.
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())

	// check pendingSafe is reset
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(0))
	// unsafe head is not restored due to forkchoiceUpdate error in TryBackupUnsafeReorg
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(2))
	// safe head cannot be advanced because batch contained invalid blocks
	require.Equal(t, sequencer.L2Safe().Number, uint64(0))
}

func TestBackupUnsafeReorgForkChoiceNotInputError(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	minTs := hexutil.Uint64(0)
	// Activate Delta hardfork
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	dp.DeployConfig.L2BlockTime = 2
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LvlInfo)
	_, dp, miner, sequencer, seqEng, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)
	l2Cl := seqEng.EthClient()
	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	rng := rand.New(rand.NewSource(1234))
	signer := types.LatestSigner(sd.L2Cfg.Config)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Create block A1 ~ A5
	for i := 0; i < 5; i++ {
		// Build a L2 block
		sequencer.ActL2StartBlock(t)
		sequencer.ActL2EndBlock(t)

		// Notify new L2 block to verifier by unsafe gossip
		seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
		require.NoError(t, err)
		verifier.ActL2UnsafeGossipReceive(seqHead)(t)
	}

	seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	// eventually correct hash for A5
	targetUnsafeHeadHash := seqHead.ExecutionPayload.BlockHash

	// only advance unsafe head to A5
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(5))
	require.Equal(t, sequencer.L2Safe().Number, uint64(0))

	// Handle unsafe payload
	verifier.ActL2PipelineFull(t)
	// only advance unsafe head to A5
	require.Equal(t, verifier.L2Unsafe().Number, uint64(5))
	require.Equal(t, verifier.L2Safe().Number, uint64(0))

	channelOut := newSpanChannelOut(t, *sd)

	for i := uint64(1); i <= sequencer.L2Unsafe().Number; i++ {
		block, err := l2Cl.BlockByNumber(t.Ctx(), new(big.Int).SetUint64(i))
		require.NoError(t, err)
		if i == 2 {
			// Make block B2 as an valid block different with unsafe block
			// Alice makes a L2 tx
			n, err := l2Cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
			require.NoError(t, err)
			validTx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
				ChainID:   sd.L2Cfg.Config.ChainID,
				Nonce:     n,
				GasTipCap: big.NewInt(2 * params.GWei),
				GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
				Gas:       params.TxGas,
				To:        &dp.Addresses.Bob,
				Value:     e2eutils.Ether(2),
			})
			block = block.WithBody(types.Body{Transactions: []*types.Transaction{block.Transactions()[0], validTx}})
		}
		if i == 3 {
			// Make block B3 as an invalid block
			invalidTx := testutils.RandomTx(rng, big.NewInt(100), signer)
			block = block.WithBody(types.Body{Transactions: []*types.Transaction{block.Transactions()[0], invalidTx}})
		}
		// Add A1, B2, B3, B4, B5 into the channel
		_, err = channelOut.AddBlock(sd.RollupCfg, block)
		require.NoError(t, err)
	}

	// Submit span batch(A1, B2, invalid B3, B4, B5)
	batcher.L2ChannelOut = channelOut
	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)

	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// let sequencer process invalid span batch
	sequencer.ActL1HeadSignal(t)
	// before stepping, make sure backupUnsafe is empty
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())
	// pendingSafe must not be advanced as well
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(0))
	// Preheat engine queue and consume A1 from batch
	sequencer.ActL2EventsUntilPending(t, 1)
	// A1 is valid original block so pendingSafe is advanced
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(1))
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(5))
	// backupUnsafe is still empty
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())

	// Process B2
	sequencer.ActL2EventsUntilPending(t, 2)
	// B2 is valid different block, triggering unsafe chain reorg
	require.Equal(t, sequencer.L2Unsafe().Number, uint64(2))
	// B2 is valid different block, triggering unsafe block backup
	require.Equal(t, targetUnsafeHeadHash, sequencer.L2BackupUnsafe().Hash)
	// B2 is valid different block, so pendingSafe is advanced
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(2))

	// B3 is invalid block
	// wait till attributes processing (excl.) before mocking errors
	sequencer.ActL2EventsUntil(t, event.Is[engine2.BuildStartEvent], 100, true)

	serverErrCnt := 2
	// mock forkChoiceUpdate failure while restoring previous unsafe chain using backupUnsafe.
	seqEng.FailL2RPC = func(call []rpc.BatchElem) error {
		for _, e := range call {
			// There may be other calls, like payload-processing-cancellation
			// based on previous invalid block, and processing of block attributes.
			if strings.HasPrefix(e.Method, "engine_forkchoiceUpdated") && e.Args[1].(*eth.PayloadAttributes) == nil {
				if serverErrCnt > 0 {
					serverErrCnt -= 1
					return gethengine.GenericServerError
				} else {
					return nil
				}
			}
		}
		return nil
	}
	// cannot drain events until specific engine error, since SyncDeriver calls Drain internally still.
	sequencer.ActL2PipelineFull(t)

	// now forkchoice succeeds
	// try to process invalid leftovers: B4, B5
	sequencer.ActL2PipelineFull(t)

	// backupUnsafe is used because forkChoiceUpdate eventually succeeded.
	// Check backupUnsafe is emptied.
	require.Equal(t, eth.L2BlockRef{}, sequencer.L2BackupUnsafe())

	// check pendingSafe is reset
	require.Equal(t, sequencer.L2PendingSafe().Number, uint64(0))
	// check backupUnsafe is applied
	require.Equal(t, uint64(5), sequencer.L2Unsafe().Number)
	require.Equal(t, targetUnsafeHeadHash, sequencer.L2Unsafe().Hash)
	// safe head cannot be advanced because batch contained invalid blocks
	require.Equal(t, sequencer.L2Safe().Number, uint64(0))
}

// builds l2 blocks within the specified range `from` - `to`
// and performs an EL sync between the sequencer and the verifier,
// then checks the validity of the payloads within a specified block range.
func PerformELSyncAndCheckPayloads(t actionsHelpers.Testing, miner *actionsHelpers.L1Miner, seqEng *actionsHelpers.L2Engine, sequencer *actionsHelpers.L2Sequencer, verEng *actionsHelpers.L2Engine, verifier *actionsHelpers.L2Verifier, seqEngCl *sources.EngineClient, from, to uint64) {
	miner.ActEmptyBlock(t)
	sequencer.ActL2PipelineFull(t)

	// Build L1 blocks on the sequencer
	for i := from; i < to; i++ {
		// Build a L2 block
		sequencer.ActL2StartBlock(t)
		sequencer.ActL2EndBlock(t)
	}

	// Wait longer to peer. This tests flakes or takes a long time when the op-geth instances are not able to peer.
	verEng.AddPeers(seqEng.Enode())

	// Insert it on the verifier
	seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	seqStart, err := seqEngCl.PayloadByNumber(t.Ctx(), from)
	require.NoError(t, err)
	verifier.ActL2InsertUnsafePayload(seqHead)(t)

	require.Eventually(t,
		func() bool {
			return seqEng.PeerCount() > 0 && verEng.PeerCount() > 0
		},
		120*time.Second, 1500*time.Millisecond,
		"Sequencer & Verifier must peer with each other for snap sync to work",
	)

	// Expect snap sync to download & execute the entire chain
	// Verify this by checking that the verifier has the correct value for block 1
	require.Eventually(t,
		func() bool {
			block, err := verifier.Eng.L2BlockRefByNumber(t.Ctx(), from)
			if err != nil {
				return false
			}
			return seqStart.ExecutionPayload.BlockHash == block.Hash
		},
		60*time.Second, 1500*time.Millisecond,
		"verifier did not snap sync",
	)
}

// verifies that a specific block number on the L2 engine has the expected label.
func VerifyBlock(t actionsHelpers.Testing, engine actionsHelpers.L2API, number uint64, label eth.BlockLabel) {
	id, err := engine.L2BlockRefByLabel(t.Ctx(), label)
	require.NoError(t, err)
	require.Equal(t, number, id.Number)
}

// submits batch at a specified block number
func BatchSubmitBlock(t actionsHelpers.Testing, miner *actionsHelpers.L1Miner, sequencer *actionsHelpers.L2Sequencer, verifier *actionsHelpers.L2Verifier, batcher *actionsHelpers.L2Batcher, dp *e2eutils.DeployParams, number uint64) {
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(number)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)
}

// TestELSync tests that a verifier will have the EL import the full chain from the sequencer
// when passed a single unsafe block. op-geth can either snap sync or full sync here.
func TestELSync(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelInfo)

	miner, seqEng, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)
	// Enable engine P2P sync
	verEng, verifier := actionsHelpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{SyncMode: sync.ELSync})

	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	PerformELSyncAndCheckPayloads(t, miner, seqEng, sequencer, verEng, verifier, seqEngCl, 0, 10)
}

func PrepareELSyncedNode(t actionsHelpers.Testing, miner *actionsHelpers.L1Miner, sequencer *actionsHelpers.L2Sequencer, seqEng *actionsHelpers.L2Engine, verifier *actionsHelpers.L2Verifier, verEng *actionsHelpers.L2Engine, seqEngCl *sources.EngineClient, batcher *actionsHelpers.L2Batcher, dp *e2eutils.DeployParams) {
	PerformELSyncAndCheckPayloads(t, miner, seqEng, sequencer, verEng, verifier, seqEngCl, 0, 10)

	// Despite downloading the blocks, it has not finished finalizing
	_, err := verifier.Eng.L2BlockRefByLabel(t.Ctx(), "safe")
	require.ErrorIs(t, err, ethereum.NotFound)

	// Insert a block on the verifier to end snap sync
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)
	seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	verifier.ActL2InsertUnsafePayload(seqHead)(t)

	// Check that safe + finalized are there
	VerifyBlock(t, verifier.Eng, 11, eth.Safe)
	VerifyBlock(t, verifier.Eng, 11, eth.Finalized)

	// Batch submit everything
	BatchSubmitBlock(t, miner, sequencer, verifier, batcher, dp, 12)

	// Verify that the batch submitted blocks are there now
	VerifyBlock(t, sequencer.Eng, 12, eth.Safe)
	VerifyBlock(t, verifier.Eng, 12, eth.Safe)
}

// TestELSyncTransitionstoCL tests that a verifier which starts with EL sync can switch back to a proper CL sync.
// It takes a sequencer & verifier through the following:
//  1. Build 10 unsafe blocks on the sequencer
//  2. Snap sync those blocks to the verifier
//  3. Build & insert 1 unsafe block from the sequencer to the verifier to end snap sync
//  4. Batch submit everything
//  5. Build 10 more unsafe blocks on the sequencer
//  6. Gossip in the highest block to the verifier. **Expect that it does not snap sync**
//  7. Then gossip the rest of the blocks to the verifier. Once this is complete it should pick up all of the unsafe blocks.
//     Prior to this PR, the test would fail at this point.
//  8. Create 1 more block & batch submit everything & assert that the verifier picked up those blocks
func TestELSyncTransitionstoCL(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	logger := testlog.Logger(t, log.LevelInfo)

	captureLog, captureLogHandler := testlog.CaptureLogger(t, log.LevelInfo)

	miner, seqEng, sequencer := actionsHelpers.SetupSequencerTest(t, sd, logger)
	batcher := actionsHelpers.NewL2Batcher(logger, sd.RollupCfg, actionsHelpers.DefaultBatcherCfg(dp), sequencer.RollupClient(), miner.EthClient(), seqEng.EthClient(), seqEng.EngineClient(t, sd.RollupCfg))
	// Enable engine P2P sync
	verEng, verifier := actionsHelpers.SetupVerifier(t, sd, captureLog, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{SyncMode: sync.ELSync})

	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), logger, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	PrepareELSyncedNode(t, miner, sequencer, seqEng, verifier, verEng, seqEngCl, batcher, dp)

	// Build another 10 L1 blocks on the sequencer
	for i := 0; i < 10; i++ {
		// Build a L2 block
		sequencer.ActL2StartBlock(t)
		sequencer.ActL2EndBlock(t)
	}

	// Now pass payloads to the derivation pipeline
	// This is a little hacky that we have to manually switch between InsertBlock
	// and UnsafeGossipReceive in the tests
	seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	verifier.ActL2UnsafeGossipReceive(seqHead)(t)
	verifier.ActL2PipelineFull(t)
	// Verify that the derivation pipeline did not request a sync to the new head. This is the core of the test, but a little fragile.
	record := captureLogHandler.FindLog(testlog.NewMessageFilter("Forkchoice requested sync to new head"), testlog.NewAttributesFilter("number", "22"))
	require.Nil(t, record, "The verifier should not request to sync to block number 22 because it is in CL mode, not EL mode at this point.")

	for i := 13; i < 23; i++ {
		seqHead, err = seqEngCl.PayloadByNumber(t.Ctx(), uint64(i))
		require.NoError(t, err)
		verifier.ActL2UnsafeGossipReceive(seqHead)(t)
	}
	verifier.ActL2PipelineFull(t)

	// Verify that the unsafe blocks are there now
	// This was failing prior to PR 9661 because op-node would attempt to immediately insert blocks into the EL inside the engine queue. op-geth
	// would not be able to fetch the second range of blocks & it would wipe out the unsafe payloads queue because op-node thought that it had a
	// higher unsafe block but op-geth did not.
	VerifyBlock(t, verifier.Eng, 22, eth.Unsafe)

	// Create 1 more block & batch submit everything
	BatchSubmitBlock(t, miner, sequencer, verifier, batcher, dp, 12)

	// Verify that the batch submitted blocks are there now
	VerifyBlock(t, sequencer.Eng, 23, eth.Safe)
	VerifyBlock(t, verifier.Eng, 23, eth.Safe)
}

func TestELSyncTransitionsToCLSyncAfterNodeRestart(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	logger := testlog.Logger(t, log.LevelInfo)

	captureLog, captureLogHandler := testlog.CaptureLogger(t, log.LevelInfo)

	miner, seqEng, sequencer := actionsHelpers.SetupSequencerTest(t, sd, logger)
	batcher := actionsHelpers.NewL2Batcher(logger, sd.RollupCfg, actionsHelpers.DefaultBatcherCfg(dp), sequencer.RollupClient(), miner.EthClient(), seqEng.EthClient(), seqEng.EngineClient(t, sd.RollupCfg))
	// Enable engine P2P sync
	verEng, verifier := actionsHelpers.SetupVerifier(t, sd, captureLog, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{SyncMode: sync.ELSync})

	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), logger, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	PrepareELSyncedNode(t, miner, sequencer, seqEng, verifier, verEng, seqEngCl, batcher, dp)

	// Create a new verifier which is essentially a new op-node with the sync mode of ELSync and default geth engine kind.
	verifier = actionsHelpers.NewL2Verifier(t, captureLog, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), altda.Disabled, verifier.Eng, sd.RollupCfg, &sync.Config{SyncMode: sync.ELSync}, actionsHelpers.DefaultVerifierCfg().SafeHeadListener, nil)

	// Build another 10 L1 blocks on the sequencer
	for i := 0; i < 10; i++ {
		// Build a L2 block
		sequencer.ActL2StartBlock(t)
		sequencer.ActL2EndBlock(t)
	}

	// Insert new block to the engine and kick off a CL sync
	seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	verifier.ActL2InsertUnsafePayload(seqHead)(t)

	// Verify that the derivation pipeline did not request a sync to the new head. This is the core of the test, but a little fragile.
	record := captureLogHandler.FindLog(testlog.NewMessageFilter("Forkchoice requested sync to new head"), testlog.NewAttributesFilter("number", "22"))
	require.Nil(t, record, "The verifier should not request to sync to block number 22 because it is in CL mode, not EL mode at this point.")

	// Verify that op-node has skipped ELSync and started CL sync because geth has finalized block from ELSync.
	record = captureLogHandler.FindLog(testlog.NewMessageFilter("Skipping EL sync and going straight to CL sync because there is a finalized block"))
	require.NotNil(t, record, "The verifier should skip EL Sync at this point.")
}

func TestForcedELSyncCLAfterNodeRestart(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	logger := testlog.Logger(t, log.LevelInfo)

	captureLog, captureLogHandler := testlog.CaptureLogger(t, log.LevelInfo)

	miner, seqEng, sequencer := actionsHelpers.SetupSequencerTest(t, sd, logger)
	batcher := actionsHelpers.NewL2Batcher(logger, sd.RollupCfg, actionsHelpers.DefaultBatcherCfg(dp), sequencer.RollupClient(), miner.EthClient(), seqEng.EthClient(), seqEng.EngineClient(t, sd.RollupCfg))
	// Enable engine P2P sync
	verEng, verifier := actionsHelpers.SetupVerifier(t, sd, captureLog, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{SyncMode: sync.ELSync})

	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), logger, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	PrepareELSyncedNode(t, miner, sequencer, seqEng, verifier, verEng, seqEngCl, batcher, dp)

	// Create a new verifier which is essentially a new op-node with the sync mode of ELSync and erigon engine kind.
	verifier2 := actionsHelpers.NewL2Verifier(t, captureLog, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), altda.Disabled, verifier.Eng, sd.RollupCfg, &sync.Config{SyncMode: sync.ELSync, SupportsPostFinalizationELSync: true}, actionsHelpers.DefaultVerifierCfg().SafeHeadListener, nil)

	// Build another 10 L1 blocks on the sequencer
	for i := 0; i < 10; i++ {
		// Build a L2 block
		sequencer.ActL2StartBlock(t)
		sequencer.ActL2EndBlock(t)
	}

	// Insert it on the verifier and kick off EL sync.
	// Syncing doesn't actually work in test,
	// but we can validate the engine is starting EL sync through p2p
	seqHead, err := seqEngCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	verifier2.ActL2InsertUnsafePayload(seqHead)(t)

	// Verify that the derivation pipeline did not request a sync to the new head. This is the core of the test, but a little fragile.
	record := captureLogHandler.FindLog(testlog.NewMessageFilter("Forkchoice requested sync to new head"), testlog.NewAttributesFilter("number", "22"))
	require.NotNil(t, record, "The verifier should request to sync to block number 22 in EL mode")

	// Verify that op-node is starting ELSync.
	record = captureLogHandler.FindLog(testlog.NewMessageFilter("Skipping EL sync and going straight to CL sync because there is a finalized block"))
	require.Nil(t, record, "The verifier should start EL Sync when l2.engineKind is not geth")
	record = captureLogHandler.FindLog(testlog.NewMessageFilter("Starting EL sync"))
	require.NotNil(t, record, "The verifier should start EL Sync when l2.engineKind is not geth")
}

func TestInvalidPayloadInSpanBatch(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	minTs := hexutil.Uint64(0)
	// Activate Delta hardfork
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	dp.DeployConfig.L2BlockTime = 2
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelInfo)
	_, _, miner, sequencer, seqEng, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)
	l2Cl := seqEng.EthClient()
	rng := rand.New(rand.NewSource(1234))
	signer := types.LatestSigner(sd.L2Cfg.Config)

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	channelOut := newSpanChannelOut(t, *sd)

	// Create block A1 ~ A12 for L1 block #0 ~ #2
	miner.ActEmptyBlock(t)
	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1HeadUnsafe(t)

	for i := uint64(1); i <= sequencer.L2Unsafe().Number; i++ {
		block, err := l2Cl.BlockByNumber(t.Ctx(), new(big.Int).SetUint64(i))
		require.NoError(t, err)
		if i == 8 {
			// Make block A8 as an invalid block
			invalidTx := testutils.RandomTx(rng, big.NewInt(100), signer)
			block = block.WithBody(types.Body{Transactions: []*types.Transaction{block.Transactions()[0], invalidTx}})
		}
		// Add A1 ~ A12 into the channel
		_, err = channelOut.AddBlock(sd.RollupCfg, block)
		require.NoError(t, err)
	}

	// Submit span batch(A1, ...,  A7, invalid A8, A9, ..., A12)
	batcher.L2ChannelOut = channelOut
	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)

	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)
	miner.ActL1SafeNext(t)
	miner.ActL1FinalizeNext(t)

	// After the verifier processed the span batch, only unsafe head should be advanced to A7.
	// Safe head is not updated because the span batch is not fully processed.
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Unsafe().Number, uint64(7))
	require.Equal(t, verifier.L2Safe().Number, uint64(0))

	channelOut = newSpanChannelOut(t, *sd)

	for i := uint64(1); i <= sequencer.L2Unsafe().Number; i++ {
		block, err := l2Cl.BlockByNumber(t.Ctx(), new(big.Int).SetUint64(i))
		require.NoError(t, err)
		if i == 1 {
			// Create valid TX
			aliceNonce, err := seqEng.EthClient().PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
			require.NoError(t, err)
			data := make([]byte, rand.Intn(100))
			gas, err := core.IntrinsicGas(data, nil, false, true, true, false)
			require.NoError(t, err)
			baseFee := seqEng.L2Chain().CurrentBlock().BaseFee
			tx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
				ChainID:   sd.L2Cfg.Config.ChainID,
				Nonce:     aliceNonce,
				GasTipCap: big.NewInt(2 * params.GWei),
				GasFeeCap: new(big.Int).Add(new(big.Int).Mul(baseFee, big.NewInt(2)), big.NewInt(2*params.GWei)),
				Gas:       gas,
				To:        &dp.Addresses.Bob,
				Value:     big.NewInt(0),
				Data:      data,
			})
			// Create valid new block B1 at the same height as A1
			block = block.WithBody(types.Body{Transactions: []*types.Transaction{block.Transactions()[0], tx}})
		}
		// Add B1, A2 ~ A12 into the channel
		_, err = channelOut.AddBlock(sd.RollupCfg, block)
		require.NoError(t, err)
	}
	// Submit span batch(B1, A2, ... A12)
	batcher.L2ChannelOut = channelOut
	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)

	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)
	miner.ActL1SafeNext(t)
	miner.ActL1FinalizeNext(t)

	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	// verifier should advance its unsafe and safe head to the height of A12.
	require.Equal(t, verifier.L2Unsafe().Number, uint64(12))
	require.Equal(t, verifier.L2Safe().Number, uint64(12))
}

func TestSpanBatchAtomicity_Consolidation(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	minTs := hexutil.Uint64(0)
	// Activate Delta hardfork
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	dp.DeployConfig.L2BlockTime = 2
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelInfo)
	_, _, miner, sequencer, seqEng, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)
	seqEngCl, err := sources.NewEngineClient(seqEng.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	targetHeadNumber := uint64(6) // L1 block time / L2 block time

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Create 6 blocks
	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1HeadUnsafe(t)
	require.Equal(t, sequencer.L2Unsafe().Number, targetHeadNumber)

	// Gossip unsafe blocks to the verifier
	for i := uint64(1); i <= sequencer.L2Unsafe().Number; i++ {
		seqHead, err := seqEngCl.PayloadByNumber(t.Ctx(), i)
		require.NoError(t, err)
		verifier.ActL2UnsafeGossipReceive(seqHead)(t)
	}
	verifier.ActL2PipelineFull(t)

	// Check if the verifier's unsafe sync is done
	require.Equal(t, sequencer.L2Unsafe().Hash, verifier.L2Unsafe().Hash)

	// Build and submit a span batch with 6 blocks
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// Start verifier safe sync
	verifier.ActL1HeadSignal(t)
	verifier.L2PipelineIdle = false
	for !verifier.L2PipelineIdle {
		// wait for next pending block
		verifier.ActL2EventsUntil(t, func(ev event.Event) bool {
			if event.Is[engine2.SafeDerivedEvent](ev) { // safe updates should only happen once the pending-safe reaches the target.
				t.Fatal("unexpected next safe update")
			}
			return event.Any(event.Is[engine2.PendingSafeUpdateEvent], event.Is[derive.DeriverIdleEvent])(ev)
		}, 1000, false)
		if verifier.L2PendingSafe().Number < targetHeadNumber {
			// If the span batch is not fully processed, the safe head must not advance.
			require.Equal(t, verifier.L2Safe().Number, uint64(0))
		} else {
			// Make sure we do the post-processing of what safety updates might happen
			// after the pending-safe event, before the next pending-safe event.
			verifier.ActL2EventsUntil(t, event.Is[engine2.PendingSafeUpdateEvent], 100, true)
			// Once the span batch is fully processed, the safe head must advance to the end of span batch.
			require.Equal(t, verifier.L2Safe().Number, targetHeadNumber)
			require.Equal(t, verifier.L2Safe(), verifier.L2PendingSafe())
		}
		// The unsafe head must not be changed
		require.Equal(t, verifier.L2Unsafe(), sequencer.L2Unsafe())
	}
}

func TestSpanBatchAtomicity_ForceAdvance(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())
	minTs := hexutil.Uint64(0)
	// Activate Delta hardfork
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	dp.DeployConfig.L2BlockTime = 2
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelInfo)
	_, _, miner, sequencer, _, verifier, _, batcher := actionsHelpers.SetupReorgTestActors(t, dp, sd, log)

	targetHeadNumber := uint64(6) // L1 block time / L2 block time

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, verifier.L2Unsafe().Number, uint64(0))

	// Create 6 blocks
	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1HeadUnsafe(t)
	require.Equal(t, sequencer.L2Unsafe().Number, targetHeadNumber)

	// Build and submit a span batch with 6 blocks
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// Start verifier safe sync
	verifier.ActL1HeadSignal(t)
	verifier.L2PipelineIdle = false
	for !verifier.L2PipelineIdle {
		// wait for next pending block
		verifier.ActL2EventsUntil(t, func(ev event.Event) bool {
			if event.Is[engine2.SafeDerivedEvent](ev) { // safe updates should only happen once the pending-safe reaches the target.
				t.Fatal("unexpected next safe update")
			}
			return event.Any(event.Is[engine2.PendingSafeUpdateEvent], event.Is[derive.DeriverIdleEvent])(ev)
		}, 1000, false)
		if verifier.L2PendingSafe().Number < targetHeadNumber {
			// If the span batch is not fully processed, the safe head must not advance.
			require.Equal(t, verifier.L2Safe().Number, uint64(0))
		} else {
			// Make sure we do the post-processing of what safety updates might happen
			// after the pending-safe event, before the next pending-safe event.
			verifier.ActL2EventsUntil(t, event.Is[engine2.PendingSafeUpdateEvent], 100, true)
			// Once the span batch is fully processed, the safe head must advance to the end of span batch.
			require.Equal(t, verifier.L2Safe().Number, targetHeadNumber)
			require.Equal(t, verifier.L2Safe(), verifier.L2PendingSafe())
		}
		// The unsafe head and the pending safe head must be the same
		require.Equal(t, verifier.L2Unsafe(), verifier.L2PendingSafe())
	}
}
