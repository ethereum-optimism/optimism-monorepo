package system

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockWallet implements types.Wallet for testing
type mockWallet struct {
	mock.Mock
}

func (m *mockWallet) PrivateKey() types.Key {
	args := m.Called()
	return args.String(0)
}

func (m *mockWallet) Address() types.Address {
	args := m.Called()
	return args.Get(0).(common.Address)
}

func (m *mockWallet) SendETH(to types.Address, amount types.Balance) types.WriteInvocation[any] {
	args := m.Called(to, amount)
	return args.Get(0).(types.WriteInvocation[any])
}

func (m *mockWallet) Balance() types.Balance {
	args := m.Called()
	return args.Get(0).(types.Balance)
}

func (m *mockWallet) Nonce() uint64 {
	args := m.Called()
	return args.Get(0).(uint64)
}

// mockTransactionProcessor implements TransactionProcessor for testing
type mockTransactionProcessor struct {
	mock.Mock
}

func (m *mockTransactionProcessor) Sign(tx Transaction, privateKey string) (Transaction, error) {
	args := m.Called(tx, privateKey)
	return args.Get(0).(Transaction), args.Error(1)
}

func (m *mockTransactionProcessor) Send(ctx context.Context, tx Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

// mockChain implements the Chain interface for testing
type mockChain struct {
	mock.Mock
	txProcessor *mockTransactionProcessor
	wallet      *mockWallet
}

func newMockChain() *mockChain {
	return &mockChain{
		txProcessor: new(mockTransactionProcessor),
		wallet:      new(mockWallet),
	}
}

func (m *mockChain) ID() types.ChainID {
	args := m.Called()
	return types.ChainID(args.Get(0).(*big.Int))
}

func (m *mockChain) GasPrice(ctx context.Context) (*big.Int, error) {
	args := m.Called(ctx)
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *mockChain) GasLimit(ctx context.Context, tx TransactionData) (uint64, error) {
	args := m.Called(ctx, tx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *mockChain) PendingNonceAt(ctx context.Context, addr common.Address) (uint64, error) {
	args := m.Called(ctx, addr)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *mockChain) SupportsEIP(ctx context.Context, eip uint64) bool {
	args := m.Called(ctx, eip)
	return args.Bool(0)
}

func (m *mockChain) ContractsRegistry() interfaces.ContractsRegistry {
	args := m.Called()
	return args.Get(0).(interfaces.ContractsRegistry)
}

func (m *mockChain) RPCURL() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockChain) TransactionProcessor() (TransactionProcessor, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return m.txProcessor, args.Error(1)
}

func (m *mockChain) Wallet(ctx context.Context, constraints ...constraints.WalletConstraint) (types.Wallet, error) {
	args := m.Called(ctx, constraints)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return m.wallet, args.Error(1)
}

func TestNewTxBuilder(t *testing.T) {
	ctx := context.Background()
	chain := newMockChain()

	tests := []struct {
		name           string
		setupMock      func()
		opts           []TxBuilderOption
		expectedTypes  []uint8
		expectedMargin uint64
	}{
		{
			name: "legacy only",
			setupMock: func() {
				chain.On("SupportsEIP", ctx, uint64(1559)).Return(false).Once()
				chain.On("SupportsEIP", ctx, uint64(4844)).Return(false).Once()
			},
			opts:           nil,
			expectedTypes:  []uint8{ethtypes.LegacyTxType},
			expectedMargin: DefaultGasLimitMarginPercent,
		},
		{
			name: "with EIP-1559",
			setupMock: func() {
				chain.On("SupportsEIP", ctx, uint64(1559)).Return(true).Once()
				chain.On("SupportsEIP", ctx, uint64(4844)).Return(false).Once()
			},
			opts:           nil,
			expectedTypes:  []uint8{ethtypes.LegacyTxType, ethtypes.DynamicFeeTxType, ethtypes.AccessListTxType},
			expectedMargin: DefaultGasLimitMarginPercent,
		},
		{
			name: "with EIP-4844",
			setupMock: func() {
				chain.On("SupportsEIP", ctx, uint64(1559)).Return(true).Once()
				chain.On("SupportsEIP", ctx, uint64(4844)).Return(true).Once()
			},
			opts:           nil,
			expectedTypes:  []uint8{ethtypes.LegacyTxType, ethtypes.DynamicFeeTxType, ethtypes.AccessListTxType, ethtypes.BlobTxType},
			expectedMargin: DefaultGasLimitMarginPercent,
		},
		{
			name: "forced tx type",
			setupMock: func() {
				// No EIP checks needed when type is forced
			},
			opts: []TxBuilderOption{
				WithTxType(ethtypes.DynamicFeeTxType),
			},
			expectedTypes:  []uint8{ethtypes.DynamicFeeTxType},
			expectedMargin: DefaultGasLimitMarginPercent,
		},
		{
			name: "custom margin",
			setupMock: func() {
				chain.On("SupportsEIP", ctx, uint64(1559)).Return(false).Once()
				chain.On("SupportsEIP", ctx, uint64(4844)).Return(false).Once()
			},
			opts: []TxBuilderOption{
				WithGasLimitMargin(50),
			},
			expectedTypes:  []uint8{ethtypes.LegacyTxType},
			expectedMargin: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			builder := NewTxBuilder(ctx, chain, tt.opts...)

			assert.Equal(t, tt.expectedTypes, builder.supportedTxTypes)
			assert.Equal(t, tt.expectedMargin, builder.gasLimitMarginPercent)
			chain.AssertExpectations(t)
		})
	}
}

func TestBuildTx(t *testing.T) {
	ctx := context.Background()
	chain := newMockChain()
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	to := common.HexToAddress("0x0987654321098765432109876543210987654321")
	chainID := big.NewInt(1)
	gasPrice := big.NewInt(1000000000) // 1 gwei
	nonce := uint64(1)

	tests := []struct {
		name      string
		setupMock func()
		opts      []TxOption
		wantType  uint8
		wantErr   bool
	}{
		{
			name: "legacy tx",
			setupMock: func() {
				chain.On("SupportsEIP", ctx, uint64(1559)).Return(false).Once()
				chain.On("SupportsEIP", ctx, uint64(4844)).Return(false).Once()
				chain.On("PendingNonceAt", ctx, addr).Return(nonce, nil).Once()
				chain.On("GasPrice", ctx).Return(gasPrice, nil).Once()
				chain.On("GasLimit", ctx, mock.Anything).Return(uint64(21000), nil).Once()
			},
			opts: []TxOption{
				WithFrom(addr),
				WithTo(to),
				WithValue(big.NewInt(100000000000000000)), // 0.1 ETH
			},
			wantType: ethtypes.LegacyTxType,
			wantErr:  false,
		},
		{
			name: "dynamic fee tx",
			setupMock: func() {
				chain.On("SupportsEIP", ctx, uint64(1559)).Return(true).Once()
				chain.On("SupportsEIP", ctx, uint64(4844)).Return(false).Once()
				chain.On("PendingNonceAt", ctx, addr).Return(nonce, nil).Once()
				chain.On("GasPrice", ctx).Return(gasPrice, nil).Once()
				chain.On("ID").Return(chainID).Once()
				chain.On("GasLimit", ctx, mock.Anything).Return(uint64(21000), nil).Once()
			},
			opts: []TxOption{
				WithFrom(addr),
				WithTo(to),
				WithValue(big.NewInt(100000000000000000)), // 0.1 ETH
			},
			wantType: ethtypes.DynamicFeeTxType,
			wantErr:  false,
		},
		{
			name: "access list tx",
			setupMock: func() {
				chain.On("SupportsEIP", ctx, uint64(1559)).Return(true).Once()
				chain.On("SupportsEIP", ctx, uint64(4844)).Return(false).Once()
				chain.On("PendingNonceAt", ctx, addr).Return(nonce, nil).Once()
				chain.On("GasPrice", ctx).Return(gasPrice, nil).Once()
				chain.On("ID").Return(chainID).Once()
				chain.On("GasLimit", ctx, mock.Anything).Return(uint64(21000), nil).Once()
			},
			opts: []TxOption{
				WithFrom(addr),
				WithTo(to),
				WithValue(big.NewInt(100000000000000000)), // 0.1 ETH
				WithAccessList(ethtypes.AccessList{
					{
						Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
						StorageKeys: []common.Hash{
							common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
						},
					},
				}),
			},
			wantType: ethtypes.AccessListTxType,
			wantErr:  false,
		},
		{
			name: "blob tx",
			setupMock: func() {
				chain.On("SupportsEIP", ctx, uint64(1559)).Return(true).Once()
				chain.On("SupportsEIP", ctx, uint64(4844)).Return(true).Once()
				chain.On("PendingNonceAt", ctx, addr).Return(nonce, nil).Once()
				chain.On("GasPrice", ctx).Return(gasPrice, nil).Once()
				chain.On("ID").Return(chainID).Once()
				chain.On("GasLimit", ctx, mock.Anything).Return(uint64(21000), nil).Once()
			},
			opts: []TxOption{
				WithFrom(addr),
				WithTo(to),
				WithValue(big.NewInt(100000000000000000)), // 0.1 ETH
				WithBlobs([]kzg4844.Blob{{}}),
				WithBlobCommitments([]kzg4844.Commitment{{}}),
				WithBlobProofs([]kzg4844.Proof{{}}),
				WithBlobHashes([]common.Hash{{}}),
			},
			wantType: ethtypes.BlobTxType,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			builder := NewTxBuilder(ctx, chain)
			tx, err := builder.BuildTx(tt.opts...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantType, tx.Type())
			chain.AssertExpectations(t)
		})
	}
}

func TestCalculateGasLimit(t *testing.T) {
	ctx := context.Background()
	chain := newMockChain()
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	tests := []struct {
		name           string
		opts           *TxOpts
		margin         uint64
		estimatedGas   uint64
		expectedLimit  uint64
		expectEstimate bool
		wantErr        bool
	}{
		{
			name: "explicit gas limit",
			opts: &TxOpts{
				from:     addr,
				to:       &addr,
				value:    big.NewInt(0),
				gasLimit: 21000,
			},
			margin:         20,
			estimatedGas:   0,
			expectedLimit:  21000,
			expectEstimate: false,
			wantErr:        false,
		},
		{
			name: "estimated with margin",
			opts: &TxOpts{
				from:  addr,
				to:    &addr,
				value: big.NewInt(0),
			},
			margin:         20,
			estimatedGas:   21000,
			expectedLimit:  25200, // 21000 * 1.2
			expectEstimate: true,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up EIP support expectations for NewTxBuilder
			chain.On("SupportsEIP", ctx, uint64(1559)).Return(false).Once()
			chain.On("SupportsEIP", ctx, uint64(4844)).Return(false).Once()

			if tt.expectEstimate {
				chain.On("GasLimit", ctx, tt.opts).Return(tt.estimatedGas, nil).Once()
			}

			builder := NewTxBuilder(ctx, chain, WithGasLimitMargin(tt.margin))
			limit, err := builder.calculateGasLimit(tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLimit, limit)
			chain.AssertExpectations(t)
		})
	}
}
