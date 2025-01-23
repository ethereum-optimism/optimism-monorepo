package interop

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/stretchr/testify/require"
)

var (
	testUserMarker = &struct{}{}
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

func TestMinimal(t *testing.T) {
	// We'll use the first L2 chain for this test
	chainIdx := uint64(0)

	systest.SystemTest(t, func(t systest.T, sys system.System) {
		ctx := t.Context()
		logger := slog.With("test", "TestMinimal", "devnet", sys.Identifier())

		chain := sys.L2(chainIdx)
		logger = logger.With("chain", chain.ID())
		logger.InfoContext(ctx, "starting test")

		funds := types.NewBalance(big.NewInt(0.5 * constants.ETH))
		user := ctx.Value(testUserMarker).(types.Wallet)

		scw0Addr := constants.SuperchainWETH
		scw0 := contracts.MustResolveContract[contracts.SuperchainWETH](chain, scw0Addr)
		logger.InfoContext(ctx, "using SuperchainWETH", "contract", scw0Addr)

		initialBalance, err := scw0.BalanceOf(user.Address()).Call(ctx)
		require.NoError(t, err)
		logger = logger.With("user", user.Address())
		logger.InfoContext(ctx, "initial balance retrieved", "balance", initialBalance)

		logger.InfoContext(ctx, "sending ETH to contract", "amount", funds)
		require.NoError(t, user.SendETH(scw0Addr, funds).Send(ctx).Wait())

		balance, err := scw0.BalanceOf(user.Address()).Call(ctx)
		require.NoError(t, err)
		logger.InfoContext(ctx, "final balance retrieved", "balance", balance)

		require.Equal(t, balance, initialBalance.Add(funds))
	},
		userFundsValidator(chainIdx, types.NewBalance(big.NewInt(1.0*constants.ETH)), testUserMarker),
	)
}

func userFundsValidator(chainIdx uint64, minFunds types.Balance, userMarker interface{}) systest.Validator {
	return func(t systest.T, sys system.System) (context.Context, error) {
		chain := sys.L2(chainIdx)
		user, err := chain.User(t.Context(), constraints.WithBalance(minFunds))
		if err != nil {
			return nil, fmt.Errorf("No available user with funds: %v", err)
		}
		return context.WithValue(t.Context(), userMarker, user), nil
	}
}
