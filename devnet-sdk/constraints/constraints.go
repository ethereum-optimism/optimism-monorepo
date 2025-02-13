package constraints

import (
	"log/slog"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

type WalletConstraint func(wallet system.Wallet) bool

func WithBalance(amount types.Balance) WalletConstraint {
	return func(wallet system.Wallet) bool {
		balance := wallet.Balance()
		slog.Debug("checking balance", "wallet", wallet.Address(), "balance", balance, "needed", amount)
		return balance.GreaterThan(amount)
	}
}
