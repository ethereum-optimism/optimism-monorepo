package client

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/metrics"
)

type Client interface {
	Close()
	RPC() RPC
	ChainID(ctx context.Context) (*big.Int, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	BlockNumber(ctx context.Context) (uint64, error)
	PeerCount(ctx context.Context) (uint64, error)
	HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error)
	TransactionSender(ctx context.Context, tx *types.Transaction, block common.Hash, index uint) (common.Address, error)
	TransactionCount(ctx context.Context, blockHash common.Hash) (uint, error)
	TransactionInBlock(ctx context.Context, blockHash common.Hash, index uint) (*types.Transaction, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	SyncProgress(ctx context.Context) (*ethereum.SyncProgress, error)
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
	NetworkID(ctx context.Context) (*big.Int, error)
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error)
	CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
	SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error)
	PendingBalanceAt(ctx context.Context, account common.Address) (*big.Int, error)
	PendingStorageAt(ctx context.Context, account common.Address, key common.Hash) ([]byte, error)
	PendingCodeAt(ctx context.Context, account common.Address) ([]byte, error)
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	PendingTransactionCount(ctx context.Context) (uint, error)
	CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
	CallContractAtHash(ctx context.Context, msg ethereum.CallMsg, blockHash common.Hash) ([]byte, error)
	PendingCallContract(ctx context.Context, msg ethereum.CallMsg) ([]byte, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
	EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

// InstrumentedClient is an Ethereum client that tracks
// Prometheus metrics for each call.
type InstrumentedClient struct {
	c   *ethclient.Client
	m   *metrics.RPCClientMetrics
	rpc RPC
}

// NewInstrumentedClient creates a new instrumented client. It takes
// a concrete *rpc.Client to prevent people from passing in an already
// instrumented client.
func NewInstrumentedClient(c *rpc.Client, m *metrics.RPCClientMetrics) *InstrumentedClient {
	return &InstrumentedClient{
		c:   ethclient.NewClient(c),
		m:   m,
		rpc: NewInstrumentedRPC(NewBaseRPCClient(c), m),
	}
}

func (ic *InstrumentedClient) Close() {
	ic.c.Close()
}

func (ic *InstrumentedClient) RPC() RPC {
	return ic.rpc
}

func (ic *InstrumentedClient) ChainID(ctx context.Context) (*big.Int, error) {
	return instrument2(ic.m, "eth_chainId", func() (*big.Int, error) {
		return ic.c.ChainID(ctx)
	})
}

func (ic *InstrumentedClient) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return instrument2(ic.m, "eth_getBlockByHash", func() (*types.Block, error) {
		return ic.c.BlockByHash(ctx, hash)
	})
}

func (ic *InstrumentedClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return instrument2(ic.m, "eth_getBlockByNumber", func() (*types.Block, error) {
		return ic.c.BlockByNumber(ctx, number)
	})
}

func (ic *InstrumentedClient) BlockNumber(ctx context.Context) (uint64, error) {
	return instrument2(ic.m, "eth_blockNumber", func() (uint64, error) {
		return ic.c.BlockNumber(ctx)
	})
}

func (ic *InstrumentedClient) PeerCount(ctx context.Context) (uint64, error) {
	return instrument2(ic.m, "net_peerCount", func() (uint64, error) {
		return ic.c.PeerCount(ctx)
	})
}

func (ic *InstrumentedClient) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return instrument2(ic.m, "eth_getHeaderByHash", func() (*types.Header, error) {
		return ic.c.HeaderByHash(ctx, hash)
	})
}

func (ic *InstrumentedClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	return instrument2(ic.m, "eth_getHeaderByNumber", func() (*types.Header, error) {
		return ic.c.HeaderByNumber(ctx, number)
	})
}

func (ic *InstrumentedClient) TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	record := ic.m.RecordRPCClientRequest("eth_getTransactionByHash")
	tx, isPending, err := ic.c.TransactionByHash(ctx, hash)
	record(err)
	return tx, isPending, err
}

func (ic *InstrumentedClient) TransactionSender(ctx context.Context, tx *types.Transaction, block common.Hash, index uint) (common.Address, error) {
	return ic.c.TransactionSender(ctx, tx, block, index)
}

func (ic *InstrumentedClient) TransactionCount(ctx context.Context, blockHash common.Hash) (uint, error) {
	return instrument2(ic.m, "eth_getTransactionCount", func() (uint, error) {
		return ic.c.TransactionCount(ctx, blockHash)
	})
}

func (ic *InstrumentedClient) TransactionInBlock(ctx context.Context, blockHash common.Hash, index uint) (*types.Transaction, error) {
	return instrument2(ic.m, "eth_getTransactionByBlockHashAndIndex", func() (*types.Transaction, error) {
		return ic.c.TransactionInBlock(ctx, blockHash, index)
	})
}

func (ic *InstrumentedClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return instrument2(ic.m, "eth_getTransactionReceipt", func() (*types.Receipt, error) {
		return ic.c.TransactionReceipt(ctx, txHash)
	})
}

func (ic *InstrumentedClient) SyncProgress(ctx context.Context) (*ethereum.SyncProgress, error) {
	return instrument2(ic.m, "eth_syncing", func() (*ethereum.SyncProgress, error) {
		return ic.c.SyncProgress(ctx)
	})
}

func (ic *InstrumentedClient) SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error) {
	return ic.c.SubscribeNewHead(ctx, ch)
}

func (ic *InstrumentedClient) NetworkID(ctx context.Context) (*big.Int, error) {
	return instrument2(ic.m, "net_version", func() (*big.Int, error) {
		return ic.c.NetworkID(ctx)
	})
}

func (ic *InstrumentedClient) BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return instrument2(ic.m, "eth_getBalance", func() (*big.Int, error) {
		return ic.c.BalanceAt(ctx, account, blockNumber)
	})
}

func (ic *InstrumentedClient) StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error) {
	return instrument2(ic.m, "eth_getStorageAt", func() ([]byte, error) {
		return ic.c.StorageAt(ctx, account, key, blockNumber)
	})
}

func (ic *InstrumentedClient) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	return instrument2(ic.m, "eth_getCode", func() ([]byte, error) {
		return ic.c.CodeAt(ctx, account, blockNumber)
	})
}

func (ic *InstrumentedClient) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return instrument2(ic.m, "eth_getTransactionCount", func() (uint64, error) {
		return ic.c.NonceAt(ctx, account, blockNumber)
	})
}

func (ic *InstrumentedClient) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	return instrument2(ic.m, "eth_getLogs", func() ([]types.Log, error) {
		return ic.c.FilterLogs(ctx, q)
	})
}

func (ic *InstrumentedClient) SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error) {
	return ic.c.SubscribeFilterLogs(ctx, q, ch)
}

func (ic *InstrumentedClient) PendingBalanceAt(ctx context.Context, account common.Address) (*big.Int, error) {
	return instrument2(ic.m, "eth_getBalance", func() (*big.Int, error) {
		return ic.c.PendingBalanceAt(ctx, account)
	})
}

func (ic *InstrumentedClient) PendingStorageAt(ctx context.Context, account common.Address, key common.Hash) ([]byte, error) {
	return instrument2(ic.m, "eth_getStorageAt", func() ([]byte, error) {
		return ic.c.PendingStorageAt(ctx, account, key)
	})
}

func (ic *InstrumentedClient) PendingCodeAt(ctx context.Context, account common.Address) ([]byte, error) {
	return instrument2(ic.m, "eth_getCode", func() ([]byte, error) {
		return ic.c.PendingCodeAt(ctx, account)
	})
}

func (ic *InstrumentedClient) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return instrument2(ic.m, "eth_getTransactionCount", func() (uint64, error) {
		return ic.c.PendingNonceAt(ctx, account)
	})
}

func (ic *InstrumentedClient) PendingTransactionCount(ctx context.Context) (uint, error) {
	return instrument2(ic.m, "eth_getBlockTransactionCountByNumber", func() (uint, error) {
		return ic.c.PendingTransactionCount(ctx)
	})
}

func (ic *InstrumentedClient) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return instrument2(ic.m, "eth_call", func() ([]byte, error) {
		return ic.c.CallContract(ctx, msg, blockNumber)
	})
}

func (ic *InstrumentedClient) CallContractAtHash(ctx context.Context, msg ethereum.CallMsg, blockHash common.Hash) ([]byte, error) {
	return instrument2(ic.m, "eth_call", func() ([]byte, error) {
		return ic.c.CallContractAtHash(ctx, msg, blockHash)
	})
}

func (ic *InstrumentedClient) PendingCallContract(ctx context.Context, msg ethereum.CallMsg) ([]byte, error) {
	return instrument2(ic.m, "eth_call", func() ([]byte, error) {
		return ic.c.PendingCallContract(ctx, msg)
	})
}

func (ic *InstrumentedClient) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return instrument2(ic.m, "eth_gasPrice", func() (*big.Int, error) {
		return ic.c.SuggestGasPrice(ctx)
	})
}

func (ic *InstrumentedClient) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return instrument2(ic.m, "eth_maxPriorityFeePerGas", func() (*big.Int, error) {
		return ic.c.SuggestGasPrice(ctx)
	})
}

func (ic *InstrumentedClient) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return instrument2(ic.m, "eth_estimateGas", func() (uint64, error) {
		return ic.c.EstimateGas(ctx, msg)
	})
}

func (ic *InstrumentedClient) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return instrument1(ic.m, "eth_sendRawTransaction", func() error {
		return ic.c.SendTransaction(ctx, tx)
	})
}

func instrument1(m metrics.RPCClientMetricer, name string, cb func() error) error {
	record := m.RecordRPCClientRequest(name)
	err := cb()
	record(err)
	return err
}

func instrument2[O any](m metrics.RPCClientMetricer, name string, cb func() (O, error)) (O, error) {
	record := m.RecordRPCClientRequest(name)
	res, err := cb()
	record(err)
	return res, err
}
