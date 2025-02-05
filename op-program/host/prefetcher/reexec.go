package prefetcher

import (
	"context"
	"errors"
	"fmt"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

type ProgramExecutor interface {
	// RunProgram derives the block at the specified blockNumber
	RunProgram(ctx context.Context, prefetcher hostcommon.Prefetcher, blockNumber uint64, chainID eth.ChainID, db l2.KeyValueStore) error
}

// nativeReExecuteBlock is a helper function that re-executes a block natively.
// It is used to populate the kv store with the data needed for the program to
// re-derive the block.
func (p *Prefetcher) nativeReExecuteBlock(
	ctx context.Context, agreedBlockHash, blockHash common.Hash, chainID eth.ChainID) error {
	// Avoid using the retrying source to prevent indefinite retries as the block may not be canonical and unavailable
	source, err := p.l2Sources.ForChainIDWithoutRetries(chainID)
	if err != nil {
		return err
	}
	notFound, err := retry.Do(ctx, maxAttempts, retry.Exponential(), func() (bool, error) {
		_, _, err := source.InfoAndTxsByHash(ctx, blockHash)
		if errors.Is(err, ethereum.NotFound) {
			return true, nil
		}
		if err != nil {
			p.logger.Warn("Failed to retrieve l2 info and txs", "hash", blockHash, "err", err)
		}
		return false, err
	})
	if !notFound && err == nil {
		// we already have the data needed for the program to re-execute
		return nil
	}
	if notFound {
		p.logger.Error("Requested block is not canonical", "block_hash", blockHash, "err", err)
	}
	// Else, i.e. there was an error, then we still want to rebuild the block

	retrying, err := p.l2Sources.ForChainID(chainID)
	if err != nil {
		return err
	}
	header, _, err := retrying.InfoAndTxsByHash(ctx, agreedBlockHash)
	if err != nil {
		return err
	}
	p.logger.Info("Re-executing block", "block_hash", blockHash, "block_number", header.NumberU64())
	if err = p.executor.RunProgram(ctx, p, header.NumberU64()+1, chainID, hostcommon.NewL2KeyValueStore(p.kvStore)); err != nil {
		return err
	}

	// Sanity check that the program execution created the requested block
	if _, err := p.kvStore.Get(preimage.Keccak256Key(blockHash).PreimageKey()); err != nil {
		return fmt.Errorf("cannot find block %v in storage after re-execution", blockHash)
	}
	return nil
}
