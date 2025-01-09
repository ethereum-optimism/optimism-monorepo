package prefetcher

import (
	"context"
	"errors"

	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

type ProgramExecutor interface {
	// RunProgram derives the block at the specified blockNumber from the agreedBlockHash
	RunProgram(ctx context.Context, prefetcher hostcommon.Prefetcher, blockNumber uint64, chainID uint64) error
}

// NativeReExecuteBlock is a helper function that re-executes a block natively.
// It is used to populate the kv store with the data needed for the program to
// re-derive the block.
func (p *Prefetcher) nativeReExecuteBlock(
	ctx context.Context, agreedBlockHash, blockHash common.Hash, chainID uint64) error {
	// Avoid retries as the block may not be canonical and unavailable
	_, _, err := p.l2Fetcher.source.InfoAndTxsByHash(ctx, blockHash)
	if err == nil {
		// we already have the data needed for the program to re-execute
		return nil
	}
	if !errors.Is(err, ethereum.NotFound) {
		p.logger.Error("Failed to fetch block", "block_hash", blockHash, "err", err)
	}

	header, _, err := p.l2Fetcher.InfoAndTxsByHash(ctx, agreedBlockHash)
	if err != nil {
		return err
	}
	p.logger.Info("Re-executing block", "block_hash", blockHash, "block_number", header.NumberU64())
	// No need to set a L2CLaim. The program will derive the blockHash even for an invalid claim.
	// Thus, the kv store is populated with the data we need
	return p.executor.RunProgram(ctx, p, header.NumberU64()+1, chainID)
}
