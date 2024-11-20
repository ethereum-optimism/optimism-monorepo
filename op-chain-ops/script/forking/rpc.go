package forking

import (
	"context"
	"fmt"
	"time"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/op-service/retry"
)

type RPCClient interface {
	CallContext(ctx context.Context, result any, method string, args ...any) error
}

type RPCSource struct {
	stateRoot common.Hash
	blockHash common.Hash

	maxAttempts int
	timeout     time.Duration
	strategy    retry.Strategy

	ctx    context.Context
	cancel context.CancelFunc

	client     RPCClient
	urlOrAlias string
}

var _ ForkSource = (*RPCSource)(nil)

func RPCSourceByNumber(urlOrAlias string, cl RPCClient, num uint64) (*RPCSource, error) {
	src := newRPCSource(urlOrAlias, cl)
	err := src.init(hexutil.Uint64(num))
	return src, err
}

func RPCSourceByHash(urlOrAlias string, cl RPCClient, h common.Hash) (*RPCSource, error) {
	src := newRPCSource(urlOrAlias, cl)
	err := src.init(h)
	return src, err
}

func newRPCSource(urlOrAlias string, cl RPCClient) *RPCSource {
	ctx, cancel := context.WithCancel(context.Background())
	return &RPCSource{
		maxAttempts: 10,
		timeout:     time.Second * 10,
		strategy:    retry.Exponential(),
		ctx:         ctx,
		cancel:      cancel,
		client:      cl,
		urlOrAlias:  urlOrAlias,
	}
}

type Header struct {
	StateRoot common.Hash `json:"stateRoot"`
	BlockHash common.Hash `json:"hash"`
}

func (r *RPCSource) init(id any) error {
	head, err := retry.Do[*Header](r.ctx, r.maxAttempts, r.strategy, func() (*Header, error) {
		var result *Header
		err := r.client.CallContext(r.ctx, &result, "eth_getBlockByNumber", id, false)
		if err == nil && result == nil {
			err = ethereum.NotFound
		}
		return result, err
	})
	if err != nil {
		return fmt.Errorf("failed to initialize RPC fork source around block %v: %w", id, err)
	}
	r.blockHash = head.BlockHash
	r.stateRoot = head.StateRoot
	return nil
}

func (c *RPCSource) URLOrAlias() string {
	return c.urlOrAlias
}

func (r *RPCSource) BlockHash() common.Hash {
	return r.blockHash
}

func (r *RPCSource) StateRoot() common.Hash {
	return r.stateRoot
}

func (r *RPCSource) Nonce(addr common.Address) (uint64, error) {
	return retry.Do[uint64](r.ctx, r.maxAttempts, r.strategy, func() (uint64, error) {
		ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		var result hexutil.Uint64
		err := r.client.CallContext(ctx, &result, "eth_getTransactionCount", addr, r.blockHash)
		return uint64(result), err
	})
}

func (r *RPCSource) Balance(addr common.Address) (*uint256.Int, error) {
	return retry.Do[*uint256.Int](r.ctx, r.maxAttempts, r.strategy, func() (*uint256.Int, error) {
		ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		var result hexutil.U256
		err := r.client.CallContext(ctx, &result, "eth_getBalance", addr, r.blockHash)
		return (*uint256.Int)(&result), err
	})
}

func (r *RPCSource) StorageAt(addr common.Address, key common.Hash) (common.Hash, error) {
	return retry.Do[common.Hash](r.ctx, r.maxAttempts, r.strategy, func() (common.Hash, error) {
		ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		var result common.Hash
		err := r.client.CallContext(ctx, &result, "eth_getStorageAt", addr, key, r.blockHash)
		return result, err
	})
}

func (r *RPCSource) Code(addr common.Address) ([]byte, error) {
	return retry.Do[[]byte](r.ctx, r.maxAttempts, r.strategy, func() ([]byte, error) {
		ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		var result hexutil.Bytes
		err := r.client.CallContext(ctx, &result, "eth_getCode", addr, r.blockHash)
		return result, err
	})
}

// Close stops any ongoing RPC requests by cancelling the RPC context
func (r *RPCSource) Close() {
	r.cancel()
}
