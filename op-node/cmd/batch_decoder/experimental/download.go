package experimental

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

type DownloadConfig struct {
	OutDir             string
	StartNum           uint64
	EndNum             uint64
	Addr               string
	ConcurrentRequests uint64
}

func Download(ctx context.Context, logger log.Logger, cfg *DownloadConfig,
	onBlock func(ctx context.Context, bl *sources.RPCBlock) error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Produce work to run
	work := make(chan uint64)
	go func() {
		for i := cfg.StartNum; i < cfg.EndNum; i++ {
			select {
			case work <- i:
			case <-ctx.Done():
				return
			}
			if count := i - cfg.StartNum; count%100 == 0 {
				logger.Info("scheduled download work", "count", count)
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

	// collect the results. Abort if any error.
	for wi := uint64(0); wi < cfg.ConcurrentRequests; wi++ {
		err := <-results
		if err != nil {
			cancel() // cancel all other work, no point in finishing it anymore
			logger.Error("Download worker failed, aborting now", "err", err)
		}
	}
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
