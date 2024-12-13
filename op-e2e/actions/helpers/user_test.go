package helpers

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	bindingspreview "github.com/ethereum-optimism/optimism/op-node/bindings/preview"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type hardforkScheduledTest struct {
	regolithTime *hexutil.Uint64
	canyonTime   *hexutil.Uint64
	deltaTime    *hexutil.Uint64
	ecotoneTime  *hexutil.Uint64
	fjordTime    *hexutil.Uint64
	graniteTime  *hexutil.Uint64
	holoceneTime *hexutil.Uint64
	runToFork    string
	allocType    config.AllocType
}

func (tc *hardforkScheduledTest) SetFork(fork string, v uint64) {
	*tc.fork(fork) = (*hexutil.Uint64)(&v)
}

func (tc *hardforkScheduledTest) GetFork(fork string) *uint64 {
	return (*uint64)(*tc.fork(fork))
}

func (tc *hardforkScheduledTest) fork(fork string) **hexutil.Uint64 {
	switch fork {
	case "holocene":
		return &tc.holoceneTime
	case "granite":
		return &tc.graniteTime
	case "fjord":
		return &tc.fjordTime
	case "ecotone":
		return &tc.ecotoneTime
	case "delta":
		return &tc.deltaTime
	case "canyon":
		return &tc.canyonTime
	case "regolith":
		return &tc.regolithTime
	default:
		panic(fmt.Errorf("unrecognized fork: %s", fork))
	}
}

func TestCrossLayerUser_Standard(t *testing.T) {
	testCrossLayerUser(t, config.AllocTypeStandard)
}

func TestCrossLayerUser_L2OO(t *testing.T) {
	testCrossLayerUser(t, config.AllocTypeL2OO)
}

// TestCrossLayerUser tests that common actions of the CrossLayerUser actor work in various hardfork configurations:
// - transact on L1
// - transact on L2
// - deposit on L1
// - withdraw from L2
// - prove tx on L1
// - wait 1 week + 1 second
// - finalize withdrawal on L1
func testCrossLayerUser(t *testing.T, allocType config.AllocType) {
	futureTime := uint64(20)
	farFutureTime := uint64(2000)

	forks := []string{
		"regolith",
		"canyon",
		"delta",
		"ecotone",
		"fjord",
		"granite",
		"holocene",
	}
	for i, fork := range forks {
		i := i
		fork := fork
		t.Run("fork_"+fork, func(t *testing.T) {
			t.Run("at_genesis", func(t *testing.T) {
				tc := hardforkScheduledTest{
					allocType: allocType,
				}
				for _, f := range forks[:i+1] { // activate, all up to and incl this fork, at genesis
					tc.SetFork(f, 0)
				}
				runCrossLayerUserTest(t, tc)
			})
			t.Run("after_genesis", func(t *testing.T) {
				tc := hardforkScheduledTest{
					allocType: allocType,
				}
				for _, f := range forks[:i] { // activate, all up to this fork, at genesis
					tc.SetFork(f, 0)
				}
				// activate this fork after genesis
				tc.SetFork(fork, futureTime)
				tc.runToFork = fork
				runCrossLayerUserTest(t, tc)
			})
			t.Run("not_yet", func(t *testing.T) {
				tc := hardforkScheduledTest{
					allocType: allocType,
				}
				for _, f := range forks[:i] { // activate, all up to this fork, at genesis
					tc.SetFork(f, 0)
				}
				// activate this fork later
				tc.SetFork(fork, farFutureTime)
				if i > 0 {
					tc.runToFork = forks[i-1]
				}
				runCrossLayerUserTest(t, tc)
			})
		})
	}
}

func runCrossLayerUserTest(gt *testing.T, test hardforkScheduledTest) {
	t := NewDefaultTesting(gt)
	params := DefaultRollupTestParams()
	params.AllocType = test.allocType
	dp := e2eutils.MakeDeployParams(t, params)
	// This overwrites all deploy-config settings,
	// so even when the deploy-config defaults change, we test the right transitions.
	dp.DeployConfig.L2GenesisRegolithTimeOffset = test.regolithTime
	dp.DeployConfig.L2GenesisCanyonTimeOffset = test.canyonTime
	dp.DeployConfig.L2GenesisDeltaTimeOffset = test.deltaTime
	dp.DeployConfig.L2GenesisEcotoneTimeOffset = test.ecotoneTime
	dp.DeployConfig.L2GenesisFjordTimeOffset = test.fjordTime
	dp.DeployConfig.L2GenesisGraniteTimeOffset = test.graniteTime
	dp.DeployConfig.L2GenesisHoloceneTimeOffset = test.holoceneTime

	if test.canyonTime != nil {
		require.Zero(t, uint64(*test.canyonTime)%uint64(dp.DeployConfig.L2BlockTime), "canyon fork must be aligned")
	}
	if test.ecotoneTime != nil {
		require.Zero(t, uint64(*test.ecotoneTime)%uint64(dp.DeployConfig.L2BlockTime), "ecotone fork must be aligned")
	}

	sd := e2eutils.Setup(t, dp, DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)

	require.Equal(t, dp.Secrets.Addresses().Batcher, dp.DeployConfig.BatchSenderAddress)
	require.Equal(t, dp.Secrets.Addresses().Proposer, dp.DeployConfig.L2OutputOracleProposer)

	miner, seqEngine, seq := SetupSequencerTest(t, sd, log)
	batcher := NewL2Batcher(log, sd.RollupCfg, DefaultBatcherCfg(dp),
		seq.RollupClient(), miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	var proposer *L2Proposer
	if test.allocType.UsesProofs() {
		optimismPortal2Contract, err := bindingspreview.NewOptimismPortal2(sd.DeploymentsL1.OptimismPortalProxy, miner.EthClient())
		require.NoError(t, err)
		respectedGameType, err := optimismPortal2Contract.RespectedGameType(&bind.CallOpts{})
		require.NoError(t, err)
		proposer = NewL2Proposer(t, log, &ProposerCfg{
			DisputeGameFactoryAddr: &sd.DeploymentsL1.DisputeGameFactoryProxy,
			ProposalInterval:       6 * time.Second,
			ProposalRetryInterval:  3 * time.Second,
			DisputeGameType:        respectedGameType,
			ProposerKey:            dp.Secrets.Proposer,
			AllowNonFinalized:      true,
			AllocType:              test.allocType,
		}, miner.EthClient(), seq.RollupClient())
	} else {
		proposer = NewL2Proposer(t, log, &ProposerCfg{
			OutputOracleAddr:      &sd.DeploymentsL1.L2OutputOracleProxy,
			ProposerKey:           dp.Secrets.Proposer,
			ProposalRetryInterval: 3 * time.Second,
			AllowNonFinalized:     true,
			AllocType:             test.allocType,
		}, miner.EthClient(), seq.RollupClient())
	}

	// need to start derivation before we can make L2 blocks
	seq.ActL2PipelineFull(t)

	l1Cl := miner.EthClient()
	l2Cl := seqEngine.EthClient()
	l2ProofCl := seqEngine.GethClient()

	addresses := e2eutils.CollectAddresses(sd, dp)

	l1UserEnv := &BasicUserEnv[*L1Bindings]{
		EthCl:          l1Cl,
		Signer:         types.LatestSigner(sd.L1Cfg.Config),
		AddressCorpora: addresses,
		Bindings:       NewL1Bindings(t, l1Cl, test.allocType),
	}
	l2UserEnv := &BasicUserEnv[*L2Bindings]{
		EthCl:          l2Cl,
		Signer:         types.LatestSigner(sd.L2Cfg.Config),
		AddressCorpora: addresses,
		Bindings:       NewL2Bindings(t, l2Cl, l2ProofCl),
	}

	alice := NewCrossLayerUser(log, dp.Secrets.Alice, rand.New(rand.NewSource(1234)), test.allocType)
	alice.L1.SetUserEnv(l1UserEnv)
	alice.L2.SetUserEnv(l2UserEnv)

	// Build at least one l2 block so we have an unsafe head with a deposit info tx (genesis block doesn't)
	seq.ActL2StartBlock(t)
	seq.ActL2EndBlock(t)

	if test.runToFork != "" {
		forkTime := test.GetFork(test.runToFork)
		require.NotNil(t, forkTime, "fork we are running up to must be configured")
		// advance L2 enough to activate the fork we are running up to
		seq.ActBuildL2ToTime(t, *forkTime)
	}
	// Check Regolith is active or not by confirming the system info tx is not a system tx
	infoTx, err := l2Cl.TransactionInBlock(t.Ctx(), seq.L2Unsafe().Hash, 0)
	require.NoError(t, err)
	require.True(t, infoTx.IsDepositTx())
	// Should only be a system tx if regolith is not enabled
	require.Equal(t, !seq.RollupCfg.IsRegolith(seq.L2Unsafe().Time), infoTx.IsSystemTx())

	// regular L2 tx, in new L2 block
	alice.L2.ActResetTxOpts(t)
	alice.L2.ActSetTxToAddr(&dp.Addresses.Bob)(t)
	alice.L2.ActMakeTx(t)
	seq.ActL2StartBlock(t)
	seqEngine.ActL2IncludeTx(alice.Address())(t)
	seq.ActL2EndBlock(t)
	alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)

	// regular L1 tx, in new L1 block
	alice.L1.ActResetTxOpts(t)
	alice.L1.ActSetTxToAddr(&dp.Addresses.Bob)(t)
	alice.L1.ActMakeTx(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(alice.Address())(t)
	miner.ActL1EndBlock(t)
	alice.L1.ActCheckReceiptStatusOfLastTx(true)(t)

	// regular Deposit, in new L1 block
	alice.ActDeposit(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(alice.Address())(t)
	miner.ActL1EndBlock(t)

	seq.ActL1HeadSignal(t)

	// sync sequencer build enough blocks to adopt latest L1 origin
	for seq.SyncStatus().UnsafeL2.L1Origin.Number < miner.l1Chain.CurrentBlock().Number.Uint64() {
		seq.ActL2StartBlock(t)
		seq.ActL2EndBlock(t)
	}
	// Now that the L2 chain adopted the latest L1 block, check that we processed the deposit
	alice.ActCheckDepositStatus(true, true)(t)

	// regular withdrawal, in new L2 block
	alice.ActStartWithdrawal(t)
	seq.ActL2StartBlock(t)
	seqEngine.ActL2IncludeTx(alice.Address())(t)
	seq.ActL2EndBlock(t)
	alice.ActCheckStartWithdrawal(true)(t)

	// build a L1 block and more L2 blocks,
	// to ensure the L2 withdrawal is old enough to be able to get into an output root proposal on L1
	miner.ActEmptyBlock(t)
	seq.ActL1HeadSignal(t)
	seq.ActBuildToL1Head(t)

	// submit everything to L1
	batcher.ActSubmitAll(t)
	// include batch on L1
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	// derive from L1, blocks will now become safe to propose
	seq.ActL2PipelineFull(t)

	// make proposals until there is nothing left to propose
	for proposer.CanPropose(t) {
		// propose it to L1
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

	// prove our withdrawal on L1
	alice.ActProveWithdrawal(t)
	// include proved withdrawal in new L1 block
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(alice.Address())(t)
	miner.ActL1EndBlock(t)
	// check withdrawal succeeded
	alice.L1.ActCheckReceiptStatusOfLastTx(true)(t)

	// A bit hacky- Mines an empty block with the time delta
	// of the finalization period (12s) + 1 in order for the
	// withdrawal to be finalized successfully.
	miner.ActL1StartBlock(13)(t)
	miner.ActL1EndBlock(t)

	// If using fault proofs we need to resolve the game
	if test.allocType.UsesProofs() {
		// Resolve the root claim
		alice.ActResolveClaim(t)
		miner.ActL1StartBlock(12)(t)
		miner.ActL1IncludeTx(alice.Address())(t)
		miner.ActL1EndBlock(t)
		// Resolve the game
		alice.L1.ActCheckReceiptStatusOfLastTx(true)(t)
		alice.ActResolve(t)
		miner.ActL1StartBlock(12)(t)
		miner.ActL1IncludeTx(alice.Address())(t)
		miner.ActL1EndBlock(t)
		// Create an empty block to pass the air-gap window
		alice.L1.ActCheckReceiptStatusOfLastTx(true)(t)
		miner.ActL1StartBlock(13)(t)
		miner.ActL1EndBlock(t)
	}

	// make the L1 finalize withdrawal tx
	alice.ActCompleteWithdrawal(t)
	// include completed withdrawal in new L1 block
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(alice.Address())(t)
	miner.ActL1EndBlock(t)
	// check withdrawal succeeded
	alice.L1.ActCheckReceiptStatusOfLastTx(true)(t)

	// Check Regolith wasn't activated during the test unintentionally
	infoTx, err = l2Cl.TransactionInBlock(t.Ctx(), seq.L2Unsafe().Hash, 0)
	require.NoError(t, err)
	require.True(t, infoTx.IsDepositTx())
	// Should only be a system tx if regolith is not enabled
	require.Equal(t, !seq.RollupCfg.IsRegolith(seq.L2Unsafe().Time), infoTx.IsSystemTx())
}
