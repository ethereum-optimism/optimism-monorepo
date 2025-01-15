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

	// Prepare binary encoding of a DynamicFeeTx
	rng := rand.New(rand.NewSource(1234))
	tx := testutils.RandomDynamicFeeTx(rng, testutils.RandomSigner(rng))
	encodedDynamicFeeTx, err := tx.MarshalBinary()
	require.NoError(t, err)

	// Binary Unmarshal / Marshal roundtrip
	o := new(OpaqueTransaction)
	o.UnmarshalBinary(encodedDynamicFeeTx)
	require.Equal(t, uint8(2), o.TxType())

	reSerialized, err := o.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, encodedDynamicFeeTx, reSerialized)

	expectedHash := tx.Hash()
	require.Equal(t, expectedHash, o.TxHash())

	// extract the transaction (this only works if it is one of the supported types)
	extractedTx, err := o.Transaction()
	require.NoError(t, err)

	// compare the binary encoding of the extracted transaction with the original
	// the rich type has extra metadata which we don't want to compare here
	e, err := extractedTx.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, encodedDynamicFeeTx, e)

	// A future, unsupported EIP 2718 tx type, unsupported at the time of writing
	hypotheticalTxBytes := append([]byte{0x66}, testutils.RandomData(rng, 12)...)

	// Binary Unmarshal / Marshal roundtrip
	p := new(OpaqueTransaction)
	p.UnmarshalBinary(hypotheticalTxBytes)
	require.Equal(t, uint8(102), p.TxType())

	reSerialized, err = p.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, hypotheticalTxBytes, reSerialized)

	// the hash should be non-zero
	expectedHash = crypto.Keccak256Hash(hypotheticalTxBytes)
	require.NotEqual(t, expectedHash, o.TxHash())

	// try to extract the rich transaction (this should return an error since the tx type is unsupported)
	extractedTx, err = p.Transaction()
	require.Nil(t, extractedTx)
	require.Error(t, err)
}
