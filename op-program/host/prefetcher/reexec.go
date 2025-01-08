package prefetcher

import (
	"context"

	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum/go-ethereum/common"
)

type ProgramExecutor interface {
	RunProgram(ctx context.Context, prefetcher hostcommon.Prefetcher, blockNumber uint64) error
}

// NativeReExecuteBlock is a helper function that re-executes a block natively.
// It is used to populate the kv store with the data needed for the program to
// re-derive the block.
func (p *Prefetcher) nativeReExecuteBlock(
	ctx context.Context, blockHash common.Hash) error {
	header, _, err := p.l2Fetcher.InfoAndTxsByHash(ctx, blockHash)
	if err != nil {
		return err
	}
	p.logger.Info("Re-executing block", "block_hash", blockHash, "block_number", header.NumberU64())
	// No need to set a L2CLaim. The program will derive the blockHash even for an invalid claim.
	// Thus, the kv store is populated with the data we need
	return p.executor.RunProgram(ctx, p, header.NumberU64())
}
