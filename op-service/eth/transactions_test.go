package eth_test

import (
	"context"
	"math/big"
	"math/rand"
	"testing"

	. "github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockL1Client struct {
	mock.Mock
}

func (m *MockL1Client) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	args := m.Called(ctx, account, blockNumber)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockL1Client) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	args := m.Called(ctx, number)
	if header, ok := args.Get(0).(*types.Header); ok {
		return header, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestTransactions_checkRecentTxs(t *testing.T) {
	tests := []struct {
		name             string
		currentBlock     int64
		blockConfirms    uint64
		expectedBlockNum uint64
		expectedFound    bool
		blocks           map[int64][]uint64 // maps blockNum --> nonceVal (one for each stubbed call)
	}{
		{
			name:             "nonceDiff_lowerBound",
			currentBlock:     500,
			blockConfirms:    5,
			expectedBlockNum: 496,
			expectedFound:    true,
			blocks: map[int64][]uint64{
				495: {5, 5},
				496: {6, 6},
				497: {6},
				500: {6},
			},
		},
		{
			name:             "nonceDiff_midRange",
			currentBlock:     500,
			blockConfirms:    5,
			expectedBlockNum: 497,
			expectedFound:    true,
			blocks: map[int64][]uint64{
				495: {5},
				496: {5},
				497: {6, 6},
				500: {6},
			},
		},
		{
			name:             "nonceDiff_upperBound",
			currentBlock:     500,
			blockConfirms:    5,
			expectedBlockNum: 500,
			expectedFound:    true,
			blocks: map[int64][]uint64{
				495: {5},
				497: {5},
				498: {5},
				499: {5},
				500: {6, 6},
			},
		},
		{
			name:             "nonce_unchanged",
			currentBlock:     500,
			blockConfirms:    5,
			expectedBlockNum: 495,
			expectedFound:    false,
			blocks: map[int64][]uint64{
				495: {6},
				500: {6},
			},
		},
		{
			name:             "reorg",
			currentBlock:     500,
			blockConfirms:    5,
			expectedBlockNum: 496,
			expectedFound:    true,
			blocks: map[int64][]uint64{
				495: {5, 5, 5},
				496: {7, 7, 7},
				497: {6, 7},
				500: {6, 7},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l1Client := new(MockL1Client)
			ctx := context.Background()

			// Setup mock responses
			l1Client.On("HeaderByNumber", ctx, (*big.Int)(nil)).Return(&types.Header{Number: big.NewInt(tt.currentBlock)}, nil)
			for blockNum, block := range tt.blocks {
				for _, nonce := range block {
					l1Client.On("NonceAt", ctx, common.Address{}, big.NewInt(blockNum)).Return(nonce, nil).Once()
				}
			}

			blockNum, found, err := CheckRecentTxs(ctx, l1Client, 5, common.Address{})
			require.NoError(t, err)
			require.Equal(t, tt.expectedFound, found)
			require.Equal(t, tt.expectedBlockNum, blockNum)

			l1Client.AssertExpectations(t)
		})
	}
}

func TestOpaqueTransaction(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))

	getBytesAndHash := func(tx *types.Transaction) ([]byte, common.Hash) {
		txBytes, err := tx.MarshalBinary()
		require.NoError(t, err)
		return txBytes, tx.Hash()
	}

	tx := testutils.RandomLegacyTx(rng, testutils.RandomSigner(rng))
	legacyTxBytes, legacyTxHash := getBytesAndHash(tx)

	tx = testutils.RandomAccessListTx(rng, testutils.RandomSigner(rng))
	accessListTxBytes, accessListTxHash := getBytesAndHash(tx)

	tx = testutils.RandomDynamicFeeTx(rng, testutils.RandomSigner(rng))
	dynamicFeeTxBytes, dyamicFeeTxHash := getBytesAndHash(tx)

	futureTxBytes := append([]byte{0x66}, testutils.RandomData(rng, 12)...)
	futureTxHash := crypto.Keccak256Hash(futureTxBytes)

	nonEIP2718TxBytes := append([]byte{0x80}, testutils.RandomData(rng, 12)...)
	nonEIP2718TxHash := crypto.Keccak256Hash(nonEIP2718TxBytes)

	type testCase struct {
		name      string
		rawTx     []byte
		txType    uint8
		txHash    common.Hash
		eip2718   bool // is this an EIP 2718 transaction?
		supported bool // is this an explicitly supported EIP 2718 transaction?
	}

	testCases := []testCase{
		{"LegacyTx(0x00)", legacyTxBytes, 0, legacyTxHash, true, true},
		{"AccessListTx(0x01)", accessListTxBytes, 1, accessListTxHash, true, true},
		{"DyamicFeeTx(0x02)", dynamicFeeTxBytes, 2, dyamicFeeTxHash, true, true},
		{"FutureTx(0x66)", futureTxBytes, 102, futureTxHash, true, false},
		{"NonEIP2718Tx(0x80)", nonEIP2718TxBytes, 128, nonEIP2718TxHash, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			o := new(OpaqueTransaction)

			if !tc.eip2718 {
				require.Panics(t, func() {
					o.UnmarshalBinary(tc.rawTx)
				})
				return
			}

			err := o.UnmarshalBinary(tc.rawTx)
			require.NoError(t, err)

			require.Equal(t, tc.txType, o.TxType())
			require.Equal(t, tc.txHash, o.TxHash())

			reSerialized, err := o.MarshalBinary()
			require.NoError(t, err)
			require.Equal(t, tc.rawTx, reSerialized)

			data, err := o.MarshalJSON()
			if tc.supported {
				require.NoError(t, err)
				p := new(OpaqueTransaction)
				err = p.UnmarshalJSON(data)
				require.NoError(t, err)
				require.Equal(t, o, p)
			} else {
				require.Error(t, err)
				require.Nil(t, data)
			}

			tx, err = o.Transaction()
			if tc.supported {
				require.NoError(t, err)
				require.Equal(t, tc.txHash, tx.Hash())
			} else {
				require.Error(t, err)
				require.Nil(t, tx)
			}
		})
	}
}
