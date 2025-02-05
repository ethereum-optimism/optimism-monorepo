package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

var ExecutionMinerRPCNamespace = "miner"

// ExecutionMinerProxyBackend implements an execution rpc proxy with a leadership check before each call.
type ExecutionMinerProxyBackend struct {
	log    log.Logger
	con    conductor
	client *ethclient.Client
}

var _ ExecutionMinerProxyAPI = (*ExecutionMinerProxyBackend)(nil)

func NewExecutionMinerProxyBackend(log log.Logger, con conductor, client *ethclient.Client) *ExecutionMinerProxyBackend {
	return &ExecutionMinerProxyBackend{
		log:    log,
		con:    con,
		client: client,
	}
}

func (api *ExecutionMinerProxyBackend) SetMaxDASize(ctx context.Context, maxTxSize hexutil.Big, maxBlockSize hexutil.Big) bool {
	var result bool
	if !api.con.Leader(ctx) {
		return false
	}
	err := api.client.Client().Call(&result, "miner_setMaxDASize", maxTxSize, maxBlockSize)
	if err != nil {
		return false
	}
	return result
}
