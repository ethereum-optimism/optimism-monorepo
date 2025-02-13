package faultproofs

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os/exec"
	"path/filepath"
	"testing"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs"
	e2e_config "github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/disputegame"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestPrecompiles_Standard(t *testing.T) {
	testPrecompiles(t, e2e_config.AllocTypeStandard)
}

func TestPrecompiles_Multithreaded(t *testing.T) {
	testPrecompiles(t, e2e_config.AllocTypeMTCannon)
}

func testPrecompiles(t *testing.T, allocType e2e_config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	for _, test := range proofs.PrecompileTestFixtures {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			op_e2e.InitParallel(t, op_e2e.UsesCannon)
			ctx := context.Background()
			genesisTime := hexutil.Uint64(0)
			cfg := e2esys.IsthmusSystemConfig(t, &genesisTime, e2esys.WithAllocType(allocType))
			// We don't need a verifier - just the sequencer is enough
			delete(cfg.Nodes, "verifier")

			sys, err := cfg.Start(t)
			require.Nil(t, err, "Error starting up system")

			log := testlog.Logger(t, log.LevelInfo)
			log.Info("genesis", "l2", sys.RollupConfig.Genesis.L2, "l1", sys.RollupConfig.Genesis.L1, "l2_time", sys.RollupConfig.Genesis.L2Time)

			l1Client := sys.NodeClient("l1")
			l2Seq := sys.NodeClient("sequencer")
			rollupClient := sys.RollupClient("sequencer")
			aliceKey := cfg.Secrets.Alice

			t.Log("Capture current L2 head as agreed starting point")
			latestBlock, err := l2Seq.BlockByNumber(ctx, nil)
			require.NoError(t, err)
			agreedL2Output, err := rollupClient.OutputAtBlock(ctx, latestBlock.NumberU64())
			require.NoError(t, err, "could not retrieve l2 agreed block")
			l2Head := agreedL2Output.BlockRef.Hash
			l2OutputRoot := agreedL2Output.OutputRoot

			receipt := helpers.SendL2Tx(t, cfg, l2Seq, aliceKey, func(opts *helpers.TxOpts) {
				opts.Gas = 1_000_000
				opts.ToAddr = &test.Address
				opts.Nonce = 0
				opts.Data = test.Input
			})

			t.Log("Determine L2 claim")
			l2ClaimBlockNumber := receipt.BlockNumber
			l2Output, err := rollupClient.OutputAtBlock(ctx, l2ClaimBlockNumber.Uint64())
			require.NoError(t, err, "could not get expected output")
			l2Claim := l2Output.OutputRoot

			t.Log("Determine L1 head that includes all batches required for L2 claim block")
			require.NoError(t, wait.ForSafeBlock(ctx, rollupClient, l2ClaimBlockNumber.Uint64()))
			l1HeadBlock, err := l1Client.BlockByNumber(ctx, nil)
			require.NoError(t, err, "get l1 head block")
			l1Head := l1HeadBlock.Hash()

			inputs := utils.LocalGameInputs{
				L1Head:        l1Head,
				L2Head:        l2Head,
				L2Claim:       common.Hash(l2Claim),
				L2OutputRoot:  common.Hash(l2OutputRoot),
				L2BlockNumber: l2ClaimBlockNumber,
			}
			runCannon(t, ctx, sys, inputs)
		})

		t.Run("DisputePrecompile-"+test.Name, func(t *testing.T) {
			op_e2e.InitParallel(t, op_e2e.UsesCannon)
			if !test.Accelerated {
				t.Skipf("%v is not accelerated so no preimgae to upload", test.Name)
			}
			ctx := context.Background()
			sys, _ := StartFaultDisputeSystem(t, WithLatestFork(), WithAllocType(allocType))

			l2Seq := sys.NodeClient("sequencer")
			aliceKey := sys.Cfg.Secrets.Alice
			receipt := helpers.SendL2Tx(t, sys.Cfg, l2Seq, aliceKey, func(opts *helpers.TxOpts) {
				opts.Gas = 1_000_000
				opts.ToAddr = &test.Address
				opts.Nonce = 0
				opts.Data = test.Input
			})

			disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
			game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", receipt.BlockNumber.Uint64(), common.Hash{0x01, 0xaa})
			require.NotNil(t, game)
			outputRootClaim := game.DisputeLastBlock(ctx)
			game.LogGameData(ctx)

			honestChallenger := game.StartChallenger(ctx, "HonestActor", challenger.WithPrivKey(sys.Cfg.Secrets.Alice))

			// Wait for the honest challenger to dispute the outputRootClaim. This creates a root of an execution game that we challenge by coercing
			// a step at a preimage trace index.
			outputRootClaim = outputRootClaim.WaitForCounterClaim(ctx)

			// Now the honest challenger is positioned as the defender of the execution game
			// We then move to challenge it to induce a preimage load
			preimageLoadCheck := game.CreateStepPreimageLoadCheck(ctx)
			game.ChallengeToPreimageLoad(ctx, outputRootClaim, sys.Cfg.Secrets.Alice, utils.FirstPreimageLoadOfType("precompile"), preimageLoadCheck, false)
			// The above method already verified the image was uploaded and step called successfully
			// So we don't waste time resolving the game - that's tested elsewhere.
			require.NoError(t, honestChallenger.Close())
		})
	}
}

func TestGranitePrecompiles_Standard(t *testing.T) {
	testGranitePrecompiles(t, e2e_config.AllocTypeStandard)
}

func TestGranitePrecompiles_Multithreaded(t *testing.T) {
	testGranitePrecompiles(t, e2e_config.AllocTypeMTCannon)
}

func testGranitePrecompiles(t *testing.T, allocType e2e_config.AllocType) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	ctx := context.Background()
	genesisTime := hexutil.Uint64(0)
	cfg := e2esys.GraniteSystemConfig(t, &genesisTime, e2esys.WithAllocType(allocType))
	// We don't need a verifier - just the sequencer is enough
	delete(cfg.Nodes, "verifier")

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	log := testlog.Logger(t, log.LevelInfo)
	log.Info("genesis", "l2", sys.RollupConfig.Genesis.L2, "l1", sys.RollupConfig.Genesis.L1, "l2_time", sys.RollupConfig.Genesis.L2Time)

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")
	rollupClient := sys.RollupClient("sequencer")
	aliceKey := cfg.Secrets.Alice

	t.Log("Capture current L2 head as agreed starting point")
	latestBlock, err := l2Seq.BlockByNumber(ctx, nil)
	require.NoError(t, err)
	agreedL2Output, err := rollupClient.OutputAtBlock(ctx, latestBlock.NumberU64())
	require.NoError(t, err, "could not retrieve l2 agreed block")
	l2Head := agreedL2Output.BlockRef.Hash
	l2OutputRoot := agreedL2Output.OutputRoot

	precompile := common.BytesToAddress([]byte{0x08})
	input := make([]byte, 113_000)
	tx := types.MustSignNewTx(aliceKey, types.LatestSignerForChainID(cfg.L2ChainIDBig()), &types.DynamicFeeTx{
		ChainID:   cfg.L2ChainIDBig(),
		Nonce:     0,
		GasTipCap: big.NewInt(1 * params.GWei),
		GasFeeCap: big.NewInt(10 * params.GWei),
		Gas:       25_000_000,
		To:        &precompile,
		Value:     big.NewInt(0),
		Data:      input,
	})
	err = l2Seq.SendTransaction(ctx, tx)
	require.NoError(t, err, "Should send bn256Pairing transaction")
	// Expect a successful receipt to retrieve the EVM call trace so we can inspect the revert reason
	receipt, err := wait.ForReceiptMaybe(ctx, l2Seq, tx.Hash(), types.ReceiptStatusSuccessful, false)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "bad elliptic curve pairing input size")

	t.Logf("Transaction hash %v", tx.Hash())
	t.Log("Determine L2 claim")
	l2ClaimBlockNumber := receipt.BlockNumber
	l2Output, err := rollupClient.OutputAtBlock(ctx, l2ClaimBlockNumber.Uint64())
	require.NoError(t, err, "could not get expected output")
	l2Claim := l2Output.OutputRoot

	t.Log("Determine L1 head that includes all batches required for L2 claim block")
	require.NoError(t, wait.ForSafeBlock(ctx, rollupClient, l2ClaimBlockNumber.Uint64()))
	l1HeadBlock, err := l1Client.BlockByNumber(ctx, nil)
	require.NoError(t, err, "get l1 head block")
	l1Head := l1HeadBlock.Hash()

	inputs := utils.LocalGameInputs{
		L1Head:        l1Head,
		L2Head:        l2Head,
		L2Claim:       common.Hash(l2Claim),
		L2OutputRoot:  common.Hash(l2OutputRoot),
		L2BlockNumber: l2ClaimBlockNumber,
	}
	runCannon(t, ctx, sys, inputs)
}

func runCannon(t *testing.T, ctx context.Context, sys *e2esys.System, inputs utils.LocalGameInputs, extraVmArgs ...string) {
	l1Endpoint := sys.NodeEndpoint("l1").RPC()
	l1Beacon := sys.L1BeaconEndpoint().RestHTTP()
	rollupEndpoint := sys.RollupEndpoint("sequencer").RPC()
	l2Endpoint := sys.NodeEndpoint("sequencer").RPC()
	cannonOpts := challenger.WithCannon(t, sys)
	dir := t.TempDir()
	proofsDir := filepath.Join(dir, "cannon-proofs")
	cfg := config.NewConfig(common.Address{}, l1Endpoint, l1Beacon, rollupEndpoint, l2Endpoint, dir)
	cfg.Cannon.L2Custom = true
	cannonOpts(&cfg)

	logger := testlog.Logger(t, log.LevelInfo).New("role", "cannon")
	executor := vm.NewExecutor(logger, metrics.NoopMetrics.ToTypedVmMetrics("cannon"), cfg.Cannon, vm.NewOpProgramServerExecutor(logger), cfg.CannonAbsolutePreState, inputs)

	t.Log("Running cannon")
	err := executor.DoGenerateProof(ctx, proofsDir, math.MaxUint, math.MaxUint, extraVmArgs...)
	require.NoError(t, err, "failed to generate proof")

	stdOut, _, err := runCmd(ctx, cfg.Cannon.VmBin, "witness", "--input", vm.FinalStatePath(proofsDir, cfg.Cannon.BinarySnapshots))
	require.NoError(t, err, "failed to run witness cmd")
	type stateData struct {
		Step     uint64 `json:"step"`
		ExitCode uint8  `json:"exitCode"`
		Exited   bool   `json:"exited"`
	}
	var data stateData
	err = json.Unmarshal([]byte(stdOut), &data)
	require.NoError(t, err, "failed to parse state data")
	require.True(t, data.Exited, "cannon did not exit")
	require.Zero(t, data.ExitCode, "cannon failed with exit code %d", data.ExitCode)
	t.Logf("Completed in %d steps", data.Step)
}

func runCmd(ctx context.Context, binary string, args ...string) (stdOut string, stdErr string, err error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	stdOut = outBuf.String()
	stdErr = errBuf.String()
	return
}
