package extract

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestL1HeadEnricher(t *testing.T) {
	t.Run("HeaderError", func(t *testing.T) {
		client := &stubBlockFetcher{err: errors.New("boom")}
		enricher := NewL1HeadBlockNumEnricher(client)
		caller := &mockGameCaller{}
		game := &types.EnrichedGameData{}
		err := enricher.Enrich(context.Background(), rpcblock.Latest, caller, game)
		require.ErrorIs(t, err, client.err)
	})

	t.Run("GetBalanceSuccess", func(t *testing.T) {
		client := &stubBlockFetcher{num: 5000}
		enricher := NewL1HeadBlockNumEnricher(client)
		caller := &mockGameCaller{}
		game := &types.EnrichedGameData{}
		err := enricher.Enrich(context.Background(), rpcblock.Latest, caller, game)
		require.NoError(t, err)
		require.Equal(t, client.num, game.L1HeadNum)
	})
}

type stubBlockFetcher struct {
	num uint64
	err error
}

func (s *stubBlockFetcher) L1BlockRefByHash(_ context.Context, _ common.Hash) (eth.L1BlockRef, error) {
	if s.err != nil {
		return eth.L1BlockRef{}, s.err
	}
	return eth.L1BlockRef{
		Number: s.num,
	}, nil
}
