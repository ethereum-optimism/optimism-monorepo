package eth

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
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
	encodedDynamicFeeTx, err := hexutil.Decode("0x02f8af017b830186a0830f4240825dc094c00e5d67c2755389aded7d8b151cbd5bcdf7ed278301e2408b68656c6c6f20776f726c64f838f7945ad5e028b664880fc7581c77547deaf776200434e1a095b358675999c4b7338ff339566349ed0ef6384876655d1b9b955e36ac165c6b80a06c33b333151b99d601320ca7f05ccb5597b9bbf1db299a63a61a780d081622f0a00ddade96dd1f0df1f67a5879b8cb06c3310818be6a2fc7d746fd73e300253b96")
	require.NoError(t, err)
	o := new(OpaqueTransaction)
	o.UnmarshalBinary([]byte(encodedDynamicFeeTx))
	require.Equal(t, uint8(2), o.TxType())

	reSerialized, err := o.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, encodedDynamicFeeTx, reSerialized)

	expectedHash := common.HexToHash("0x6a1d6ffccedd6b9b53e1e81fc625cdd1ad1e48993b6c6d6ee58df55245271dc3")
	require.Equal(t, expectedHash, o.TxHash())
}
