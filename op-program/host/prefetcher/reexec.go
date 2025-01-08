package prefetcher

import (
	"context"

	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// NativeReExecuteBlock is a helper function that re-executes a block natively.
// It is used to populate the kv store with the data needed for the program to
// re-derive the block.
func (p *Prefetcher) NativeReExecuteBlock(
	ctx context.Context, blockHash common.Hash) error {
	header, _, err := p.l2Fetcher.InfoAndTxsByHash(ctx, blockHash)
	if err != nil {
		return err
	}
	newCfg := *p.hostConfig
	newCfg.L2ClaimBlockNumber = header.NumberU64()
	p.logger.Info("Re-executing block", "block_hash", blockHash, "block_number", header.NumberU64())
	// No need to set a L2CLaim. The program will derive the blockHash even for an invalid claim.
	// Thus, the kv store is populated with the data we need

	withPrefetcher := hostcommon.WithPrefetcher(
		func(context.Context, log.Logger, kvstore.KV, *config.Config) (hostcommon.Prefetcher, error) {
			return NewPrefetcher(
				p.logger,
				p.l1Fetcher,
				p.l1BlobFetcher,
				p.l2Fetcher,
				p.kvStore,
				p.chainConfig,
				nil, // disable recursive block execution
			), nil
		})
	return hostcommon.FaultProofProgram(ctx, p.logger, &newCfg, withPrefetcher)
}
