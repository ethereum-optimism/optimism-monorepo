package sources

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/retry"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// simpleMockRPC is needed for some tests where the return value dynamically
// depends on the input, so that the test can set the function.
type simpleMockRPC struct {
	callFn      func(ctx context.Context, result any, method string, args ...any) error
	batchCallFn func(ctx context.Context, b []rpc.BatchElem) error
}

func (m *simpleMockRPC) CallContext(ctx context.Context, result any, method string, args ...any) error {
	return m.callFn(ctx, result, method, args...)
}

func (m *simpleMockRPC) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	return m.batchCallFn(ctx, b)
}

func TestBasicRPCReceiptsFetcher_Reuse(t *testing.T) {
	require := require.New(t)
	batchSize, txCount := 2, uint64(4)
	block, receipts := randomRpcBlockAndReceipts(rand.New(rand.NewSource(123)), txCount)
	txHashes := make([]common.Hash, 0, len(receipts))
	recMap := make(map[common.Hash]*types.Receipt, len(receipts))
	for _, rec := range receipts {
		txHashes = append(txHashes, rec.TxHash)
		recMap[rec.TxHash] = rec
	}
	mrpc := new(simpleMockRPC)
	rp := NewBasicRPCReceiptsFetcher(mrpc, batchSize)

	// prepare mock
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	// 1st fetching
	response := map[common.Hash]bool{
		txHashes[0]: true,
		txHashes[1]: true,
		txHashes[2]: false,
		txHashes[3]: false,
	}
	var numCalls atomic.Int32
	mrpc.batchCallFn = func(_ context.Context, b []rpc.BatchElem) (err error) {
		numCalls.Add(1)
		for i, el := range b {
			if el.Method == "eth_getTransactionReceipt" {
				txHash := el.Args[0].(common.Hash)
				if response[txHash] {
					// The IterativeBatchCall expects that the values are written
					// to the fields of the allocated *types.Receipt.
					**(el.Result.(**types.Receipt)) = *recMap[txHash]
				} else {
					err = errors.Join(err, fmt.Errorf("receipt[%d] error, hash %x", i, txHash))
				}
			} else {
				err = errors.Join(err, fmt.Errorf("unknown method %s", el.Method))
			}
		}
		return err
	}

	bInfo, _, _ := block.Info(true, true)

	// 1st fetching should result in errors
	recs, err := rp.FetchReceipts(ctx, bInfo, txHashes)
	require.Error(err)
	require.Nil(recs)
	require.EqualValues(2, numCalls.Load())

	// prepare 2nd fetching - all should succeed now
	response[txHashes[2]] = true
	response[txHashes[3]] = true
	recs, err = rp.FetchReceipts(ctx, bInfo, txHashes)
	require.NoError(err)
	require.NotNil(recs)
	for i, rec := range recs {
		requireEqualReceipt(t, receipts[i], rec)
	}
	require.EqualValues(3, numCalls.Load())
}

func TestBasicRPCReceiptsFetcher_Concurrency(t *testing.T) {
	require := require.New(t)
	const numFetchers = 32
	const batchSize, txCount = 4, 16
	const numBatchCalls = txCount / batchSize
	block, receipts := randomRpcBlockAndReceipts(rand.New(rand.NewSource(123)), txCount)
	recMap := make(map[common.Hash]*types.Receipt, len(receipts))
	for _, rec := range receipts {
		recMap[rec.TxHash] = rec
	}

	boff := &retry.ExponentialStrategy{
		Min:       0,
		Max:       time.Second,
		MaxJitter: 100 * time.Millisecond,
	}
	err := retry.Do0(context.Background(), 10, boff, func() error {
		mrpc := new(mockRPC)
		rp := NewBasicRPCReceiptsFetcher(mrpc, batchSize)

		// prepare mock
		var numCalls atomic.Int32
		mrpc.On("BatchCallContext", mock.Anything, mock.AnythingOfType("[]rpc.BatchElem")).
			Run(func(args mock.Arguments) {
				numCalls.Add(1)
				els := args.Get(1).([]rpc.BatchElem)
				for _, el := range els {
					if el.Method == "eth_getTransactionReceipt" {
						txHash := el.Args[0].(common.Hash)
						// The IterativeBatchCall expects that the values are written
						// to the fields of the allocated *types.Receipt.
						**(el.Result.(**types.Receipt)) = *recMap[txHash]
					}
				}
			}).
			Return([]error{nil})

		runConcurrentFetchingTest(t, rp, numFetchers, receipts, block)

		mrpc.AssertExpectations(t)
		finalNumCalls := int(numCalls.Load())

		if finalNumCalls == 0 {
			return errors.New("batchCallContext should have been called")
		}

		if finalNumCalls >= numFetchers*numBatchCalls {
			return errors.New("some IterativeBatchCalls should have been shared")
		}
		return nil
	})
	require.NoError(err)
}

func runConcurrentFetchingTest(t *testing.T, rp ReceiptsProvider, numFetchers int, receipts types.Receipts, block *RPCBlock) {
	require := require.New(t)
	txHashes := receiptTxHashes(receipts)

	// start n fetchers
	type fetchResult struct {
		rs  types.Receipts
		err error
	}
	fetchResults := make(chan fetchResult, numFetchers)
	barrier := make(chan struct{})
	bInfo, _, _ := block.Info(true, true)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	for i := 0; i < numFetchers; i++ {
		go func() {
			<-barrier
			recs, err := rp.FetchReceipts(ctx, bInfo, txHashes)
			fetchResults <- fetchResult{rs: recs, err: err}
		}()
	}
	close(barrier) // Go!

	// assert results
	for i := 0; i < numFetchers; i++ {
		select {
		case f := <-fetchResults:
			require.NoError(f.err)
			require.Len(f.rs, len(receipts))
			for j, r := range receipts {
				requireEqualReceipt(t, r, f.rs[j])
			}
		case <-ctx.Done():
			t.Fatal("Test timeout")
		}
	}
}

func receiptTxHashes(receipts types.Receipts) []common.Hash {
	txHashes := make([]common.Hash, 0, len(receipts))
	for _, rec := range receipts {
		txHashes = append(txHashes, rec.TxHash)
	}
	return txHashes
}
