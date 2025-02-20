package dsl

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

type DSLUser struct {
	t     helpers.Testing
	index uint64
	keys  devkeys.Keys
}

func (u *DSLUser) TransactOpts(chain *Chain) (*bind.TransactOpts, common.Address) {
	privKey, err := u.keys.Secret(devkeys.ChainUserKeys(chain.ChainID.ToBig())(u.index))
	require.NoError(u.t, err)
	opts, err := bind.NewKeyedTransactorWithChainID(privKey, chain.ChainID.ToBig())
	require.NoError(u.t, err)
	opts.GasTipCap = big.NewInt(params.GWei)

	return opts, crypto.PubkeyToAddress(privKey.PublicKey)
}
