package system

import (
	"context"
	"math/big"

	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	coreTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type genSystem[T Chain] interface {
	Identifier() string
	L1() T
	L2s() []T
}

// System represents a complete Optimism system with L1 and L2 chains
type System = genSystem[Chain]

type LowLevelSystem = genSystem[LowLevelChain]

// Chain represents an Ethereum chain (L1 or L2)
type Chain interface {
	ID() types.ChainID
	Wallets(ctx context.Context) ([]Wallet, error)
	ContractsRegistry() interfaces.ContractsRegistry
	SupportsEIP(ctx context.Context, eip uint64) bool

	GasPrice(ctx context.Context) (*big.Int, error)
	GasLimit(ctx context.Context, tx TransactionData) (uint64, error)
	PendingNonceAt(ctx context.Context, address common.Address) (uint64, error)
}

// LowLevelChain is a Chain that gives direct access to the low level RPC client.
type LowLevelChain interface {
	Chain
	RPCURL() string
	Client() (*ethclient.Client, error)
}

// Wallet represents a chain wallet.
// In particular it can process transactions.
type Wallet interface {
	PrivateKey() types.Key
	Address() types.Address
	SendETH(to types.Address, amount types.Balance) types.WriteInvocation[any]
	Balance() types.Balance
	Nonce() uint64

	TransactionProcessor
}

// TransactionProcessor is a helper interface for signing and sending transactions.
type TransactionProcessor interface {
	Sign(tx Transaction) (Transaction, error)
	Send(ctx context.Context, tx Transaction) error
}

// Transaction interfaces:

// TransactionData is the input for a transaction creation.
type TransactionData interface {
	From() common.Address
	To() *common.Address
	Value() *big.Int
	Data() []byte
}

// Transaction is the instantiated transaction object.
type Transaction interface {
	Type() uint8
	Hash() common.Hash
	TransactionData
}

// RawTransaction is an optional interface that can be implemented by a Transaction
// to provide access to the raw transaction data.
// It is currently necessary to perform processing operations (signing, sending)
// on the transaction. We would need to do better here.
type RawTransaction interface {
	Raw() *coreTypes.Transaction
}

// Specialized interop interfaces:

// InteropSystem extends System with interoperability features
type InteropSystem interface {
	System
	InteropSet() InteropSet
}

// InteropSet provides access to L2 chains in an interop environment
type InteropSet interface {
	L2s() []Chain
}
