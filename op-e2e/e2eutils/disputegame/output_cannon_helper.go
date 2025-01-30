package disputegame

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"io"
	"math/big"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/cannon"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/outputs"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/split"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type OutputCannonGameHelper struct {
	OutputGameHelper
}

func (g *OutputCannonGameHelper) StartChallenger(ctx context.Context, name string, options ...challenger.Option) *challenger.Helper {
	opts := []challenger.Option{
		challenger.WithCannon(g.T, g.System),
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
	return NewOutputHonestHelper(g.T, g.Require, &g.OutputGameHelper, g.Game, accessor)
}

type PreimageLoadCheck func(types.TraceProvider, uint64) error

func (g *OutputCannonGameHelper) CreateStepLargePreimageLoadCheck(ctx context.Context, sender common.Address) PreimageLoadCheck {
	return func(provider types.TraceProvider, targetTraceIndex uint64) error {
		// Fetch the challenge period
		challengePeriod := g.ChallengePeriod(ctx)

		// Get the preimage data
		execDepth := g.ExecDepth(ctx)
		_, _, preimageData, err := provider.GetStepData(ctx, types.NewPosition(execDepth, big.NewInt(int64(targetTraceIndex))))
		g.Require.NoError(err)

		// Wait until the challenge period has started by checking until the challenge
		// period start time is not zero by calling the ChallengePeriodStartTime method
		g.WaitForChallengePeriodStart(ctx, sender, preimageData)

		challengePeriodStart := g.ChallengePeriodStartTime(ctx, sender, preimageData)
		challengePeriodEnd := challengePeriodStart + challengePeriod

		// Time travel past the challenge period.
		g.System.AdvanceTime(time.Duration(challengePeriod) * time.Second)
		g.Require.NoError(wait.ForBlockWithTimestamp(ctx, g.System.NodeClient("l1"), challengePeriodEnd))

		// Assert that the preimage was indeed loaded by an honest challenger
		g.WaitForPreimageInOracle(ctx, preimageData)
		return nil
	}
}

func (g *OutputCannonGameHelper) CreateStepPreimageLoadCheck(ctx context.Context) PreimageLoadCheck {
	return func(provider types.TraceProvider, targetTraceIndex uint64) error {
		execDepth := g.ExecDepth(ctx)
		_, _, preimageData, err := provider.GetStepData(ctx, types.NewPosition(execDepth, big.NewInt(int64(targetTraceIndex))))
		g.Require.NoError(err)
		g.WaitForPreimageInOracle(ctx, preimageData)
		return nil
	}
}

// ChallengeToPreimageLoad challenges the supplied execution root claim by inducing a step that requires a preimage to be loaded
// It does this by:
// 1. Identifying the first state transition that loads a global preimage
// 2. Descending the execution game tree to reach the step that loads the preimage
// 3. Asserting that the preimage was indeed loaded by an honest challenger (assuming the preimage is not preloaded)
// This expects an odd execution game depth in order for the honest challenger to step on our leaf claim
func (g *OutputCannonGameHelper) ChallengeToPreimageLoad(ctx context.Context, outputRootClaim *ClaimHelper, challengerKey *ecdsa.PrivateKey, preimage utils.PreimageOpt, preimageCheck PreimageLoadCheck, preloadPreimage bool) {
	// Identifying the first state transition that loads a global preimage
	provider, _ := g.createCannonTraceProvider(ctx, "sequencer", outputRootClaim, challenger.WithPrivKey(challengerKey))
	targetTraceIndex, err := provider.FindStep(ctx, 0, preimage)
	g.Require.NoError(err)

	splitDepth := g.SplitDepth(ctx)
	execDepth := g.ExecDepth(ctx)
	g.Require.NotEqual(outputRootClaim.Position.TraceIndex(execDepth).Uint64(), targetTraceIndex, "cannot move to defend a terminal trace index")
	g.Require.EqualValues(splitDepth+1, outputRootClaim.Depth(), "supplied claim must be the root of an execution game")
	g.Require.EqualValues(execDepth%2, 1, "execution game depth must be odd") // since we're challenging the execution root claim

	if preloadPreimage {
		_, _, preimageData, err := provider.GetStepData(ctx, types.NewPosition(execDepth, big.NewInt(int64(targetTraceIndex))))
		g.Require.NoError(err)
		g.UploadPreimage(ctx, preimageData)
		g.WaitForPreimageInOracle(ctx, preimageData)
	}

	// Descending the execution game tree to reach the step that loads the preimage
	bisectTraceIndex := func(claim *ClaimHelper) *ClaimHelper {
		execClaimPosition, err := claim.Position.RelativeToAncestorAtDepth(splitDepth + 1)
		g.Require.NoError(err)

		claimTraceIndex := execClaimPosition.TraceIndex(execDepth).Uint64()
		g.T.Logf("Bisecting: Into targetTraceIndex %v: claimIndex=%v at depth=%v. claimPosition=%v execClaimPosition=%v claimTraceIndex=%v",
			targetTraceIndex, claim.Index, claim.Depth(), claim.Position, execClaimPosition, claimTraceIndex)

		// We always want to position ourselves such that the challenger generates proofs for the targetTraceIndex as prestate
		if execClaimPosition.Depth() == execDepth-1 {
			if execClaimPosition.TraceIndex(execDepth).Uint64() == targetTraceIndex {
				newPosition := execClaimPosition.Attack()
				correct, err := provider.Get(ctx, newPosition)
				g.Require.NoError(err)
				g.T.Logf("Bisecting: Attack correctly for step at newPosition=%v execIndexAtDepth=%v", newPosition, newPosition.TraceIndex(execDepth))
				return claim.Attack(ctx, correct)
			} else if execClaimPosition.TraceIndex(execDepth).Uint64() > targetTraceIndex {
				g.T.Logf("Bisecting: Attack incorrectly for step")
				return claim.Attack(ctx, common.Hash{0xdd})
			} else if execClaimPosition.TraceIndex(execDepth).Uint64()+1 == targetTraceIndex {
				g.T.Logf("Bisecting: Defend incorrectly for step")
				return claim.Defend(ctx, common.Hash{0xcc})
			} else {
				newPosition := execClaimPosition.Defend()
				correct, err := provider.Get(ctx, newPosition)
				g.Require.NoError(err)
				g.T.Logf("Bisecting: Defend correctly for step at newPosition=%v execIndexAtDepth=%v", newPosition, newPosition.TraceIndex(execDepth))
				return claim.Defend(ctx, correct)
			}
		}

		// Attack or Defend depending on whether the claim we're responding to is to the left or right of the trace index
		// Induce the honest challenger to attack or defend depending on whether our new position will be to the left or right of the trace index
		if execClaimPosition.TraceIndex(execDepth).Uint64() < targetTraceIndex && claim.Depth() != splitDepth+1 {
			newPosition := execClaimPosition.Defend()
			if newPosition.TraceIndex(execDepth).Uint64() < targetTraceIndex {
				g.T.Logf("Bisecting: Defend correct. newPosition=%v execIndexAtDepth=%v", newPosition, newPosition.TraceIndex(execDepth))
				correct, err := provider.Get(ctx, newPosition)
				g.Require.NoError(err)
				return claim.Defend(ctx, correct)
			} else {
				g.T.Logf("Bisecting: Defend incorrect. newPosition=%v execIndexAtDepth=%v", newPosition, newPosition.TraceIndex(execDepth))
				return claim.Defend(ctx, common.Hash{0xaa})
			}
		} else {
			newPosition := execClaimPosition.Attack()
			if newPosition.TraceIndex(execDepth).Uint64() < targetTraceIndex {
				g.T.Logf("Bisecting: Attack correct. newPosition=%v execIndexAtDepth=%v", newPosition, newPosition.TraceIndex(execDepth))
				correct, err := provider.Get(ctx, newPosition)
				g.Require.NoError(err)
				return claim.Attack(ctx, correct)
			} else {
				g.T.Logf("Bisecting: Attack incorrect. newPosition=%v execIndexAtDepth=%v", newPosition, newPosition.TraceIndex(execDepth))
				return claim.Attack(ctx, common.Hash{0xbb})
			}
		}
	}

	g.LogGameData(ctx)
	// Initial bisect to put us on defense
	mover := bisectTraceIndex(outputRootClaim)
	leafClaim := g.DefendClaim(ctx, mover, bisectTraceIndex, WithoutWaitingForStep())

	// Validate that the preimage was loaded correctly
	g.Require.NoError(preimageCheck(provider, targetTraceIndex))

	// Now the preimage is available wait for the step call to succeed.
	leafClaim.WaitForCountered(ctx)
	g.LogGameData(ctx)
}

func (g *OutputCannonGameHelper) VerifyPreimage(ctx context.Context, outputRootClaim *ClaimHelper, preimageKey preimage.Key) {
	execDepth := g.ExecDepth(ctx)

	// Identifying the first state transition that loads a global preimage
	provider, localContext := g.createCannonTraceProvider(ctx, "sequencer", outputRootClaim, challenger.WithPrivKey(TestKey))
	start := uint64(0)
	found := false
	for offset := uint32(0); ; offset += 4 {
		preimageOpt := utils.PreimageLoad(preimageKey, offset)
		g.T.Logf("Searching for step with key %x and offset %v", preimageKey.PreimageKey(), offset)
		targetTraceIndex, err := provider.FindStep(ctx, start, preimageOpt)
		if errors.Is(err, io.EOF) {
			// Did not find any more reads
			g.Require.True(found, "Should have found at least one preimage read")
			g.T.Logf("Searching for step with key %x and offset %v did not find another read", preimageKey.PreimageKey(), offset)
			return
		}
		g.Require.NoError(err, "Failed to find step that loads requested preimage")
		start = targetTraceIndex
		found = true

		g.T.Logf("Target trace index: %v", targetTraceIndex)
		pos := types.NewPosition(execDepth, new(big.Int).SetUint64(targetTraceIndex))
		g.Require.Equal(targetTraceIndex, pos.TraceIndex(execDepth).Uint64())

		prestate, proof, oracleData, err := provider.GetStepData(ctx, pos)
		g.Require.NoError(err, "Failed to get step data")
		g.Require.NotNil(oracleData, "Should have had required preimage oracle data")
		g.Require.Equal(common.Hash(preimageKey.PreimageKey()).Bytes(), oracleData.OracleKey, "Must have correct preimage key")

		candidate, err := g.Game.UpdateOracleTx(ctx, uint64(outputRootClaim.Index), oracleData)
		g.Require.NoError(err, "failed to get oracle")
		transactions.RequireSendTx(g.T, ctx, g.Client, candidate, g.PrivKey)

		expectedPostState, err := provider.Get(ctx, pos)
		g.Require.NoError(err, "Failed to get expected post state")

		vm, err := g.Game.Vm(ctx)
		g.Require.NoError(err, "Failed to get VM address")

		abi, err := bindings.MIPSMetaData.GetAbi()
		g.Require.NoError(err, "Failed to load MIPS ABI")
		caller := batching.NewMultiCaller(g.Client.Client(), batching.DefaultBatchSize)
		result, err := caller.SingleCall(ctx, rpcblock.Latest, &batching.ContractCall{
			Abi:    abi,
			Addr:   vm.Addr(),
			Method: "step",
			Args: []interface{}{
				prestate, proof, localContext,
			},
			From: g.Addr,
		})
		g.Require.NoError(err, "Failed to call step")
		actualPostState := result.GetBytes32(0)
		g.Require.Equal(expectedPostState, common.Hash(actualPostState))
	}
}

func (g *OutputCannonGameHelper) createCannonTraceProvider(ctx context.Context, l2Node string, outputRootClaim *ClaimHelper, options ...challenger.Option) (*cannon.CannonTraceProviderForTest, common.Hash) {
	splitDepth := g.SplitDepth(ctx)
	g.Require.EqualValues(outputRootClaim.Depth(), splitDepth+1, "outputRootClaim must be the root of an execution game")

	logger := testlog.Logger(g.T, log.LevelInfo).New("role", "CannonTraceProvider", "game", g.Addr)
	opt := g.defaultChallengerOptions()
	opt = append(opt, options...)
	cfg := challenger.NewChallengerConfig(g.T, g.System, l2Node, opt...)

	l2Client := g.System.NodeClient(l2Node)

	prestateBlock, poststateBlock, err := g.Game.GetBlockRange(ctx)
	g.Require.NoError(err, "Failed to load block range")
	rollupClient := g.System.RollupClient(l2Node)
	prestateProvider := outputs.NewPrestateProvider(rollupClient, prestateBlock)
	l1Head := g.GetL1Head(ctx)
	outputProvider := outputs.NewTraceProvider(logger, prestateProvider, rollupClient, l2Client, l1Head, splitDepth, prestateBlock, poststateBlock)

	var localContext common.Hash
	selector := split.NewSplitProviderSelector(outputProvider, splitDepth, func(ctx context.Context, depth types.Depth, pre types.Claim, post types.Claim) (types.TraceProvider, error) {
		agreed, disputed, err := outputs.FetchProposals(ctx, outputProvider, pre, post)
		g.Require.NoError(err)
		g.T.Logf("Using trace between blocks %v and %v\n", agreed.L2BlockNumber, disputed.L2BlockNumber)
		localInputs, err := utils.FetchLocalInputsFromProposals(ctx, l1Head.Hash, l2Client, agreed, disputed)
		g.Require.NoError(err, "Failed to fetch local inputs")
		localContext = split.CreateLocalContext(pre, post)
		dir := filepath.Join(cfg.Datadir, "cannon-trace")
		subdir := filepath.Join(dir, localContext.Hex())
		return cannon.NewTraceProviderForTest(logger, metrics.NoopMetrics.ToTypedVmMetrics(types.TraceTypeCannon.String()), cfg, localInputs, subdir, g.MaxDepth(ctx)-splitDepth-1), nil
	})

	claims, err := g.Game.GetAllClaims(ctx, rpcblock.Latest)
	g.Require.NoError(err)
	game := types.NewGameState(claims, g.MaxDepth(ctx))

	provider, err := selector(ctx, game, game.Claims()[outputRootClaim.ParentIndex], outputRootClaim.Position)
	g.Require.NoError(err)
	translatingProvider := provider.(*trace.TranslatingProvider)
	return translatingProvider.Original().(*cannon.CannonTraceProviderForTest), localContext
}

func (g *OutputCannonGameHelper) defaultChallengerOptions() []challenger.Option {
	return []challenger.Option{
		challenger.WithCannon(g.T, g.System),
		challenger.WithFactoryAddress(g.FactoryAddr),
		challenger.WithGameAddress(g.Addr),
	}
}
