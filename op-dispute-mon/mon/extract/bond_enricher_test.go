package extract

import (
	"context"
	"errors"
	"math/big"
	"testing"

	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	monTypes "github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// makeTestGame returns an enriched game with 3 claims and a list of expected recipients.
func makeTestGame() (*monTypes.EnrichedGameData, []common.Address) {
	game := &monTypes.EnrichedGameData{
		Recipients: map[common.Address]bool{
			common.Address{0x02}: true,
			common.Address{0x03}: true,
			common.Address{0x04}: true,
		},
		Claims: []monTypes.EnrichedClaim{
			{
				Claim: faultTypes.Claim{
					ClaimData: faultTypes.ClaimData{
						Position: faultTypes.NewPositionFromGIndex(big.NewInt(1)),
					},
					Claimant:    common.Address{0x01},
					CounteredBy: common.Address{0x02},
				},
				Resolved: true,
			},
			{
				Claim: faultTypes.Claim{
					ClaimData: faultTypes.ClaimData{
						Bond:     big.NewInt(5),
						Position: faultTypes.NewPositionFromGIndex(big.NewInt(2)),
					},
					Claimant:    common.Address{0x03},
					CounteredBy: common.Address{},
				},
			},
			{
				Claim: faultTypes.Claim{
					ClaimData: faultTypes.ClaimData{
						Bond:     big.NewInt(7),
						Position: faultTypes.NewPositionFromGIndex(big.NewInt(3)),
					},
					Claimant:    common.Address{0x03},
					CounteredBy: common.Address{0x04},
				},
			},
		},
	}
	recipients := []common.Address{
		game.Claims[0].CounteredBy,
		game.Claims[1].Claimant,
		game.Claims[2].CounteredBy,
	}
	return game, recipients
}

func TestBondEnricher(t *testing.T) {
	t.Run("GetCreditsFails", func(t *testing.T) {
		enricher := NewBondEnricher()
		caller := &mockGameCaller{creditsErr: errors.New("nope")}
		game, _ := makeTestGame()
		err := enricher.Enrich(context.Background(), rpcblock.Latest, caller, game)
		require.ErrorIs(t, err, caller.creditsErr)
	})

	t.Run("GetCreditsWrongNumberOfResults", func(t *testing.T) {
		enricher := NewBondEnricher()
		caller := &mockGameCaller{extraCredit: []*big.Int{big.NewInt(4)}}
		game, _ := makeTestGame()
		err := enricher.Enrich(context.Background(), rpcblock.Latest, caller, game)
		require.ErrorIs(t, err, ErrIncorrectCreditCount)
	})

	t.Run("GetCreditsSuccess", func(t *testing.T) {
		game, recipients := makeTestGame()
		enricher := NewBondEnricher()
		expectedCredits := map[common.Address]*big.Int{
			recipients[0]: big.NewInt(20),
			recipients[1]: big.NewInt(30),
			recipients[2]: big.NewInt(40),
		}
		caller := &mockGameCaller{credits: expectedCredits, bondDistributionMode: faultTypes.RefundDistributionMode}
		err := enricher.Enrich(context.Background(), rpcblock.Latest, caller, game)
		require.NoError(t, err)

		require.Equal(t, len(recipients), len(caller.requestedCredits))
		for _, recipient := range recipients {
			require.Contains(t, caller.requestedCredits, recipient)
		}
		require.Equal(t, expectedCredits, game.Credits)
		require.Equal(t, faultTypes.RefundDistributionMode, game.BondDistributionMode)
	})
}
