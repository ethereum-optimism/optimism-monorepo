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

// walletFundsValidator creates a PreconditionValidator that ensures a wallet with sufficient funds
// is available on the specified L2 chain. If successful, it stores the wallet in the context
// using the provided userMarker as the key.
//
// Parameters:
//   - chainIdx: The index of the L2 chain to check for wallets
//   - minFunds: The minimum balance required for a wallet to be selected
//   - userMarker: A unique object to use as a key for storing the wallet in the context
//
// The validator:
// 1. Retrieves all wallets from the specified L2 chain
// 2. Checks each wallet to find one with at least the minimum required balance
// 3. If found, stores the wallet in the context with the provided marker
//
// Returns an error if no wallet with sufficient funds is found.
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
