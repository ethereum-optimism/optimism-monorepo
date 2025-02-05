package helpers

import (
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	e2ecfg "github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type Env struct {
	Log  log.Logger
	Logs *testlog.CapturingHandler

	DeployParams *e2eutils.DeployParams
	SetupData    *e2eutils.SetupData

	Miner       *L1Miner
	Seq         *L2Sequencer
	SeqEngine   *L2Engine
	Verifier    *L2Verifier
	VerifEngine *L2Engine
	Batcher     *L2Batcher
	Alice       *CrossLayerUser

	AddressCorpora []common.Address
}

type EnvOpt struct {
	DeployConfigMod func(*genesis.DeployConfig)
}

func WithActiveFork(fork rollup.ForkName, offset uint64) EnvOpt {
	return EnvOpt{
		DeployConfigMod: func(d *genesis.DeployConfig) {
			d.ActivateForkAtOffset(fork, offset)
		},
	}
}

func WithActiveGenesisFork(fork rollup.ForkName) EnvOpt {
	return WithActiveFork(fork, 0)
}

// DefaultFork specifies the default fork to use when setting up the action test environment.
// Currently manually set to Holocene.
// Replace with `var DefaultFork = func() rollup.ForkName { return rollup.AllForks[len(rollup.AllForks)-1] }()` after Interop launch.
const DefaultFork = rollup.Holocene

// SetupEnv sets up a default action test environment. If no fork is specified, the default fork as
// specified by the package variable [defaultFork] is used.
func SetupEnv(t Testing, opts ...EnvOpt) (env Env) {
	dp := e2eutils.MakeDeployParams(t, DefaultRollupTestParams())
	env.DeployParams = dp

	log, logs := testlog.CaptureLogger(t, log.LevelDebug)
	env.Log, env.Logs = log, logs

	dp.DeployConfig.ActivateForkAtGenesis(DefaultFork)
	for _, opt := range opts {
		if dcMod := opt.DeployConfigMod; dcMod != nil {
			dcMod(dp.DeployConfig)
		}
	}

	sd := e2eutils.Setup(t, dp, DefaultAlloc)
	env.SetupData = sd
	env.AddressCorpora = e2eutils.CollectAddresses(sd, dp)

	env.Miner, env.SeqEngine, env.Seq = SetupSequencerTest(t, sd, log)
	env.Miner.ActL1SetFeeRecipient(common.Address{'A'})
	env.VerifEngine, env.Verifier = SetupVerifier(t, sd, log, env.Miner.L1Client(t, sd.RollupCfg), env.Miner.BlobStore(), &sync.Config{})
	rollupSeqCl := env.Seq.RollupClient()
	env.Batcher = NewL2Batcher(log, sd.RollupCfg, DefaultBatcherCfg(dp),
		rollupSeqCl, env.Miner.EthClient(), env.SeqEngine.EthClient(), env.SeqEngine.EngineClient(t, sd.RollupCfg))

	alice := NewCrossLayerUser(log, dp.Secrets.Alice, rand.New(rand.NewSource(0xa57b)), e2ecfg.AllocTypeStandard)
	alice.L1.SetUserEnv(env.L1UserEnv(t))
	alice.L2.SetUserEnv(env.L2UserEnv(t))
	env.Alice = alice

	return
}

func (env Env) L1UserEnv(t Testing) *BasicUserEnv[*L1Bindings] {
	l1EthCl := env.Miner.EthClient()
	return &BasicUserEnv[*L1Bindings]{
		EthCl:          l1EthCl,
		Signer:         types.LatestSigner(env.SetupData.L1Cfg.Config),
		AddressCorpora: env.AddressCorpora,
		Bindings:       NewL1Bindings(t, l1EthCl, e2ecfg.AllocTypeStandard),
	}
}

func (env Env) L2UserEnv(t Testing) *BasicUserEnv[*L2Bindings] {
	l2EthCl := env.SeqEngine.EthClient()
	return &BasicUserEnv[*L2Bindings]{
		EthCl:          l2EthCl,
		Signer:         types.LatestSigner(env.SetupData.L2Cfg.Config),
		AddressCorpora: env.AddressCorpora,
		Bindings:       NewL2Bindings(t, l2EthCl, env.SeqEngine.GethClient()),
	}
}

func (env Env) ActBatchSubmitAllAndMine(t Testing) (l1InclusionBlock *types.Block) {
	env.Batcher.ActSubmitAll(t)
	batchTx := env.Batcher.LastSubmitted
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(batchTx.Hash())(t)
	return env.Miner.ActL1EndBlock(t)
}
