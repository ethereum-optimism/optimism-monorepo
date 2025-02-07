package client

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ClientRegistry is a Registry implementation that uses an ethclient.Client
type ClientRegistry struct {
	Client *ethclient.Client
}

var _ interfaces.ContractsRegistry = (*ClientRegistry)(nil)

func (r *ClientRegistry) SuperchainWETH(address types.Address) (interfaces.SuperchainWETH, error) {
	binding, err := bindings.NewSuperchainWETH(address, r.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SuperchainWETH binding: %w", err)
	}
	return &superchainWETHBinding{
		contractAddress: address,
		client:          r.Client,
		binding:         binding,
	}, nil
}
