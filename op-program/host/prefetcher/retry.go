package prefetcher

import (
	"context"
	"math"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	hosttypes "github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

const maxAttempts = math.MaxInt // Succeed or die trying

type RetryingL1Source struct {
	logger   log.Logger
	source   L1Source
	strategy retry.Strategy
}

func NewRetryingL1Source(logger log.Logger, source L1Source) *RetryingL1Source {
	return &RetryingL1Source{
		logger:   logger,
		source:   source,
		strategy: retry.Exponential(),
	}
}

func (s *RetryingL1Source) InfoByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, error) {
	return retry.Do(ctx, maxAttempts, s.strategy, func() (eth.BlockInfo, error) {
		res, err := s.source.InfoByHash(ctx, blockHash)
		if err != nil {
			s.logger.Warn("Failed to retrieve info", "hash", blockHash, "err", err)
		}
		return res, err
	})
}

func (s *RetryingL1Source) InfoAndTxsByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Transactions, error) {
	return retry.Do2(ctx, maxAttempts, s.strategy, func() (eth.BlockInfo, types.Transactions, error) {
		i, t, err := s.source.InfoAndTxsByHash(ctx, blockHash)
		if err != nil {
			s.logger.Warn("Failed to retrieve l1 info and txs", "hash", blockHash, "err", err)
		}
		return i, t, err
	})
}

func (s *RetryingL1Source) FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error) {
	return retry.Do2(ctx, maxAttempts, s.strategy, func() (eth.BlockInfo, types.Receipts, error) {
		i, r, err := s.source.FetchReceipts(ctx, blockHash)
		if err != nil {
			s.logger.Warn("Failed to fetch receipts", "hash", blockHash, "err", err)
		}
		return i, r, err
	})
}

var _ L1Source = (*RetryingL1Source)(nil)

type RetryingL1BlobSource struct {
	logger   log.Logger
	source   L1BlobSource
	strategy retry.Strategy
}

func NewRetryingL1BlobSource(logger log.Logger, source L1BlobSource) *RetryingL1BlobSource {
	return &RetryingL1BlobSource{
		logger:   logger,
		source:   source,
		strategy: retry.Exponential(),
	}
}

func (s *RetryingL1BlobSource) GetBlobSidecars(ctx context.Context, ref eth.L1BlockRef, hashes []eth.IndexedBlobHash) ([]*eth.BlobSidecar, error) {
	return retry.Do(ctx, maxAttempts, s.strategy, func() ([]*eth.BlobSidecar, error) {
		sidecars, err := s.source.GetBlobSidecars(ctx, ref, hashes)
		if err != nil {
			s.logger.Warn("Failed to retrieve blob sidecars", "ref", ref, "err", err)
		}
		return sidecars, err
	})
}

func (s *RetryingL1BlobSource) GetBlobs(ctx context.Context, ref eth.L1BlockRef, hashes []eth.IndexedBlobHash) ([]*eth.Blob, error) {
	return retry.Do(ctx, maxAttempts, s.strategy, func() ([]*eth.Blob, error) {
		blobs, err := s.source.GetBlobs(ctx, ref, hashes)
		if err != nil {
			s.logger.Warn("Failed to retrieve blobs", "ref", ref, "err", err)
		}
		return blobs, err
	})
}

var _ L1BlobSource = (*RetryingL1BlobSource)(nil)

type RetryingL2Source struct {
	logger   log.Logger
	source   hosttypes.L2Source
	strategy retry.Strategy
}

func (s *RetryingL2Source) RollupConfig() *rollup.Config {
	return s.source.RollupConfig()
}

func (s *RetryingL2Source) ExperimentalEnabled() bool {
	return s.source.ExperimentalEnabled()
}

func (s *RetryingL2Source) InfoAndTxsByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Transactions, error) {
	return retry.Do2(ctx, maxAttempts, s.strategy, func() (eth.BlockInfo, types.Transactions, error) {
		i, t, err := s.source.InfoAndTxsByHash(ctx, blockHash)
		if err != nil {
			s.logger.Warn("Failed to retrieve l2 info and txs", "hash", blockHash, "err", err)
		}
		return i, t, err
	})
}

func (s *RetryingL2Source) NodeByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	return retry.Do(ctx, maxAttempts, s.strategy, func() ([]byte, error) {
		n, err := s.source.NodeByHash(ctx, hash)
		if err != nil {
			s.logger.Warn("Failed to retrieve node", "hash", hash, "err", err)
		}
		return n, err
	})
}

func (s *RetryingL2Source) CodeByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	return retry.Do(ctx, maxAttempts, s.strategy, func() ([]byte, error) {
		c, err := s.source.CodeByHash(ctx, hash)
		if err != nil {
			s.logger.Warn("Failed to retrieve code", "hash", hash, "err", err)
		}
		return c, err
	})
}

func (s *RetryingL2Source) FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error) {
	return retry.Do2(ctx, maxAttempts, s.strategy, func() (eth.BlockInfo, types.Receipts, error) {
		i, r, err := s.source.FetchReceipts(ctx, blockHash)
		if err != nil {
			s.logger.Warn("Failed to fetch receipts", "hash", blockHash, "err", err)
		}
		return i, r, err
	})
}

func (s *RetryingL2Source) OutputByRoot(ctx context.Context, blockRoot common.Hash) (eth.Output, error) {
	return retry.Do(ctx, maxAttempts, s.strategy, func() (eth.Output, error) {
		o, err := s.source.OutputByRoot(ctx, blockRoot)
		if err != nil {
			s.logger.Warn("Failed to fetch l2 output", "block", blockRoot, "err", err)
			return o, err
		}
		return o, nil
	})
}

func (s *RetryingL2Source) OutputByNumber(ctx context.Context, blockNum uint64) (eth.Output, error) {
	return retry.Do(ctx, maxAttempts, s.strategy, func() (eth.Output, error) {
		o, err := s.source.OutputByNumber(ctx, blockNum)
		if err != nil {
			s.logger.Warn("Failed to fetch l2 output", "block", blockNum, "err", err)
			return o, err
		}
		return o, nil
	})
}

func (s *RetryingL2Source) GetProof(ctx context.Context, address common.Address, storage []common.Hash, blockTag string) (*eth.AccountResult, error) {
	// these aren't retried because they are currently experimental and can be slow
	return s.source.GetProof(ctx, address, storage, blockTag)
}

func (s *RetryingL2Source) PayloadExecutionWitness(ctx context.Context, parentHash common.Hash, payloadAttributes eth.PayloadAttributes) (*eth.ExecutionWitness, error) {
	// these aren't retried because they are currently experimental and can be slow
	return s.source.PayloadExecutionWitness(ctx, parentHash, payloadAttributes)
}

func NewRetryingL2Source(logger log.Logger, source hosttypes.L2Source) *RetryingL2Source {
	return &RetryingL2Source{
		logger:   logger,
		source:   source,
		strategy: retry.Exponential(),
	}
}

var _ hosttypes.L2Source = (*RetryingL2Source)(nil)
