package constraints

import (
	"log/slog"

	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

type WalletConstraint interface {
	CheckWallet(wallet types.Wallet) bool
}

type WalletConstraintFunc func(wallet types.Wallet) bool

func (f WalletConstraintFunc) CheckWallet(wallet types.Wallet) bool {
	return f(wallet)
}

func WithBalance(amount types.Balance) WalletConstraint {
	return WalletConstraintFunc(func(wallet types.Wallet) bool {
		balance := wallet.Balance()
		slog.Debug("checking balance", "wallet", wallet.Address(), "balance", balance, "needed", amount)
		return balance.GreaterThan(amount)
	})
}
