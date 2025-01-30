package sources

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
)

type L1ClientConfig struct {
	EthClientConfig
}

func L1ClientDefaultConfig(config *rollup.Config, trustRPC bool, kind RPCProviderKind) *L1ClientConfig {
	// Cache 3/2 worth of sequencing window of receipts and txs
	span := int(config.SeqWindowSize) * 3 / 2
	return L1ClientSimpleConfig(trustRPC, kind, span)
}

func L1ClientSimpleConfig(trustRPC bool, kind RPCProviderKind, cacheSize int) *L1ClientConfig {
	return &L1ClientConfig{
		EthClientConfig: EthClientConfig{
			// receipts and transactions are cached per block
			ReceiptsCacheSize:     cacheSize,
			TransactionsCacheSize: cacheSize,
			HeadersCacheSize:      cacheSize,
			PayloadsCacheSize:     cacheSize,
			MaxRequestsPerBatch:   20, // TODO: tune batch param
			MaxConcurrentRequests: 10,
			TrustRPC:              trustRPC,
			MustBePostMerge:       false,
			RPCProviderKind:       kind,
			MethodResetDuration:   time.Minute,
			BlockRefsCacheSize:    cacheSize,
		},
	}
}

// L1Client provides typed bindings to retrieve L1 data from an RPC source,
// with optimized batch requests, cached results, and flag to not trust the RPC
// (i.e. to verify all returned contents against corresponding block hashes).
type L1Client struct {
	*EthClient
}

// NewL1Client wraps a RPC with bindings to fetch L1 data, while logging errors, tracking metrics (optional), and caching.
func NewL1Client(client client.RPC, log log.Logger, metrics caching.Metrics, config *L1ClientConfig) (*L1Client, error) {
	ethClient, err := NewEthClient(client, log, metrics, &config.EthClientConfig)
	if err != nil {
		return nil, err
	}

	return &L1Client{
		EthClient: ethClient,
	}, nil
}

// L1BlockRefByLabel returns the [eth.L1BlockRef] for the given block label.
// Notice, we cannot cache a block reference by label because labels are not guaranteed to be unique.
func (s *L1Client) L1BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L1BlockRef, error) {
	return s.BlockRefByLabel(ctx, label)
}

// L1BlockRefByNumber returns an [eth.L1BlockRef] for the given block number.
// Notice, we cannot cache a block reference by number because L1 re-orgs can invalidate the cached block reference.
func (s *L1Client) L1BlockRefByNumber(ctx context.Context, num uint64) (eth.L1BlockRef, error) {
	return s.BlockRefByNumber(ctx, num)
}

// L1BlockRefByHash returns the [eth.L1BlockRef] for the given block hash.
// We cache the block reference by hash as it is safe to assume collision will not occur.
func (s *L1Client) L1BlockRefByHash(ctx context.Context, hash common.Hash) (eth.L1BlockRef, error) {
	return s.BlockRefByHash(ctx, hash)
}
