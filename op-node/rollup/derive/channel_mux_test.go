package derive

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestChannelMux_LaterHolocene(t *testing.T) {
	log := testlog.Logger(t, log.LevelTrace)
	ctx := context.Background()
	l1A := eth.L1BlockRef{Time: 0, Hash: common.Hash{0xaa}}
	l1B := eth.L1BlockRef{Time: 12, Hash: common.Hash{0xbb}}
	cfg := &rollup.Config{
		HoloceneTime: &l1B.Time,
	}
	spec := rollup.NewChainSpec(cfg)
	m := metrics.NoopMetrics
	c := NewChannelMux(log, spec, nil, m)

	require.Nil(t, c.RawChannelProvider)

	err := c.Reset(ctx, l1A, eth.SystemConfig{})
	require.Equal(t, io.EOF, err)
	require.IsType(t, new(ChannelBank), c.RawChannelProvider)

	c.Transform(rollup.Holocene)
	require.IsType(t, new(ChannelAssembler), c.RawChannelProvider)

	err = c.Reset(ctx, l1B, eth.SystemConfig{})
	require.Equal(t, io.EOF, err)
	require.IsType(t, new(ChannelAssembler), c.RawChannelProvider)

	err = c.Reset(ctx, l1A, eth.SystemConfig{})
	require.Equal(t, io.EOF, err)
	require.IsType(t, new(ChannelBank), c.RawChannelProvider)
}

func TestChannelMux_ActiveHolocene(t *testing.T) {
	log := testlog.Logger(t, log.LevelTrace)
	ctx := context.Background()
	l1A := eth.L1BlockRef{Time: 42, Hash: common.Hash{0xaa}}
	cfg := &rollup.Config{
		HoloceneTime: &l1A.Time,
	}
	spec := rollup.NewChainSpec(cfg)
	// without the fake input, the panic check later would panic because of the Origin() call
	prev := &fakeChannelBankInput{}
	m := metrics.NoopMetrics
	c := NewChannelMux(log, spec, prev, m)

	require.Nil(t, c.RawChannelProvider)

	err := c.Reset(ctx, l1A, eth.SystemConfig{})
	require.Equal(t, io.EOF, err)
	require.IsType(t, new(ChannelAssembler), c.RawChannelProvider)

	require.Panics(t, func() { c.Transform(rollup.Holocene) })
}
