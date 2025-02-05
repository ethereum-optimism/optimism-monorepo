package derive

import (
	"context"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type fakeChannelBankInput struct {
	origin eth.L1BlockRef
	data   []struct {
		frame Frame
		err   error
	}
}

func (f *fakeChannelBankInput) Origin() eth.L1BlockRef {
	return f.origin
}

func (f *fakeChannelBankInput) NextFrame(_ context.Context) (Frame, error) {
	if len(f.data) == 0 {
		return Frame{}, io.EOF
	}
	out := f.data[0]
	f.data = f.data[1:]
	return out.frame, out.err
}

func (f *fakeChannelBankInput) AddFrame(frame Frame, err error) {
	f.data = append(f.data, struct {
		frame Frame
		err   error
	}{frame: frame, err: err})
}

// ExpectNextFrameData takes a set of test frame & turns into the raw data
// for reading into the channel bank via `NextData`
func (f *fakeChannelBankInput) AddFrames(frames ...testFrame) {
	for _, frame := range frames {
		f.AddFrame(frame.ToFrame(), nil)
	}
}

var _ NextFrameProvider = (*fakeChannelBankInput)(nil)

// format: <channelID-data>:<frame-number>:<content><optional-last-frame-marker "!">
// example: "abc:0:helloworld!"
type testFrame string

func (tf testFrame) ChannelID() ChannelID {
	parts := strings.Split(string(tf), ":")
	return strChannelID(parts[0])
}

func strChannelID(s string) ChannelID {
	var chID ChannelID
	copy(chID[:], s)
	return chID
}

func (tf testFrame) FrameNumber() uint64 {
	parts := strings.Split(string(tf), ":")
	frameNum, err := strconv.ParseUint(parts[1], 0, 64)
	if err != nil {
		panic(err)
	}
	return frameNum
}

func (tf testFrame) IsLast() bool {
	parts := strings.Split(string(tf), ":")
	return strings.HasSuffix(parts[2], "!")
}

func (tf testFrame) Content() []byte {
	parts := strings.Split(string(tf), ":")
	return []byte(strings.TrimSuffix(parts[2], "!"))
}

func (tf testFrame) ToFrame() Frame {
	return Frame{
		ID:          tf.ChannelID(),
		FrameNumber: uint16(tf.FrameNumber()),
		Data:        tf.Content(),
		IsLast:      tf.IsLast(),
	}
}

func TestChannelBankSimple(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	a := testutils.RandomBlockRef(rng)

	input := &fakeChannelBankInput{origin: a}
	input.AddFrames("a:0:first", "a:2:third!")
	input.AddFrames("a:1:second")

	spec := rollup.NewChainSpec(&rollup.Config{ChannelTimeoutBedrock: 10})
	cb := NewChannelBank(testlog.Logger(t, log.LevelCrit), spec, input, metrics.NoopMetrics)

	// Load the first frame
	out, err := cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load the third frame
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load the second frame
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Pull out the channel data
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, err)
	require.Equal(t, "firstsecondthird", string(out))

	// No more data
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, out)
	require.Equal(t, io.EOF, err)
}

// TestChannelBankInterleavedPreCanyon ensure that the channel bank can handle frames from multiple channels
// that arrive out of order. Per the specs, the first channel to arrive (not the first to be completed)
// is returned first prior to the Canyon network upgrade
func TestChannelBankInterleavedPreCanyon(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	a := testutils.RandomBlockRef(rng)

	input := &fakeChannelBankInput{origin: a}
	input.AddFrames("a:0:first", "b:2:trois!")
	input.AddFrames("b:1:deux", "a:2:third!")
	input.AddFrames("b:0:premiere")
	input.AddFrames("a:1:second")

	spec := rollup.NewChainSpec(&rollup.Config{ChannelTimeoutBedrock: 10})
	cb := NewChannelBank(testlog.Logger(t, log.LevelCrit), spec, input, metrics.NoopMetrics)

	// Load a:0
	out, err := cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load b:2
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load b:1
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load a:2
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load b:0 & Channel b is complete, but channel a was opened first
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load a:1
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Pull out the channel a
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, err)
	require.Equal(t, "firstsecondthird", string(out))

	// Pull out the channel b
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, err)
	require.Equal(t, "premieredeuxtrois", string(out))

	// No more data
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, out)
	require.Equal(t, io.EOF, err)
}

// TestChannelBankInterleaved ensure that the channel bank can handle frames from multiple channels
// that arrive out of order. Per the specs (post Canyon), the first channel to be complete should be
// returned
func TestChannelBankInterleaved(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	a := testutils.RandomBlockRef(rng)

	input := &fakeChannelBankInput{origin: a}
	input.AddFrames("a:0:first", "b:2:trois!")
	input.AddFrames("b:1:deux", "a:2:third!")
	input.AddFrames("b:0:premiere")
	input.AddFrames("a:1:second")

	ct := uint64(0)
	spec := rollup.NewChainSpec(&rollup.Config{ChannelTimeoutBedrock: 10, CanyonTime: &ct})
	cb := NewChannelBank(testlog.Logger(t, log.LevelCrit), spec, input, metrics.NoopMetrics)

	// Load a:0
	out, err := cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load b:2
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load b:1
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load a:2
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load b:0 & Channel b is complete. Channel a was opened first but isn't ready
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Pull out the channel b because it's ready first.
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, err)
	require.Equal(t, "premieredeuxtrois", string(out))

	// Load a:1
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Pull out the channel a
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, err)
	require.Equal(t, "firstsecondthird", string(out))

	// No more data
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, out)
	require.Equal(t, io.EOF, err)
}

func TestChannelBankDuplicates(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	a := testutils.RandomBlockRef(rng)

	input := &fakeChannelBankInput{origin: a}
	input.AddFrames("a:0:first", "a:2:third!")
	input.AddFrames("a:0:altfirst", "a:2:altthird!")
	input.AddFrames("a:1:second")

	spec := rollup.NewChainSpec(&rollup.Config{ChannelTimeoutBedrock: 10})
	cb := NewChannelBank(testlog.Logger(t, log.LevelCrit), spec, input, metrics.NoopMetrics)

	// Load the first frame
	out, err := cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load the third frame
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load the duplicate frames
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Load the second frame
	out, err = cb.NextRawChannel(context.Background())
	require.ErrorIs(t, err, NotEnoughData)
	require.Equal(t, []byte(nil), out)

	// Pull out the channel data. Expect to see the original set & not the duplicates
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, err)
	require.Equal(t, "firstsecondthird", string(out))

	// No more data
	out, err = cb.NextRawChannel(context.Background())
	require.Nil(t, out)
	require.Equal(t, io.EOF, err)
}
