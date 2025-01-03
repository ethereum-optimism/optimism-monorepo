package utils

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
)

type ReassembleConfig struct {
	Datadir string
}

func Reassemble(ctx context.Context, logger log.Logger, cfg *ReassembleConfig) error {
	// TODO: v2 of batch-decoder reassemble functionality, with better indexing of blocks,
	// and re-use the L1-block-fetching results, rather than fetching everything last-minute with singular try.
	// With chain divergence and long ranges of blocks I believe this is important.

	// TODO iterate over all L1 blocks (sorted)
	// TODO load all txs, return sorted list of frames
	// TODO assemble channels from frames, tag with completion L1 block
	// TODO decode batches from channels, tag with completion L1 block
	return nil
}
