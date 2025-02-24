package contracts

import (
	"context"
	_ "embed"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed abis/FaultDisputeGame-0.8.0.json
var faultDisputeGameAbi020 []byte

var resolvedBondAmount = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

var (
	methodGameDuration = "gameDuration"
)

type FaultDisputeGameContract080 struct {
	FaultDisputeGameContractLatest
}

// GetGameMetadata returns the game's L1 head, L2 block number, root claim, status, and max clock duration.
func (f *FaultDisputeGameContract080) GetGameMetadata(ctx context.Context, block rpcblock.Block) (GameMetadata, error) {
	defer f.metrics.StartContractRequest("GetGameMetadata")()
	results, err := f.multiCaller.Call(ctx, block,
		f.contract.Call(methodL1Head),
		f.contract.Call(methodL2BlockNumber),
		f.contract.Call(methodRootClaim),
		f.contract.Call(methodStatus),
		f.contract.Call(methodGameDuration))
	if err != nil {
		return GameMetadata{}, fmt.Errorf("failed to retrieve game metadata: %w", err)
	}
	if len(results) != 5 {
		return GameMetadata{}, fmt.Errorf("expected 5 results but got %v", len(results))
	}
	l1Head := results[0].GetHash(0)
	l2BlockNumber := results[1].GetBigInt(0).Uint64()
	rootClaim := results[2].GetHash(0)
	status, err := gameTypes.GameStatusFromUint8(results[3].GetUint8(0))
	if err != nil {
		return GameMetadata{}, fmt.Errorf("failed to convert game status: %w", err)
	}
	duration := results[4].GetUint64(0)
	return GameMetadata{
		L1Head:                  l1Head,
		L2BlockNum:              l2BlockNumber,
		RootClaim:               rootClaim,
		Status:                  status,
		MaxClockDuration:        duration / 2,
		L2BlockNumberChallenged: false,
	}, nil
}

func (f *FaultDisputeGameContract080) GetMaxClockDuration(ctx context.Context) (time.Duration, error) {
	defer f.metrics.StartContractRequest("GetMaxClockDuration")()
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodGameDuration))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch game duration: %w", err)
	}
	return time.Duration(result.GetUint64(0)) * time.Second / 2, nil
}

func (f *FaultDisputeGameContract080) GetClaim(ctx context.Context, idx uint64) (types.Claim, error) {
	claim, err := f.FaultDisputeGameContractLatest.GetClaim(ctx, idx)
	if err != nil {
		return types.Claim{}, err
	}
	// Replace the resolved sentinel with what the bond would have been
	if claim.Bond.Cmp(resolvedBondAmount) == 0 {
		bond, err := f.GetRequiredBond(ctx, claim.Position)
		if err != nil {
			return types.Claim{}, err
		}
		claim.Bond = bond
	}
	return claim, nil
}

func (f *FaultDisputeGameContract080) GetAllClaims(ctx context.Context, block rpcblock.Block) ([]types.Claim, error) {
	claims, err := f.FaultDisputeGameContractLatest.GetAllClaims(ctx, block)
	if err != nil {
		return nil, err
	}
	resolvedClaims := make([]*types.Claim, 0, len(claims))
	positions := make([]*big.Int, 0, len(claims))
	for i, claim := range claims {
		if claim.Bond.Cmp(resolvedBondAmount) == 0 {
			resolvedClaims = append(resolvedClaims, &claims[i])
			positions = append(positions, claim.Position.ToGIndex())
		}
	}
	bonds, err := f.GetRequiredBonds(ctx, block, positions...)
	if err != nil {
		return nil, fmt.Errorf("failed to get required bonds for resolved claims: %w", err)
	}
	for i, bond := range bonds {
		resolvedClaims[i].Bond = bond
	}
	return claims, nil
}

func (f *FaultDisputeGameContract080) IsResolved(ctx context.Context, block rpcblock.Block, claims ...types.Claim) ([]bool, error) {
	rawClaims, err := f.FaultDisputeGameContractLatest.GetAllClaims(ctx, block)
	if err != nil {
		return nil, fmt.Errorf("failed to get raw claim data: %w", err)
	}
	results := make([]bool, len(claims))
	for i, claim := range claims {
		results[i] = rawClaims[claim.ContractIndex].Bond.Cmp(resolvedBondAmount) == 0
	}
	return results, nil
}

func (f *FaultDisputeGameContract080) CallResolveClaim(ctx context.Context, claimIdx uint64) error {
	defer f.metrics.StartContractRequest("CallResolveClaim")()
	call := f.resolveClaimCall(claimIdx)
	_, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, call)
	if err != nil {
		return fmt.Errorf("failed to call resolve claim: %w", err)
	}
	return nil
}

func (f *FaultDisputeGameContract080) ResolveClaimTx(claimIdx uint64) (txmgr.TxCandidate, error) {
	call := f.resolveClaimCall(claimIdx)
	return call.ToTxCandidate()
}

func (f *FaultDisputeGameContract080) resolveClaimCall(claimIdx uint64) *batching.ContractCall {
	return f.contract.Call(methodResolveClaim, new(big.Int).SetUint64(claimIdx))
}

func (f *FaultDisputeGameContract080) IsL2BlockNumberChallenged(_ context.Context, _ rpcblock.Block) (bool, error) {
	return false, nil
}

func (f *FaultDisputeGameContract080) ChallengeL2BlockNumberTx(_ *types.InvalidL2BlockNumberChallenge) (txmgr.TxCandidate, error) {
	return txmgr.TxCandidate{}, ErrChallengeL2BlockNotSupported
}

func (f *FaultDisputeGameContract080) AttackTx(ctx context.Context, parent types.Claim, pivot common.Hash) (txmgr.TxCandidate, error) {
	call := f.contract.Call(methodAttack, big.NewInt(int64(parent.ContractIndex)), pivot)
	return f.txWithBond(ctx, parent.Position.Attack(), call)
}

func (f *FaultDisputeGameContract080) DefendTx(ctx context.Context, parent types.Claim, pivot common.Hash) (txmgr.TxCandidate, error) {
	call := f.contract.Call(methodDefend, big.NewInt(int64(parent.ContractIndex)), pivot)
	return f.txWithBond(ctx, parent.Position.Defend(), call)
}

func (f *FaultDisputeGameContract080) GetBondDistributionMode(ctx context.Context, block rpcblock.Block) (types.BondDistributionMode, error) {
	return types.LegacyDistributionMode, nil
}
