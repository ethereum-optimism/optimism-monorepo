package derive

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/mocks"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/params"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestPruneFrameQueue(t *testing.T) {
	for _, tt := range []struct {
		desc     string
		frames   []testFrame
		expected []testFrame
	}{
		{
			desc:     "empty",
			frames:   []testFrame{},
			expected: []testFrame{},
		},
		{
			desc:     "one",
			frames:   []testFrame{"a:2:"},
			expected: []testFrame{"a:2:"},
		},
		{
			desc:     "one-last",
			frames:   []testFrame{"a:2:!"},
			expected: []testFrame{"a:2:!"},
		},
		{
			desc:     "last-new",
			frames:   []testFrame{"a:2:!", "b:0:"},
			expected: []testFrame{"a:2:!", "b:0:"},
		},
		{
			desc:     "last-ooo",
			frames:   []testFrame{"a:2:!", "b:1:"},
			expected: []testFrame{"a:2:!"},
		},
		{
			desc:     "middle-lastooo",
			frames:   []testFrame{"b:1:", "a:2:!"},
			expected: []testFrame{"b:1:"},
		},
		{
			desc:     "middle-first",
			frames:   []testFrame{"b:1:", "a:0:"},
			expected: []testFrame{"a:0:"},
		},
		{
			desc:     "last-first",
			frames:   []testFrame{"b:1:!", "a:0:"},
			expected: []testFrame{"b:1:!", "a:0:"},
		},
		{
			desc:     "last-ooo",
			frames:   []testFrame{"b:1:!", "b:2:"},
			expected: []testFrame{"b:1:!"},
		},
		{
			desc:     "ooo",
			frames:   []testFrame{"b:1:", "b:3:"},
			expected: []testFrame{"b:1:"},
		},
		{
			desc:     "other-ooo",
			frames:   []testFrame{"b:1:", "c:3:"},
			expected: []testFrame{"b:1:"},
		},
		{
			desc:     "other-ooo-last",
			frames:   []testFrame{"b:1:", "c:3:", "b:2:!"},
			expected: []testFrame{"b:1:", "b:2:!"},
		},
		{
			desc:     "ooo-resubmit",
			frames:   []testFrame{"b:1:", "b:3:!", "b:2:", "b:3:!"},
			expected: []testFrame{"b:1:", "b:2:", "b:3:!"},
		},
		{
			desc:     "first-discards-multiple",
			frames:   []testFrame{"c:0:", "c:1:", "c:2:", "d:0:", "c:3:!"},
			expected: []testFrame{"d:0:"},
		},
		{
			desc:     "complex",
			frames:   []testFrame{"b:1:", "b:2:!", "a:0:", "c:1:!", "a:1:", "a:2:!", "c:0:", "c:1:", "d:0:", "c:2:!", "e:0:"},
			expected: []testFrame{"b:1:", "b:2:!", "a:0:", "a:1:", "a:2:!", "e:0:"},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			pfs := pruneFrameQueue(testFramesToFrames(tt.frames...))
			require.Equal(t, testFramesToFrames(tt.expected...), pfs)
		})
	}
}

func TestFrameQueue_NextFrame(t *testing.T) {
	t.Run("pre-holocene", func(t *testing.T) { testFrameQueue_NextFrame(t, false) })
	t.Run("holocene", func(t *testing.T) { testFrameQueue_NextFrame(t, true) })
}

func testFrameQueue_NextFrame(t *testing.T, holocene bool) {
	lgr := testlog.Logger(t, slog.LevelWarn)
	cfg := &rollup.Config{}
	dp := mocks.NewNextDataProvider(t)
	fq := NewFrameQueue(lgr, cfg, dp)

	inFrames := testFramesToFrames("b:1:", "b:2:!", "a:0:", "c:1:!", "a:1:", "a:2:!", "c:0:", "c:1:", "d:0:", "c:2:!", "e:0:")
	var expFrames []Frame
	if holocene {
		cfg.HoloceneTime = ptr(uint64(0))
		// expect pruned frames with Holocene
		expFrames = testFramesToFrames("b:1:", "b:2:!", "a:0:", "a:1:", "a:2:!", "e:0:")
	} else {
		expFrames = inFrames
	}

	var inBuf bytes.Buffer
	inBuf.WriteByte(params.DerivationVersion0)
	for _, f := range inFrames {
		require.NoError(t, f.MarshalBinary(&inBuf))
	}

	dp.On("Origin").Return(eth.L1BlockRef{})
	dp.On("NextData", mock.Anything).Return(inBuf.Bytes(), nil).Once()
	dp.On("NextData", mock.Anything).Return(nil, io.EOF)

	gotFrames := make([]Frame, 0, len(expFrames))
	for i := 0; i <= len(inFrames); i++ { // make sure we hit EOF case
		frame, err := fq.NextFrame(context.Background())
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			break
		}
		require.NoError(t, err)
		gotFrames = append(gotFrames, frame)
	}
	require.Equal(t, expFrames, gotFrames)
}

func ptr[T any](t T) *T { return &t }

func testFramesToFrames(tfs ...testFrame) []Frame {
	fs := make([]Frame, 0, len(tfs))
	for _, f := range tfs {
		fs = append(fs, f.ToFrame())
	}
	return fs
}
