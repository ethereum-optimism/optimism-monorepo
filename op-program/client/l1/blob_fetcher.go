package l1

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

var InvalidHashesLengthError = errors.New("invalid hashes length")

type BlobFetcher struct {
	logger log.Logger
	oracle Oracle
}

var _ derive.L1BlobsFetcher = (*BlobFetcher)(nil)

func NewBlobFetcher(logger log.Logger, oracle Oracle) *BlobFetcher {
	return &BlobFetcher{
		logger: logger,
		oracle: oracle,
	}
}

// GetBlobs fetches blobs that were confirmed in the given L1 block with the given indexed blob hashes.
func (b *BlobFetcher) GetBlobs(ctx context.Context, ref eth.L1BlockRef, hashes []eth.IndexedBlobHash) ([]*eth.Blob, error) {
	blobs := make([]*eth.Blob, len(hashes))
	for i := 0; i < len(hashes); i++ {
		b.logger.Info("Fetching blob", "l1_ref", ref.Hash, "blob_versioned_hash", hashes[i].Hash, "index", hashes[i].Index)
		blobs[i] = b.oracle.GetBlob(ref, hashes[i])
	}
	return blobs, nil
}
