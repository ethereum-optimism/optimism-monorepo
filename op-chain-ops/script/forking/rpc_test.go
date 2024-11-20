package forking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/retry"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRPCClient implements RPCClient interface for testing
type MockRPCClient struct {
	mock.Mock
}

func (m *MockRPCClient) CallContext(ctx context.Context, result any, method string, args ...any) error {
	return m.Called(ctx, result, method, args).Error(0)
}

func TestRPCSourceInitialization(t *testing.T) {
	mockClient := new(MockRPCClient)
	expectedStateRoot := common.HexToHash("0x1234")
	expectedBlockHash := common.HexToHash("0x5678")

	t.Run("initialization by block number", func(t *testing.T) {
		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("**forking.Header"),
			"eth_getBlockByNumber", []any{hexutil.Uint64(123), false}).
			Run(func(args mock.Arguments) {
				result := args.Get(1).(**Header)
				*result = &Header{
					StateRoot: expectedStateRoot,
					BlockHash: expectedBlockHash,
				}
			}).
			Return(nil).Once()

		source, err := RPCSourceByNumber("test_url", mockClient, 123)
		require.NoError(t, err)
		require.Equal(t, expectedStateRoot, source.StateRoot())
		require.Equal(t, expectedBlockHash, source.BlockHash())
	})

	t.Run("initialization by block hash", func(t *testing.T) {
		blockHash := common.HexToHash("0xabcd")
		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("**forking.Header"),
			"eth_getBlockByNumber", []any{blockHash, false}).
			Run(func(args mock.Arguments) {
				result := args.Get(1).(**Header)
				*result = &Header{
					StateRoot: expectedStateRoot,
					BlockHash: expectedBlockHash,
				}
			}).
			Return(nil).Once()

		source, err := RPCSourceByHash("test_url", mockClient, blockHash)
		require.NoError(t, err)
		require.Equal(t, expectedStateRoot, source.StateRoot())
		require.Equal(t, expectedBlockHash, source.BlockHash())
	})

	t.Run("initialization failure", func(t *testing.T) {
		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("**forking.Header"),
			"eth_getBlockByNumber", []any{hexutil.Uint64(999), false}).
			Return(ethereum.NotFound).Times(2)

		src := newRPCSource("test_url", mockClient)
		strategy := retry.Exponential()
		strategy.(*retry.ExponentialStrategy).Max = 100 * time.Millisecond
		src.strategy = strategy
		src.maxAttempts = 2
		require.Error(t, src.init(hexutil.Uint64(999)))
	})
}

func TestRPCSourceDataRetrieval(t *testing.T) {
	mockClient := new(MockRPCClient)
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	blockHash := common.HexToHash("0xabcd")

	source := &RPCSource{
		blockHash:   blockHash,
		client:      mockClient,
		ctx:         context.Background(),
		strategy:    retry.Exponential(),
		maxAttempts: 10,
		timeout:     time.Second * 10,
	}

	t.Run("get nonce", func(t *testing.T) {
		expectedNonce := uint64(5)
		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("*hexutil.Uint64"),
			"eth_getTransactionCount", []any{testAddr, blockHash}).
			Run(func(args mock.Arguments) {
				result := args.Get(1).(*hexutil.Uint64)
				*result = hexutil.Uint64(expectedNonce)
			}).
			Return(nil).Once()

		nonce, err := source.Nonce(testAddr)
		require.NoError(t, err)
		require.Equal(t, expectedNonce, nonce)
	})

	t.Run("get balance", func(t *testing.T) {
		expectedBalance := uint256.NewInt(1000)
		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("*hexutil.U256"),
			"eth_getBalance", []any{testAddr, blockHash}).
			Run(func(args mock.Arguments) {
				result := args.Get(1).(*hexutil.U256)
				*(*uint256.Int)(result) = *expectedBalance
			}).
			Return(nil).Once()

		balance, err := source.Balance(testAddr)
		require.NoError(t, err)
		require.Equal(t, expectedBalance, balance)
	})

	t.Run("get storage", func(t *testing.T) {
		storageKey := common.HexToHash("0x1234")
		expectedValue := common.HexToHash("0x5678")
		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("*common.Hash"),
			"eth_getStorageAt", []any{testAddr, storageKey, blockHash}).
			Run(func(args mock.Arguments) {
				result := args.Get(1).(*common.Hash)
				*result = expectedValue
			}).
			Return(nil).Once()

		value, err := source.StorageAt(testAddr, storageKey)
		require.NoError(t, err)
		require.Equal(t, expectedValue, value)
	})

	t.Run("get code", func(t *testing.T) {
		expectedCode := []byte{1, 2, 3, 4}
		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("*hexutil.Bytes"),
			"eth_getCode", []any{testAddr, blockHash}).
			Run(func(args mock.Arguments) {
				result := args.Get(1).(*hexutil.Bytes)
				*result = expectedCode
			}).
			Return(nil).Once()

		code, err := source.Code(testAddr)
		require.NoError(t, err)
		require.Equal(t, expectedCode, code)
	})
}

func TestRPCSourceRetry(t *testing.T) {
	mockClient := new(MockRPCClient)
	testAddr := common.HexToAddress("0x1234")
	blockHash := common.HexToHash("0xabcd")
	strategy := retry.Exponential()
	strategy.(*retry.ExponentialStrategy).Max = 100 * time.Millisecond

	source := &RPCSource{
		blockHash:   blockHash,
		client:      mockClient,
		ctx:         context.Background(),
		strategy:    strategy,
		maxAttempts: 3,
		timeout:     time.Second * 10,
	}

	t.Run("retry on temporary error", func(t *testing.T) {
		tempError := errors.New("temporary network error")

		// Fail twice, succeed on third attempt
		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("*hexutil.Uint64"),
			"eth_getTransactionCount", []any{testAddr, blockHash}).
			Return(tempError).Times(2)

		mockClient.On("CallContext", mock.Anything, mock.AnythingOfType("*hexutil.Uint64"),
			"eth_getTransactionCount", []any{testAddr, blockHash}).
			Run(func(args mock.Arguments) {
				result := args.Get(1).(*hexutil.Uint64)
				*result = hexutil.Uint64(5)
			}).
			Return(nil).Once()

		nonce, err := source.Nonce(testAddr)
		require.NoError(t, err)
		require.Equal(t, uint64(5), nonce)
	})
}

func TestRPCSourceClose(t *testing.T) {
	mockClient := new(MockRPCClient)
	source := newRPCSource("test_url", mockClient)

	// Verify context is active before close
	require.NoError(t, source.ctx.Err())

	source.Close()

	// Verify context is cancelled after close
	require.Error(t, source.ctx.Err())
	require.Equal(t, context.Canceled, source.ctx.Err())
}
