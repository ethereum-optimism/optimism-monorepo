package prefetcher

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestNewL2Sources(t *testing.T) {
	t.Run("NoSources", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelInfo)
		_, err := NewRetryingL2Sources(context.Background(), logger, nil, nil, nil)
		require.ErrorIs(t, err, ErrNoSources)
	})

	t.Run("SingleSource", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		config, l2Rpc, experimentalRpc := chain(4)
		src, err := NewRetryingL2Sources(context.Background(), logger,
			[]*rollup.Config{config},
			[]client.RPC{l2Rpc},
			[]client.RPC{experimentalRpc})
		require.NoError(t, err)
		require.Len(t, src.Sources, 1)
		require.True(t, src.Sources[eth.ChainIDFromUInt64(4)].ExperimentalEnabled())
	})

	t.Run("MultipleSources", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		config1, l2Rpc1, experimentalRpc1 := chain(1)
		config2, l2Rpc2, experimentalRpc2 := chain(2)
		src, err := NewRetryingL2Sources(context.Background(), logger,
			[]*rollup.Config{config1, config2},
			[]client.RPC{l2Rpc1, l2Rpc2},
			[]client.RPC{experimentalRpc1, experimentalRpc2})
		require.NoError(t, err)
		require.Len(t, src.Sources, 2)
		require.True(t, src.Sources[eth.ChainIDFromUInt64(1)].ExperimentalEnabled())
		require.True(t, src.Sources[eth.ChainIDFromUInt64(2)].ExperimentalEnabled())
	})

	t.Run("ExperimentalRPCsAreOptional", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		config1, l2Rpc1, _ := chain(1)
		config2, l2Rpc2, experimentalRpc2 := chain(2)
		src, err := NewRetryingL2Sources(context.Background(), logger,
			[]*rollup.Config{config1, config2},
			[]client.RPC{l2Rpc1, l2Rpc2},
			[]client.RPC{experimentalRpc2})
		require.NoError(t, err)
		require.Len(t, src.Sources, 2)
		require.Same(t, src.Sources[eth.ChainIDFromUInt64(1)].RollupConfig(), config1)
		require.False(t, src.Sources[eth.ChainIDFromUInt64(1)].ExperimentalEnabled())

		require.Same(t, src.Sources[eth.ChainIDFromUInt64(2)].RollupConfig(), config2)
		require.True(t, src.Sources[eth.ChainIDFromUInt64(2)].ExperimentalEnabled())
	})

	t.Run("RollupMissingL2URL", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		config1, _, _ := chain(1)
		config2, l2Rpc2, experimentalRpc2 := chain(2)
		_, err := NewRetryingL2Sources(context.Background(), logger,
			[]*rollup.Config{config1, config2},
			[]client.RPC{l2Rpc2},
			[]client.RPC{experimentalRpc2})
		require.ErrorIs(t, err, ErrNoL2ForRollup)
	})

	t.Run("L2URLWithoutConfig", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		_, l2Rpc1, _ := chain(1)
		config2, l2Rpc2, experimentalRpc2 := chain(2)
		_, err := NewRetryingL2Sources(context.Background(), logger,
			[]*rollup.Config{config2},
			[]client.RPC{l2Rpc1, l2Rpc2},
			[]client.RPC{experimentalRpc2})
		require.ErrorIs(t, err, ErrNoRollupForL2)
	})

	t.Run("DuplicateL2URLsForSameChain", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		config1, l2Rpc1, _ := chain(1)
		_, l2Rpc2, _ := chain(1)
		_, err := NewRetryingL2Sources(context.Background(), logger,
			[]*rollup.Config{config1},
			[]client.RPC{l2Rpc1, l2Rpc2},
			nil)
		require.ErrorIs(t, err, ErrDuplicateL2URLs)
	})

	t.Run("ExperimentalURLWithoutConfig", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		_, _, experimentalRpc1 := chain(1)
		config2, l2Rpc2, experimentalRpc2 := chain(2)
		_, err := NewRetryingL2Sources(context.Background(), logger,
			[]*rollup.Config{config2},
			[]client.RPC{l2Rpc2},
			[]client.RPC{experimentalRpc1, experimentalRpc2})
		require.ErrorIs(t, err, ErrNoRollupForExperimental)
	})

	t.Run("DuplicateExperimentalURLsForSameChain", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelDebug)
		config1, l2RPC, experimentalRpc1 := chain(1)
		_, _, experimentalRpc2 := chain(1)
		_, err := NewRetryingL2Sources(context.Background(), logger,
			[]*rollup.Config{config1},
			[]client.RPC{l2RPC},
			[]client.RPC{experimentalRpc1, experimentalRpc2})
		require.ErrorIs(t, err, ErrDuplicateExperimentsURLs)
	})
}

func chain(id uint64) (*rollup.Config, client.RPC, client.RPC) {
	chainID := new(big.Int).SetUint64(id)
	return &rollup.Config{L2ChainID: chainID}, &chainIDRPC{id: chainID}, &chainIDRPC{id: chainID}
}

type chainIDRPC struct {
	id *big.Int
}

func (c *chainIDRPC) Close() {
	panic("implement me")
}

func (c *chainIDRPC) CallContext(ctx context.Context, result any, method string, args ...any) error {
	if method != "eth_chainId" {
		return fmt.Errorf("invalid method: %s", method)
	}
	resultOut := result.(*hexutil.Big)
	*resultOut = (hexutil.Big)(*c.id)
	return nil
}

func (c *chainIDRPC) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	panic("implement me")
}

func (c *chainIDRPC) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	panic("implement me")
}
