package compressor

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
)

type RatioCompressor struct {
	config Config

	inputBytes int
	compressor derive.ChannelCompressor
}

// NewRatioCompressor creates a new derive.Compressor implementation that uses the target
// size and a compression ratio parameter to determine how much data can be written to
// the compressor before it's considered full. The full calculation is as follows:
//
//	full = uncompressedLength * ApproxComprRatio >= targetFrameSize * targetNumFrames
func NewRatioCompressor(config Config) (derive.Compressor, error) {
	c := &RatioCompressor{
		config: config,
	}

	compressor, err := derive.NewChannelCompressor(config.CompressionAlgo)
	if err != nil {
		return nil, err
	}
	c.compressor = compressor

	return c, nil
}

func (t *RatioCompressor) Write(p []byte) (int, error) {
	if err := t.FullErr(); err != nil {
		return 0, err
	}
	t.inputBytes += len(p)
	return t.compressor.Write(p)
}

func (t *RatioCompressor) Close() error {
	return t.compressor.Close()
}

func (t *RatioCompressor) Read(p []byte) (int, error) {
	return t.compressor.Read(p)
}

func (t *RatioCompressor) Reset() {
	t.compressor.Reset()
	t.inputBytes = 0
}

func (t *RatioCompressor) Len() int {
	return t.compressor.Len()
}

func (t *RatioCompressor) Flush() error {
	return t.compressor.Flush()
}

func (t *RatioCompressor) FullErr() error {
	if t.inputTargetReached() {
		return derive.ErrCompressorFull
	}
	return nil
}

// InputThreshold calculates the input data threshold in bytes from the given
// parameters.
func (t *RatioCompressor) InputThreshold() uint64 {
	return uint64(float64(t.config.TargetOutputSize) / t.config.ApproxComprRatio)
}

// inputTargetReached says whether the target amount of input data has been
// reached in this channel builder. No more blocks can be added afterwards.
func (t *RatioCompressor) inputTargetReached() bool {
	return uint64(t.inputBytes) >= t.InputThreshold()
}
