package script

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// DefaultFoundryGasLimit is set to int64.max in foundry.toml
	DefaultFoundryGasLimit = 9223372036854775807
)

type Context struct {
	ChainID      *big.Int
	Sender       common.Address
	Origin       common.Address
	FeeRecipient common.Address
	GasLimit     uint64
	BlockNum     uint64
	Timestamp    uint64
	PrevRandao   common.Hash
	BlobHashes   []common.Hash
}

var DefaultContext = Context{
	ChainID:      big.NewInt(1337),
	Sender:       addresses.DefaultSenderAddr,
	Origin:       addresses.DefaultSenderAddr,
	FeeRecipient: common.Address{},
	GasLimit:     DefaultFoundryGasLimit,
	BlockNum:     0,
	Timestamp:    0,
	PrevRandao:   common.Hash{},
	BlobHashes:   []common.Hash{},
}
