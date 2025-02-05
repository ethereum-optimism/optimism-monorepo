package derive

import (
	"context"
	"io"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestBatchMux_LaterHolocene(t *testing.T) {
	log := testlog.Logger(t, log.LevelTrace)
	ctx := context.Background()
	l1A := eth.L1BlockRef{Time: 0, Hash: common.Hash{0xaa}}
	l1B := eth.L1BlockRef{Time: 12, Hash: common.Hash{0xbb}}
	cfg := &rollup.Config{
		HoloceneTime: &l1B.Time,
	}
	b := NewBatchMux(log, cfg, nil, nil)

	require.Nil(t, b.SingularBatchProvider)

	err := b.Reset(ctx, l1A, eth.SystemConfig{})
	require.Equal(t, io.EOF, err)
	require.IsType(t, new(BatchQueue), b.SingularBatchProvider)
	require.Equal(t, l1A, b.SingularBatchProvider.(*BatchQueue).origin)

	b.Transform(rollup.Holocene)
	require.IsType(t, new(BatchStage), b.SingularBatchProvider)
	require.Equal(t, l1A, b.SingularBatchProvider.(*BatchStage).origin)

	err = b.Reset(ctx, l1B, eth.SystemConfig{})
	require.Equal(t, io.EOF, err)
	require.IsType(t, new(BatchStage), b.SingularBatchProvider)
	require.Equal(t, l1B, b.SingularBatchProvider.(*BatchStage).origin)

	err = b.Reset(ctx, l1A, eth.SystemConfig{})
	require.Equal(t, io.EOF, err)
	require.IsType(t, new(BatchQueue), b.SingularBatchProvider)
	require.Equal(t, l1A, b.SingularBatchProvider.(*BatchQueue).origin)
}

func TestBatchMux_ActiveHolocene(t *testing.T) {
	log := testlog.Logger(t, log.LevelTrace)
	ctx := context.Background()
	l1A := eth.L1BlockRef{Time: 42, Hash: common.Hash{0xaa}}
	cfg := &rollup.Config{
		HoloceneTime: &l1A.Time,
	}
	// without the fake input, the panic check later would panic because of the Origin() call
	prev := &fakeBatchQueueInput{origin: l1A}
	b := NewBatchMux(log, cfg, prev, nil)

	require.Nil(t, b.SingularBatchProvider)

	err := b.Reset(ctx, l1A, eth.SystemConfig{})
	require.Equal(t, io.EOF, err)
	require.IsType(t, new(BatchStage), b.SingularBatchProvider)
	require.Equal(t, l1A, b.SingularBatchProvider.(*BatchStage).origin)

	require.Panics(t, func() { b.Transform(rollup.Holocene) })
}
