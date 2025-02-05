package l1access

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type mockL1Source struct {
	l1BlockRefByNumberFn func(context.Context, uint64) (eth.L1BlockRef, error)
	l1BlockRefByLabelFn  func(context.Context, eth.BlockLabel) (eth.L1BlockRef, error)
}

func (m *mockL1Source) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	if m.l1BlockRefByNumberFn != nil {
		return m.l1BlockRefByNumberFn(ctx, number)
	}
	return eth.L1BlockRef{}, nil
}

func (m *mockL1Source) L1BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L1BlockRef, error) {
	if m.l1BlockRefByLabelFn != nil {
		return m.l1BlockRefByLabelFn(ctx, label)
	}
	return eth.L1BlockRef{}, nil
}

// TestL1Accessor tests the L1Accessor
// confirming that it can fetch L1BlockRefs by number
// and the confirmation depth is respected
func TestL1Accessor(t *testing.T) {
	log := testlog.Logger(t, slog.LevelDebug)
	source := &mockL1Source{}
	source.l1BlockRefByNumberFn = func(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
		return eth.L1BlockRef{
			Number: number,
		}, nil
	}
	accessor := NewL1Accessor(context.Background(), log, source)
	accessor.tip = eth.BlockID{Number: 10}

	// Test L1BlockRefByNumber
	ref, err := accessor.L1BlockRefByNumber(context.Background(), 5)
	require.NoError(t, err)
	require.Equal(t, uint64(5), ref.Number)

	// Test L1BlockRefByNumber with number in excess of tip height
	ref, err = accessor.L1BlockRefByNumber(context.Background(), 9)
	require.Error(t, err)

	// attach a new source
	source2 := &mockL1Source{}
	accessor.AttachClient(source2, false)
	require.Equal(t, source2, accessor.client)

}
