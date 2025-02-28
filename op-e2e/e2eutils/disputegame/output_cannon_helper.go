package disputegame

import (
	"context"
	"crypto/ecdsa"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/outputs"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type OutputCannonGameHelper struct {
	OutputGameHelper
	CannonHelper
}

func NewOutputCannonGameHelper(t *testing.T, client *ethclient.Client, opts *bind.TransactOpts, key *ecdsa.PrivateKey, game contracts.FaultDisputeGameContract, factoryAddr common.Address, gameAddr common.Address, provider *outputs.OutputTraceProvider, system DisputeSystem) *OutputCannonGameHelper {
	outputGameHelper := NewOutputGameHelper(t, require.New(t), client, opts, key, game, factoryAddr, gameAddr, provider, system)
	defaultChallengerOptions := func() []challenger.Option {
		return []challenger.Option{
			challenger.WithCannon(t, system),
			challenger.WithFactoryAddress(factoryAddr),
			challenger.WithGameAddress(gameAddr),
		}
	}
	return &OutputCannonGameHelper{
		OutputGameHelper: *outputGameHelper,
		CannonHelper:     *NewCannonHelper(&outputGameHelper.SplitGameHelper, defaultChallengerOptions),
	}
}

type HonestActorConfig struct {
	PrestateBlock  uint64
	PoststateBlock uint64
	ChallengerOpts []challenger.Option
}

type HonestActorOpt func(cfg *HonestActorConfig)

func WithClaimedL2BlockNumber(num uint64) HonestActorOpt {
	return func(cfg *HonestActorConfig) {
		cfg.PoststateBlock = num
	}
}

func WithPrivKey(privKey *ecdsa.PrivateKey) HonestActorOpt {
	return func(cfg *HonestActorConfig) {
		cfg.ChallengerOpts = append(cfg.ChallengerOpts, challenger.WithPrivKey(privKey))
	}
}

func (g *OutputCannonGameHelper) CreateHonestActor(ctx context.Context, l2Node string, options ...HonestActorOpt) *OutputHonestHelper {
	logger := testlog.Logger(g.T, log.LevelInfo).New("role", "HonestHelper", "game", g.Addr)
	l2Client := g.System.NodeClient(l2Node)

	realPrestateBlock, realPostStateBlock, err := g.Game.GetBlockRange(ctx)
	g.Require.NoError(err, "Failed to load block range")
	splitDepth := g.SplitDepth(ctx)
	rollupClient := g.System.RollupClient(l2Node)
	actorCfg := &HonestActorConfig{
		PrestateBlock:  realPrestateBlock,
		PoststateBlock: realPostStateBlock,
		ChallengerOpts: g.defaultChallengerOptions(),
	}
	for _, option := range options {
		option(actorCfg)
	}

	cfg := challenger.NewChallengerConfig(g.T, g.System, l2Node, actorCfg.ChallengerOpts...)
	dir := filepath.Join(cfg.Datadir, "honest")
	prestateProvider := outputs.NewPrestateProvider(rollupClient, actorCfg.PrestateBlock)
	l1Head := g.GetL1Head(ctx)
	accessor, err := outputs.NewOutputCannonTraceAccessor(
		logger, metrics.NoopMetrics, cfg.Cannon, vm.NewOpProgramServerExecutor(logger), l2Client, prestateProvider, cfg.CannonAbsolutePreState, rollupClient, dir, l1Head, splitDepth, actorCfg.PrestateBlock, actorCfg.PoststateBlock)
	g.Require.NoError(err, "Failed to create output cannon trace accessor")
	return NewOutputHonestHelper(g.T, g.Require, &g.OutputGameHelper.SplitGameHelper, g.Game, accessor)
}
