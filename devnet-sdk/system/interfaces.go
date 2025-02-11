package system

import (
	"context"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

// System represents a complete Optimism system with L1 and L2 chains
type System interface {
	Identifier() string
	L1() Chain
	// TODO: fix the chain ID type
	L2(uint64) Chain
}

// Chain represents an Ethereum chain (L1 or L2)
type Chain interface {
	RPCURL() string
	ID() types.ChainID
	Wallet(ctx context.Context, constraints ...constraints.WalletConstraint) (types.Wallet, error)
	ContractsRegistry() interfaces.ContractsRegistry
}

// InteropSystem extends System with interoperability features
type InteropSystem interface {
	System
	InteropSet() InteropSet
}

// InteropSet provides access to L2 chains in an interop environment
type InteropSet interface {
	L2(uint64) Chain
}
