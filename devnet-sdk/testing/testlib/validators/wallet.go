package validators

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

type WalletGetter = func(context.Context) system.Wallet

func walletFundsValidator(chainIdx uint64, minFunds types.Balance, userMarker interface{}) systest.PreconditionValidator {
	constraint := constraints.WithBalance(minFunds)
	return func(t systest.T, sys system.System) (context.Context, error) {
		chain := sys.L2s()[chainIdx]
		wallets, err := chain.Wallets(t.Context())
		if err != nil {
			return nil, err
		}

		for _, wallet := range wallets {
			if constraint.CheckWallet(wallet) {
				return context.WithValue(t.Context(), userMarker, wallet), nil
			}
		}

		return nil, fmt.Errorf("no available wallet with balance of at least of %s", minFunds)

	}
}

func AcquireL2WalletWithFunds(chainIdx uint64, minFunds types.Balance) (WalletGetter, systest.PreconditionValidator) {
	userMarker := &struct{}{}
	validator := walletFundsValidator(chainIdx, minFunds, userMarker)
	return func(ctx context.Context) system.Wallet {
		return ctx.Value(userMarker).(system.Wallet)
	}, validator
}
