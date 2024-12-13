package derive

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	rolluptest "github.com/ethereum-optimism/optimism/op-node/rollup/test"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestChannelStage_NextData(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		frames      [][]testFrame
		expErr      []error
		expData     []string
		expChID     []string
		rlpOverride *uint64
	}{
		{
			desc: "simple",
			frames: [][]testFrame{
				{"a:0:first!"},
			},
			expErr:  []error{nil},
			expData: []string{"first"},
			expChID: []string{""},
		},
		{
			desc: "simple-two",
			frames: [][]testFrame{
				{"a:0:first", "a:1:second!"},
			},
			expErr:  []error{nil},
			expData: []string{"firstsecond"},
			expChID: []string{""},
		},
		{
			desc: "drop-other",
			frames: [][]testFrame{
				{"a:0:first", "b:1:foo"},
				{"a:1:second", "c:1:bar!"},
				{"a:2:third!"},
			},
			expErr:  []error{io.EOF, io.EOF, nil},
			expData: []string{"", "", "firstsecondthird"},
			expChID: []string{"a", "a", ""},
		},
		{
			desc: "drop-non-first",
			frames: [][]testFrame{
				{"a:1:foo"},
			},
			expErr:  []error{io.EOF},
			expData: []string{""},
			expChID: []string{""},
		},
		{
			desc: "first-discards",
			frames: [][]testFrame{
				{"b:0:foo"},
				{"a:0:first!"},
			},
			expErr:  []error{io.EOF, nil},
			expData: []string{"", "first"},
			expChID: []string{"b", ""},
		},
		{
			desc: "already-closed",
			frames: [][]testFrame{
				{"a:0:foo"},
				{"a:1:bar!", "a:2:baz!"},
			},
			expErr:  []error{io.EOF, nil},
			expData: []string{"", "foobar"},
			expChID: []string{"a", ""},
		},
		{
			desc: "max-size",
			frames: [][]testFrame{
				{"a:0:0123456789!"},
			},
			expErr:      []error{nil},
			expData:     []string{"0123456789"},
			expChID:     []string{""},
			rlpOverride: ptr[uint64](frameOverhead + 10),
		},
		{
			desc: "oversized",
			frames: [][]testFrame{
				{"a:0:0123456789x!"},
			},
			expErr:      []error{io.EOF},
			expData:     []string{""},
			expChID:     []string{""},
			rlpOverride: ptr[uint64](frameOverhead + 10),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fq := &fakeChannelBankInput{}
			lgr := testlog.Logger(t, slog.LevelWarn)
			spec := &rolluptest.ChainSpec{
				ChainSpec: rollup.NewChainSpec(&rollup.Config{}),

				MaxRLPBytesPerChannelOverride: tc.rlpOverride,
			}
			cs := NewChannelAssembler(lgr, spec, fq, metrics.NoopMetrics)

			for i, fs := range tc.frames {
				fq.AddFrames(fs...)
				data, err := cs.NextRawChannel(context.Background())
				require.Equal(t, tc.expData[i], string(data))
				require.ErrorIs(t, tc.expErr[i], err)
				// invariant: never holds a ready channel
				require.True(t, cs.channel == nil || !cs.channel.IsReady())

				cid := tc.expChID[i]
				if cid == "" {
					require.Nil(t, cs.channel)
				} else {
					require.Equal(t, strChannelID(cid), cs.channel.ID())
				}
			}

			// final call should always be io.EOF after exhausting frame queue
			data, err := cs.NextRawChannel(context.Background())
			require.Nil(t, data)
			require.Equal(t, io.EOF, err)
		})
	}
}

func TestChannelStage_NextData_Timeout(t *testing.T) {
	require := require.New(t)
	fq := &fakeChannelBankInput{}
	lgr := testlog.Logger(t, slog.LevelWarn)
	spec := rollup.NewChainSpec(&rollup.Config{GraniteTime: ptr(uint64(0))}) // const channel timeout
	cs := NewChannelAssembler(lgr, spec, fq, metrics.NoopMetrics)

	fq.AddFrames("a:0:foo")
	data, err := cs.NextRawChannel(context.Background())
	require.Nil(data)
	require.Equal(io.EOF, err)
	require.NotNil(cs.channel)
	require.Equal(strChannelID("a"), cs.channel.ID())

	// move close to timeout
	fq.origin.Number = spec.ChannelTimeout(0)
	fq.AddFrames("a:1:bar")
	data, err = cs.NextRawChannel(context.Background())
	require.Nil(data)
	require.Equal(io.EOF, err)
	require.NotNil(cs.channel)
	require.Equal(strChannelID("a"), cs.channel.ID())

	// timeout channel by moving origin past timeout
	fq.origin.Number = spec.ChannelTimeout(0) + 1
	fq.AddFrames("a:2:baz!")
	data, err = cs.NextRawChannel(context.Background())
	require.Nil(data)
	require.Equal(io.EOF, err)
	require.Nil(cs.channel)
}
