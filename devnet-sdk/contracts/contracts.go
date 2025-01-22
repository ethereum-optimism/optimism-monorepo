package contracts

import (
	"context"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func ResolveContract[T any](chain system.Chain, address types.Address) T {
	var t T
	return t
}

type SuperchainWETH interface {
	BalanceOf(user types.Address) types.ReadInvocation[types.Balance]
}

type superchainWETHBinding struct {
	chain           system.Chain
	contractAddress types.Address
}

var _ SuperchainWETH = (*superchainWETHBinding)(nil)

type balanceImpl struct {
	chain           system.Chain
	contractAddress types.Address
	user            types.Address
}

func (i *balanceImpl) Call(ctx context.Context) types.Balance {
	conn, err := ethclient.Dial(i.chain.RPCURL())
	if err != nil {
		return 0
	}
	sc, err := bindings.NewSuperchainWETH(common.HexToAddress(string(i.contractAddress)), conn)
	if err != nil {
		return 0
	}
	balance, err := sc.BalanceOf(nil, common.HexToAddress(string(i.user)))
	if err != nil {
		return 0
	}
	return types.Balance(balance.Uint64())
}

func (b *superchainWETHBinding) BalanceOf(user types.Address) types.ReadInvocation[types.Balance] {
	return &balanceImpl{
		chain:           b.chain,
		contractAddress: b.contractAddress,
		user:            user,
	}
}
