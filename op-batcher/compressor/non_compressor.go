package compressor

import (
	"bytes"
	"compress/zlib"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
)

type NonCompressor struct {
	config Config

	buf      bytes.Buffer
	compress *zlib.Writer

	fullErr error
}

// NewNonCompressor creates a new derive.Compressor implementation that doesn't
// compress by using zlib.NoCompression.
// It flushes to the underlying buffer any data from a prior write call.
// This is very suboptimal behavior and should only be used in tests.
// The NonCompressor can be used in tests to create a partially flushed channel.
// If the output buffer size after a write exceeds TargetFrameSize*TargetNumFrames,
// the compressor is marked as full, but the write succeeds.
func NewNonCompressor(config Config) (derive.Compressor, error) {
	c := &NonCompressor{
		config: config,
	}

	var err error
	c.compress, err = zlib.NewWriterLevel(&c.buf, zlib.NoCompression)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (t *NonCompressor) Write(p []byte) (int, error) {
	if err := t.compress.Flush(); err != nil {
		return 0, err
	}
	n, err := t.compress.Write(p)
	if err != nil {
		return 0, err
	}
	if uint64(t.buf.Len()) > t.config.TargetOutputSize {
		t.fullErr = derive.ErrCompressorFull
	}
	return n, nil
}

func (t *NonCompressor) Close() error {
	return t.compress.Close()
}

func (t *NonCompressor) Read(p []byte) (int, error) {
	return t.buf.Read(p)
}

func (t *NonCompressor) Reset() {
	t.buf.Reset()
	t.compress.Reset(&t.buf)
	t.fullErr = nil
}

func (t *NonCompressor) Len() int {
	return t.buf.Len()
}

func (t *NonCompressor) Flush() error {
	return t.compress.Flush()
}

func (t *NonCompressor) FullErr() error {
	return t.fullErr
}
