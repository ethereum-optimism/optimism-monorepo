package contracts

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type contractConstructor func(chain system.Chain, address types.Address) interface{}

var contractConstructors = map[types.Address]contractConstructor{
	constants.SuperchainWETH: newSuperchainWETH,
}

func ResolveContract[T any](chain system.Chain, address types.Address) (T, error) {
	var t T
	constructor, ok := contractConstructors[address]
	if !ok {
		return t, fmt.Errorf("no constructor found for contract %s", address)
	}
	return constructor(chain, address).(T), nil
}

func MustResolveContract[T any](chain system.Chain, address types.Address) T {
	t, err := ResolveContract[T](chain, address)
	if err != nil {
		panic(err)
	}
	return t
}

type SuperchainWETH interface {
	BalanceOf(user types.Address) types.ReadInvocation[types.Balance]
}

func newSuperchainWETH(chain system.Chain, address types.Address) interface{} {
	return &superchainWETHBinding{
		chain:           chain,
		contractAddress: address,
	}
}

type superchainWETHBinding struct {
	chain           system.Chain
	contractAddress types.Address
	binding         *bindings.SuperchainWETH
	mu              sync.Mutex
}

func (b *superchainWETHBinding) getBinding() (*bindings.SuperchainWETH, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.binding != nil {
		return b.binding, nil
	}

	client, err := b.chain.Client()
	if err != nil {
		return nil, err
	}

	binding, err := bindings.NewSuperchainWETH(common.HexToAddress(string(b.contractAddress)), client)
	if err != nil {
		return nil, err
	}

	b.binding = binding
	return binding, nil
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
	binding, err := i.parent.getBinding()
	if err != nil {
		return 0, fmt.Errorf("failed to get contract binding: %w", err)
	}

	balance, err := binding.BalanceOf(nil, common.HexToAddress(string(i.user)))
	if err != nil {
		return 0, err
	}
	return types.Balance(balance.Uint64()), nil
}
