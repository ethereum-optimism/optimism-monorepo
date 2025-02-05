package prefetcher

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	hosttypes "github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestRetryingL1Source(t *testing.T) {
	ctx := context.Background()
	hash := common.Hash{0xab}
	info := &testutils.MockBlockInfo{InfoHash: hash}
	// The mock really doesn't like returning nil for a eth.BlockInfo so return a value we expect to be ignored instead
	wrongInfo := &testutils.MockBlockInfo{InfoHash: common.Hash{0x99}}
	txs := types.Transactions{
		&types.Transaction{},
	}
	rcpts := types.Receipts{
		&types.Receipt{},
	}

	t.Run("InfoByHash Success", func(t *testing.T) {
		source, mock := createL1Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectInfoByHash(hash, info, nil)

		result, err := source.InfoByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, result)
	})

	t.Run("InfoByHash Error", func(t *testing.T) {
		source, mock := createL1Source(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectInfoByHash(hash, wrongInfo, expectedErr)
		mock.ExpectInfoByHash(hash, info, nil)

		result, err := source.InfoByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, result)
	})

	t.Run("InfoAndTxsByHash Success", func(t *testing.T) {
		source, mock := createL1Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectInfoAndTxsByHash(hash, info, txs, nil)

		actualInfo, actualTxs, err := source.InfoAndTxsByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, actualInfo)
		require.Equal(t, txs, actualTxs)
	})

	t.Run("InfoAndTxsByHash Error", func(t *testing.T) {
		source, mock := createL1Source(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectInfoAndTxsByHash(hash, wrongInfo, nil, expectedErr)
		mock.ExpectInfoAndTxsByHash(hash, info, txs, nil)

		actualInfo, actualTxs, err := source.InfoAndTxsByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, actualInfo)
		require.Equal(t, txs, actualTxs)
	})

	t.Run("FetchReceipts Success", func(t *testing.T) {
		source, mock := createL1Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectFetchReceipts(hash, info, rcpts, nil)

		actualInfo, actualRcpts, err := source.FetchReceipts(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, actualInfo)
		require.Equal(t, rcpts, actualRcpts)
	})

	t.Run("FetchReceipts Error", func(t *testing.T) {
		source, mock := createL1Source(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectFetchReceipts(hash, wrongInfo, nil, expectedErr)
		mock.ExpectFetchReceipts(hash, info, rcpts, nil)

		actualInfo, actualRcpts, err := source.FetchReceipts(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, actualInfo)
		require.Equal(t, rcpts, actualRcpts)
	})
}

func createL1Source(t *testing.T) (*RetryingL1Source, *testutils.MockL1Source) {
	logger := testlog.Logger(t, log.LevelDebug)
	mock := &testutils.MockL1Source{}
	source := NewRetryingL1Source(logger, mock)
	// Avoid sleeping in tests by using a fixed retry strategy with no delay
	source.strategy = retry.Fixed(0)
	return source, mock
}

func TestRetryingL1BlobSource(t *testing.T) {
	ctx := context.Background()
	blob := GetRandBlob(t, 0xFACADE)
	commitment, err := blob.ComputeKZGCommitment()
	require.NoError(t, err)
	versionedHash := eth.KZGToVersionedHash(commitment)
	blobHash := eth.IndexedBlobHash{Hash: versionedHash, Index: 0xFACADE}
	l1BlockRef := eth.L1BlockRef{Time: 0}

	t.Run("GetBlobs Success", func(t *testing.T) {
		source, mock := createL1BlobSource(t)
		defer mock.AssertExpectations(t)
		mock.ExpectOnGetBlobs(
			ctx,
			l1BlockRef,
			[]eth.IndexedBlobHash{blobHash},
			[]*eth.Blob{(*eth.Blob)(&blob)},
			nil,
		)

		result, err := source.GetBlobs(ctx, l1BlockRef, []eth.IndexedBlobHash{blobHash})
		require.NoError(t, err)
		require.Equal(t, len(result), 1)
		require.Equal(t, blob[:], result[0][:])
	})

	t.Run("GetBlobs Error", func(t *testing.T) {
		source, mock := createL1BlobSource(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectOnGetBlobs(
			ctx,
			l1BlockRef,
			[]eth.IndexedBlobHash{blobHash},
			nil,
			expectedErr,
		)
		mock.ExpectOnGetBlobs(
			ctx,
			l1BlockRef,
			[]eth.IndexedBlobHash{blobHash},
			[]*eth.Blob{(*eth.Blob)(&blob)},
			nil,
		)

		result, err := source.GetBlobs(ctx, l1BlockRef, []eth.IndexedBlobHash{blobHash})
		require.NoError(t, err)
		require.Equal(t, len(result), 1)
		require.Equal(t, blob[:], result[0][:])
	})

	t.Run("GetBlobSidecars Success", func(t *testing.T) {
		source, mock := createL1BlobSource(t)
		defer mock.AssertExpectations(t)
		mock.ExpectOnGetBlobSidecars(
			ctx,
			l1BlockRef,
			[]eth.IndexedBlobHash{blobHash},
			(eth.Bytes48)(commitment),
			[]*eth.Blob{(*eth.Blob)(&blob)},
			nil,
		)

		result, err := source.GetBlobSidecars(ctx, l1BlockRef, []eth.IndexedBlobHash{blobHash})
		require.NoError(t, err)
		require.Equal(t, len(result), 1)
		require.Equal(t, blob[:], result[0].Blob[:])
		require.Equal(t, blobHash.Index, uint64(result[0].Index))
		require.Equal(t, (eth.Bytes48)(commitment), result[0].KZGCommitment)
	})

	t.Run("GetBlobSidecars Error", func(t *testing.T) {
		source, mock := createL1BlobSource(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectOnGetBlobSidecars(
			ctx,
			l1BlockRef,
			[]eth.IndexedBlobHash{blobHash},
			(eth.Bytes48)(commitment),
			[]*eth.Blob{(*eth.Blob)(&blob)},
			expectedErr,
		)
		mock.ExpectOnGetBlobSidecars(
			ctx,
			l1BlockRef,
			[]eth.IndexedBlobHash{blobHash},
			(eth.Bytes48)(commitment),
			[]*eth.Blob{(*eth.Blob)(&blob)},
			nil,
		)

		result, err := source.GetBlobSidecars(ctx, l1BlockRef, []eth.IndexedBlobHash{blobHash})
		require.NoError(t, err)
		require.Equal(t, len(result), 1)
		require.Equal(t, blob[:], result[0].Blob[:])
		require.Equal(t, blobHash.Index, uint64(result[0].Index))
		require.Equal(t, (eth.Bytes48)(commitment), result[0].KZGCommitment)
	})
}

func createL1BlobSource(t *testing.T) (*RetryingL1BlobSource, *testutils.MockBlobsFetcher) {
	logger := testlog.Logger(t, log.LvlDebug)
	mock := &testutils.MockBlobsFetcher{}
	source := NewRetryingL1BlobSource(logger, mock)
	// Avoid sleeping in tests by using a fixed retry strategy with no delay
	source.strategy = retry.Fixed(0)
	return source, mock
}

func TestRetryingL2Source(t *testing.T) {
	ctx := context.Background()
	hash := common.Hash{0xab}
	blockNum := uint64(14982)
	info := &testutils.MockBlockInfo{InfoHash: hash, InfoNum: blockNum}
	// The mock really doesn't like returning nil for a eth.BlockInfo so return a value we expect to be ignored instead
	wrongInfo := &testutils.MockBlockInfo{InfoHash: common.Hash{0x99}}
	txs := types.Transactions{
		&types.Transaction{},
	}
	rcpts := types.Receipts{
		&types.Receipt{},
	}
	data := []byte{1, 2, 3, 4, 5}
	output := &eth.OutputV0{}
	wrongOutput := &eth.OutputV0{BlockHash: common.Hash{0x99}}

	t.Run("InfoAndTxsByHash Success", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectInfoAndTxsByHash(hash, info, txs, nil)

		actualInfo, actualTxs, err := source.InfoAndTxsByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, actualInfo)
		require.Equal(t, txs, actualTxs)
	})

	t.Run("InfoAndTxsByHash Error", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectInfoAndTxsByHash(hash, wrongInfo, nil, expectedErr)
		mock.ExpectInfoAndTxsByHash(hash, info, txs, nil)

		actualInfo, actualTxs, err := source.InfoAndTxsByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, actualInfo)
		require.Equal(t, txs, actualTxs)
	})

	t.Run("NodeByHash Success", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectNodeByHash(hash, data, nil)

		actual, err := source.NodeByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, data, actual)
	})

	t.Run("NodeByHash Error", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectNodeByHash(hash, nil, expectedErr)
		mock.ExpectNodeByHash(hash, data, nil)

		actual, err := source.NodeByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, data, actual)
	})

	t.Run("CodeByHash Success", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectCodeByHash(hash, data, nil)

		actual, err := source.CodeByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, data, actual)
	})

	t.Run("CodeByHash Error", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectCodeByHash(hash, nil, expectedErr)
		mock.ExpectCodeByHash(hash, data, nil)

		actual, err := source.CodeByHash(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, data, actual)
	})

	t.Run("FetchReceipts Success", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectFetchReceipts(hash, info, rcpts, nil)

		actualInfo, actualRcpts, err := source.FetchReceipts(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, info, actualInfo)
		require.Equal(t, rcpts, actualRcpts)
	})

	t.Run("OutputByRoot Success", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectOutputByRoot(hash, output, nil)

		actualOutput, err := source.OutputByRoot(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, output, actualOutput)
	})

	t.Run("OutputByRoot Error", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectOutputByRoot(hash, wrongOutput, expectedErr)
		mock.ExpectOutputByRoot(hash, output, nil)

		actualOutput, err := source.OutputByRoot(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, output, actualOutput)
	})

	t.Run("OutputByNumber Success", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		mock.ExpectOutputByNumber(blockNum, output, nil)

		actualOutput, err := source.OutputByNumber(ctx, blockNum)
		require.NoError(t, err)
		require.Equal(t, output, actualOutput)
	})

	t.Run("OutputByNumber Error", func(t *testing.T) {
		source, mock := createL2Source(t)
		defer mock.AssertExpectations(t)
		expectedErr := errors.New("boom")
		mock.ExpectOutputByNumber(blockNum, wrongOutput, expectedErr)
		mock.ExpectOutputByNumber(blockNum, output, nil)

		actualOutput, err := source.OutputByNumber(ctx, blockNum)
		require.NoError(t, err)
		require.Equal(t, output, actualOutput)
	})
}

func createL2Source(t *testing.T) (*RetryingL2Source, *MockL2Source) {
	logger := testlog.Logger(t, log.LevelDebug)
	mock := &MockL2Source{}
	source := NewRetryingL2Source(logger, mock)
	// Avoid sleeping in tests by using a fixed retry strategy with no delay
	source.strategy = retry.Fixed(0)
	return source, mock
}

type MockL2Source struct {
	mock.Mock
}

func (m *MockL2Source) ExperimentalEnabled() bool {
	out := m.Mock.MethodCalled("ExperimentalEnabled")
	return out[0].(bool)
}

func (m *MockL2Source) RollupConfig() *rollup.Config {
	out := m.Mock.MethodCalled("RollupConfig")
	return out[0].(*rollup.Config)
}

func (m *MockL2Source) PayloadExecutionWitness(ctx context.Context, parentHash common.Hash, payloadAttributes eth.PayloadAttributes) (*eth.ExecutionWitness, error) {
	out := m.Mock.MethodCalled("PayloadExecutionWitness", parentHash, payloadAttributes)
	return out[0].(*eth.ExecutionWitness), *out[1].(*error)
}
func (m *MockL2Source) GetProof(ctx context.Context, address common.Address, storage []common.Hash, blockTag string) (*eth.AccountResult, error) {
	out := m.Mock.MethodCalled("GetProof", address, storage, blockTag)
	return out[0].(*eth.AccountResult), *out[1].(*error)
}

func (m *MockL2Source) InfoAndTxsByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Transactions, error) {
	out := m.Mock.MethodCalled("InfoAndTxsByHash", blockHash)
	return out[0].(eth.BlockInfo), out[1].(types.Transactions), *out[2].(*error)
}

func (m *MockL2Source) NodeByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	out := m.Mock.MethodCalled("NodeByHash", hash)
	return out[0].([]byte), *out[1].(*error)
}

func (m *MockL2Source) CodeByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	out := m.Mock.MethodCalled("CodeByHash", hash)
	return out[0].([]byte), *out[1].(*error)
}

func (m *MockL2Source) FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error) {
	out := m.Mock.MethodCalled("FetchReceipts", blockHash)
	return out[0].(eth.BlockInfo), out[1].(types.Receipts), *out[2].(*error)
}

func (m *MockL2Source) OutputByRoot(ctx context.Context, blockRoot common.Hash) (eth.Output, error) {
	out := m.Mock.MethodCalled("OutputByRoot", blockRoot)
	return out[0].(eth.Output), *out[1].(*error)
}

func (m *MockL2Source) OutputByNumber(ctx context.Context, blockNum uint64) (eth.Output, error) {
	out := m.Mock.MethodCalled("OutputByNumber", blockNum)
	return out[0].(eth.Output), *out[1].(*error)
}

func (m *MockL2Source) ExpectInfoAndTxsByHash(blockHash common.Hash, info eth.BlockInfo, txs types.Transactions, err error) {
	m.Mock.On("InfoAndTxsByHash", blockHash).Once().Return(info, txs, &err)
}

func (m *MockL2Source) ExpectNodeByHash(hash common.Hash, node []byte, err error) {
	m.Mock.On("NodeByHash", hash).Once().Return(node, &err)
}

func (m *MockL2Source) ExpectCodeByHash(hash common.Hash, code []byte, err error) {
	m.Mock.On("CodeByHash", hash).Once().Return(code, &err)
}

func (m *MockL2Source) ExpectFetchReceipts(blockHash common.Hash, info eth.BlockInfo, rcpts types.Receipts, err error) {
	m.Mock.On("FetchReceipts", blockHash).Once().Return(info, rcpts, &err)
}

func (m *MockL2Source) ExpectOutputByRoot(blockHash common.Hash, output eth.Output, err error) {
	m.Mock.On("OutputByRoot", blockHash).Once().Return(output, &err)
}

func (m *MockL2Source) ExpectOutputByNumber(blockNum uint64, output eth.Output, err error) {
	m.Mock.On("OutputByNumber", blockNum).Once().Return(output, &err)
}

var _ hosttypes.L2Source = (*MockL2Source)(nil)
