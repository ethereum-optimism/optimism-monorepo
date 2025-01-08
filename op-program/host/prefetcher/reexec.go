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
func NativeReExecuteBlock(
	ctx context.Context, prefetcher *Prefetcher, blockHash common.Hash) error {
	header, _, err := prefetcher.l2Fetcher.InfoAndTxsByHash(ctx, blockHash)
	if err != nil {
		return err
	}
	newCfg := *prefetcher.hostConfig
	newCfg.L2ClaimBlockNumber = header.NumberU64()
	// No need to set a L2CLaim. The program will derive the blockHash even for an invalid claim.
	// Thus, the kv store is populated with the data we need

	withPrefetcher := hostcommon.WithPrefetcher(
		func(context.Context, log.Logger, kvstore.KV, *config.Config) (hostcommon.Prefetcher, error) {
			return NewPrefetcher(
				prefetcher.logger,
				prefetcher.l1Fetcher,
				prefetcher.l1BlobFetcher,
				prefetcher.l2Fetcher,
				prefetcher.kvStore,
				prefetcher.chainConfig,
				nil, // disable recursive block execution
			), nil
		})
	return hostcommon.FaultProofProgram(ctx, prefetcher.logger, &newCfg, withPrefetcher)
}
