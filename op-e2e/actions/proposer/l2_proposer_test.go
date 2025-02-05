package proposer

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	upgradesHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/upgrades/helpers"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/bindingspreview"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

// TestProposerBatchType run each proposer-related test case in singular batch mode and span batch mode.
func TestProposerBatchType(t *testing.T) {
	t.Run("SingularBatch/Standard", func(t *testing.T) {
		runProposerTest(t, nil, config.AllocTypeStandard)
	})
	t.Run("SingularBatch/L2OO", func(t *testing.T) {
		runProposerTest(t, nil, config.AllocTypeL2OO)
	})
	t.Run("SpanBatch/Standard", func(t *testing.T) {
		deltaTimeOffset := hexutil.Uint64(0)
		runProposerTest(t, &deltaTimeOffset, config.AllocTypeStandard)
	})
	t.Run("SpanBatch/L2OO", func(t *testing.T) {
		deltaTimeOffset := hexutil.Uint64(0)
		runProposerTest(t, &deltaTimeOffset, config.AllocTypeL2OO)
	})
}

func runProposerTest(gt *testing.T, deltaTimeOffset *hexutil.Uint64, allocType config.AllocType) {
	t := actionsHelpers.NewDefaultTesting(gt)
	params := actionsHelpers.DefaultRollupTestParams()
	params.AllocType = allocType
	dp := e2eutils.MakeDeployParams(t, params)
	upgradesHelpers.ApplyDeltaTimeOffset(dp, deltaTimeOffset)
	sd := e2eutils.Setup(t, dp, actionsHelpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)

	rollupSeqCl := sequencer.RollupClient()
	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, actionsHelpers.DefaultBatcherCfg(dp),
		rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	var proposer *actionsHelpers.L2Proposer
	if allocType.UsesProofs() {
		optimismPortal2Contract, err := bindingspreview.NewOptimismPortal2(sd.DeploymentsL1.OptimismPortalProxy, miner.EthClient())
		require.NoError(t, err)
		respectedGameType, err := optimismPortal2Contract.RespectedGameType(&bind.CallOpts{})
		require.NoError(t, err)
		proposer = actionsHelpers.NewL2Proposer(t, log, &actionsHelpers.ProposerCfg{
			DisputeGameFactoryAddr: &sd.DeploymentsL1.DisputeGameFactoryProxy,
			ProposalInterval:       6 * time.Second,
			ProposalRetryInterval:  3 * time.Second,
			DisputeGameType:        respectedGameType,
			ProposerKey:            dp.Secrets.Proposer,
			AllowNonFinalized:      true,
			AllocType:              allocType,
		}, miner.EthClient(), rollupSeqCl)
	} else {
		proposer = actionsHelpers.NewL2Proposer(t, log, &actionsHelpers.ProposerCfg{
			OutputOracleAddr:      &sd.DeploymentsL1.L2OutputOracleProxy,
			ProposerKey:           dp.Secrets.Proposer,
			ProposalRetryInterval: 3 * time.Second,
			AllowNonFinalized:     false,
			AllocType:             allocType,
		}, miner.EthClient(), rollupSeqCl)
	}

	// L1 block
	miner.ActEmptyBlock(t)
	// L2 block
	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)
	sequencer.ActBuildToL1Head(t)
	// submit and include in L1
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)
	// finalize the first and second L1 blocks, including the batch
	miner.ActL1SafeNext(t)
	miner.ActL1SafeNext(t)
	miner.ActL1FinalizeNext(t)
	miner.ActL1FinalizeNext(t)
	// derive and see the L2 chain fully finalize
	sequencer.ActL2PipelineFull(t)
	sequencer.ActL1SafeSignal(t)
	sequencer.ActL1FinalizedSignal(t)
	sequencer.ActL2PipelineFull(t)
	require.Equal(t, sequencer.SyncStatus().UnsafeL2, sequencer.SyncStatus().FinalizedL2)
	require.True(t, proposer.CanPropose(t))

	// make proposals until there is nothing left to propose
	for proposer.CanPropose(t) {
		proposer.ActMakeProposalTx(t)
		// include proposal on L1
		miner.ActL1StartBlock(12)(t)
		miner.ActL1IncludeTx(dp.Addresses.Proposer)(t)
		miner.ActL1EndBlock(t)
		// Check proposal was successful
		receipt, err := miner.EthClient().TransactionReceipt(t.Ctx(), proposer.LastProposalTx())
		require.NoError(t, err)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "proposal failed")
	}

	// check that L1 stored the expected output root
	if allocType.UsesProofs() {
		optimismPortal2Contract, err := bindingspreview.NewOptimismPortal2(sd.DeploymentsL1.OptimismPortalProxy, miner.EthClient())
		require.NoError(t, err)
		respectedGameType, err := optimismPortal2Contract.RespectedGameType(&bind.CallOpts{})
		require.NoError(t, err)
		disputeGameFactoryContract, err := bindings.NewDisputeGameFactory(sd.DeploymentsL1.DisputeGameFactoryProxy, miner.EthClient())
		require.NoError(t, err)
		gameCount, err := disputeGameFactoryContract.GameCount(&bind.CallOpts{})
		require.NoError(t, err)
		require.Greater(t, gameCount.Uint64(), uint64(0), "game count must be greater than 0")
		latestGames, err := disputeGameFactoryContract.FindLatestGames(&bind.CallOpts{}, respectedGameType, new(big.Int).Sub(gameCount, common.Big1), common.Big1)
		require.NoError(t, err)
		require.Greater(t, len(latestGames), 0, "latest games must be greater than 0")
		latestGame := latestGames[0]
		gameBlockNumber := new(big.Int)
		gameBlockNumber.SetBytes(latestGame.ExtraData[0:32])
		block, err := seqEngine.EthClient().BlockByNumber(t.Ctx(), gameBlockNumber)
		require.NoError(t, err)
		require.Less(t, block.Time(), latestGame.Timestamp, "output is registered with L1 timestamp of proposal tx, past L2 block")
		outputComputed, err := sequencer.RollupClient().OutputAtBlock(t.Ctx(), gameBlockNumber.Uint64())
		require.NoError(t, err)
		require.Equal(t, eth.Bytes32(latestGame.RootClaim), outputComputed.OutputRoot, "output roots must match")
	} else {
		outputOracleContract, err := bindings.NewL2OutputOracle(sd.DeploymentsL1.L2OutputOracleProxy, miner.EthClient())
		require.NoError(t, err)
		blockNumber, err := outputOracleContract.LatestBlockNumber(&bind.CallOpts{})
		require.NoError(t, err)
		require.Greater(t, int64(blockNumber.Uint64()), int64(0), "latest block number must be greater than 0")
		block, err := seqEngine.EthClient().BlockByNumber(t.Ctx(), blockNumber)
		require.NoError(t, err)
		outputOnL1, err := outputOracleContract.GetL2OutputAfter(&bind.CallOpts{}, blockNumber)
		require.NoError(t, err)
		require.Less(t, block.Time(), outputOnL1.Timestamp.Uint64(), "output is registered with L1 timestamp of proposal tx, past L2 block")
		outputComputed, err := sequencer.RollupClient().OutputAtBlock(t.Ctx(), blockNumber.Uint64())
		require.NoError(t, err)
		require.Equal(t, eth.Bytes32(outputOnL1.OutputRoot), outputComputed.OutputRoot, "output roots must match")
	}
}
