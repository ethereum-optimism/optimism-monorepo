package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

type DownloadConfig struct {
	StartNum           uint64
	EndNum             uint64
	Addr               string
	ConcurrentRequests uint64
}

func DownloadRange(ctx context.Context, logger log.Logger, cfg *DownloadConfig,
	onBlock func(ctx context.Context, bl *sources.RPCBlock) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Produce work to run
	work := make(chan uint64)
	start := time.Now()
	go func() {
		lastUpdate := time.Now()
		for i := cfg.StartNum; i < cfg.EndNum; i++ {
			select {
			case work <- i:
			case <-ctx.Done():
				return
			}
			if count := i - cfg.StartNum; count%100 == 0 {
				now := time.Now()
				// speed in number of blocks downloaded per second,
				// as measured over the last check
				speed := float64(100) / (float64(now.Sub(lastUpdate)) / float64(time.Second))
				total := cfg.EndNum - cfg.StartNum
				remaining := total - count
				eta := time.Duration(float64(remaining)/speed) * time.Second
				elapsed := now.Sub(start)
				logger.Info("scheduled download work",
					"count", count, "elapsed", elapsed,
					"speed", fmt.Sprintf("%.3f blocks / sec", speed),
					"remaining", remaining,
					"eta", eta)
				lastUpdate = now
			}
		}
		close(work) // only close if we finished all the work
	}()

	// run the work split into parallel workers that each have their own resources
	results := make(chan error, cfg.ConcurrentRequests)
	for wi := uint64(0); wi < cfg.ConcurrentRequests; wi++ {
		go func(wi uint64) {
			logger := logger.New("worker", wi)
			err := downloadWorker(ctx, work, logger, cfg.Addr, onBlock)
			if err != nil {
				err = fmt.Errorf("worker %d failed: %w", wi, err)
			}
			results <- err
		}(wi)
	}

	var result error

	// collect the results. Abort if any error.
	for wi := uint64(0); wi < cfg.ConcurrentRequests; wi++ {
		err := <-results
		if err != nil {
			cancel() // cancel all other work, no point in finishing it anymore
			logger.Error("Download worker failed, aborting now", "err", err)
			result = errors.Join(result, err)
		}
	}
	return result
}

func downloadWorker(ctx context.Context, work <-chan uint64,
	logger log.Logger, addr string, onBlock func(ctx context.Context, bl *sources.RPCBlock) error) error {
	cl, err := client.NewRPC(ctx, logger, addr,
		client.WithRateLimit(10, 100),
		client.WithDialAttempts(10))
	if err != nil {
		return fmt.Errorf("failed to open RPC client: %w", err)
	}
	for {
		var num uint64
		select {
		case <-ctx.Done():
			logger.Warn("closing early")
			return ctx.Err()
		case v, ok := <-work:
			if !ok {
				logger.Info("done")
				return nil
			}
			num = v
		}

		err := retry.Do0(ctx, 10, retry.Exponential(), func() error {
			bl, err := fetchBlockByNumber(ctx, cl, num)
			if err != nil {
				logger.Warn("Failed to fetch block", "num", num, "err", err)
				return err
			}
			if bl == nil {
				logger.Warn("Cannot find block", "num", num)
				return nil
			}
			return onBlock(ctx, bl)
		})
		if err != nil {
			return err
		}
	}
}

// Entry entry typing between L1 and L2 block downloads,
// for quick scanning of known blocks.
type Entry struct {
	Block struct {
		Number     hexutil.Uint64 `json:"number"`
		Hash       common.Hash    `json:"hash"`
		ParentHash common.Hash    `json:"parentHash"`
	} `json:"block"`
}

func DownloadGaps(ctx context.Context, logger log.Logger, cfg *DownloadConfig,
	onBlock func(ctx context.Context, bl *sources.RPCBlock) error, blocksDir string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cl, err := client.NewRPC(ctx, logger, cfg.Addr,
		client.WithRateLimit(10, 100),
		client.WithDialAttempts(10))

	// block ID -> parent block
	knownBlocks := map[eth.BlockID]eth.BlockID{}

	// Initialize set of known blocks with what we have on disk.
	entries, err := os.ReadDir(blocksDir)
	if err != nil {
		return fmt.Errorf("failed to read existing blocks dir: %w", err)
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		p := filepath.Join(blocksDir, entry.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("failed to read %q: %w", p, err)
		}
		var x Entry
		if err := json.Unmarshal(data, &x); err != nil {
			return fmt.Errorf("failed to read block %s: %w", p, err)
		}
		id := eth.BlockID{
			Hash:   x.Block.Hash,
			Number: uint64(x.Block.Number),
		}
		parentID := eth.BlockID{
			Hash:   x.Block.ParentHash,
			Number: uint64(x.Block.Number) - 1,
		}
		if parentID.Hash == (common.Hash{}) {
			parentID.Number = 0
		}
		knownBlocks[id] = parentID
	}
	logger.Info("Scanned initial list of known blocks to search for gaps",
		"known", len(knownBlocks))
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		unknownBlocks := map[eth.BlockID]struct{}{}
		// find all parent-blocks we don't already know of
		for _, parentID := range knownBlocks {
			if parentID.Number < cfg.StartNum || parentID.Number >= cfg.EndNum {
				continue // don't fetch out-of-range blocks
			}
			if _, ok := knownBlocks[parentID]; !ok {
				unknownBlocks[parentID] = struct{}{}
			}
		}
		if len(unknownBlocks) == 0 {
			break
		}
		logger.Info("Scanned known blocks",
			"unknown", len(unknownBlocks), "known", len(knownBlocks))
		// fetch all unknown blocks.
		for id := range unknownBlocks {
			err := retry.Do0(ctx, 10, retry.Exponential(), func() error {
				bl, err := fetchBlockByHash(ctx, cl, id.Hash)
				if err != nil {
					logger.Warn("Failed to fetch block", "id", id, "err", err)
					return err
				}
				if bl == nil {
					logger.Warn("Cannot find block", "id", id)
					return retryErr // endpoint may be backed by different nodes, one may have the block still
				}
				// mark the block as known
				knownBlocks[id] = eth.BlockID{Hash: bl.ParentHash, Number: uint64(bl.Number) - 1}
				return onBlock(ctx, bl)
			})
			if err != nil {
				if errors.Is(err, retryErr) {
					// If we have retried a lot, but still don't have it,
					// just mark it as known, with reference to itself.
					// We won't retry after this.
					knownBlocks[id] = id
				}
				return err
			}
		}
		// Repeat for remaining unknown blocks.
	}
	return nil
}

var retryErr = errors.New("retry")

// fetchBlockByNumber fetches a block by number.
// This may return nil, nil if the block is not found.
func fetchBlockByNumber(ctx context.Context, cl client.RPC, num uint64) (*sources.RPCBlock, error) {
	var block *sources.RPCBlock
	err := cl.CallContext(ctx, &block, "eth_getBlockByNumber", hexutil.Uint64(num).String(), true)
	if err != nil {
		return nil, err
	}
	return block, nil
}

// fetchBlockByHash fetches a block by hash.
// This may return nil, nil if the block is not found.
func fetchBlockByHash(ctx context.Context, cl client.RPC, hash common.Hash) (*sources.RPCBlock, error) {
	var block *sources.RPCBlock
	err := cl.CallContext(ctx, &block, "eth_getBlockByHash", hash.String(), true)
	if err != nil {
		return nil, err
	}
	return block, nil
}
