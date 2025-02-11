package sources

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSupervisorClient_SuperRootAtTimestamp(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		rpc := new(mockRPC)
		defer rpc.AssertExpectations(t)
		client := NewSupervisorClient(rpc)

		timestamp := hexutil.Uint64(245)

		expected := eth.SuperRootResponse{
			CrossSafeDerivedFrom: eth.BlockID{
				Hash:   common.Hash{0xaa, 0xbb, 0xcc},
				Number: 304,
			},
			Timestamp: uint64(timestamp),
			SuperRoot: eth.Bytes32{0xff},
			Chains:    nil,
		}
		rpc.On("CallContext", ctx, new(eth.SuperRootResponse),
			"supervisor_superRootAtTimestamp", []any{timestamp}).Run(func(args mock.Arguments) {
			*args[1].(*eth.SuperRootResponse) = expected
		}).Return([]error{nil})
		result, err := client.SuperRootAtTimestamp(ctx, timestamp)
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("NotFound", func(t *testing.T) {
		ctx := context.Background()
		rpc := new(mockRPC)
		defer rpc.AssertExpectations(t)
		client := NewSupervisorClient(rpc)

		timestamp := hexutil.Uint64(245)

		rpc.On("CallContext", ctx, new(eth.SuperRootResponse),
			"supervisor_superRootAtTimestamp", []any{timestamp}).Return([]error{errors.New("blah blah blah: not found")})
		_, err := client.SuperRootAtTimestamp(ctx, timestamp)
		require.ErrorIs(t, err, ethereum.NotFound)
	})
}

func TestSupervisorClient_AllSafeDerivedAt(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		rpc := new(mockRPC)
		defer rpc.AssertExpectations(t)
		client := NewSupervisorClient(rpc)

		input := eth.BlockID{Hash: common.Hash{0xdd}, Number: 48213}

		expected := map[eth.ChainID]eth.BlockID{
			eth.ChainIDFromUInt64(4): eth.BlockID{Hash: common.Hash{1, 2, 3}, Number: 304},
			eth.ChainIDFromUInt64(7): eth.BlockID{Hash: common.Hash{0xaa}, Number: 482},
		}
		rpc.On("CallContext", ctx, new(map[eth.ChainID]eth.BlockID),
			"supervisor_allSafeDerivedAt", []any{input}).Run(func(args mock.Arguments) {
			*args[1].(*map[eth.ChainID]eth.BlockID) = expected
		}).Return([]error{nil})
		result, err := client.AllSafeDerivedAt(ctx, input)
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("NotFound", func(t *testing.T) {
		ctx := context.Background()
		rpc := new(mockRPC)
		defer rpc.AssertExpectations(t)
		client := NewSupervisorClient(rpc)

		input := eth.BlockID{Hash: common.Hash{0xdd}, Number: 48213}

		rpc.On("CallContext", ctx, new(map[eth.ChainID]eth.BlockID),
			"supervisor_allSafeDerivedAt", []any{input}).Return([]error{errors.New("blah blah blah: not found")})
		_, err := client.AllSafeDerivedAt(ctx, input)
		require.ErrorIs(t, err, ethereum.NotFound)
	})
}
