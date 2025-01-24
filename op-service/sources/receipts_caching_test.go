package sources

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockReceiptsProvider struct {
	mock.Mock
}

func (m *mockReceiptsProvider) FetchReceipts(ctx context.Context, blockInfo eth.BlockInfo, txHashes []common.Hash) (types.Receipts, error) {
	block := eth.ToBlockID(blockInfo)
	args := m.Called(ctx, block, txHashes)
	return args.Get(0).(types.Receipts), args.Error(1)
}

func TestCachingReceiptsProvider_Caching(t *testing.T) {
	block, receipts := randomRpcBlockAndReceipts(rand.New(rand.NewSource(69)), 4)
	txHashes := receiptTxHashes(receipts)
	blockid := block.BlockID()
	mrp := new(mockReceiptsProvider)
	rp := NewCachingReceiptsProvider(mrp, nil, 1)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	mrp.On("FetchReceipts", ctx, blockid, txHashes).
		Return(types.Receipts(receipts), error(nil)).
		Once() // receipts should be cached after first fetch

	bInfo, _, _ := block.Info(true, true)
	for i := 0; i < 4; i++ {
		gotRecs, err := rp.FetchReceipts(ctx, bInfo, txHashes)
		require.NoError(t, err)
		for i, gotRec := range gotRecs {
			requireEqualReceipt(t, receipts[i], gotRec)
		}
	}
	mrp.AssertExpectations(t)
}

func TestCachingReceiptsProvider_Concurrency(t *testing.T) {
	block, receipts := randomRpcBlockAndReceipts(rand.New(rand.NewSource(69)), 4)
	txHashes := receiptTxHashes(receipts)
	blockid := block.BlockID()
	mrp := new(mockReceiptsProvider)
	rp := NewCachingReceiptsProvider(mrp, nil, 1)

	mrp.On("FetchReceipts", mock.Anything, blockid, txHashes).
		Return(types.Receipts(receipts), error(nil)).
		Once() // receipts should be cached after first fetch

	runConcurrentFetchingTest(t, rp, 32, receipts, block)

	mrp.AssertExpectations(t)
}
