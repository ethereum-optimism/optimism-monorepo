package empty

import (
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

// EmptyRegistry represents a registry that returns not found errors for all contract accesses
type EmptyRegistry struct{}

var _ interfaces.ContractsRegistry = (*EmptyRegistry)(nil)

func (r *EmptyRegistry) SuperchainWETH(address types.Address) (interfaces.SuperchainWETH, error) {
	return nil, &interfaces.ErrContractNotFound{
		ContractType: "SuperchainWETH",
		Address:      address,
	}
}

func (r *EmptyRegistry) EventLogger(address types.Address) (interfaces.EventLogger, error) {
	return nil, &interfaces.ErrContractNotFound{
		ContractType: "EventLogger",
		Address:      address,
	}
}
