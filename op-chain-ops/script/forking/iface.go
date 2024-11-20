package forking

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type VMStateDB interface {
	vm.StateDB
	Finalise(deleteEmptyObjects bool)
	// SetBalance sets the balance of an account. Not part of the geth VM StateDB interface (add/sub balance are).
	SetBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason)
}

// ForkID is an identifier of a fork
type ForkID uint256.Int

func ForkIDFromBig(b *big.Int) ForkID {
	return ForkID(*uint256.MustFromBig(b))
}

// U256 returns a uint256 copy of the fork ID, for usage inside the EVM.
func (id *ForkID) U256() *uint256.Int {
	return new(uint256.Int).Set((*uint256.Int)(id))
}

func (id ForkID) String() string {
	return (*uint256.Int)(&id).String()
}

// ForkSource is a read-only source for ethereum state,
// that can be used to fork a ForkableState.
type ForkSource interface {
	// URLOrAlias returns the URL or alias that the fork uses. This is not unique to a single fork.
	URLOrAlias() string
	// StateRoot returns the accounts-trie root of the committed-to state.
	// This root must never change.
	StateRoot() common.Hash
	// Nonce returns 0, without error, if the account does not exist.
	Nonce(addr common.Address) (uint64, error)
	// Balance returns 0, without error, if the account does not exist.
	Balance(addr common.Address) (*uint256.Int, error)
	// StorageAt returns a zeroed hash, without error, if the storage does not exist.
	StorageAt(addr common.Address, key common.Hash) (common.Hash, error)
	// Code returns an empty byte slice, without error, if no code exists.
	Code(addr common.Address) ([]byte, error)
}
