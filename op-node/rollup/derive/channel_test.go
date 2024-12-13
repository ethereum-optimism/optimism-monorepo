package derive

import (
	"bytes"
	"compress/zlib"
	"math/big"
	"math/rand"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

type frameValidityTC struct {
	name      string
	frames    []Frame
	shouldErr []bool
	sizes     []uint64
	holocene  bool
}

func (tc *frameValidityTC) Run(t *testing.T) {
	id := [16]byte{0xff}
	block := eth.L1BlockRef{}
	ch := NewChannel(id, block, tc.holocene)

	if len(tc.frames) != len(tc.shouldErr) || len(tc.frames) != len(tc.sizes) {
		t.Errorf("lengths should be the same. frames: %d, shouldErr: %d, sizes: %d", len(tc.frames), len(tc.shouldErr), len(tc.sizes))
	}

	for i, frame := range tc.frames {
		err := ch.AddFrame(frame, block)
		if tc.shouldErr[i] {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, tc.sizes[i], ch.Size())
	}
}

// TestFrameValidity inserts a list of frames into the channel. It checks if an error
// should be returned by `AddFrame` as well as checking the size of the channel.
func TestFrameValidity(t *testing.T) {
	id := [16]byte{0xff}
	testCases := []frameValidityTC{
		{
			name:      "wrong channel",
			frames:    []Frame{{ID: [16]byte{0xee}}},
			shouldErr: []bool{true},
			sizes:     []uint64{0},
		},
		{
			name: "double close",
			frames: []Frame{
				{ID: id, FrameNumber: 2, IsLast: true, Data: []byte("four")},
				{ID: id, FrameNumber: 1, IsLast: true},
			},
			shouldErr: []bool{false, true},
			sizes:     []uint64{204, 204},
		},
		{
			name: "duplicate frame",
			frames: []Frame{
				{ID: id, FrameNumber: 2, Data: []byte("four")},
				{ID: id, FrameNumber: 2, Data: []byte("seven__")},
			},
			shouldErr: []bool{false, true},
			sizes:     []uint64{204, 204},
		},
		{
			name: "duplicate closing frames",
			frames: []Frame{
				{ID: id, FrameNumber: 2, IsLast: true, Data: []byte("four")},
				{ID: id, FrameNumber: 2, IsLast: true, Data: []byte("seven__")},
			},
			shouldErr: []bool{false, true},
			sizes:     []uint64{204, 204},
		},
		{
			name: "frame past closing",
			frames: []Frame{
				{ID: id, FrameNumber: 2, IsLast: true, Data: []byte("four")},
				{ID: id, FrameNumber: 10, Data: []byte("seven__")},
			},
			shouldErr: []bool{false, true},
			sizes:     []uint64{204, 204},
		},
		{
			name: "prune after close frame",
			frames: []Frame{
				{ID: id, FrameNumber: 10, IsLast: false, Data: []byte("seven__")},
				{ID: id, FrameNumber: 2, IsLast: true, Data: []byte("four")},
			},
			shouldErr: []bool{false, false},
			sizes:     []uint64{207, 204},
		},
		{
			name: "multiple valid frames",
			frames: []Frame{
				{ID: id, FrameNumber: 10, Data: []byte("seven__")},
				{ID: id, FrameNumber: 2, Data: []byte("four")},
			},
			shouldErr: []bool{false, false},
			sizes:     []uint64{207, 411},
		},
		{
			name:     "holocene non first",
			holocene: true,
			frames: []Frame{
				{ID: id, FrameNumber: 2, Data: []byte("four")},
			},
			shouldErr: []bool{true},
			sizes:     []uint64{0},
		},
		{
			name:     "holocene out of order",
			holocene: true,
			frames: []Frame{
				{ID: id, FrameNumber: 0, Data: []byte("four")},
				{ID: id, FrameNumber: 2, Data: []byte("seven__")},
			},
			shouldErr: []bool{false, true},
			sizes:     []uint64{204, 204},
		},
		{
			name:     "holocene in order",
			holocene: true,
			frames: []Frame{
				{ID: id, FrameNumber: 0, Data: []byte("four")},
				{ID: id, FrameNumber: 1, Data: []byte("seven__")},
				{ID: id, FrameNumber: 2, IsLast: true, Data: []byte("2_")},
			},
			shouldErr: []bool{false, false, false},
			sizes:     []uint64{204, 411, 613},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.Run)
	}
}

func TestBatchReader(t *testing.T) {
	rng := rand.New(rand.NewSource(0x543331))

	batchCount := 3
	batches := make([]*BatchData, batchCount)
	for i := 0; i < batchCount; i++ {
		singularBatch := RandomSingularBatch(rng, 20, big.NewInt(333))
		batchDataInput := NewBatchData(singularBatch)
		batches[i] = batchDataInput
	}

	encodedBatch := new(bytes.Buffer)
	for _, batchData := range batches {
		err := batchData.EncodeRLP(encodedBatch)
		require.NoError(t, err)
	}

	const Zstd CompressionAlgo = "zstd" // invalid algo
	compressor := func(ca CompressionAlgo) func(buf *bytes.Buffer, t *testing.T) {
		switch {
		case ca == Zlib:
			return func(buf *bytes.Buffer, t *testing.T) {
				writer := zlib.NewWriter(buf)
				_, err := writer.Write(encodedBatch.Bytes())
				require.NoError(t, err)
				require.NoError(t, writer.Close())
			}
		case ca.IsBrotli():
			return func(buf *bytes.Buffer, t *testing.T) {
				buf.WriteByte(ChannelVersionBrotli)
				lvl := GetBrotliLevel(ca)
				writer := brotli.NewWriterLevel(buf, lvl)
				_, err := writer.Write(encodedBatch.Bytes())
				require.NoError(t, err)
				require.NoError(t, writer.Close())
			}
		case ca == Zstd: // invalid algo
			return func(buf *bytes.Buffer, t *testing.T) {
				buf.WriteByte(0x02) // invalid channel version byte
				writer, err := zstd.NewWriter(buf)
				require.NoError(t, err)
				_, err = writer.Write(encodedBatch.Bytes())
				require.NoError(t, err)
				require.NoError(t, writer.Close())
			}
		default:
			panic("unexpected test algo")
		}
	}

	testCases := []struct {
		name      string
		algo      CompressionAlgo
		isFjord   bool
		expectErr bool
	}{
		{
			name:    "zlib-post-fjord",
			algo:    Zlib,
			isFjord: true,
		},
		{
			name:    "zlib-pre-fjord",
			algo:    Zlib,
			isFjord: false,
		},
		{
			name:    "brotli-post-fjord",
			algo:    Brotli,
			isFjord: true,
		},
		{
			name:      "brotli-pre-fjord",
			algo:      Brotli,
			isFjord:   false,
			expectErr: true, // expect an error because brotli is not supported before Fjord
		},
		{
			name:    "brotli9-post-fjord",
			algo:    Brotli9,
			isFjord: true,
		},
		{
			name:      "brotli9-pre-fjord",
			algo:      Brotli9,
			isFjord:   false,
			expectErr: true, // expect an error because brotli is not supported before Fjord
		},
		{
			name:    "brotli10-post-fjord",
			algo:    Brotli10,
			isFjord: true,
		},
		{
			name:    "brotli11-post-fjord",
			algo:    Brotli11,
			isFjord: true,
		},
		{
			name:      "zstd-post-fjord",
			algo:      Zstd,
			expectErr: true,
			isFjord:   true,
		},
	}

	for _, tc := range testCases {
		compressed := new(bytes.Buffer)
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			compressor(tc.algo)(compressed, t)
			reader, err := BatchReader(bytes.NewReader(compressed.Bytes()), 120000, tc.isFjord)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			for i := 0; i < batchCount; i++ {
				batchData, err := reader()
				require.NoError(t, err)
				require.NotNil(t, batchData)
				if tc.algo.IsBrotli() {
					// special case because reader doesn't decode level
					batches[i].ComprAlgo = Brotli
				} else {
					batches[i].ComprAlgo = tc.algo
				}
				require.Equal(t, batches[i], batchData)
			}

			// further read should error out
			_, err = reader()
			require.Error(t, err)
		})
	}
}
