package interfaces

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

// ErrContractNotFound indicates that a contract is not available at the requested address
type ErrContractNotFound struct {
	ContractType string
	Address      types.Address
}

func (e *ErrContractNotFound) Error() string {
	return fmt.Sprintf("%s contract not found at %s", e.ContractType, e.Address)
}

// ContractsRegistry provides access to all supported contract instances
type ContractsRegistry interface {
	SuperchainWETH(address types.Address) (SuperchainWETH, error)
}

// SuperchainWETH represents the interface for interacting with the SuperchainWETH contract
type SuperchainWETH interface {
	BalanceOf(user types.Address) types.ReadInvocation[types.Balance]
}
