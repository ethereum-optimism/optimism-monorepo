package derive

import (
	"bytes"
	"io"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/params"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func FuzzFrameUnmarshalBinary(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		buf := bytes.NewBuffer(data)
		var f Frame
		_ = (&f).UnmarshalBinary(buf)
	})
}

func FuzzParseFrames(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		frames, err := ParseFrames(data)
		if err != nil && len(frames) != 0 {
			t.Fatal("non-nil error with an amount of return data")
		} else if err == nil && len(frames) == 0 {
			t.Fatal("must return data with a non-nil error")
		}
	})
}

func TestFrameMarshaling(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 16; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			frame := randomFrame(rng)
			var data bytes.Buffer
			require.NoError(t, frame.MarshalBinary(&data))

			frame0 := new(Frame)
			require.NoError(t, frame0.UnmarshalBinary(&data))
			require.Equal(t, frame, frame0)
		})
	}
}

func TestFrameUnmarshalNoData(t *testing.T) {
	frame0 := new(Frame)
	err := frame0.UnmarshalBinary(bytes.NewReader([]byte{}))
	require.Error(t, err)
	require.ErrorIs(t, err, io.EOF)
}

func TestFrameUnmarshalTruncated(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 16 (channel_id) ++ 2 (frame_number) ++ 4 (frame_data_length) ++
	// frame_data_length (frame_data) ++ 1 (is_last)
	for _, tr := range []struct {
		desc     string
		truncate func([]byte) []byte
		genData  bool // whether data should be generated
	}{
		{
			desc: "truncate-channel_id-half",
			truncate: func(data []byte) []byte {
				return data[:8]
			},
		},
		{
			desc: "truncate-frame_number-full",
			truncate: func(data []byte) []byte {
				return data[:16]
			},
		},
		{
			desc: "truncate-frame_number-half",
			truncate: func(data []byte) []byte {
				return data[:17]
			},
		},
		{
			desc: "truncate-frame_data_length-full",
			truncate: func(data []byte) []byte {
				return data[:18]
			},
		},
		{
			desc: "truncate-frame_data_length-half",
			truncate: func(data []byte) []byte {
				return data[:20]
			},
			genData: true, // for non-zero frame_data_length
		},
		{
			desc: "truncate-frame_data-full",
			truncate: func(data []byte) []byte {
				return data[:22]
			},
			genData: true, // for non-zero frame_data_length
		},
		{
			desc: "truncate-frame_data-last-byte",
			truncate: func(data []byte) []byte {
				return data[:len(data)-2]
			},
			genData: true,
		},
		{
			desc: "truncate-is_last",
			truncate: func(data []byte) []byte {
				return data[:len(data)-1]
			},
			genData: true,
		},
	} {
		t.Run(tr.desc, func(t *testing.T) {
			var opts []frameOpt
			if !tr.genData {
				opts = []frameOpt{frameWithDataLen(0)}
			}
			frame := randomFrame(rng, opts...)
			var data bytes.Buffer
			require.NoError(t, frame.MarshalBinary(&data))

			tdata := tr.truncate(data.Bytes())

			frame0 := new(Frame)
			err := frame0.UnmarshalBinary(bytes.NewReader(tdata))
			require.Error(t, err)
			require.ErrorIs(t, err, io.ErrUnexpectedEOF)
		})
	}
}

func TestFrameUnmarshalInvalidIsLast(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	frame := randomFrame(rng, frameWithDataLen(16))
	var data bytes.Buffer
	require.NoError(t, frame.MarshalBinary(&data))

	idata := data.Bytes()
	idata[len(idata)-1] = 2 // invalid is_last

	frame0 := new(Frame)
	err := frame0.UnmarshalBinary(bytes.NewReader(idata))
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid byte")
}

func TestParseFramesNoData(t *testing.T) {
	frames, err := ParseFrames(nil)
	require.Empty(t, frames)
	require.Error(t, err)
}

func TestParseFramesInvalidVer(t *testing.T) {
	frames, err := ParseFrames([]byte{42})
	require.Empty(t, frames)
	require.Error(t, err)
}

func TestParseFramesOnlyVersion(t *testing.T) {
	frames, err := ParseFrames([]byte{params.DerivationVersion0})
	require.Empty(t, frames)
	require.Error(t, err)
}

func TestParseFrames(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	numFrames := rng.Intn(16) + 1
	frames := make([]Frame, 0, numFrames)
	for i := 0; i < numFrames; i++ {
		frames = append(frames, *randomFrame(rng))
	}
	data, err := txMarshalFrames(frames)
	require.NoError(t, err)

	frames0, err := ParseFrames(data)
	require.NoError(t, err)
	require.Equal(t, frames, frames0)
}

func TestParseFramesTruncated(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	numFrames := rng.Intn(16) + 1
	frames := make([]Frame, 0, numFrames)
	for i := 0; i < numFrames; i++ {
		frames = append(frames, *randomFrame(rng))
	}
	data, err := txMarshalFrames(frames)
	require.NoError(t, err)
	data = data[:len(data)-2] // truncate last 2 bytes

	frames0, err := ParseFrames(data)
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Empty(t, frames0)
}

// txMarshalFrames creates the tx payload for the given frames, i.e., it first
// writes the version byte to a buffer and then appends all binary-marshaled
// frames.
func txMarshalFrames(frames []Frame) ([]byte, error) {
	var data bytes.Buffer
	if err := data.WriteByte(params.DerivationVersion0); err != nil {
		return nil, err
	}
	for _, frame := range frames {
		if err := frame.MarshalBinary(&data); err != nil {
			return nil, err
		}
	}
	return data.Bytes(), nil
}

func randomFrame(rng *rand.Rand, opts ...frameOpt) *Frame {
	var id ChannelID
	_, err := rng.Read(id[:])
	if err != nil {
		panic(err)
	}

	frame := &Frame{
		ID:          id,
		FrameNumber: uint16(rng.Int31n(math.MaxUint16 + 1)),
		IsLast:      testutils.RandomBool(rng),
	}

	// evaluate options
	for _, opt := range opts {
		opt(rng, frame)
	}

	// default if no option set field
	if frame.Data == nil {
		datalen := int(rng.Intn(MaxFrameLen + 1))
		frame.Data = testutils.RandomData(rng, datalen)
	}

	return frame
}

type frameOpt func(*rand.Rand, *Frame)

func frameWithDataLen(l int) frameOpt {
	return func(rng *rand.Rand, frame *Frame) {
		frame.Data = testutils.RandomData(rng, l)
	}
}
