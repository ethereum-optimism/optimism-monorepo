package contracts

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Registry provides access to all supported contract instances
type Registry interface {
	SuperchainWETH(address types.Address) SuperchainWETH
}

// ClientRegistry is a Registry implementation that uses an ethclient.Client
type ClientRegistry struct {
	client *ethclient.Client
}

// NewClientRegistry creates a new Registry that uses the provided client
func NewClientRegistry(client *ethclient.Client) interfaces.ContractsRegistry {
	return &ClientRegistry{client: client}
}

func (r *ClientRegistry) SuperchainWETH(address types.Address) (interfaces.SuperchainWETH, error) {
	binding, err := bindings.NewSuperchainWETH(common.HexToAddress(string(address)), r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SuperchainWETH binding: %w", err)
	}
	return &superchainWETHBinding{
		contractAddress: address,
		client:          r.client,
		binding:         binding,
	}, nil
}

// EmptyRegistry represents a registry that returns not found errors for all contract accesses
type EmptyRegistry struct{}

func (r *EmptyRegistry) SuperchainWETH(address types.Address) (interfaces.SuperchainWETH, error) {
	return nil, &interfaces.ErrContractNotFound{
		ContractType: "SuperchainWETH",
		Address:      address,
	}
}

type SuperchainWETH interface {
	BalanceOf(user types.Address) types.ReadInvocation[types.Balance]
}

type superchainWETHBinding struct {
	contractAddress types.Address
	client          *ethclient.Client
	binding         *bindings.SuperchainWETH
	mu              sync.Mutex
}

func (b *superchainWETHBinding) BalanceOf(user types.Address) types.ReadInvocation[types.Balance] {
	return &balanceImpl{
		parent: b,
		user:   user,
	}
}

type balanceImpl struct {
	parent *superchainWETHBinding
	user   types.Address
}

func (i *balanceImpl) Call(ctx context.Context) (types.Balance, error) {
	balance, err := i.parent.binding.BalanceOf(nil, common.HexToAddress(string(i.user)))
	if err != nil {
		return types.Balance{}, err
	}
	return types.NewBalance(balance), nil
}
