package proofs

import (
	"fmt"
	"testing"

	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestPragueForkAfterGenesis(gt *testing.T) {
	type testCase struct {
		name         string
		useSetCodeTx bool
	}

	testCases := []testCase{
		{
			name: "calldata", useSetCodeTx: false,
		},
		{
			name: "setCode", useSetCodeTx: true,
		},
	}

	runL1PragueTest := func(gt *testing.T, testCfg *helpers.TestCfg[testCase]) {
		t := actionsHelpers.NewDefaultTesting(gt)
		env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(),
			helpers.NewBatcherCfg(
				func(c *actionsHelpers.BatcherCfg) {
					c.DataAvailabilityType = batcherFlags.CalldataType
				},
			),
			func(dp *genesis.DeployConfig) {
				t := hexutil.Uint64(24) // Activate at second l1 block
				dp.L1PragueTimeOffset = &t
			},
		)

		miner, batcher, verifier, sequencer := env.Miner, env.Batcher, env.Sequencer, env.Sequencer

		// utils
		checkVerifierDerivedToL1Head := func(t actionsHelpers.StatefulTesting) {
			l1Head := miner.L1Chain().CurrentBlock()
			currentL1 := verifier.SyncStatus().CurrentL1
			require.Equal(t, l1Head.Number.Int64(), int64(currentL1.Number), "verifier should derive up to and including the L1 head")
			require.Equal(t, l1Head.Hash(), currentL1.Hash, "verifier should derive up to and including the L1 head")
		}

		buildUnsafeL2AndSubmit := func(useSetCode bool) {
			sequencer.ActL1HeadSignal(t)
			sequencer.ActBuildToL1Head(t)

			miner.ActL1StartBlock(12)(t)
			if useSetCode {
				batcher.ActBufferAll(t)
				batcher.ActL2ChannelClose(t)
				batcher.ActSubmitSetCodeTx(t)
			} else {
				batcher.ActSubmitAll(t)
			}
			miner.ActL1IncludeTx(batcher.BatcherAddr)(t)
			miner.ActL1EndBlock(t)
		}

		checkPragueStatusOnL1 := func(active bool) {
			l1Head := miner.L1Chain().CurrentBlock()
			if active {
				// require.True(t, sd.L1Cfg.Config.IsPrague(l1Head.Number, l1Head.Time), "Prague should be active")
				require.NotNil(t, l1Head.RequestsHash, "Prague header requests hash should be non-nil")
			} else {
				// require.False(t, sd.L1Cfg.Config.IsPrague(l1Head.Number, l1Head.Time), "Prague should not be active yet")
				require.Nil(t, l1Head.RequestsHash, "Prague header requests hash should be nil")
			}
		}

		syncVerifierAndCheck := func(t actionsHelpers.StatefulTesting) {
			verifier.ActL1HeadSignal(t)
			verifier.ActL2PipelineFull(t)
			checkVerifierDerivedToL1Head(t)
		}

		// Check initially Prague is not activated
		checkPragueStatusOnL1(false)

		// Start op-nodes
		sequencer.ActL2PipelineFull(t)
		verifier.ActL2PipelineFull(t)

		// Build L1 blocks, crossing the fork boundary
		miner.ActEmptyBlock(t) // block 1
		miner.ActEmptyBlock(t) // Prague activates here (block 2)

		// Here's a block with a type 4 deposit transaction
		miner.ActL1StartBlock(12)(t) // block 3
		tx, err := actionsHelpers.PrepareSignedSetCodeTx(
			*uint256.MustFromBig(env.Sd.L1Cfg.Config.ChainID),
			env.Dp.Secrets.Alice,
			env.Alice.L1.Signer(),
			env.Alice.L1.PendingNonce(t), // nonce
			env.Sd.DeploymentsL1.OptimismPortalProxy,
			[]byte{})
		require.NoError(t, err, "failed to prepare set code tx")
		err = miner.EthClient().SendTransaction(t.Ctx(), tx)
		require.NoError(t, err, "failed to send set code tx")
		miner.ActL1IncludeTx(env.Alice.Address())(t)
		miner.ActL1EndBlock(t)

		// Check that Prague is active on L1
		checkPragueStatusOnL1(true)

		// Cache safe head before verifier sync
		safeL2Before := verifier.SyncStatus().SafeL2

		// Build L2 unsafe chain and batch it to L1 using either calldata or
		// EIP-7702 setCode txs
		// https://github.com/ethereum/EIPs/blob/master/EIPS/eip-7702.md
		buildUnsafeL2AndSubmit(testCfg.Custom.useSetCodeTx)

		// Check verifier derived from Prague L1 blocks
		syncVerifierAndCheck(t)

		// Check safe head did or did not change,
		// depending on tx type used by batcher:
		safeL2After := verifier.SyncStatus().SafeL2
		if testCfg.Custom.useSetCodeTx {
			require.Equal(t, safeL2Before, safeL2After, "safe head should not have changed (set code batcher tx ignored)")
			require.Equal(t, uint64(0), verifier.SyncStatus().SafeL2.L1Origin.Number, "l1 origin of l2 safe should not have changed (set code batcher tx ignored)")
		} else {
			require.Greater(t, safeL2After.Number, safeL2Before.Number, "safe head should have progressed (calldata batcher tx derived from)")
			require.Equal(t, uint64(3), verifier.SyncStatus().SafeL2.L1Origin.Number, "l1 origin of l2 safe should have progressed (calldata batcher tx derived from)")
		}

		env.RunFaultProofProgram(t, safeL2After.Number, testCfg.CheckResult, testCfg.InputParams...)
	}

	matrix := helpers.NewMatrix[testCase]()
	defer matrix.Run(gt)

	for _, tc := range testCases {
		matrix.AddTestCase(
			fmt.Sprintf("HonestClaim-%s", tc.name),
			tc,
			helpers.NewForkMatrix(helpers.LatestFork),
			runL1PragueTest,
			helpers.ExpectNoError(),
		)
		matrix.AddTestCase(
			fmt.Sprintf("JunkClaim-%s", tc.name),
			tc,
			helpers.NewForkMatrix(helpers.LatestFork),
			runL1PragueTest,
			helpers.ExpectError(claim.ErrClaimNotValid),
			helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
		)
	}
}
