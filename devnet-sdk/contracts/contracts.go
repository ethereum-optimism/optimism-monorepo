package contracts

import (
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/registry/client"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/registry/empty"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Registry provides access to all supported contract instances
type Registry interface {
	SuperchainWETH(address types.Address) SuperchainWETH
}

type SuperchainWETH interface {
	BalanceOf(user types.Address) types.ReadInvocation[types.Balance]
}

// NewClientRegistry creates a new Registry that uses the provided client
func NewClientRegistry(c *ethclient.Client) interfaces.ContractsRegistry {
	return &client.ClientRegistry{Client: c}
}

func NewEmptyRegistry() interfaces.ContractsRegistry {
	return &empty.EmptyRegistry{}
}
