package upgrades

import (
	"context"
	"crypto/ecdsa"
	crand "crypto/rand"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	"math/big"
	"math/rand"
	"testing"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	upgradesHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/upgrades/helpers"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"

	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

// TestDropSpanBatchBeforeHardfork tests behavior of op-node before Delta hardfork.
// op-node must drop SpanBatch before Delta hardfork.
func TestDropSpanBatchBeforeHardfork(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20, // larger than L1 block time we simulate in this test (12)
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	dp := e2eutils.MakeDeployParams(t, p)
	// do not activate Delta hardfork for verifier
	upgradesHelpers.ApplyDeltaTimeOffset(dp, nil)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelError)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)
	verifEngine, verifier := actionsHelpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})
	rollupSeqCl := sequencer.RollupClient()

	// Force batcher to submit SpanBatches to L1.
	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, &actionsHelpers.BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           dp.Secrets.Batcher,
		ForceSubmitSpanBatch: true,
		DataAvailabilityType: batcherFlags.CalldataType,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	// Alice makes a L2 tx
	cl := seqEngine.EthClient()
	n, err := cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
	require.NoError(t, err)
	signer := types.LatestSigner(sd.L2Cfg.Config)
	tx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
		ChainID:   sd.L2Cfg.Config.ChainID,
		Nonce:     n,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
		Gas:       params.TxGas,
		To:        &dp.Addresses.Bob,
		Value:     e2eutils.Ether(2),
	})
	require.NoError(gt, cl.SendTransaction(t.Ctx(), tx))

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Make L2 block
	sequencer.ActL2StartBlock(t)
	seqEngine.ActL2IncludeTx(dp.Addresses.Alice)(t)
	sequencer.ActL2EndBlock(t)

	// batch submit to L1. batcher should submit span batches.
	batcher.ActL2BatchBuffer(t)
	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)

	// confirm batch on L1
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)
	bl := miner.L1Chain().CurrentBlock()
	log.Info("bl", "txs", len(miner.L1Chain().GetBlockByHash(bl.Hash()).Transactions()))

	// Now make enough L1 blocks that the verifier will have to derive a L2 block
	// It will also eagerly derive the block from the batcher
	for i := uint64(0); i < sd.RollupCfg.SeqWindowSize; i++ {
		miner.ActL1StartBlock(12)(t)
		miner.ActL1EndBlock(t)
	}

	// try to sync verifier from L1 batch. but verifier should drop every span batch.
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, uint64(1), verifier.SyncStatus().SafeL2.L1Origin.Number)

	verifCl := verifEngine.EthClient()
	for i := int64(1); i < int64(verifier.L2Safe().Number); i++ {
		block, _ := verifCl.BlockByNumber(t.Ctx(), big.NewInt(i))
		require.NoError(t, err)
		// because verifier drops every span batch, it should generate empty blocks.
		// so every block has only L1 attribute deposit transaction.
		require.Equal(t, block.Transactions().Len(), 1)
	}
	// check that the tx from alice is not included in verifier's chain
	_, _, err = verifCl.TransactionByHash(t.Ctx(), tx.Hash())
	require.ErrorIs(t, err, ethereum.NotFound)
}

// TestHardforkMiddleOfSpanBatch tests behavior of op-node Delta hardfork.
// If Delta activation time is in the middle of time range of a SpanBatch, op-node must drop the batch.
func TestHardforkMiddleOfSpanBatch(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20, // larger than L1 block time we simulate in this test (12)
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	dp := e2eutils.MakeDeployParams(t, p)

	// Activate HF in the middle of the first epoch
	deltaOffset := hexutil.Uint64(6)
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &deltaOffset)
	// Applies to HF that goes into Delta. Otherwise we end up with more upgrade txs and things during this case.
	dp.DeployConfig.L2GenesisEcotoneTimeOffset = nil
	dp.DeployConfig.L2GenesisFjordTimeOffset = nil
	dp.DeployConfig.L2GenesisGraniteTimeOffset = nil
	dp.DeployConfig.L2GenesisHoloceneTimeOffset = nil

	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelError)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)
	verifEngine, verifier := actionsHelpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})
	minerCl := miner.EthClient()
	rollupSeqCl := sequencer.RollupClient()

	// Force batcher to submit SpanBatches to L1.
	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, &actionsHelpers.BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           dp.Secrets.Batcher,
		ForceSubmitSpanBatch: true,
		DataAvailabilityType: batcherFlags.CalldataType,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	// Alice makes a L2 tx
	cl := seqEngine.EthClient()
	n, err := cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
	require.NoError(t, err)
	signer := types.LatestSigner(sd.L2Cfg.Config)
	tx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
		ChainID:   sd.L2Cfg.Config.ChainID,
		Nonce:     n,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
		Gas:       params.TxGas,
		To:        &dp.Addresses.Bob,
		Value:     e2eutils.Ether(2),
	})
	require.NoError(gt, cl.SendTransaction(t.Ctx(), tx))

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)

	// Make a L2 block with the TX
	sequencer.ActL2StartBlock(t)
	seqEngine.ActL2IncludeTx(dp.Addresses.Alice)(t)
	sequencer.ActL2EndBlock(t)

	// HF is not activated yet
	unsafeOriginNum := new(big.Int).SetUint64(sequencer.L2Unsafe().L1Origin.Number)
	unsafeHeader, err := minerCl.HeaderByNumber(t.Ctx(), unsafeOriginNum)
	require.NoError(t, err)
	require.False(t, sd.RollupCfg.IsDelta(unsafeHeader.Time))

	// Make L2 blocks until the next epoch
	sequencer.ActBuildToL1Head(t)

	// HF is activated for the last unsafe block
	unsafeOriginNum = new(big.Int).SetUint64(sequencer.L2Unsafe().L1Origin.Number)
	unsafeHeader, err = minerCl.HeaderByNumber(t.Ctx(), unsafeOriginNum)
	require.NoError(t, err)
	require.True(t, sd.RollupCfg.IsDelta(unsafeHeader.Time))

	// Batch submit to L1. batcher should submit span batches.
	batcher.ActSubmitAll(t)

	// Confirm batch on L1
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)
	bl := miner.L1Chain().CurrentBlock()
	log.Info("bl", "txs", len(miner.L1Chain().GetBlockByHash(bl.Hash()).Transactions()))

	// Now make enough L1 blocks that the verifier will have to derive a L2 block
	// It will also eagerly derive the block from the batcher
	for i := uint64(0); i < sd.RollupCfg.SeqWindowSize; i++ {
		miner.ActL1StartBlock(12)(t)
		miner.ActL1EndBlock(t)
	}

	// Try to sync verifier from L1 batch. but verifier should drop every span batch.
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, uint64(2), verifier.SyncStatus().SafeL2.L1Origin.Number)

	verifCl := verifEngine.EthClient()
	for i := int64(1); i < int64(verifier.L2Safe().Number); i++ {
		block, _ := verifCl.BlockByNumber(t.Ctx(), big.NewInt(i))
		require.NoError(t, err)
		// Because verifier drops every span batch, it should generate empty blocks.
		// So every block has only L1 attribute deposit transaction.
		require.Equal(t, block.Transactions().Len(), 1)
	}
	// Check that the tx from alice is not included in verifier's chain
	_, _, err = verifCl.TransactionByHash(t.Ctx(), tx.Hash())
	require.ErrorIs(t, err, ethereum.NotFound)
}

// TestAcceptSingularBatchAfterHardfork tests behavior of op-node after Delta hardfork.
// op-node must accept SingularBatch after Delta hardfork.
func TestAcceptSingularBatchAfterHardfork(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20, // larger than L1 block time we simulate in this test (12)
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	minTs := hexutil.Uint64(0)
	dp := e2eutils.MakeDeployParams(t, p)

	// activate Delta hardfork for verifier.
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelError)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)
	verifEngine, verifier := actionsHelpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})
	rollupSeqCl := sequencer.RollupClient()

	// Force batcher to submit SingularBatches to L1.
	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, &actionsHelpers.BatcherCfg{
		MinL1TxSize:              0,
		MaxL1TxSize:              128_000,
		BatcherKey:               dp.Secrets.Batcher,
		ForceSubmitSingularBatch: true,
		DataAvailabilityType:     batcherFlags.CalldataType,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	// Alice makes a L2 tx
	cl := seqEngine.EthClient()
	n, err := cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
	require.NoError(t, err)
	signer := types.LatestSigner(sd.L2Cfg.Config)
	tx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
		ChainID:   sd.L2Cfg.Config.ChainID,
		Nonce:     n,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
		Gas:       params.TxGas,
		To:        &dp.Addresses.Bob,
		Value:     e2eutils.Ether(2),
	})
	require.NoError(gt, cl.SendTransaction(t.Ctx(), tx))

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Make L2 block
	sequencer.ActL2StartBlock(t)
	seqEngine.ActL2IncludeTx(dp.Addresses.Alice)(t)
	sequencer.ActL2EndBlock(t)

	// batch submit to L1. batcher should submit singular batches.
	batcher.ActL2BatchBuffer(t)
	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)

	// confirm batch on L1
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)
	bl := miner.L1Chain().CurrentBlock()
	log.Info("bl", "txs", len(miner.L1Chain().GetBlockByHash(bl.Hash()).Transactions()))

	// Now make enough L1 blocks that the verifier will have to derive a L2 block
	// It will also eagerly derive the block from the batcher
	for i := uint64(0); i < sd.RollupCfg.SeqWindowSize; i++ {
		miner.ActL1StartBlock(12)(t)
		miner.ActL1EndBlock(t)
	}

	// sync verifier from L1 batch in otherwise empty sequence window
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, uint64(1), verifier.SyncStatus().SafeL2.L1Origin.Number)

	// check that the tx from alice made it into the L2 chain
	verifCl := verifEngine.EthClient()
	vTx, isPending, err := verifCl.TransactionByHash(t.Ctx(), tx.Hash())
	require.NoError(t, err)
	require.False(t, isPending)
	require.NotNil(t, vTx)
}

// TestMixOfBatchesAfterHardfork tests behavior of op-node after Delta hardfork.
// op-node must accept SingularBatch and SpanBatch in sequence.
func TestMixOfBatchesAfterHardfork(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20, // larger than L1 block time we simulate in this test (12)
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	minTs := hexutil.Uint64(0)
	dp := e2eutils.MakeDeployParams(t, p)

	// Activate Delta hardfork for verifier.
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelError)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)
	verifEngine, verifier := actionsHelpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})
	rollupSeqCl := sequencer.RollupClient()
	seqEngCl := seqEngine.EthClient()

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)
	miner.ActEmptyBlock(t)

	var txHashes [4]common.Hash
	for i := 0; i < 4; i++ {
		// Alice makes a L2 tx
		n, err := seqEngCl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
		require.NoError(t, err)
		signer := types.LatestSigner(sd.L2Cfg.Config)
		tx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
			ChainID:   sd.L2Cfg.Config.ChainID,
			Nonce:     n,
			GasTipCap: big.NewInt(2 * params.GWei),
			GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
			Gas:       params.TxGas,
			To:        &dp.Addresses.Bob,
			Value:     e2eutils.Ether(2),
		})
		require.NoError(gt, seqEngCl.SendTransaction(t.Ctx(), tx))
		txHashes[i] = tx.Hash()

		// Make L2 block
		sequencer.ActL1HeadSignal(t)
		sequencer.ActL2StartBlock(t)
		seqEngine.ActL2IncludeTx(dp.Addresses.Alice)(t)
		sequencer.ActL2EndBlock(t)
		sequencer.ActBuildToL1Head(t)

		// Select batcher mode
		batcherCfg := actionsHelpers.BatcherCfg{
			MinL1TxSize:              0,
			MaxL1TxSize:              128_000,
			BatcherKey:               dp.Secrets.Batcher,
			ForceSubmitSpanBatch:     i%2 == 0, // Submit SpanBatch for odd numbered batches
			ForceSubmitSingularBatch: i%2 == 1, // Submit SingularBatch for even numbered batches
			DataAvailabilityType:     batcherFlags.CalldataType,
		}
		batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, &batcherCfg, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))
		// Submit all new blocks
		batcher.ActSubmitAll(t)

		// Confirm batch on L1
		miner.ActL1StartBlock(12)(t)
		miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
		miner.ActL1EndBlock(t)
	}

	// Now make enough L1 blocks that the verifier will have to derive a L2 block
	// It will also eagerly derive the block from the batcher
	for i := uint64(0); i < sd.RollupCfg.SeqWindowSize; i++ {
		miner.ActL1StartBlock(12)(t)
		miner.ActL1EndBlock(t)
	}

	// Sync verifier from L1 batch in otherwise empty sequence window
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)
	require.Equal(t, uint64(5), verifier.SyncStatus().SafeL2.L1Origin.Number)

	// Check that the tx from alice made it into the L2 chain
	verifCl := verifEngine.EthClient()
	for _, txHash := range txHashes {
		vTx, isPending, err := verifCl.TransactionByHash(t.Ctx(), txHash)
		require.NoError(t, err)
		require.False(t, isPending)
		require.NotNil(t, vTx)
	}
}

// TestSpanBatchEmptyChain tests derivation of empty chain using SpanBatch.
func TestSpanBatchEmptyChain(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20,
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	dp := e2eutils.MakeDeployParams(t, p)
	minTs := hexutil.Uint64(0)
	// Activate Delta hardfork
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelError)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)
	_, verifier := actionsHelpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})

	rollupSeqCl := sequencer.RollupClient()
	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, actionsHelpers.DefaultBatcherCfg(dp),
		rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	// Make 1200 empty L2 blocks (L1BlockTime / L2BlockTime * 100)
	for i := 0; i < 100; i++ {
		sequencer.ActBuildToL1Head(t)

		if i%10 == 9 {
			// batch submit to L1
			batcher.ActSubmitAll(t)

			// Since the unsafe head could be changed due to the reorg during derivation, save the current unsafe head.
			unsafeHead := sequencer.L2Unsafe().ID()

			// confirm batch on L1
			miner.ActL1StartBlock(12)(t)
			miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
			miner.ActL1EndBlock(t)

			sequencer.ActL1HeadSignal(t)
			sequencer.ActL2PipelineFull(t)

			// After derivation pipeline, the safe head must be same as latest unsafe head
			// i.e. There must be no reorg during derivation pipeline.
			require.Equal(t, sequencer.L2Safe().ID(), unsafeHead)
		} else {
			miner.ActEmptyBlock(t)
			sequencer.ActL1HeadSignal(t)
		}
	}

	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	require.Equal(t, sequencer.L2Unsafe(), sequencer.L2Safe())
	require.Equal(t, verifier.L2Unsafe(), verifier.L2Safe())
	require.Equal(t, sequencer.L2Safe(), verifier.L2Safe())
}

// TestSpanBatchLowThroughputChain tests derivation of low-throughput chain using SpanBatch.
func TestSpanBatchLowThroughputChain(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20,
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	dp := e2eutils.MakeDeployParams(t, p)
	minTs := hexutil.Uint64(0)
	// Activate Delta hardfork
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelError)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)
	_, verifier := actionsHelpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})

	rollupSeqCl := sequencer.RollupClient()
	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, actionsHelpers.DefaultBatcherCfg(dp),
		rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))
	cl := seqEngine.EthClient()

	const numTestUsers = 5
	var privKeys [numTestUsers]*ecdsa.PrivateKey
	var addrs [numTestUsers]common.Address
	for i := 0; i < numTestUsers; i++ {
		// Create a new test account
		privateKey, err := dp.Secrets.Wallet.PrivateKey(accounts.Account{
			URL: accounts.URL{
				Path: fmt.Sprintf("m/44'/60'/0'/0/%d", 10+i),
			},
		})
		privKeys[i] = privateKey
		addr := crypto.PubkeyToAddress(privateKey.PublicKey)
		require.NoError(t, err)
		addrs[i] = addr

		bal, err := cl.BalanceAt(context.Background(), addr, nil)
		require.NoError(gt, err)
		require.Equal(gt, 1, bal.Cmp(common.Big0), "account %d must have non-zero balance, address: %s, balance: %d", i, addr, bal)
	}

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	miner.ActEmptyBlock(t)
	totalTxCount := 0
	// Make 600 L2 blocks (L1BlockTime / L2BlockTime * 50) including 1~3 txs
	for i := 0; i < 50; i++ {
		for sequencer.L2Unsafe().L1Origin.Number < sequencer.SyncStatus().HeadL1.Number {
			sequencer.ActL2StartBlock(t)
			// fill the block with random number of L2 txs
			for j := 0; j < rand.Intn(3); j++ {
				userIdx := totalTxCount % numTestUsers
				signer := types.LatestSigner(sd.L2Cfg.Config)
				data := make([]byte, rand.Intn(100))
				_, err := crand.Read(data[:]) // fill with random bytes
				require.NoError(t, err)
				gas, err := core.IntrinsicGas(data, nil, false, true, true, false)
				require.NoError(t, err)
				baseFee := seqEngine.L2Chain().CurrentBlock().BaseFee
				nonce, err := cl.PendingNonceAt(t.Ctx(), addrs[userIdx])
				require.NoError(t, err)
				tx := types.MustSignNewTx(privKeys[userIdx], signer, &types.DynamicFeeTx{
					ChainID:   sd.L2Cfg.Config.ChainID,
					Nonce:     nonce,
					GasTipCap: big.NewInt(2 * params.GWei),
					GasFeeCap: new(big.Int).Add(new(big.Int).Mul(baseFee, big.NewInt(2)), big.NewInt(2*params.GWei)),
					Gas:       gas,
					To:        &dp.Addresses.Bob,
					Value:     big.NewInt(0),
					Data:      data,
				})
				require.NoError(gt, cl.SendTransaction(t.Ctx(), tx))
				seqEngine.ActL2IncludeTx(addrs[userIdx])(t)
				totalTxCount += 1
			}
			sequencer.ActL2EndBlock(t)
		}

		if i%10 == 9 {
			// batch submit to L1
			batcher.ActSubmitAll(t)

			// Since the unsafe head could be changed due to the reorg during derivation, save the current unsafe head.
			unsafeHead := sequencer.L2Unsafe().ID()

			// confirm batch on L1
			miner.ActL1StartBlock(12)(t)
			miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
			miner.ActL1EndBlock(t)

			sequencer.ActL1HeadSignal(t)
			sequencer.ActL2PipelineFull(t)

			// After derivation pipeline, the safe head must be same as latest unsafe head
			// i.e. There must be no reorg during derivation pipeline.
			require.Equal(t, sequencer.L2Safe().ID(), unsafeHead)
		} else {
			miner.ActEmptyBlock(t)
			sequencer.ActL1HeadSignal(t)
		}
	}

	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	require.Equal(t, sequencer.L2Unsafe(), sequencer.L2Safe())
	require.Equal(t, verifier.L2Unsafe(), verifier.L2Safe())
	require.Equal(t, sequencer.L2Safe(), verifier.L2Safe())
}

func TestBatchEquivalence(gt *testing.T) {
	t := actionsHelpers.NewDefaultTesting(gt)
	log := testlog.Logger(t, log.LevelError)

	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20, // larger than L1 block time we simulate in this test (12)
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	// Delta activated deploy config
	dp := e2eutils.MakeDeployParams(t, p)
	minTs := hexutil.Uint64(0)
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)
	sdDeltaActivated := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)

	// Delta deactivated deploy config
	rcfg := *sdDeltaActivated.RollupCfg
	rcfg.DeltaTime = nil
	sdDeltaDeactivated := &e2eutils.SetupData{
		L1Cfg:         sdDeltaActivated.L1Cfg,
		L2Cfg:         sdDeltaActivated.L2Cfg,
		RollupCfg:     &rcfg,
		DeploymentsL1: sdDeltaActivated.DeploymentsL1,
	}

	// Setup sequencer
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sdDeltaActivated, log)
	rollupSeqCl := sequencer.RollupClient()
	seqEngCl := seqEngine.EthClient()

	// Setup Delta activated spanVerifier
	_, spanVerifier := actionsHelpers.SetupVerifier(t, sdDeltaActivated, log, miner.L1Client(t, sdDeltaActivated.RollupCfg), miner.BlobStore(), &sync.Config{})

	// Setup Delta deactivated spanVerifier
	_, singularVerifier := actionsHelpers.SetupVerifier(t, sdDeltaDeactivated, log, miner.L1Client(t, sdDeltaDeactivated.RollupCfg), miner.BlobStore(), &sync.Config{})

	// Setup SpanBatcher
	spanBatcher := actionsHelpers.NewL2Batcher(log, sdDeltaActivated.RollupCfg, &actionsHelpers.BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           dp.Secrets.Batcher,
		ForceSubmitSpanBatch: true,
		DataAvailabilityType: batcherFlags.CalldataType,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sdDeltaActivated.RollupCfg))

	// Setup SingularBatcher
	singularBatcher := actionsHelpers.NewL2Batcher(log, sdDeltaDeactivated.RollupCfg, &actionsHelpers.BatcherCfg{
		MinL1TxSize:              0,
		MaxL1TxSize:              128_000,
		BatcherKey:               dp.Secrets.Batcher,
		ForceSubmitSingularBatch: true,
		DataAvailabilityType:     batcherFlags.CalldataType,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sdDeltaDeactivated.RollupCfg))

	const numTestUsers = 5
	var privKeys [numTestUsers]*ecdsa.PrivateKey
	var addrs [numTestUsers]common.Address
	for i := 0; i < numTestUsers; i++ {
		// Create a new test account
		privateKey, err := dp.Secrets.Wallet.PrivateKey(accounts.Account{
			URL: accounts.URL{
				Path: fmt.Sprintf("m/44'/60'/0'/0/%d", 10+i),
			},
		})
		privKeys[i] = privateKey
		addr := crypto.PubkeyToAddress(privateKey.PublicKey)
		require.NoError(t, err)
		addrs[i] = addr
	}

	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	totalTxCount := 0
	// Build random blocks
	for sequencer.L2Unsafe().L1Origin.Number < sequencer.SyncStatus().HeadL1.Number {
		sequencer.ActL2StartBlock(t)
		// fill the block with random number of L2 txs
		for j := 0; j < rand.Intn(3); j++ {
			userIdx := totalTxCount % numTestUsers
			signer := types.LatestSigner(sdDeltaActivated.L2Cfg.Config)
			data := make([]byte, rand.Intn(100))
			_, err := crand.Read(data[:]) // fill with random bytes
			require.NoError(t, err)
			gas, err := core.IntrinsicGas(data, nil, false, true, true, false)
			require.NoError(t, err)
			baseFee := seqEngine.L2Chain().CurrentBlock().BaseFee
			nonce, err := seqEngCl.PendingNonceAt(t.Ctx(), addrs[userIdx])
			require.NoError(t, err)
			tx := types.MustSignNewTx(privKeys[userIdx], signer, &types.DynamicFeeTx{
				ChainID:   sdDeltaActivated.L2Cfg.Config.ChainID,
				Nonce:     nonce,
				GasTipCap: big.NewInt(2 * params.GWei),
				GasFeeCap: new(big.Int).Add(new(big.Int).Mul(baseFee, big.NewInt(2)), big.NewInt(2*params.GWei)),
				Gas:       gas,
				To:        &dp.Addresses.Bob,
				Value:     big.NewInt(0),
				Data:      data,
			})
			require.NoError(gt, seqEngCl.SendTransaction(t.Ctx(), tx))
			seqEngine.ActL2IncludeTx(addrs[userIdx])(t)
			totalTxCount += 1
		}
		sequencer.ActL2EndBlock(t)
	}

	// Submit SpanBatch
	spanBatcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// Run derivation pipeline for verifiers
	spanVerifier.ActL1HeadSignal(t)
	spanVerifier.ActL2PipelineFull(t)
	singularVerifier.ActL1HeadSignal(t)
	singularVerifier.ActL2PipelineFull(t)

	// Delta activated spanVerifier must be synced
	require.Equal(t, spanVerifier.L2Safe(), sequencer.L2Unsafe())
	// Delta deactivated spanVerifier could not derive SpanBatch
	require.Equal(t, singularVerifier.L2Safe().Number, uint64(0))

	// Submit SingularBatches
	singularBatcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// Run derivation pipeline for verifiers
	spanVerifier.ActL1HeadSignal(t)
	spanVerifier.ActL2PipelineFull(t)
	singularVerifier.ActL1HeadSignal(t)
	singularVerifier.ActL2PipelineFull(t)

	// Delta deactivated spanVerifier must be synced
	require.Equal(t, spanVerifier.L2Safe(), singularVerifier.L2Safe())
}
