package system

import "github.com/ethereum-optimism/optimism/devnet-sdk/types"

type System interface {
	Chain(chainID types.ChainID) Chain
}

type Chain interface {
	ContractAddress(contractID string) types.Address
}

type InteropSystem interface {
	System
	InteropSet() InteropSet
}

type InteropSet interface {
	Chain(chainID types.ChainID) Chain
}
