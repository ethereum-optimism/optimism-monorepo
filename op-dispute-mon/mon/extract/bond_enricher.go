package extract

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	monTypes "github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/exp/maps"
)

var _ Enricher = (*BondEnricher)(nil)

var ErrIncorrectCreditCount = errors.New("incorrect credit count")

type BondCaller interface {
	GetCredits(context.Context, rpcblock.Block, ...common.Address) ([]*big.Int, error)
	GetBondDistributionMode(context.Context, rpcblock.Block) (types.BondDistributionMode, error)
}

type BondEnricher struct{}

func NewBondEnricher() *BondEnricher {
	return &BondEnricher{}
}

func (b *BondEnricher) Enrich(ctx context.Context, block rpcblock.Block, caller GameCaller, game *monTypes.EnrichedGameData) error {
	recipientAddrs := maps.Keys(game.Recipients)
	credits, err := caller.GetCredits(ctx, block, recipientAddrs...)
	if err != nil {
		return err
	}
	if len(credits) != len(recipientAddrs) {
		return fmt.Errorf("%w, requested %v values but got %v", ErrIncorrectCreditCount, len(recipientAddrs), len(credits))
	}
	game.Credits = make(map[common.Address]*big.Int)
	for i, credit := range credits {
		game.Credits[recipientAddrs[i]] = credit
	}
	game.BondDistributionMode, err = caller.GetBondDistributionMode(ctx, block)
	if err != nil {
		return err
	}
	return nil
}
