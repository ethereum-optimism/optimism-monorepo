package disputegame

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/preimages"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/outputs"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/errutil"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

const defaultTimeout = 5 * time.Minute

type OutputGameHelper struct {
	T                     *testing.T
	Require               *require.Assertions
	Client                *ethclient.Client
	Opts                  *bind.TransactOpts
	PrivKey               *ecdsa.PrivateKey
	Game                  contracts.FaultDisputeGameContract
	FactoryAddr           common.Address
	Addr                  common.Address
	CorrectOutputProvider *outputs.OutputTraceProvider
	System                DisputeSystem
}

func NewOutputGameHelper(t *testing.T, require *require.Assertions, client *ethclient.Client, opts *bind.TransactOpts, privKey *ecdsa.PrivateKey,
	game contracts.FaultDisputeGameContract, factoryAddr common.Address, addr common.Address, correctOutputProvider *outputs.OutputTraceProvider, system DisputeSystem, allocType config.AllocType) *OutputGameHelper {
	return &OutputGameHelper{
		T:                     t,
		Require:               require,
		Client:                client,
		Opts:                  opts,
		PrivKey:               privKey,
		Game:                  game,
		FactoryAddr:           factoryAddr,
		Addr:                  addr,
		CorrectOutputProvider: correctOutputProvider,
		System:                system,
	}
}

type moveCfg struct {
	Opts        *bind.TransactOpts
	ignoreDupes bool
}

type MoveOpt interface {
	Apply(cfg *moveCfg)
}

type moveOptFn func(c *moveCfg)

func (f moveOptFn) Apply(c *moveCfg) {
	f(c)
}

func WithTransactOpts(Opts *bind.TransactOpts) MoveOpt {
	return moveOptFn(func(c *moveCfg) {
		c.Opts = Opts
	})
}

func WithIgnoreDuplicates() MoveOpt {
	return moveOptFn(func(c *moveCfg) {
		c.ignoreDupes = true
	})
}

func (g *OutputGameHelper) SplitDepth(ctx context.Context) types.Depth {
	splitDepth, err := g.Game.GetSplitDepth(ctx)
	g.Require.NoError(err, "failed to load split depth")
	return splitDepth
}

func (g *OutputGameHelper) ExecDepth(ctx context.Context) types.Depth {
	return g.MaxDepth(ctx) - g.SplitDepth(ctx) - 1
}

func (g *OutputGameHelper) L2BlockNum(ctx context.Context) uint64 {
	_, blockNum, err := g.Game.GetBlockRange(ctx)
	g.Require.NoError(err, "failed to load l2 block number")
	return blockNum
}

func (g *OutputGameHelper) StartingBlockNum(ctx context.Context) uint64 {
	blockNum, _, err := g.Game.GetBlockRange(ctx)
	g.Require.NoError(err, "failed to load starting block number")
	return blockNum
}

func (g *OutputGameHelper) DisputeLastBlock(ctx context.Context) *ClaimHelper {
	return g.DisputeBlock(ctx, g.L2BlockNum(ctx))
}

// DisputeBlock posts claims from both the honest and dishonest actor to progress the output root part of the game
// through to the split depth and the claims are setup such that the last block in the game range is the block
// to execute cannon on. ie the first block the honest and dishonest actors disagree about is the l2 block of the game.
func (g *OutputGameHelper) DisputeBlock(ctx context.Context, disputeBlockNum uint64) *ClaimHelper {
	dishonestValue := g.GetClaimValue(ctx, 0)
	correctRootClaim := g.correctOutputRoot(ctx, types.NewPositionFromGIndex(big.NewInt(1)))
	rootIsValid := dishonestValue == correctRootClaim
	if rootIsValid {
		// Ensure that the dishonest actor is actually posting invalid roots.
		// Otherwise, the honest challenger will defend our counter and ruin everything.
		dishonestValue = common.Hash{0xff, 0xff, 0xff}
	}
	pos := types.NewPositionFromGIndex(big.NewInt(1))
	getClaimValue := func(parentClaim *ClaimHelper, claimPos types.Position) common.Hash {
		claimBlockNum, err := g.CorrectOutputProvider.ClaimedBlockNumber(claimPos)
		g.Require.NoError(err, "failed to calculate claim block number")
		if claimBlockNum < disputeBlockNum {
			// Use the correct output root for all claims prior to the dispute block number
			// This pushes the game to dispute the last block in the range
			return g.correctOutputRoot(ctx, claimPos)
		}
		if rootIsValid == parentClaim.AgreesWithOutputRoot() {
			// We are responding to a parent claim that agrees with a valid root, so we're being dishonest
			return dishonestValue
		} else {
			// Otherwise we must be the honest actor so use the correct root
			return g.correctOutputRoot(ctx, claimPos)
		}
	}

	claim := g.RootClaim(ctx)
	for !claim.IsOutputRootLeaf(ctx) {
		parentClaimBlockNum, err := g.CorrectOutputProvider.ClaimedBlockNumber(pos)
		g.Require.NoError(err, "failed to calculate parent claim block number")
		if parentClaimBlockNum >= disputeBlockNum {
			pos = pos.Attack()
			claim = claim.Attack(ctx, getClaimValue(claim, pos))
		} else {
			pos = pos.Defend()
			claim = claim.Defend(ctx, getClaimValue(claim, pos))
		}
	}
	return claim
}

func (g *OutputGameHelper) RootClaim(ctx context.Context) *ClaimHelper {
	claim := g.getClaim(ctx, 0)
	return newClaimHelper(g, 0, claim)
}

func (g *OutputGameHelper) WaitForCorrectOutputRoot(ctx context.Context, claimIdx int64) {
	g.WaitForClaimCount(ctx, claimIdx+1)
	claim := g.getClaim(ctx, claimIdx)
	output := g.correctOutputRoot(ctx, claim.Position)
	g.Require.EqualValuesf(output, claim.Value, "Incorrect output root at claim %v at position %v", claimIdx, claim.Position.ToGIndex().Uint64())
}

func (g *OutputGameHelper) correctOutputRoot(ctx context.Context, pos types.Position) common.Hash {
	outputRoot, err := g.CorrectOutputProvider.Get(ctx, pos)
	g.Require.NoErrorf(err, "Failed to get correct output for position %v", pos)
	return outputRoot
}

func (g *OutputGameHelper) MaxClockDuration(ctx context.Context) time.Duration {
	duration, err := g.Game.GetMaxClockDuration(ctx)
	g.Require.NoError(err, "failed to get max clock duration")
	return duration
}

func (g *OutputGameHelper) WaitForNoAvailableCredit(ctx context.Context, addr common.Address) {
	timedCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		bal, _, err := g.Game.GetCredit(timedCtx, addr)
		if err != nil {
			return false, err
		}
		g.T.Log("Waiting for zero available credit", "current", bal, "addr", addr)
		return bal.Cmp(big.NewInt(0)) == 0, nil
	})
	if err != nil {
		g.LogGameData(ctx)
		g.Require.NoError(err, "Failed to wait for zero available credit")
	}
}

func (g *OutputGameHelper) AvailableCredit(ctx context.Context, addr common.Address) *big.Int {
	credit, _, err := g.Game.GetCredit(ctx, addr)
	g.Require.NoErrorf(err, "Failed to fetch available credit for %v", addr)
	return credit
}

func (g *OutputGameHelper) CreditUnlockDuration(ctx context.Context) time.Duration {
	_, delay, _, err := g.Game.GetBalanceAndDelay(ctx, rpcblock.Latest)
	g.Require.NoError(err, "Failed to get withdrawal delay")
	return delay
}

func (g *OutputGameHelper) WethBalance(ctx context.Context, addr common.Address) *big.Int {
	balance, _, _, err := g.Game.GetBalanceAndDelay(ctx, rpcblock.Latest)
	g.Require.NoError(err, "Failed to get WETH balance")
	return balance
}

// WaitForClaimCount waits until there are at least count claims in the game.
// This does not check that the number of claims is exactly the specified count to avoid intermittent failures
// where a challenger posts an additional claim before this method sees the number of claims it was waiting for.
func (g *OutputGameHelper) WaitForClaimCount(ctx context.Context, count int64) {
	timedCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		actual, err := g.Game.GetClaimCount(timedCtx)
		if err != nil {
			return false, err
		}
		g.T.Log("Waiting for claim count", "current", actual, "expected", count, "game", g.Addr)
		return int64(actual) >= count, nil
	})
	if err != nil {
		g.LogGameData(ctx)
		g.Require.NoErrorf(err, "Did not find expected claim count %v", count)
	}
}

type ContractClaim struct {
	ParentIndex uint32
	CounteredBy common.Address
	Claimant    common.Address
	Bond        *big.Int
	Claim       [32]byte
	Position    *big.Int
	Clock       *big.Int
}

func (g *OutputGameHelper) MaxDepth(ctx context.Context) types.Depth {
	depth, err := g.Game.GetMaxGameDepth(ctx)
	g.Require.NoError(err, "Failed to load game depth")
	return depth
}

func (g *OutputGameHelper) waitForClaim(ctx context.Context, timeout time.Duration, errorMsg string, predicate func(claimIdx int64, claim types.Claim) bool) (int64, types.Claim) {
	timedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var matchedClaim types.Claim
	var matchClaimIdx int64
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		claims, err := g.Game.GetAllClaims(ctx, rpcblock.Latest)
		if err != nil {
			return false, fmt.Errorf("retrieve all claims: %w", err)
		}
		// Search backwards because the new claims are at the end and more likely the ones we want.
		for i := len(claims) - 1; i >= 0; i-- {
			claim := claims[i]
			if predicate(int64(i), claim) {
				matchClaimIdx = int64(i)
				matchedClaim = claim
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil { // Avoid waiting time capturing game data when there's no error
		g.Require.NoErrorf(err, "%v\n%v", errorMsg, g.GameData(ctx))
	}
	return matchClaimIdx, matchedClaim
}

func (g *OutputGameHelper) waitForNoClaim(ctx context.Context, errorMsg string, predicate func(claim types.Claim) bool) {
	timedCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		claims, err := g.Game.GetAllClaims(ctx, rpcblock.Latest)
		if err != nil {
			return false, fmt.Errorf("retrieve all claims: %w", err)
		}
		// Search backwards because the new claims are at the end and more likely the ones we want.
		for i := len(claims) - 1; i >= 0; i-- {
			claim := claims[i]
			if predicate(claim) {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil { // Avoid waiting time capturing game data when there's no error
		g.Require.NoErrorf(err, "%v\n%v", errorMsg, g.GameData(ctx))
	}
}

func (g *OutputGameHelper) GetClaimValue(ctx context.Context, claimIdx int64) common.Hash {
	g.WaitForClaimCount(ctx, claimIdx+1)
	claim := g.getClaim(ctx, claimIdx)
	return claim.Value
}

func (g *OutputGameHelper) getAllClaims(ctx context.Context) []types.Claim {
	claims, err := g.Game.GetAllClaims(ctx, rpcblock.Latest)
	g.Require.NoError(err, "Failed to get all claims")
	return claims
}

// getClaim retrieves the claim data for a specific index.
// Note that it is deliberately not exported as tests should use WaitForClaim to avoid race conditions.
func (g *OutputGameHelper) getClaim(ctx context.Context, claimIdx int64) types.Claim {
	claimData, err := g.Game.GetClaim(ctx, uint64(claimIdx))
	if err != nil {
		g.Require.NoErrorf(err, "retrieve claim %v", claimIdx)
	}
	return claimData
}

func (g *OutputGameHelper) WaitForClaimAtDepth(ctx context.Context, depth types.Depth) {
	g.waitForClaim(
		ctx,
		defaultTimeout,
		fmt.Sprintf("Could not find claim depth %v", depth),
		func(_ int64, claim types.Claim) bool {
			return claim.Depth() == depth
		})
}

func (g *OutputGameHelper) WaitForClaimAtMaxDepth(ctx context.Context, countered bool) {
	maxDepth := g.MaxDepth(ctx)
	g.waitForClaim(
		ctx,
		defaultTimeout,
		fmt.Sprintf("Could not find claim depth %v with countered=%v", maxDepth, countered),
		func(_ int64, claim types.Claim) bool {
			return claim.Depth() == maxDepth && (claim.CounteredBy != common.Address{}) == countered
		})
}

func (g *OutputGameHelper) WaitForAllClaimsCountered(ctx context.Context) {
	g.waitForNoClaim(
		ctx,
		"Did not find all claims countered",
		func(claim types.Claim) bool {
			return claim.CounteredBy == common.Address{}
		})
}

func (g *OutputGameHelper) Resolve(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	candidate, err := g.Game.ResolveTx()
	g.Require.NoError(err)
	transactions.RequireSendTx(g.T, ctx, g.Client, candidate, g.PrivKey)
}

func (g *OutputGameHelper) Status(ctx context.Context) gameTypes.GameStatus {
	status, err := g.Game.GetStatus(ctx)
	g.Require.NoError(err)
	return status
}

func (g *OutputGameHelper) WaitForBondModeDecided(ctx context.Context) {
	g.T.Logf("Waiting for game %v to have bond mode set", g.Addr)
	timedCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		bondMode, err := g.Game.GetBondDistributionMode(ctx, rpcblock.Latest)
		g.Require.NoError(err)
		return bondMode != types.UndecidedDistributionMode, nil
	})
	g.Require.NoError(err, "Failed to wait for bond mode to be set")
}

func (g *OutputGameHelper) WaitForGameStatus(ctx context.Context, expected gameTypes.GameStatus) {
	g.T.Logf("Waiting for game %v to have status %v", g.Addr, expected)
	timedCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		ctx, cancel := context.WithTimeout(timedCtx, 30*time.Second)
		defer cancel()
		status, err := g.Game.GetStatus(ctx)
		if err != nil {
			return false, fmt.Errorf("game status unavailable: %w", err)
		}
		g.T.Logf("Game %v has state %v, waiting for state %v", g.Addr, status, expected)
		return expected == status, nil
	})
	g.Require.NoErrorf(err, "wait for Game status. Game state: \n%v", g.GameData(ctx))
}

func (g *OutputGameHelper) WaitForInactivity(ctx context.Context, numInactiveBlocks int, untilGameEnds bool) {
	g.T.Logf("Waiting for game %v to have no activity for %v blocks", g.Addr, numInactiveBlocks)
	headCh := make(chan *gethtypes.Header, 100)
	headSub, err := g.Client.SubscribeNewHead(ctx, headCh)
	g.Require.NoError(err)
	defer headSub.Unsubscribe()

	var lastActiveBlock uint64
	for {
		if untilGameEnds && g.Status(ctx) != gameTypes.GameStatusInProgress {
			break
		}
		select {
		case head := <-headCh:
			if lastActiveBlock == 0 {
				lastActiveBlock = head.Number.Uint64()
				continue
			} else if lastActiveBlock+uint64(numInactiveBlocks) < head.Number.Uint64() {
				return
			}
			block, err := g.Client.BlockByNumber(ctx, head.Number)
			g.Require.NoError(err)
			numActions := 0
			for _, tx := range block.Transactions() {
				if tx.To().Hex() == g.Addr.Hex() {
					numActions++
				}
			}
			if numActions != 0 {
				g.T.Logf("Game %v has %v actions in block %d. Resetting inactivity timeout", g.Addr, numActions, block.NumberU64())
				lastActiveBlock = head.Number.Uint64()
			}
		case err := <-headSub.Err():
			g.Require.NoError(err)
		case <-ctx.Done():
			g.Require.Fail("Context canceled", ctx.Err())
		}
	}
}

func (g *OutputGameHelper) WaitForL2BlockNumberChallenged(ctx context.Context) {
	g.T.Logf("Waiting for game %v to have L2 block number challenged", g.Addr)
	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		return g.Game.IsL2BlockNumberChallenged(ctx, rpcblock.Latest)
	})
	g.Require.NoError(err, "L2 block number was not challenged in time")
}

// Mover is a function that either attacks or defends the claim at parentClaimIdx
type Mover func(parent *ClaimHelper) *ClaimHelper

// Stepper is a function that attempts to perform a step against the claim at parentClaimIdx
type Stepper func(parentClaimIdx int64)

type defendClaimCfg struct {
	skipWaitingForStep bool
}

type DefendClaimOpt func(cfg *defendClaimCfg)

func WithoutWaitingForStep() DefendClaimOpt {
	return func(cfg *defendClaimCfg) {
		cfg.skipWaitingForStep = true
	}
}

// DefendClaim uses the supplied Mover to perform moves in an attempt to defend the supplied claim.
// It is assumed that the specified claim is invalid and that an honest op-challenger is already running.
// When the game has reached the maximum depth it waits for the honest challenger to counter the leaf claim with step.
// Returns the final leaf claim
func (g *OutputGameHelper) DefendClaim(ctx context.Context, claim *ClaimHelper, performMove Mover, Opts ...DefendClaimOpt) *ClaimHelper {
	g.T.Logf("Defending claim %v at depth %v", claim.Index, claim.Depth())
	cfg := &defendClaimCfg{}
	for _, opt := range Opts {
		opt(cfg)
	}
	for !claim.IsMaxDepth(ctx) {
		g.LogGameData(ctx)
		// Wait for the challenger to counter
		claim = claim.WaitForCounterClaim(ctx)
		g.LogGameData(ctx)

		// Respond with our own move
		claim = performMove(claim)
	}

	if !cfg.skipWaitingForStep {
		claim.WaitForCountered(ctx)
	}
	return claim
}

// ChallengeClaim uses the supplied functions to perform moves and steps in an attempt to challenge the supplied claim.
// It is assumed that the claim being disputed is valid and that an honest op-challenger is already running.
// When the game has reached the maximum depth it calls the Stepper to attempt to counter the leaf claim.
// Since the output root is valid, it should not be possible for the Stepper to call step successfully.
func (g *OutputGameHelper) ChallengeClaim(ctx context.Context, claim *ClaimHelper, performMove Mover, attemptStep Stepper) {
	for !claim.IsMaxDepth(ctx) {
		g.LogGameData(ctx)
		// Perform our move
		claim = performMove(claim)

		// Wait for the challenger to counter
		g.LogGameData(ctx)
		claim = claim.WaitForCounterClaim(ctx)
	}

	// Confirm the game has reached max depth and the last claim hasn't been countered
	g.WaitForClaimAtMaxDepth(ctx, false)
	g.LogGameData(ctx)

	// It's on us to call step if we want to win but shouldn't be possible
	attemptStep(claim.Index)
}

func (g *OutputGameHelper) WaitForNewClaim(ctx context.Context, checkPoint int64) (int64, error) {
	return g.waitForNewClaim(ctx, checkPoint, defaultTimeout)
}

func (g *OutputGameHelper) waitForNewClaim(ctx context.Context, checkPoint int64, timeout time.Duration) (int64, error) {
	timedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var newClaimLen int64
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		actual, err := g.Game.GetClaimCount(ctx)
		if err != nil {
			return false, err
		}
		newClaimLen = int64(actual)
		return int64(actual) > checkPoint, nil
	})
	return newClaimLen, err
}

func (g *OutputGameHelper) moveCfg(Opts ...MoveOpt) *moveCfg {
	cfg := &moveCfg{
		Opts: g.Opts,
	}
	for _, opt := range Opts {
		opt.Apply(cfg)
	}
	return cfg
}

func (g *OutputGameHelper) Attack(ctx context.Context, claimIdx int64, claim common.Hash, Opts ...MoveOpt) {
	g.T.Logf("Attacking claim %v with value %v", claimIdx, claim)
	cfg := g.moveCfg(Opts...)

	claimData, err := g.Game.GetClaim(ctx, uint64(claimIdx))
	g.Require.NoError(err, "Failed to get claim data")
	attackPos := claimData.Position.Attack()

	candidate, err := g.Game.AttackTx(ctx, claimData, claim)
	g.Require.NoError(err, "Failed to create tx candidate")
	_, _, err = transactions.SendTx(ctx, g.Client, candidate, g.PrivKey)
	if err != nil {
		if cfg.ignoreDupes && g.hasClaim(ctx, claimIdx, attackPos, claim) {
			return
		}
		g.Require.NoErrorf(err, "Defend transaction failed. Game state: \n%v", g.GameData(ctx))
	}
}

func (g *OutputGameHelper) Defend(ctx context.Context, claimIdx int64, claim common.Hash, Opts ...MoveOpt) {
	g.T.Logf("Defending claim %v with value %v", claimIdx, claim)
	cfg := g.moveCfg(Opts...)

	claimData, err := g.Game.GetClaim(ctx, uint64(claimIdx))
	g.Require.NoError(err, "Failed to get claim data")
	defendPos := claimData.Position.Defend()

	candidate, err := g.Game.DefendTx(ctx, claimData, claim)
	g.Require.NoError(err, "Failed to create tx candidate")
	_, _, err = transactions.SendTx(ctx, g.Client, candidate, g.PrivKey)
	if err != nil {
		if cfg.ignoreDupes && g.hasClaim(ctx, claimIdx, defendPos, claim) {
			return
		}
		g.Require.NoErrorf(err, "Defend transaction failed. Game state: \n%v", g.GameData(ctx))
	}
}

func (g *OutputGameHelper) hasClaim(ctx context.Context, parentIdx int64, pos types.Position, value common.Hash) bool {
	claims := g.getAllClaims(ctx)
	for _, claim := range claims {
		if int64(claim.ParentContractIndex) == parentIdx && claim.Position.ToGIndex().Cmp(pos.ToGIndex()) == 0 && claim.Value == value {
			return true
		}
	}
	return false
}

// StepFails attempts to call step and verifies that it fails with ValidStep()
func (g *OutputGameHelper) StepFails(ctx context.Context, claimIdx int64, isAttack bool, stateData []byte, proof []byte) {
	g.T.Logf("Attempting step against claim %v isAttack: %v", claimIdx, isAttack)
	candidate, err := g.Game.StepTx(uint64(claimIdx), isAttack, stateData, proof)
	g.Require.NoError(err, "Failed to create tx candidate")
	_, _, err = transactions.SendTx(ctx, g.Client, candidate, g.PrivKey, transactions.WithReceiptFail())
	err = errutil.TryAddRevertReason(err)
	g.Require.Error(err, "Transaction should fail")
	validStepErr := "0xfb4e40dd"
	invalidPrestateErr := "0x696550ff"
	if !strings.Contains(err.Error(), validStepErr) && !strings.Contains(err.Error(), invalidPrestateErr) {
		g.Require.Failf("Revert reason should be abi encoded ValidStep() or InvalidPrestate() but was: %v", err.Error())
	}
}

// ResolveClaim resolves a single subgame
func (g *OutputGameHelper) ResolveClaim(ctx context.Context, claimIdx int64) {
	candidate, err := g.Game.ResolveClaimTx(uint64(claimIdx))
	g.Require.NoError(err, "Failed to create resolve claim candidate tx")
	transactions.RequireSendTx(g.T, ctx, g.Client, candidate, g.PrivKey)
}

// ChallengePeriod returns the challenge period fetched from the PreimageOracle contract.
// The returned uint64 value is the number of seconds for the challenge period.
func (g *OutputGameHelper) ChallengePeriod(ctx context.Context) uint64 {
	oracle := g.oracle(ctx)
	period, err := oracle.ChallengePeriod(ctx)
	g.Require.NoError(err, "Failed to get challenge period")
	return period
}

// WaitForChallengePeriodStart waits for the challenge period to start for a given large preimage claim.
func (g *OutputGameHelper) WaitForChallengePeriodStart(ctx context.Context, sender common.Address, data *types.PreimageOracleData) {
	timedCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		ctx, cancel := context.WithTimeout(timedCtx, 30*time.Second)
		defer cancel()
		timestamp := g.ChallengePeriodStartTime(ctx, sender, data)
		g.T.Log("Waiting for challenge period start", "timestamp", timestamp, "key", data.OracleKey, "game", g.Addr)
		return timestamp > 0, nil
	})
	if err != nil {
		g.LogGameData(ctx)
		g.Require.NoErrorf(err, "Failed to get challenge start period for preimage data %v", data)
	}
}

// ChallengePeriodStartTime returns the start time of the challenge period for a given large preimage claim.
// If the returned start time is 0, the challenge period has not started.
func (g *OutputGameHelper) ChallengePeriodStartTime(ctx context.Context, sender common.Address, data *types.PreimageOracleData) uint64 {
	oracle := g.oracle(ctx)
	uuid := preimages.NewUUID(sender, data)
	metadata, err := oracle.GetProposalMetadata(ctx, rpcblock.Latest, keccakTypes.LargePreimageIdent{
		Claimant: sender,
		UUID:     uuid,
	})
	g.Require.NoError(err, "Failed to get proposal metadata")
	if len(metadata) == 0 {
		return 0
	}
	return metadata[0].Timestamp
}

func (g *OutputGameHelper) WaitForPreimageInOracle(ctx context.Context, data *types.PreimageOracleData) {
	timedCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	oracle := g.oracle(ctx)
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		g.T.Logf("Waiting for preimage (%v) to be present in oracle", common.Bytes2Hex(data.OracleKey))
		return oracle.GlobalDataExists(ctx, data)
	})
	g.Require.NoErrorf(err, "Did not find preimage (%v) in oracle", common.Bytes2Hex(data.OracleKey))
}

func (g *OutputGameHelper) UploadPreimage(ctx context.Context, data *types.PreimageOracleData) {
	oracle := g.oracle(ctx)
	tx, err := oracle.AddGlobalDataTx(data)
	g.Require.NoError(err, "Failed to create preimage upload tx")
	transactions.RequireSendTx(g.T, ctx, g.Client, tx, g.PrivKey)
}

func (g *OutputGameHelper) oracle(ctx context.Context) contracts.PreimageOracleContract {
	oracle, err := g.Game.GetOracle(ctx)
	g.Require.NoError(err, "Failed to create oracle contract")
	return oracle
}

func (g *OutputGameHelper) GameData(ctx context.Context) string {
	maxDepth := g.MaxDepth(ctx)
	splitDepth := g.SplitDepth(ctx)
	claims, err := g.Game.GetAllClaims(ctx, rpcblock.Latest)
	g.Require.NoError(err, "Fetching claims")
	info := fmt.Sprintf("Claim count: %v\n", len(claims))
	for i, claim := range claims {
		pos := claim.Position
		extra := ""
		if pos.Depth() <= splitDepth {
			blockNum, err := g.CorrectOutputProvider.ClaimedBlockNumber(pos)
			if err != nil {
			} else {
				extra = fmt.Sprintf("Block num: %v", blockNum)
			}
		}
		info = info + fmt.Sprintf("%v - Position: %v, Depth: %v, IndexAtDepth: %v Trace Index: %v, ClaimHash: %v, Countered By: %v, ParentIndex: %v Claimant: %v Bond: %v %v\n",
			i, claim.Position.ToGIndex().Int64(), pos.Depth(), pos.IndexAtDepth(), pos.TraceIndex(maxDepth), claim.Value.Hex(), claim.CounteredBy, claim.ParentContractIndex, claim.Claimant, claim.Bond, extra)
	}
	l2BlockNum := g.L2BlockNum(ctx)
	status, err := g.Game.GetStatus(ctx)
	g.Require.NoError(err, "Load game status")
	return fmt.Sprintf("Game %v - %v - L2 Block: %v - Split Depth: %v - Max Depth: %v:\n%v\n",
		g.Addr, status, l2BlockNum, splitDepth, maxDepth, info)
}

func (g *OutputGameHelper) LogGameData(ctx context.Context) {
	g.T.Log(g.GameData(ctx))
}

func (g *OutputGameHelper) Credit(ctx context.Context, addr common.Address) *big.Int {
	amt, _, err := g.Game.GetCredit(ctx, addr)
	g.Require.NoError(err)
	return amt
}

func (g *OutputGameHelper) GetL1Head(ctx context.Context) eth.BlockID {
	l1HeadHash, err := g.Game.GetL1Head(ctx)
	g.Require.NoError(err, "Failed to load L1 head")
	l1Header, err := g.Client.HeaderByHash(ctx, l1HeadHash)
	g.Require.NoError(err, "Failed to load L1 header")
	l1Head := eth.HeaderBlockID(l1Header)
	return l1Head
}
