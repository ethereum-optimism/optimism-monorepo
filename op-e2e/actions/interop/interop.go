package interop

import (
	"context"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/interop"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/frontend"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

const (
	foundryArtifactsDir = "../../../packages/contracts-bedrock/forge-artifacts"
	sourceMapDir        = "../../../packages/contracts-bedrock"
)

// Chain holds the most common per-chain action-test data and actors
type Chain struct {
	ChainID types.ChainID

	RollupCfg   *rollup.Config
	ChainCfg    *params.ChainConfig
	BatcherAddr common.Address

	Sequencer       *helpers.L2Sequencer
	SequencerEngine *helpers.L2Engine
	Batcher         *helpers.L2Batcher
}

// InteropSetup holds the chain deployment and config contents, before instantiating any services.
type InteropSetup struct {
	Log        log.Logger
	Deployment *interopgen.WorldDeployment
	Out        *interopgen.WorldOutput
	DepSet     *depset.StaticConfigDependencySet
	Keys       devkeys.Keys
	T          helpers.Testing
}

// InteropActors holds a bundle of global actors and actors of 2 chains.
type InteropActors struct {
	L1Miner    *helpers.L1Miner
	Supervisor *SupervisorActor
	ChainA     *Chain
	ChainB     *Chain
}

// SetupInterop creates an InteropSetup to instantiate actors on, with 2 L2 chains.
func SetupInterop(t helpers.Testing) *InteropSetup {
	logger := testlog.Logger(t, log.LevelDebug)

	recipe := interopgen.InteropDevRecipe{
		L1ChainID:        900100,
		L2ChainIDs:       []uint64{900200, 900201},
		GenesisTimestamp: uint64(time.Now().Unix() + 3),
	}
	hdWallet, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(t, err)
	worldCfg, err := recipe.Build(hdWallet)
	require.NoError(t, err)

	// create the foundry artifacts and source map
	foundryArtifacts := foundry.OpenArtifactsDir(foundryArtifactsDir)
	sourceMap := foundry.NewSourceMapFS(os.DirFS(sourceMapDir))

	// deploy the world, using the logger, foundry artifacts, source map, and world configuration
	worldDeployment, worldOutput, err := interopgen.Deploy(logger, foundryArtifacts, sourceMap, worldCfg)
	require.NoError(t, err)

	return &InteropSetup{
		Log:        logger,
		Deployment: worldDeployment,
		Out:        worldOutput,
		DepSet:     worldToDepSet(t, worldOutput),
		Keys:       hdWallet,
		T:          t,
	}
}

func (is *InteropSetup) CreateActors() *InteropActors {
	l1Miner := helpers.NewL1Miner(is.T, is.Log.New("role", "l1Miner"), is.Out.L1.Genesis)
	supervisorAPI := NewSupervisor(is.T, is.Log, is.DepSet)
	require.NoError(is.T, supervisorAPI.Start(is.T.Ctx()))
	is.T.Cleanup(func() {
		require.NoError(is.T, supervisorAPI.Stop(context.Background()))
	})
	chainA := createL2Services(is.T, is.Log, l1Miner, is.Keys, is.Out.L2s["900200"], supervisorAPI)
	chainB := createL2Services(is.T, is.Log, l1Miner, is.Keys, is.Out.L2s["900201"], supervisorAPI)
	// Hook up L2 RPCs to supervisor, to fetch event data from
	require.NoError(is.T, supervisorAPI.AddL2RPC(is.T.Ctx(), chainA.SequencerEngine.HTTPEndpoint()))
	require.NoError(is.T, supervisorAPI.AddL2RPC(is.T.Ctx(), chainB.SequencerEngine.HTTPEndpoint()))
	return &InteropActors{
		L1Miner:    l1Miner,
		Supervisor: supervisorAPI,
		ChainA:     chainA,
		ChainB:     chainB,
	}
}

// SupervisorActor represents a supervisor, instrumented to run synchronously for action-test purposes.
type SupervisorActor struct {
	backend *backend.SupervisorBackend
	frontend.QueryFrontend
	frontend.AdminFrontend
	frontend.UpdatesFrontend
}

func (sa *SupervisorActor) SyncEvents(t helpers.Testing, chainID types.ChainID) {
	require.NoError(t, sa.backend.SyncEvents(chainID))
}

func (sa *SupervisorActor) SyncCrossUnsafe(t helpers.Testing, chainID types.ChainID) {
	require.NoError(t, sa.backend.SyncCrossUnsafe(chainID))
}

func (sa *SupervisorActor) SyncCrossSafe(t helpers.Testing, chainID types.ChainID) {
	require.NoError(t, sa.backend.SyncCrossSafe(chainID))
}

// worldToDepSet converts a set of chain configs into a dependency-set for the supervisor.
func worldToDepSet(t helpers.Testing, worldOutput *interopgen.WorldOutput) *depset.StaticConfigDependencySet {
	depSetCfg := make(map[types.ChainID]*depset.StaticConfigDependency)
	for _, out := range worldOutput.L2s {
		depSetCfg[types.ChainIDFromBig(out.Genesis.Config.ChainID)] = &depset.StaticConfigDependency{
			ChainIndex:     types.ChainIndex(out.Genesis.Config.ChainID.Uint64()),
			ActivationTime: 0,
			HistoryMinTime: 0,
		}
	}
	depSet, err := depset.NewStaticConfigDependencySet(depSetCfg)
	require.NoError(t, err)
	return depSet
}

// NewSupervisor creates a new SupervisorActor, to action-test the supervisor with.
func NewSupervisor(t helpers.Testing, logger log.Logger, depSet depset.DependencySetSource) *SupervisorActor {
	logger = logger.New("role", "supervisor")
	supervisorDataDir := t.TempDir()
	logger.Info("supervisor data dir", "dir", supervisorDataDir)
	svCfg := &config.Config{
		DependencySetSource:   depSet,
		SynchronousProcessors: true,
		Datadir:               supervisorDataDir,
	}
	b, err := backend.NewSupervisorBackend(t.Ctx(),
		logger.New("role", "supervisor"), metrics.NoopMetrics, svCfg)
	require.NoError(t, err)
	return &SupervisorActor{
		backend: b,
		QueryFrontend: frontend.QueryFrontend{
			Supervisor: b,
		},
		AdminFrontend: frontend.AdminFrontend{
			Supervisor: b,
		},
		UpdatesFrontend: frontend.UpdatesFrontend{
			Supervisor: b,
		},
	}
}

// createL2Services creates a Chain bundle, with the given configs, and attached to the given L1 miner.
func createL2Services(
	t helpers.Testing,
	logger log.Logger,
	l1Miner *helpers.L1Miner,
	keys devkeys.Keys,
	output *interopgen.L2Output,
	interopBackend interop.InteropBackend,
) *Chain {
	logger = logger.New("chain", output.Genesis.Config.ChainID)

	jwtPath := e2eutils.WriteDefaultJWT(t)

	eng := helpers.NewL2Engine(t, logger.New("role", "engine"), output.Genesis, jwtPath)

	seqCl, err := sources.NewEngineClient(eng.RPCClient(), logger, nil, sources.EngineClientDefaultConfig(output.RollupCfg))
	require.NoError(t, err)

	l1F, err := sources.NewL1Client(l1Miner.RPCClient(), logger, nil,
		sources.L1ClientDefaultConfig(output.RollupCfg, false, sources.RPCKindStandard))
	require.NoError(t, err)

	seq := helpers.NewL2Sequencer(t, logger.New("role", "sequencer"), l1F,
		l1Miner.BlobStore(), altda.Disabled, seqCl, output.RollupCfg,
		0, interopBackend)

	batcherKey, err := keys.Secret(devkeys.ChainOperatorKey{
		ChainID: output.Genesis.Config.ChainID,
		Role:    devkeys.BatcherRole,
	})
	require.NoError(t, err)

	batcherCfg := &helpers.BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           batcherKey,
		DataAvailabilityType: batcherFlags.CalldataType,
	}

	batcher := helpers.NewL2Batcher(logger.New("role", "batcher"), output.RollupCfg, batcherCfg,
		seq.RollupClient(), l1Miner.EthClient(),
		eng.EthClient(), eng.EngineClient(t, output.RollupCfg))

	return &Chain{
		ChainID:         types.ChainIDFromBig(output.Genesis.Config.ChainID),
		RollupCfg:       output.RollupCfg,
		ChainCfg:        output.Genesis.Config,
		BatcherAddr:     crypto.PubkeyToAddress(batcherKey.PublicKey),
		Sequencer:       seq,
		SequencerEngine: eng,
		Batcher:         batcher,
	}
}
