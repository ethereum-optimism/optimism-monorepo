package disputegame

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/outputs"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

type OutputGameHelper struct {
	SplitGameHelper
	ClaimedBlockNumber func(pos types.Position) (uint64, error)
}

func NewOutputGameHelper(t *testing.T, require *require.Assertions, client *ethclient.Client, opts *bind.TransactOpts, privKey *ecdsa.PrivateKey,
	game contracts.FaultDisputeGameContract, factoryAddr common.Address, addr common.Address, correctOutputProvider *outputs.OutputTraceProvider, system DisputeSystem) *OutputGameHelper {
	return &OutputGameHelper{
		SplitGameHelper: SplitGameHelper{
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
			DescribePosition: func(pos types.Position, splitDepth types.Depth) string {
				if pos.Depth() > splitDepth {
					return ""
				}
				blockNum, err := correctOutputProvider.ClaimedBlockNumber(pos)
				if err != nil {
					return ""
				}
				return fmt.Sprintf("Block num: %v", blockNum)
			},
		},
		ClaimedBlockNumber: correctOutputProvider.ClaimedBlockNumber,
	}
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
	correctRootClaim := g.correctClaimValue(ctx, types.NewPositionFromGIndex(big.NewInt(1)))
	rootIsValid := dishonestValue == correctRootClaim
	if rootIsValid {
		// Ensure that the dishonest actor is actually posting invalid roots.
		// Otherwise, the honest challenger will defend our counter and ruin everything.
		dishonestValue = common.Hash{0xff, 0xff, 0xff}
	}
	pos := types.NewPositionFromGIndex(big.NewInt(1))
	getClaimValue := func(parentClaim *ClaimHelper, claimPos types.Position) common.Hash {
		claimBlockNum, err := g.ClaimedBlockNumber(claimPos)
		g.Require.NoError(err, "failed to calculate claim block number")
		if claimBlockNum < disputeBlockNum {
			// Use the correct output root for all claims prior to the dispute block number
			// This pushes the game to dispute the last block in the range
			return g.correctClaimValue(ctx, claimPos)
		}
		if rootIsValid == parentClaim.AgreesWithOutputRoot() {
			// We are responding to a parent claim that agrees with a valid root, so we're being dishonest
			return dishonestValue
		} else {
			// Otherwise we must be the honest actor so use the correct root
			return g.correctClaimValue(ctx, claimPos)
		}
	}

	claim := g.RootClaim(ctx)
	for !claim.IsOutputRootLeaf(ctx) {
		parentClaimBlockNum, err := g.ClaimedBlockNumber(pos)
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

func (g *OutputGameHelper) WaitForL2BlockNumberChallenged(ctx context.Context) {
	g.T.Logf("Waiting for game %v to have L2 block number challenged", g.Addr)
	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		return g.Game.IsL2BlockNumberChallenged(ctx, rpcblock.Latest)
	})
	g.Require.NoError(err, "L2 block number was not challenged in time")
}
