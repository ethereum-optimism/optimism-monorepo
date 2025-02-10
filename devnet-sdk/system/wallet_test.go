package system

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// testWallet is a minimal wallet implementation for testing balance functionality
type testWallet struct {
	privateKey types.Key
	address    types.Address
	chain      *mockChainForBalance // Use concrete type to access mock client directly
}

func (w *testWallet) Balance() types.Balance {
	// Use the mock client directly instead of going through getClient()
	balance, err := w.chain.client.BalanceAt(context.Background(), w.address, nil)
	if err != nil {
		return types.NewBalance(new(big.Int))
	}

	return types.NewBalance(balance)
}

// mockEthClient implements a mock ethereum client for testing
type mockEthClient struct {
	mock.Mock
}

func (m *mockEthClient) BalanceAt(ctx context.Context, account types.Address, blockNumber *big.Int) (*big.Int, error) {
	args := m.Called(ctx, account, blockNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

// mockChainForBalance implements just enough of the chain interface for balance testing
type mockChainForBalance struct {
	mock.Mock
	client *mockEthClient
}

func TestWalletBalance(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mockChainForBalance)
		expectedValue *big.Int
	}{
		{
			name: "successful balance fetch",
			setupMock: func(m *mockChainForBalance) {
				balance := big.NewInt(1000000000000000000) // 1 ETH
				m.client.On("BalanceAt", mock.Anything, mock.Anything, mock.Anything).Return(balance, nil)
			},
			expectedValue: big.NewInt(1000000000000000000),
		},
		{
			name: "balance fetch error returns zero",
			setupMock: func(m *mockChainForBalance) {
				m.client.On("BalanceAt", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expectedValue: new(big.Int),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockChain := &mockChainForBalance{
				client: new(mockEthClient),
			}
			tt.setupMock(mockChain)

			w := &testWallet{
				privateKey: "test-key",
				address:    types.Address{},
				chain:      mockChain,
			}

			balance := w.Balance()
			assert.Equal(t, 0, balance.Int.Cmp(tt.expectedValue))

			mockChain.AssertExpectations(t)
			mockChain.client.AssertExpectations(t)
		})
	}
}
