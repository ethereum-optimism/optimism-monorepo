package driver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDurationRecorded(t *testing.T) {
	num := uint64(1234)
	hash := common.Hash{0xaa}
	ref := eth.L1BlockRef{Number: num}
	info := &testutils.MockBlockInfo{}
	expectedErr := errors.New("test error")

	tests := []struct {
		method string
		expect func(inner *testutils.MockL1Source)
		call   func(t *testing.T, fetcher *MeteredL1Fetcher, inner *testutils.MockL1Source)
	}{
		{
			method: "L1BlockRefByLabel",
			call: func(t *testing.T, fetcher *MeteredL1Fetcher, inner *testutils.MockL1Source) {
				inner.ExpectL1BlockRefByLabel(eth.Finalized, ref, expectedErr)

				result, err := fetcher.L1BlockRefByLabel(context.Background(), eth.Finalized)
				require.Equal(t, ref, result)
				require.Equal(t, expectedErr, err)
			},
		},
		{
			method: "L1BlockRefByNumber",
			call: func(t *testing.T, fetcher *MeteredL1Fetcher, inner *testutils.MockL1Source) {
				inner.ExpectL1BlockRefByNumber(num, ref, expectedErr)

				result, err := fetcher.L1BlockRefByNumber(context.Background(), num)
				require.Equal(t, ref, result)
				require.Equal(t, expectedErr, err)
			},
		},
		{
			method: "L1BlockRefByHash",
			call: func(t *testing.T, fetcher *MeteredL1Fetcher, inner *testutils.MockL1Source) {
				inner.ExpectL1BlockRefByHash(hash, ref, expectedErr)

				result, err := fetcher.L1BlockRefByHash(context.Background(), hash)
				require.Equal(t, ref, result)
				require.Equal(t, expectedErr, err)
			},
		},
		{
			method: "InfoByHash",
			call: func(t *testing.T, fetcher *MeteredL1Fetcher, inner *testutils.MockL1Source) {
				inner.ExpectInfoByHash(hash, info, expectedErr)

				result, err := fetcher.InfoByHash(context.Background(), hash)
				require.Equal(t, info, result)
				require.Equal(t, expectedErr, err)
			},
		},
		{
			method: "InfoAndTxsByHash",
			call: func(t *testing.T, fetcher *MeteredL1Fetcher, inner *testutils.MockL1Source) {
				txs := make([]eth.GenericTx, 1)
				inner.ExpectInfoAndTxsByHash(hash, info, txs, expectedErr)

				actualInfo, actualTxs, err := fetcher.InfoAndTxsByHash(context.Background(), hash)
				require.Equal(t, info, actualInfo)
				require.Equal(t, txs, actualTxs)
				require.Equal(t, expectedErr, err)
			},
		},
		{
			method: "FetchReceipts",
			call: func(t *testing.T, fetcher *MeteredL1Fetcher, inner *testutils.MockL1Source) {
				rcpts := types.Receipts{
					&types.Receipt{},
				}
				inner.ExpectFetchReceipts(hash, info, rcpts, expectedErr)

				actualInfo, actualRcpts, err := fetcher.FetchReceipts(context.Background(), hash)
				require.Equal(t, info, actualInfo)
				require.Equal(t, rcpts, actualRcpts)
				require.Equal(t, expectedErr, err)
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.method, func(t *testing.T) {
			duration := 200 * time.Millisecond
			fetcher, inner, metrics := createFetcher(duration)
			defer inner.AssertExpectations(t)
			defer metrics.AssertExpectations(t)

			metrics.ExpectRecordRequestTime(test.method, duration)

			test.call(t, fetcher, inner)
		})
	}
}

// createFetcher creates a MeteredL1Fetcher with a mock inner.
// The clock used to calculate the current time will advance by clockIncrement on each call, making it appear as if
// each request takes that amount of time to execute.
func createFetcher(clockIncrement time.Duration) (*MeteredL1Fetcher, *testutils.MockL1Source, *mockMetrics) {
	inner := &testutils.MockL1Source{}
	currTime := time.UnixMilli(1294812934000000)
	clock := func() time.Time {
		currTime = currTime.Add(clockIncrement)
		return currTime
	}
	metrics := &mockMetrics{}
	fetcher := MeteredL1Fetcher{
		inner:   inner,
		metrics: metrics,
		now:     clock,
	}
	return &fetcher, inner, metrics
}

type mockMetrics struct {
	mock.Mock
}

func (m *mockMetrics) RecordL1RequestTime(method string, duration time.Duration) {
	m.MethodCalled("RecordL1RequestTime", method, duration)
}

func (m *mockMetrics) ExpectRecordRequestTime(method string, duration time.Duration) {
	m.On("RecordL1RequestTime", method, duration).Once()
}
