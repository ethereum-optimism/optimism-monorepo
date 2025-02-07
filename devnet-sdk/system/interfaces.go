package system

import (
	"context"
	"math/big"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	coreTypes "github.com/ethereum/go-ethereum/core/types"
)

// System represents a complete Optimism system with L1 and L2 chains
type System interface {
	Identifier() string
	L1() Chain
	// TODO: fix the chain ID type
	L2(uint64) Chain
}

// Chain represents an Ethereum chain (L1 or L2)
type Chain interface {
	RPCURL() string
	ID() types.ChainID
	Wallet(ctx context.Context, constraints ...constraints.WalletConstraint) (types.Wallet, error)
	ContractsRegistry() interfaces.ContractsRegistry
	SupportsEIP(ctx context.Context, eip uint64) bool

	TransactionProcessor() (TransactionProcessor, error)

	GasPrice(ctx context.Context) (*big.Int, error)
	GasLimit(ctx context.Context, tx TransactionData) (uint64, error)
	PendingNonceAt(ctx context.Context, address common.Address) (uint64, error)
}

type TransactionProcessor interface {
	Sign(tx Transaction, privateKey string) (Transaction, error)
	Send(ctx context.Context, tx Transaction) error
}

// InteropSystem extends System with interoperability features
type InteropSystem interface {
	System
	InteropSet() InteropSet
}

// InteropSet provides access to L2 chains in an interop environment
type InteropSet interface {
	L2(uint64) Chain
}

type TransactionData interface {
	From() common.Address
	To() *common.Address
	Value() *big.Int
	Data() []byte
}

type Transaction interface {
	Type() uint8
	Hash() common.Hash
	TransactionData
}

// RawTransaction is an optional interface that can be implemented by a Transaction
// to provide access to the raw transaction data.
// It is currently necessary to perform processing operations (signing, sending)
// on the transaction.
type RawTransaction interface {
	Raw() *coreTypes.Transaction
}
