package extract

import (
	"context"
	"fmt"

	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
)

const metricsLabel = "game_caller_creator"

type GameCallerMetrics interface {
	caching.Metrics
	contractMetrics.ContractMetricer
}
type GameCaller interface {
	GetGameMetadata(context.Context, rpcblock.Block) (common.Hash, uint64, common.Hash, types.GameStatus, uint64, error)
	GetAllClaims(context.Context, rpcblock.Block) ([]faultTypes.Claim, error)
	BondCaller
	BalanceCaller
}

type GameCallerCreator struct {
	m      GameCallerMetrics
	cache  *caching.LRUCache[common.Address, *contracts.FaultDisputeGameContract]
	caller *batching.MultiCaller
}

func NewGameCallerCreator(m GameCallerMetrics, caller *batching.MultiCaller) *GameCallerCreator {
	return &GameCallerCreator{
		m:      m,
		caller: caller,
		cache:  caching.NewLRUCache[common.Address, *contracts.FaultDisputeGameContract](m, metricsLabel, 100),
	}
}

func (g *GameCallerCreator) CreateContract(game types.GameMetadata) (GameCaller, error) {
	if fdg, ok := g.cache.Get(game.Proxy); ok {
		return fdg, nil
	}
	switch game.GameType {
	case faultTypes.CannonGameType, faultTypes.AlphabetGameType:
		fdg, err := contracts.NewFaultDisputeGameContract(g.m, game.Proxy, g.caller)
		if err != nil {
			return nil, fmt.Errorf("failed to create FaultDisputeGameContract: %w", err)
		}
		g.cache.Add(game.Proxy, fdg)
		return fdg, nil
	default:
		return nil, fmt.Errorf("unsupported game type: %d", game.GameType)
	}
}
