package extract

import (
	"context"
	"fmt"

	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"

	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
)

const metricsLabel = "game_caller_creator"

type GameCallerMetrics interface {
	caching.Metrics
	contractMetrics.ContractMetricer
}

type GameCaller interface {
	GetWithdrawals(context.Context, rpcblock.Block, ...common.Address) ([]*contracts.WithdrawalRequest, error)
	GetGameMetadata(context.Context, rpcblock.Block) (contracts.GameMetadata, error)
	GetAllClaims(context.Context, rpcblock.Block) ([]faultTypes.Claim, error)
	BondCaller
	BalanceCaller
	ClaimCaller
}

type GameCallerCreator struct {
	m      GameCallerMetrics
	cache  *caching.LRUCache[common.Address, contracts.FaultDisputeGameContract]
	caller *batching.MultiCaller
}

func NewGameCallerCreator(m GameCallerMetrics, caller *batching.MultiCaller) *GameCallerCreator {
	return &GameCallerCreator{
		m:      m,
		caller: caller,
		cache:  caching.NewLRUCache[common.Address, contracts.FaultDisputeGameContract](m, metricsLabel, 100),
	}
}

func (g *GameCallerCreator) CreateContract(ctx context.Context, game gameTypes.GameMetadata) (GameCaller, error) {
	if fdg, ok := g.cache.Get(game.Proxy); ok {
		return fdg, nil
	}
	switch faultTypes.GameType(game.GameType) {
	case faultTypes.CannonGameType,
		faultTypes.PermissionedGameType,
		faultTypes.AsteriscGameType,
		faultTypes.AlphabetGameType,
		faultTypes.FastGameType,
		faultTypes.AsteriscKonaGameType:
		fdg, err := contracts.NewFaultDisputeGameContract(ctx, g.m, game.Proxy, g.caller)
		if err != nil {
			return nil, fmt.Errorf("failed to create fault dispute game contract: %w", err)
		}
		g.cache.Add(game.Proxy, fdg)
		return fdg, nil
	default:
		return nil, fmt.Errorf("unsupported game type: %d", game.GameType)
	}
}
