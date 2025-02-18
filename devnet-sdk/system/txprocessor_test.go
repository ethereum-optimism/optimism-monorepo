package system

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

func (m *mockEthClient) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func TestTransactionProcessor_Sign(t *testing.T) {
	// Test private key and corresponding address
	// DO NOT use this key for anything other than testing
	testKey := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	testAddr := common.HexToAddress("0x96216849c49358B10257cb55b28eA603c874b05E")

	chainID := big.NewInt(1)
	client := new(mockEthClient)

	// Create a wallet with the test key
	chain := newChain(chainID.String(), "http://localhost:8545", nil)
	wallet, err := newWallet(testKey, testAddr, chain)
	assert.NoError(t, err)

	processor := &transactionProcessor{
		client:     client,
		chainID:    chainID,
		privateKey: wallet.PrivateKey(),
	}

	invalidProcessor := &transactionProcessor{
		client:  client,
		chainID: chainID,
		// No private key set
	}

	tests := []struct {
		name       string
		processor  *transactionProcessor
		tx         Transaction
		wantType   uint8
		wantErr    bool
		errMessage string
	}{
		{
			name:      "legacy tx",
			processor: processor,
			tx: &EthTx{
				tx: types.NewTransaction(
					0,
					testAddr,
					big.NewInt(1),
					21000,
					big.NewInt(1),
					nil,
				),
				from:   testAddr,
				txType: types.LegacyTxType,
			},
			wantType: types.LegacyTxType,
			wantErr:  false,
		},
		{
			name:      "dynamic fee tx",
			processor: processor,
			tx: &EthTx{
				tx: types.NewTx(&types.DynamicFeeTx{
					ChainID:   chainID,
					Nonce:     0,
					GasTipCap: big.NewInt(1),
					GasFeeCap: big.NewInt(1),
					Gas:       21000,
					To:        &testAddr,
					Value:     big.NewInt(1),
					Data:      nil,
				}),
				from:   testAddr,
				txType: types.DynamicFeeTxType,
			},
			wantType: types.DynamicFeeTxType,
			wantErr:  false,
		},
		{
			name:      "invalid private key",
			processor: invalidProcessor,
			tx: &EthTx{
				tx: types.NewTransaction(
					0,
					testAddr,
					big.NewInt(1),
					21000,
					big.NewInt(1),
					nil,
				),
				from:   testAddr,
				txType: types.LegacyTxType,
			},
			wantErr:    true,
			errMessage: "private key is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signedTx, err := tt.processor.Sign(tt.tx)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMessage)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, signedTx)
			assert.Equal(t, tt.wantType, signedTx.Type())
			assert.Equal(t, tt.tx.From(), signedTx.From())
		})
	}
}

func TestTransactionProcessor_Send(t *testing.T) {
	chainID := big.NewInt(1)
	client := new(mockEthClient)
	processor := NewTransactionProcessor(client, chainID)
	ctx := context.Background()

	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	tx := types.NewTransaction(
		0,
		testAddr,
		big.NewInt(1),
		21000,
		big.NewInt(1),
		nil,
	)

	tests := []struct {
		name       string
		tx         Transaction
		setupMock  func()
		wantErr    bool
		errMessage string
	}{
		{
			name: "successful send",
			tx: &EthTx{
				tx:     tx,
				from:   testAddr,
				txType: types.LegacyTxType,
			},
			setupMock: func() {
				client.On("SendTransaction", ctx, tx).Return(nil).Once()
			},
			wantErr: false,
		},
		{
			name: "send error",
			tx: &EthTx{
				tx:     tx,
				from:   testAddr,
				txType: types.LegacyTxType,
			},
			setupMock: func() {
				client.On("SendTransaction", ctx, tx).Return(fmt.Errorf("send failed")).Once()
			},
			wantErr:    true,
			errMessage: "failed to send transaction",
		},
		{
			name: "not a raw transaction",
			tx: &mockTransaction{
				from: testAddr,
			},
			setupMock:  func() {},
			wantErr:    true,
			errMessage: "transaction is not signed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			err := processor.Send(ctx, tt.tx)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMessage)
				return
			}

			assert.NoError(t, err)
			client.AssertExpectations(t)
		})
	}
}

// mockTransaction implements Transaction for testing
type mockTransaction struct {
	from common.Address
}

func (m *mockTransaction) Hash() common.Hash    { return common.Hash{} }
func (m *mockTransaction) From() common.Address { return m.from }
func (m *mockTransaction) To() *common.Address  { return nil }
func (m *mockTransaction) Value() *big.Int      { return nil }
func (m *mockTransaction) Data() []byte         { return nil }
func (m *mockTransaction) Type() uint8          { return 0 }
