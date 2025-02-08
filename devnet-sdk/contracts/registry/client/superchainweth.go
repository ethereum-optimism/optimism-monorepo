package client

import (
	"context"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type superchainWETHBinding struct {
	contractAddress types.Address
	client          *ethclient.Client
	binding         *bindings.SuperchainWETH
}

var _ interfaces.SuperchainWETH = (*superchainWETHBinding)(nil)

func (b *superchainWETHBinding) BalanceOf(addr types.Address) types.ReadInvocation[types.Balance] {
	return &superchainWETHBalanceOfImpl{
		contract: b,
		addr:     addr,
	}
}

type superchainWETHBalanceOfImpl struct {
	contract *superchainWETHBinding
	addr     types.Address
}

func (i *superchainWETHBalanceOfImpl) Call(ctx context.Context) (types.Balance, error) {
	balance, err := i.contract.binding.BalanceOf(nil, i.addr)
	if err != nil {
		return types.Balance{}, err
	}
	return types.NewBalance(balance), nil
}
