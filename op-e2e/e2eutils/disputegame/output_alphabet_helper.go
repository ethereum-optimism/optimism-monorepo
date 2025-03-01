package disputegame

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/outputs"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/log"
)

type OutputAlphabetGameHelper struct {
	OutputGameHelper
}

func (g *OutputAlphabetGameHelper) StartChallenger(
	ctx context.Context,
	l2Node string,
	name string,
	options ...challenger.Option,
) *challenger.Helper {
	opts := []challenger.Option{
		challenger.WithAlphabet(),
		challenger.WithFactoryAddress(g.FactoryAddr),
		challenger.WithGameAddress(g.Addr),
	}
	opts = append(opts, options...)
	c := challenger.NewChallenger(g.T, ctx, g.System, name, opts...)
	g.T.Cleanup(func() {
		_ = c.Close()
	})
	return c
}

func (g *OutputAlphabetGameHelper) CreateHonestActor(ctx context.Context, l2Node string) *OutputHonestHelper {
	logger := testlog.Logger(g.T, log.LevelInfo).New("role", "HonestHelper", "game", g.Addr)
	prestateBlock, poststateBlock, err := g.Game.GetBlockRange(ctx)
	g.Require.NoError(err, "Get block range")
	splitDepth := g.SplitDepth(ctx)
	l1Head := g.GetL1Head(ctx)
	rollupClient := g.System.RollupClient(l2Node)
	l2Client := g.System.NodeClient(l2Node)
	prestateProvider := outputs.NewPrestateProvider(rollupClient, prestateBlock)
	correctTrace, err := outputs.NewOutputAlphabetTraceAccessor(logger, metrics.NoopMetrics, prestateProvider, rollupClient, l2Client, l1Head, splitDepth, prestateBlock, poststateBlock)
	g.Require.NoError(err, "Create trace accessor")
	return NewOutputHonestHelper(g.T, g.Require, &g.OutputGameHelper.SplitGameHelper, g.Game, correctTrace)
}

func (g *OutputAlphabetGameHelper) CreateDishonestHelper(ctx context.Context, l2Node string, defender bool) *DishonestHelper {
	return newDishonestHelper(&g.OutputGameHelper, g.CreateHonestActor(ctx, l2Node), defender)
}
