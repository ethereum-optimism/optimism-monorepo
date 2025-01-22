package system

import (
	"context"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

type System interface {
	Chain(chainID types.ChainID) Chain
	ContractAddress(contractID string) types.Address
}

type Chain interface {
	RPCURL() string
	ContractAddress(contractID string) types.Address
	User(ctx context.Context, constraints ...constraints.Constraint) (types.Address, error)
}

type InteropSystem interface {
	System
	InteropSet() InteropSet
}

type InteropSet interface {
	Chain(chainID types.ChainID) Chain
}
