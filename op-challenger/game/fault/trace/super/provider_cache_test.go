package super

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/alphabet"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestProviderCache(t *testing.T) {
	claimInfo := ClaimInfo{
		AgreedPrestate: []byte{1, 2, 3, 4},
		Claim:          common.Hash{0xaa, 0x66},
	}
	depth := types.Depth(6)
	var createdProvider types.TraceProvider
	creator := func(ctx context.Context, localContext common.Hash, depth types.Depth, claimInfo ClaimInfo) (types.TraceProvider, error) {
		createdProvider = alphabet.NewTraceProvider(big.NewInt(0), depth)
		return createdProvider, nil
	}
	localContext1 := common.Hash{0xdd}
	localContext2 := common.Hash{0xee}

	cache := NewProviderCache(metrics.NoopMetrics, "test", creator)

	// Create on first call
	provider1, err := cache.GetOrCreate(context.Background(), localContext1, depth, claimInfo)
	require.NoError(t, err)
	require.Same(t, createdProvider, provider1, "should return created trace provider")

	// Return the cached provider on subsequent calls.
	createdProvider = nil
	cached, err := cache.GetOrCreate(context.Background(), localContext1, depth, claimInfo)
	require.NoError(t, err)
	require.Same(t, provider1, cached, "should return exactly the same instance from cache")
	require.Nil(t, createdProvider)

	// Create a new provider when the local context is different
	createdProvider = nil
	otherProvider, err := cache.GetOrCreate(context.Background(), localContext2, depth, claimInfo)
	require.NoError(t, err)
	require.Same(t, otherProvider, createdProvider, "should return newly created trace provider")
	require.NotSame(t, otherProvider, provider1, "should not use cached provider for different local context")
}

func TestProviderCache_DoNotCacheErrors(t *testing.T) {
	callCount := 0
	providerErr := errors.New("boom")
	creator := func(ctx context.Context, localContext common.Hash, depth types.Depth, claimInfo ClaimInfo) (types.TraceProvider, error) {
		callCount++
		return nil, providerErr
	}
	localContext1 := common.Hash{0xdd}

	cache := NewProviderCache(metrics.NoopMetrics, "test", creator)
	provider, err := cache.GetOrCreate(context.Background(), localContext1, 6, ClaimInfo{})
	require.Nil(t, provider)
	require.ErrorIs(t, err, providerErr)
	require.Equal(t, 1, callCount)

	// Should call the creator again on the second attempt
	provider, err = cache.GetOrCreate(context.Background(), localContext1, 6, ClaimInfo{})
	require.Nil(t, provider)
	require.ErrorIs(t, err, providerErr)
	require.Equal(t, 2, callCount)
}
