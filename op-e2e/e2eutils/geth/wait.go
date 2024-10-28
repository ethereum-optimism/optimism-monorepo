package geth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	errStrTxIdxingInProgress = "transaction indexing is in progress"
	waitForBlockMaxRetries   = 3
)

// errTimeout represents a timeout
var errTimeout = errors.New("timeout")

func WaitForL1OriginOnL2(rollupCfg *rollup.Config, l1BlockNum uint64, client *ethclient.Client, timeout time.Duration) (*types.Block, error) {
	timeoutCh := time.After(timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	headChan := make(chan *types.Header, 100)
	headSub, err := client.SubscribeNewHead(ctx, headChan)
	if err != nil {
		return nil, err
	}
	defer headSub.Unsubscribe()

	for {
		select {
		case head := <-headChan:
			block, err := client.BlockByNumber(ctx, head.Number)
			if err != nil {
				return nil, err
			}
			l1Info, err := derive.L1BlockInfoFromBytes(rollupCfg, block.Time(), block.Transactions()[0].Data())
			if err != nil {
				return nil, err
			}
			if l1Info.Number >= l1BlockNum {
				return block, nil
			}

		case err := <-headSub.Err():
			return nil, fmt.Errorf("error in head subscription: %w", err)
		case <-timeoutCh:
			return nil, errTimeout
		}
	}
}

func WaitForTransaction(hash common.Hash, client *ethclient.Client, timeout time.Duration) (*types.Receipt, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if receipt != nil && err == nil {
			return receipt, nil
		} else if err != nil &&
			!(errors.Is(err, ethereum.NotFound) || strings.Contains(err.Error(), errStrTxIdxingInProgress)) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			tip, err := client.BlockByNumber(context.Background(), nil)
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("receipt for transaction %s not found. tip block number is %d: %w", hash.Hex(), tip.NumberU64(), errTimeout)
		case <-ticker.C:
		}
	}
}

type waitForBlockOptions struct {
	noChangeTimeout time.Duration
	absoluteTimeout time.Duration
}

func WithNoChangeTimeout(timeout time.Duration) WaitForBlockOption {
	return func(o *waitForBlockOptions) {
		o.noChangeTimeout = timeout
	}
}

func WithAbsoluteTimeout(timeout time.Duration) WaitForBlockOption {
	return func(o *waitForBlockOptions) {
		o.absoluteTimeout = timeout
	}
}

type WaitForBlockOption func(*waitForBlockOptions)

// WaitForBlock waits for the chain to advance to the provided block number. It can be configured with
// two different timeout: an absolute timeout, and a no change timeout. The absolute timeout caps
// the maximum amount of time this method will run. The no change timeout will return an error if the
// block number does not change within that time window. This is useful to bail out early in the event
// of a stuck chain, but allow things to continue if the chain is still advancing.
//
// This function will also retry fetch errors up to three times before returning an error in order to
// protect against transient network problems. This function uses polling rather than websockets.
func WaitForBlock(number *big.Int, client *ethclient.Client, opts ...WaitForBlockOption) (*types.Block, error) {
	defaultOpts := &waitForBlockOptions{
		noChangeTimeout: 30 * time.Second,
		absoluteTimeout: 3 * time.Minute,
	}
	for _, opt := range opts {
		opt(defaultOpts)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultOpts.absoluteTimeout)
	defer cancel()

	lastAdvancement := time.Now()
	lastBlockNumber := big.NewInt(0)

	pollTicker := time.NewTicker(500 * time.Millisecond)
	defer pollTicker.Stop()
	var errCount int

	for {
		head, err := client.BlockByNumber(ctx, nil)
		if err != nil {
			errCount++
			if errCount >= waitForBlockMaxRetries {
				return nil, fmt.Errorf("head fetching exceeded max retries. last error: %w", err)
			}
			continue
		}

		errCount = 0

		if head.Number().Cmp(number) >= 0 {
			return client.BlockByNumber(ctx, number)
		}

		if head.Number().Cmp(lastBlockNumber) != 0 {
			lastBlockNumber = head.Number()
			lastAdvancement = time.Now()
		}

		if time.Since(lastAdvancement) > defaultOpts.noChangeTimeout {
			return nil, fmt.Errorf("block number %d has not changed in %s", lastBlockNumber, defaultOpts.noChangeTimeout)
		}

		select {
		case <-pollTicker.C:
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func WaitForBlockToBeFinalized(number *big.Int, client *ethclient.Client, timeout time.Duration) (*types.Block, error) {
	return waitForBlockTag(number, client, timeout, rpc.FinalizedBlockNumber)
}

func WaitForBlockToBeSafe(number *big.Int, client *ethclient.Client, timeout time.Duration) (*types.Block, error) {
	return waitForBlockTag(number, client, timeout, rpc.SafeBlockNumber)
}

// waitForBlockTag polls for a block number to reach the specified tag & then returns that block at the number.
func waitForBlockTag(number *big.Int, client *ethclient.Client, timeout time.Duration, tag rpc.BlockNumber) (*types.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Wait for it to be finalized. Poll every half second.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	tagBigInt := big.NewInt(tag.Int64())

	for {
		select {
		case <-ticker.C:
			block, err := client.BlockByNumber(ctx, tagBigInt)
			if err != nil {
				// If block is not found (e.g. upon startup of chain, when there is no "finalized block" yet)
				// then it may be found later. Keep wait loop running.
				if strings.Contains(err.Error(), "block not found") {
					continue
				}
				return nil, err
			}
			if block != nil && block.NumberU64() >= number.Uint64() {
				return client.BlockByNumber(ctx, number)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
