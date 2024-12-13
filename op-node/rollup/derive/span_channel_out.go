package derive

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
)

type SpanChannelOut struct {
	id ChannelID
	// Frame ID of the next frame to emit. Increment after emitting
	frame uint64
	// rlp is the encoded, uncompressed data of the channel. length must be less than MAX_RLP_BYTES_PER_CHANNEL
	// it is a double buffer to allow us to "undo" the last change to the RLP structure when the target size is exceeded
	rlp [2]*bytes.Buffer
	// rlpIndex is the index of the current rlp buffer
	rlpIndex int
	// lastCompressedRLPSize tracks the *uncompressed* size of the last RLP buffer that was compressed
	// it is used to measure the growth of the RLP buffer when adding a new batch to optimize compression
	lastCompressedRLPSize int
	// the compressor for the channel
	compressor ChannelCompressor
	// target is the target size of the compressed data
	target uint64
	// closed indicates if the channel is closed
	closed bool
	// full indicates if the channel is full
	full error
	// spanBatch is the batch being built, which immutably holds genesis timestamp and chain ID, but otherwise can be reset
	spanBatch *SpanBatch

	// maxBlocksPerSpanBatch is an optional limit on the number of blocks per span batch.
	// If non-zero, a new span batch will be started after the current span batch has
	// reached this maximum.
	maxBlocksPerSpanBatch int

	// sealedRLPBytes stores the sealed number of input RLP bytes. This is used when maxBlocksPerSpanBatch is non-zero
	// to seal full span batches (that have reached the max block count) in the rlp slices.
	sealedRLPBytes int

	chainSpec *rollup.ChainSpec
}

func (co *SpanChannelOut) ID() ChannelID {
	return co.id
}

func (co *SpanChannelOut) setRandomID() error {
	_, err := rand.Read(co.id[:])
	return err
}

type SpanChannelOutOption func(co *SpanChannelOut)

func WithMaxBlocksPerSpanBatch(maxBlock int) SpanChannelOutOption {
	return func(co *SpanChannelOut) {
		co.maxBlocksPerSpanBatch = maxBlock
	}
}

func NewSpanChannelOut(targetOutputSize uint64, compressionAlgo CompressionAlgo, chainSpec *rollup.ChainSpec, opts ...SpanChannelOutOption) (*SpanChannelOut, error) {
	c := &SpanChannelOut{
		id:        ChannelID{},
		frame:     0,
		spanBatch: NewSpanBatch(chainSpec.L2GenesisTime(), chainSpec.L2ChainID()),
		rlp:       [2]*bytes.Buffer{{}, {}},
		target:    targetOutputSize,
		chainSpec: chainSpec,
	}
	var err error
	if err = c.setRandomID(); err != nil {
		return nil, err
	}

	if c.compressor, err = NewChannelCompressor(compressionAlgo); err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (co *SpanChannelOut) Reset() error {
	co.closed = false
	co.full = nil
	co.frame = 0
	co.sealedRLPBytes = 0
	co.rlp[0].Reset()
	co.rlp[1].Reset()
	co.lastCompressedRLPSize = 0
	co.compressor.Reset()
	co.resetSpanBatch()
	// setting the new randomID is the only part of the reset that can fail
	return co.setRandomID()
}

func (co *SpanChannelOut) resetSpanBatch() {
	co.spanBatch = NewSpanBatch(co.spanBatch.GenesisTimestamp, co.spanBatch.ChainID)
}

// activeRLP returns the active RLP buffer using the current rlpIndex
func (co *SpanChannelOut) activeRLP() *bytes.Buffer {
	return co.rlp[co.rlpIndex]
}

// inactiveRLP returns the inactive RLP buffer using the current rlpIndex
func (co *SpanChannelOut) inactiveRLP() *bytes.Buffer {
	return co.rlp[(co.rlpIndex+1)%2]
}

// swapRLP switches the active and inactive RLP buffers by modifying the rlpIndex
func (co *SpanChannelOut) swapRLP() {
	co.rlpIndex = (co.rlpIndex + 1) % 2
}

// AddBlock adds a block to the channel. It returns the block's L1BlockInfo
// and an error if there is a problem adding the block. The only sentinel error
// that it returns is ErrTooManyRLPBytes. If this error is returned, the channel
// should be closed and a new one should be made.
func (co *SpanChannelOut) AddBlock(rollupCfg *rollup.Config, block *types.Block) (*L1BlockInfo, error) {
	if co.closed {
		return nil, ErrChannelOutAlreadyClosed
	}

	batch, l1Info, err := BlockToSingularBatch(rollupCfg, block)
	if err != nil {
		return nil, fmt.Errorf("converting block to batch: %w", err)
	}
	return l1Info, co.addSingularBatch(batch, l1Info.SequenceNumber)
}

// addSingularBatch adds a SingularBatch to the channel, compressing the data if necessary.
// if the new batch would make the channel exceed the target size, the last batch is reverted,
// and the compression happens on the previous RLP buffer instead
// if the input is too small to need compression, data is accumulated but not compressed
func (co *SpanChannelOut) addSingularBatch(batch *SingularBatch, seqNum uint64) error {
	// sentinel error for closed or full channel
	if co.closed {
		return ErrChannelOutAlreadyClosed
	}
	if err := co.FullErr(); err != nil {
		return err
	}

	co.ensureOpenSpanBatch()
	// update the SpanBatch with the SingularBatch
	if err := co.spanBatch.AppendSingularBatch(batch, seqNum); err != nil {
		return fmt.Errorf("failed to append SingularBatch to SpanBatch: %w", err)
	}
	// convert Span batch to RawSpanBatch
	rawSpanBatch, err := co.spanBatch.ToRawSpanBatch()
	if err != nil {
		return fmt.Errorf("failed to convert SpanBatch into RawSpanBatch: %w", err)
	}

	// switch to the other buffer and truncate it for new use
	// (the RLP buffer which is being made inactive holds the RLP encoded span batch(es)
	// just before the new batch was added)
	co.swapRLP()
	active := co.activeRLP()
	active.Truncate(co.sealedRLPBytes)
	if err = rlp.Encode(active, NewBatchData(rawSpanBatch)); err != nil {
		return fmt.Errorf("failed to encode RawSpanBatch into bytes: %w", err)
	}

	// Fjord increases the max RLP bytes per channel. Activation of this change in the derivation pipeline
	// is dependent on the timestamp of the L1 block that this channel got included in. So using the timestamp
	// of the current batch guarantees that this channel will be included in an L1 block with a timestamp well after
	// the Fjord activation.
	maxRLPBytesPerChannel := co.chainSpec.MaxRLPBytesPerChannel(batch.Timestamp)
	if active.Len() > int(maxRLPBytesPerChannel) {
		return fmt.Errorf("could not take %d bytes as replacement of channel of %d bytes, max is %d. err: %w",
			active.Len(), co.inactiveRLP().Len(), maxRLPBytesPerChannel, ErrTooManyRLPBytes)
	}

	// if the compressed data *plus* the new rlp data is under the target size, return early
	// this optimizes out cases where the compressor will obviously come in under the target size
	rlpGrowth := active.Len() - co.lastCompressedRLPSize
	if uint64(co.compressor.Len()+rlpGrowth) < co.target {
		return nil
	}

	// we must compress the data to check if we've met or exceeded the target size
	if err = co.compress(); err != nil {
		return err
	}

	// if the channel is now full, either return the compressed data, or the compressed previous data
	if err := co.FullErr(); err != nil {
		// if it's the first singular batch/block of the channel, it *must* fit in
		if co.sealedRLPBytes == 0 && co.spanBatch.GetBlockCount() == 1 {
			return nil
		}

		// if we just perfectly filled up the channel, also return nil to retain block
		if uint64(co.compressor.Len()) == co.target {
			return nil
		}

		// if there is more than one batch in the channel, we revert the last batch
		// by switching the RLP buffer and doing a fresh compression
		co.swapRLP()
		if cerr := co.compress(); cerr != nil {
			return cerr
		}
		// return the full error
		return err
	}

	return nil
}

func (co *SpanChannelOut) ensureOpenSpanBatch() {
	if co.maxBlocksPerSpanBatch == 0 || co.spanBatch.GetBlockCount() < co.maxBlocksPerSpanBatch {
		return
	}
	// we assume that the full span batch has been written to the last active rlp buffer
	active, inactive := co.activeRLP(), co.inactiveRLP()
	if inactive.Len() > active.Len() {
		panic("inactive rlp unexpectedly larger")
	}
	co.sealedRLPBytes = active.Len()
	// Copy active to inactive rlp buffer so both have the same sealed state
	// and resetting by truncation works as intended.
	inactive.Reset()
	// err is guaranteed to always be nil
	_, _ = inactive.Write(active.Bytes())
	co.resetSpanBatch()
}

// compress compresses the active RLP buffer and checks if the compressed data is over the target size.
// it resets all the compression buffers because Span Batches aren't meant to be compressed incrementally.
func (co *SpanChannelOut) compress() error {
	co.compressor.Reset()
	// we write Bytes() of the active RLP to the compressor, so the active RLP's
	// buffer is not advanced as a ReadWriter, making it possible to later use
	// Truncate().
	rlpBytes := co.activeRLP().Bytes()
	if _, err := co.compressor.Write(rlpBytes); err != nil {
		return err
	}
	co.lastCompressedRLPSize = len(rlpBytes)
	if err := co.compressor.Close(); err != nil {
		return err
	}
	co.checkFull()
	return nil
}

// InputBytes returns the total amount of RLP-encoded input bytes.
func (co *SpanChannelOut) InputBytes() int {
	return co.activeRLP().Len()
}

// ReadyBytes returns the total amount of compressed bytes that are ready to be output.
// Span Channel Out does not provide early output, so this will always be 0 until the channel is closed or full
func (co *SpanChannelOut) ReadyBytes() int {
	if co.closed || co.FullErr() != nil {
		return co.compressor.Len()
	}
	return 0
}

// Flush implements the Channel Out
// Span Channel Out manages the flushing of the compressor internally, so this is a no-op
func (co *SpanChannelOut) Flush() error {
	return nil
}

// checkFull sets the full error if the compressed data is over the target size.
// the error is only set once, and the channel is considered full from that point on
func (co *SpanChannelOut) checkFull() {
	// if the channel is already full, don't update further
	if co.full != nil {
		return
	}
	if uint64(co.compressor.Len()) >= co.target {
		co.full = ErrCompressorFull
	}
}

func (co *SpanChannelOut) FullErr() error {
	return co.full
}

func (co *SpanChannelOut) Close() error {
	if co.closed {
		return ErrChannelOutAlreadyClosed
	}
	co.closed = true
	// if the channel was already full,
	// the compressor is already flushed and closed
	if co.FullErr() != nil {
		return nil
	}
	// if this channel is not full, we need to compress the last batch
	// this also flushes/closes the compressor
	return co.compress()
}

// OutputFrame writes a frame to w with a given max size and returns the frame
// number.
// Use `ReadyBytes`, `Flush`, and `Close` to modify the ready buffer.
// Returns an error if the `maxSize` < FrameV0OverHeadSize.
// Returns io.EOF when the channel is closed & there are no more frames.
// Returns nil if there is still more buffered data.
// Returns an error if it ran into an error during processing.
func (co *SpanChannelOut) OutputFrame(w *bytes.Buffer, maxSize uint64) (uint16, error) {
	// Check that the maxSize is large enough for the frame overhead size.
	if maxSize < FrameV0OverHeadSize {
		return 0, ErrMaxFrameSizeTooSmall
	}

	f := createEmptyFrame(co.id, co.frame, co.ReadyBytes(), co.closed, maxSize)

	if _, err := io.ReadFull(co.compressor.GetCompressed(), f.Data); err != nil {
		return 0, err
	}

	if err := f.MarshalBinary(w); err != nil {
		return 0, err
	}

	co.frame += 1
	fn := f.FrameNumber
	if f.IsLast {
		return fn, io.EOF
	} else {
		return fn, nil
	}
}
