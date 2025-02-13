package system

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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
	// Use the mock client directly instead of going through Client()
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

func (m *mockEthClient) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	args := m.Called(ctx, account)
	return args.Get(0).(uint64), args.Error(1)
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
				privateKey: crypto.ToECDSAUnsafe(common.FromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")),
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

type internalMockChain struct {
	*mockChain
}

func (m *internalMockChain) Client() (*ethclient.Client, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ethclient.Client), args.Error(1)
}

func TestNewWallet(t *testing.T) {
	pk := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	addr := types.Address(common.HexToAddress("0x5678"))
	chain := &chain{}

	w, err := newWallet(pk, addr, chain)
	assert.NoError(t, err)

	// The private key is converted to ECDSA, so we can't compare directly with the input string
	assert.NotNil(t, w.privateKey)
	assert.Equal(t, addr, w.address)
	assert.Equal(t, chain, w.chain)
}

func TestWallet_Address(t *testing.T) {
	addr := types.Address(common.HexToAddress("0x5678"))
	w := &wallet{address: addr}

	assert.Equal(t, addr, w.Address())
}

func TestWallet_SendETH(t *testing.T) {
	ctx := context.Background()
	mockChain := newMockChain()
	internalChain := &internalMockChain{mockChain}

	// Use a valid 256-bit private key (32 bytes)
	testPrivateKey := crypto.ToECDSAUnsafe(common.FromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"))

	// Derive the address from the private key
	fromAddr := crypto.PubkeyToAddress(testPrivateKey.PublicKey)

	w := &wallet{
		privateKey: testPrivateKey,
		address:    types.Address(fromAddr),
		chain:      internalChain,
	}

	toAddr := types.Address(common.HexToAddress("0x5678"))
	amount := types.NewBalance(big.NewInt(1000000))

	chainID := big.NewInt(1)

	// Mock chain ID for all calls
	mockChain.On("ID").Return(types.ChainID(chainID)).Maybe()

	// Mock EIP support checks
	mockChain.On("SupportsEIP", ctx, uint64(1559)).Return(false)
	mockChain.On("SupportsEIP", ctx, uint64(4844)).Return(false)

	// Mock gas price and limit
	mockChain.On("GasPrice", ctx).Return(big.NewInt(1000000000), nil)
	mockChain.On("GasLimit", ctx, mock.Anything).Return(uint64(21000), nil)

	// Mock nonce retrieval
	mockChain.On("PendingNonceAt", ctx, fromAddr).Return(uint64(0), nil)

	// Mock client access
	client, err := ethclient.Dial("http://this.domain.definitely.does.not.exist:8545")
	assert.NoError(t, err)
	mockChain.On("Client").Return(client, nil)

	// Create the send invocation
	invocation := w.SendETH(toAddr, amount)
	assert.NotNil(t, invocation)

	// Send the transaction
	result := invocation.Send(ctx)
	assert.Error(t, result.Error()) // We expect an error since the client can't connect

	mockChain.AssertExpectations(t)
}

func TestWallet_Balance(t *testing.T) {
	mockChain := newMockChain()
	internalChain := &internalMockChain{mockChain}
	w := &wallet{
		chain: internalChain,
	}

	// Test error case when client is not available
	mockChain.On("Client").Return(nil, assert.AnError).Once()
	balance := w.Balance()
	assert.Equal(t, types.Balance{}, balance)
}

func TestWallet_Nonce(t *testing.T) {
	mockChain := newMockChain()
	internalChain := &internalMockChain{mockChain}
	w := &wallet{
		chain: internalChain,
	}

	// Test error case when client is not available
	mockChain.On("Client").Return(nil, assert.AnError).Once()
	nonce := w.Nonce()
	assert.Equal(t, uint64(0), nonce)
}
