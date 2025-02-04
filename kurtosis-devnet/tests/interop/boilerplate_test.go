package interop

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

func walletFundsValidator(chainIdx uint64, minFunds types.Balance, userMarker interface{}) systest.PreconditionValidator {
	return func(t systest.T, sys system.System) (context.Context, error) {
		chain := sys.L2(chainIdx)
		user, err := chain.Wallet(t.Context(), constraints.WithBalance(minFunds))
		if err != nil {
			return nil, fmt.Errorf("No available wallet with funds: %w", err)
		}
		return context.WithValue(t.Context(), userMarker, user), nil
	}
}
