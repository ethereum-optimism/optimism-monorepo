package helpers

import (
	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/upgrades/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func SetupSequencerTest(t Testing, sd *e2eutils.SetupData, log log.Logger, opts ...SequencerOpt) (*L1Miner, *L2Engine, *L2Sequencer) {
	jwtPath := e2eutils.WriteDefaultJWT(t)
	cfg := DefaultSequencerConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	miner := NewL1Miner(t, log.New("role", "l1-miner"), sd.L1Cfg)

	l1F, err := sources.NewL1Client(miner.RPCClient(), log, nil, sources.L1ClientDefaultConfig(sd.RollupCfg, false, sources.RPCKindStandard))
	require.NoError(t, err)
	engine := NewL2Engine(t, log.New("role", "sequencer-engine"), sd.L2Cfg, jwtPath, EngineWithP2P())
	l2Cl, err := sources.NewEngineClient(engine.RPCClient(), log, nil, sources.EngineClientDefaultConfig(sd.RollupCfg))
	require.NoError(t, err)

	sequencer := NewL2Sequencer(t, log.New("role", "sequencer"), l1F, miner.BlobStore(), altda.Disabled, l2Cl, sd.RollupCfg, 0, cfg.InteropBackend)
	return miner, engine, sequencer
}

func SetupVerifier(t Testing, sd *e2eutils.SetupData, log log.Logger,
	l1F derive.L1Fetcher, blobSrc derive.L1BlobsFetcher, syncCfg *sync.Config, opts ...VerifierOpt) (*L2Engine, *L2Verifier) {
	cfg := DefaultVerifierCfg()
	for _, opt := range opts {
		opt(cfg)
	}
	jwtPath := e2eutils.WriteDefaultJWT(t)
	engine := NewL2Engine(t, log.New("role", "verifier-engine"), sd.L2Cfg, jwtPath, EngineWithP2P())
	engCl := engine.EngineClient(t, sd.RollupCfg)
	verifier := NewL2Verifier(t, log.New("role", "verifier"), l1F, blobSrc, altda.Disabled, engCl, sd.RollupCfg, syncCfg, cfg.SafeHeadListener, cfg.InteropBackend)
	return engine, verifier
}

func SetupVerifierOnlyTest(t Testing, sd *e2eutils.SetupData, log log.Logger) (*L1Miner, *L2Engine, *L2Verifier) {
	miner := NewL1Miner(t, log, sd.L1Cfg)
	l1Cl := miner.L1Client(t, sd.RollupCfg)
	engine, verifier := SetupVerifier(t, sd, log, l1Cl, miner.BlobStore(), &sync.Config{})
	return miner, engine, verifier
}

func SetupReorgTest(t Testing, config *e2eutils.TestParams, deltaTimeOffset *hexutil.Uint64) (*e2eutils.SetupData, *e2eutils.DeployParams, *L1Miner, *L2Sequencer, *L2Engine, *L2Verifier, *L2Engine, *L2Batcher) {
	dp := e2eutils.MakeDeployParams(t, config)
	helpers.ApplyDeltaTimeOffset(dp, deltaTimeOffset)

	sd := e2eutils.Setup(t, dp, DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)

	return SetupReorgTestActors(t, dp, sd, log)
}

func SetupReorgTestActors(t Testing, dp *e2eutils.DeployParams, sd *e2eutils.SetupData, log log.Logger) (*e2eutils.SetupData, *e2eutils.DeployParams, *L1Miner, *L2Sequencer, *L2Engine, *L2Verifier, *L2Engine, *L2Batcher) {
	miner, seqEngine, sequencer := SetupSequencerTest(t, sd, log)
	miner.ActL1SetFeeRecipient(common.Address{'A'})
	sequencer.ActL2PipelineFull(t)
	verifEngine, verifier := SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})
	rollupSeqCl := sequencer.RollupClient()
	batcher := NewL2Batcher(log, sd.RollupCfg, DefaultBatcherCfg(dp),
		rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))
	return sd, dp, miner, sequencer, seqEngine, verifier, verifEngine, batcher
}
