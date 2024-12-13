package helpers

import (
	"context"
	"math/rand"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	e2ecfg "github.com/ethereum-optimism/optimism/op-e2e/config"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-program/host"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-program/host/prefetcher"
	hostTypes "github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

// L2FaultProofEnv is a test harness for a fault provable L2 chain.
type L2FaultProofEnv struct {
	log       log.Logger
	Logs      *testlog.CapturingHandler
	Batcher   *helpers.L2Batcher
	Sequencer *helpers.L2Sequencer
	Engine    *helpers.L2Engine
	engCl     *sources.EngineClient
	Sd        *e2eutils.SetupData
	Dp        *e2eutils.DeployParams
	Miner     *helpers.L1Miner
	Alice     *helpers.CrossLayerUser
	Bob       *helpers.CrossLayerUser
}

type deployConfigOverride func(*genesis.DeployConfig)

func NewL2FaultProofEnv[c any](t helpers.Testing, testCfg *TestCfg[c], tp *e2eutils.TestParams, batcherCfg *helpers.BatcherCfg, deployConfigOverrides ...deployConfigOverride) *L2FaultProofEnv {
	log, logs := testlog.CaptureLogger(t, log.LevelDebug)

	dp := NewDeployParams(t, tp, func(dp *e2eutils.DeployParams) {
		genesisBlock := hexutil.Uint64(0)

		// Enable cancun always
		dp.DeployConfig.L1CancunTimeOffset = &genesisBlock

		// Enable L2 feature.
		switch testCfg.Hardfork {
		case Regolith:
			dp.DeployConfig.L2GenesisRegolithTimeOffset = &genesisBlock
		case Canyon:
			dp.DeployConfig.L2GenesisCanyonTimeOffset = &genesisBlock
		case Delta:
			dp.DeployConfig.L2GenesisDeltaTimeOffset = &genesisBlock
		case Ecotone:
			dp.DeployConfig.L2GenesisEcotoneTimeOffset = &genesisBlock
		case Fjord:
			dp.DeployConfig.L2GenesisFjordTimeOffset = &genesisBlock
		case Granite:
			dp.DeployConfig.L2GenesisGraniteTimeOffset = &genesisBlock
		case Holocene:
			dp.DeployConfig.L2GenesisHoloceneTimeOffset = &genesisBlock
		}

		for _, override := range deployConfigOverrides {
			override(dp.DeployConfig)
		}
	})
	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)

	jwtPath := e2eutils.WriteDefaultJWT(t)
	cfg := &helpers.SequencerCfg{VerifierCfg: *helpers.DefaultVerifierCfg()}

	miner := helpers.NewL1Miner(t, log.New("role", "l1-miner"), sd.L1Cfg)

	l1Cl, err := sources.NewL1Client(miner.RPCClient(), log, nil, sources.L1ClientDefaultConfig(sd.RollupCfg, false, sources.RPCKindStandard))
	require.NoError(t, err)
	engine := helpers.NewL2Engine(t, log.New("role", "sequencer-engine"), sd.L2Cfg, jwtPath, helpers.EngineWithP2P())
	l2EngineCl, err := sources.NewEngineClient(engine.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	sequencer := helpers.NewL2Sequencer(t, log.New("role", "sequencer"), l1Cl, miner.BlobStore(), altda.Disabled, l2EngineCl, sd.RollupCfg, 0, cfg.InteropBackend)
	miner.ActL1SetFeeRecipient(common.Address{0xCA, 0xFE, 0xBA, 0xBE})
	sequencer.ActL2PipelineFull(t)
	engCl := engine.EngineClient(t, sd.RollupCfg)

	// Set the batcher key to the secret key of the batcher
	batcherCfg.BatcherKey = dp.Secrets.Batcher
	batcher := helpers.NewL2Batcher(log, sd.RollupCfg, batcherCfg, sequencer.RollupClient(), miner.EthClient(), engine.EthClient(), engCl)

	addresses := e2eutils.CollectAddresses(sd, dp)
	l1EthCl := miner.EthClient()
	l2EthCl := engine.EthClient()
	l1UserEnv := &helpers.BasicUserEnv[*helpers.L1Bindings]{
		EthCl:          l1EthCl,
		Signer:         types.LatestSigner(sd.L1Cfg.Config),
		AddressCorpora: addresses,
		Bindings:       helpers.NewL1Bindings(t, l1EthCl, e2ecfg.AllocTypeStandard),
	}
	l2UserEnv := &helpers.BasicUserEnv[*helpers.L2Bindings]{
		EthCl:          l2EthCl,
		Signer:         types.LatestSigner(sd.L2Cfg.Config),
		AddressCorpora: addresses,
		Bindings:       helpers.NewL2Bindings(t, l2EthCl, engine.GethClient()),
	}
	alice := helpers.NewCrossLayerUser(log, dp.Secrets.Alice, rand.New(rand.NewSource(0xa57b)), e2ecfg.AllocTypeStandard)
	alice.L1.SetUserEnv(l1UserEnv)
	alice.L2.SetUserEnv(l2UserEnv)
	bob := helpers.NewCrossLayerUser(log, dp.Secrets.Bob, rand.New(rand.NewSource(0xbeef)), e2ecfg.AllocTypeStandard)
	bob.L1.SetUserEnv(l1UserEnv)
	bob.L2.SetUserEnv(l2UserEnv)

	return &L2FaultProofEnv{
		log:       log,
		Logs:      logs,
		Batcher:   batcher,
		Sequencer: sequencer,
		Engine:    engine,
		engCl:     engCl,
		Sd:        sd,
		Dp:        dp,
		Miner:     miner,
		Alice:     alice,
		Bob:       bob,
	}
}

type FixtureInputParam func(f *FixtureInputs)

type CheckResult func(helpers.Testing, error)

func ExpectNoError() CheckResult {
	return func(t helpers.Testing, err error) {
		require.NoError(t, err, "fault proof program should have succeeded")
	}
}

func ExpectError(expectedErr error) CheckResult {
	return func(t helpers.Testing, err error) {
		require.ErrorIs(t, err, expectedErr, "fault proof program should have failed with expected error")
	}
}

func WithL2Claim(claim common.Hash) FixtureInputParam {
	return func(f *FixtureInputs) {
		f.L2Claim = claim
	}
}

func WithL2BlockNumber(num uint64) FixtureInputParam {
	return func(f *FixtureInputs) {
		f.L2BlockNumber = num
	}
}

func (env *L2FaultProofEnv) RunFaultProofProgram(t helpers.Testing, l2ClaimBlockNum uint64, checkResult CheckResult, fixtureInputParams ...FixtureInputParam) {
	// Fetch the pre and post output roots for the fault proof.
	l2PreBlockNum := l2ClaimBlockNum - 1
	if l2ClaimBlockNum == 0 {
		// If we are at genesis, we assert that we don't move the chain at all.
		l2PreBlockNum = 0
	}
	preRoot, err := env.Sequencer.RollupClient().OutputAtBlock(t.Ctx(), l2PreBlockNum)
	require.NoError(t, err)
	claimRoot, err := env.Sequencer.RollupClient().OutputAtBlock(t.Ctx(), l2ClaimBlockNum)
	require.NoError(t, err)
	l1Head := env.Miner.L1Chain().CurrentBlock()

	fixtureInputs := &FixtureInputs{
		L2BlockNumber: l2ClaimBlockNum,
		L2Claim:       common.Hash(claimRoot.OutputRoot),
		L2Head:        preRoot.BlockRef.Hash,
		L2OutputRoot:  common.Hash(preRoot.OutputRoot),
		L2ChainID:     env.Sd.RollupCfg.L2ChainID.Uint64(),
		L1Head:        l1Head.Hash(),
	}
	for _, apply := range fixtureInputParams {
		apply(fixtureInputs)
	}

	// Run the fault proof program from the state transition from L2 block l2ClaimBlockNum - 1 -> l2ClaimBlockNum.
	workDir := t.TempDir()
	if IsKonaConfigured() {
		fakeBeacon := fakebeacon.NewBeacon(
			env.log,
			env.Miner.BlobStore(),
			env.Sd.L1Cfg.Timestamp,
			12,
		)
		require.NoError(t, fakeBeacon.Start("127.0.0.1:0"))
		defer fakeBeacon.Close()

		err = RunKonaNative(t, workDir, env, env.Miner.HTTPEndpoint(), fakeBeacon.BeaconAddr(), env.Engine.HTTPEndpoint(), *fixtureInputs)
		checkResult(t, err)
	} else {
		programCfg := NewOpProgramCfg(
			t,
			env,
			fixtureInputs,
		)
		withInProcessPrefetcher := host.WithPrefetcher(func(ctx context.Context, logger log.Logger, kv kvstore.KV, cfg *config.Config) (host.Prefetcher, error) {
			// Set up in-process L1 sources
			l1Cl := env.Miner.L1Client(t, env.Sd.RollupCfg)
			l1BlobFetcher := env.Miner.BlobSource()

			// Set up in-process L2 source
			l2ClCfg := sources.L2ClientDefaultConfig(env.Sd.RollupCfg, true)
			l2RPC := env.Engine.RPCClient()
			l2Client, err := host.NewL2Client(l2RPC, env.log, nil, &host.L2ClientConfig{L2ClientConfig: l2ClCfg, L2Head: cfg.L2Head})
			require.NoError(t, err, "failed to create L2 client")
			l2DebugCl := host.NewL2SourceWithClient(logger, l2Client, sources.NewDebugClient(l2RPC.CallContext))

			return prefetcher.NewPrefetcher(logger, l1Cl, l1BlobFetcher, l2DebugCl, kv), nil
		})
		err = host.FaultProofProgram(t.Ctx(), env.log, programCfg, withInProcessPrefetcher)
		checkResult(t, err)
	}
	tryDumpTestFixture(t, err, t.Name(), env, *fixtureInputs, workDir)
}

type TestParam func(p *e2eutils.TestParams)

func NewTestParams(params ...TestParam) *e2eutils.TestParams {
	dfault := helpers.DefaultRollupTestParams()
	for _, apply := range params {
		apply(dfault)
	}
	return dfault
}

type DeployParam func(p *e2eutils.DeployParams)

func NewDeployParams(t helpers.Testing, tp *e2eutils.TestParams, params ...DeployParam) *e2eutils.DeployParams {
	dfault := e2eutils.MakeDeployParams(t, tp)
	for _, apply := range params {
		apply(dfault)
	}
	return dfault
}

type BatcherCfgParam func(c *helpers.BatcherCfg)

func NewBatcherCfg(params ...BatcherCfgParam) *helpers.BatcherCfg {
	dfault := &helpers.BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		DataAvailabilityType: batcherFlags.BlobsType,
	}
	for _, apply := range params {
		apply(dfault)
	}
	return dfault
}

type OpProgramCfgParam func(p *config.Config)

func NewOpProgramCfg(
	t helpers.Testing,
	env *L2FaultProofEnv,
	fi *FixtureInputs,
	params ...OpProgramCfgParam,
) *config.Config {
	dfault := config.NewConfig(env.Sd.RollupCfg, env.Sd.L2Cfg.Config, fi.L1Head, fi.L2Head, fi.L2OutputRoot, fi.L2Claim, fi.L2BlockNumber)

	if dumpFixtures {
		dfault.DataDir = t.TempDir()
		dfault.DataFormat = hostTypes.DataFormatPebble
	}

	for _, apply := range params {
		apply(dfault)
	}
	return dfault
}
