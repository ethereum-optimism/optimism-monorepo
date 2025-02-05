package super

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
	"github.com/ethereum/go-ethereum/common"
)

type ProviderCache struct {
	cache   *caching.LRUCache[common.Hash, types.TraceProvider]
	creator ProposalTraceProviderCreator
}

func (c *ProviderCache) GetOrCreate(ctx context.Context, localContext common.Hash, depth types.Depth, claimInfo ClaimInfo) (types.TraceProvider, error) {
	provider, ok := c.cache.Get(localContext)
	if ok {
		return provider, nil
	}
	provider, err := c.creator(ctx, localContext, depth, claimInfo)
	if err != nil {
		return nil, err
	}
	c.cache.Add(localContext, provider)
	return provider, nil
}

func NewProviderCache(m caching.Metrics, metricsLabel string, creator ProposalTraceProviderCreator) *ProviderCache {
	cache := caching.NewLRUCache[common.Hash, types.TraceProvider](m, metricsLabel, 100)
	return &ProviderCache{
		cache:   cache,
		creator: creator,
	}
}
