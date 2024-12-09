package derive

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/params"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	ErrMaxFrameSizeTooSmall    = errors.New("maxSize is too small to fit the fixed frame overhead")
	ErrNotDepositTx            = errors.New("first transaction in block is not a deposit tx")
	ErrTooManyRLPBytes         = errors.New("batch would cause RLP bytes to go over limit")
	ErrChannelOutAlreadyClosed = errors.New("channel-out already closed")
	ErrCompressorFull          = errors.New("compressor is full")
)

// FrameV0OverHeadSize is the absolute minimum size of a frame.
// This is the fixed overhead frame size, calculated as specified
// in the [Frame Format] specs: 16 + 2 + 4 + 1 = 23 bytes.
//
// [Frame Format]: https://github.com/ethereum-optimism/specs/blob/main/specs/protocol/derivation.md#frame-format
const FrameV0OverHeadSize = 23

type Compressor interface {
	// Writer is used to write uncompressed data which will be compressed. Should return
	// ErrCompressorFull if the compressor is full and no more data should be written.
	io.Writer
	// Closer Close function should be called before reading any data.
	io.Closer
	// Reader is used to Read compressed data; should only be called after Close.
	io.Reader
	// Reset will reset all written data
	Reset()
	// Len returns an estimate of the current length of the compressed data; calling Flush will
	// increase the accuracy at the expense of a poorer compression ratio.
	Len() int
	// Flush flushes any uncompressed data to the compression buffer. This will result in a
	// non-optimal compression ratio.
	Flush() error
	// FullErr returns ErrCompressorFull if the compressor is known to be full. Note that
	// calls to Write will fail if an error is returned from this method, but calls to Write
	// can still return ErrCompressorFull even if this does not.
	FullErr() error
}

type ChannelOut interface {
	ID() ChannelID
	Reset() error
	AddBlock(*rollup.Config, *types.Block) (*L1BlockInfo, error)
	InputBytes() int
	ReadyBytes() int
	Flush() error
	FullErr() error
	Close() error
	OutputFrame(*bytes.Buffer, uint64) (uint16, error)
}

type SingularChannelOut struct {
	id ChannelID
	// Frame ID of the next frame to emit. Increment after emitting
	frame uint64
	// rlpLength is the uncompressed size of the channel. Must be less than MAX_RLP_BYTES_PER_CHANNEL
	rlpLength int

	// Compressor stage. Write input data to it
	compress Compressor

	closed bool

	chainSpec *rollup.ChainSpec
}

func (co *SingularChannelOut) ID() ChannelID {
	return co.id
}

func NewSingularChannelOut(compress Compressor, chainSpec *rollup.ChainSpec) (*SingularChannelOut, error) {
	c := &SingularChannelOut{
		id:        ChannelID{},
		frame:     0,
		rlpLength: 0,
		compress:  compress,
		chainSpec: chainSpec,
	}
	_, err := rand.Read(c.id[:])
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (co *SingularChannelOut) Reset() error {
	co.frame = 0
	co.rlpLength = 0
	co.compress.Reset()
	co.closed = false
	_, err := rand.Read(co.id[:])
	return err
}

// AddBlock adds a block to the channel. It returns the block's L1BlockInfo
// and an error if there is a problem adding the block. The only sentinel error
// that it returns is ErrTooManyRLPBytes. If this error is returned, the channel
// should be closed and a new one should be made.
func (co *SingularChannelOut) AddBlock(rollupCfg *rollup.Config, block *types.Block) (*L1BlockInfo, error) {
	if co.closed {
		return nil, ErrChannelOutAlreadyClosed
	}

	batch, l1Info, err := BlockToSingularBatch(rollupCfg, block)
	if err != nil {
		return nil, fmt.Errorf("converting block to batch: %w", err)
	}
	return l1Info, co.addSingularBatch(batch, l1Info.SequenceNumber)
}

// addSingularBatch adds a batch to the channel. It returns
// an error if there is a problem adding the batch. The only sentinel error
// that it returns is ErrTooManyRLPBytes. If this error is returned, the channel
// should be closed and a new one should be made.
func (co *SingularChannelOut) addSingularBatch(batch *SingularBatch, _ uint64) error {
	if co.closed {
		return ErrChannelOutAlreadyClosed
	}

	// We encode to a temporary buffer to determine the encoded length to
	// ensure that the total size of all RLP elements is less than or equal to MAX_RLP_BYTES_PER_CHANNEL
	var buf bytes.Buffer
	if err := rlp.Encode(&buf, NewBatchData(batch)); err != nil {
		return err
	}

	// Fjord increases the max RLP bytes per channel. Activation of this change in the derivation pipeline
	// is dependent on the timestamp of the L1 block that this channel got included in. So using the timestamp
	// of the current batch guarantees that this channel will be included in an L1 block with a timestamp well after
	// the Fjord activation.
	maxRLPBytesPerChannel := co.chainSpec.MaxRLPBytesPerChannel(batch.Timestamp)
	if co.rlpLength+buf.Len() > int(maxRLPBytesPerChannel) {
		return fmt.Errorf("could not add %d bytes to channel of %d bytes, max is %d. err: %w",
			buf.Len(), co.rlpLength, maxRLPBytesPerChannel, ErrTooManyRLPBytes)
	}
	co.rlpLength += buf.Len()

	// avoid using io.Copy here, because we need all or nothing
	_, err := co.compress.Write(buf.Bytes())
	return err
}

// InputBytes returns the total amount of RLP-encoded input bytes.
func (co *SingularChannelOut) InputBytes() int {
	return co.rlpLength
}

// ReadyBytes returns the number of bytes that the channel out can immediately output into a frame.
// Use `Flush` or `Close` to move data from the compression buffer into the ready buffer if more bytes
// are needed. Add blocks may add to the ready buffer, but it is not guaranteed due to the compression stage.
func (co *SingularChannelOut) ReadyBytes() int {
	return co.compress.Len()
}

// Flush flushes the internal compression stage to the ready buffer. It enables pulling a larger & more
// complete frame. It reduces the compression efficiency.
func (co *SingularChannelOut) Flush() error {
	return co.compress.Flush()
}

func (co *SingularChannelOut) FullErr() error {
	return co.compress.FullErr()
}

func (co *SingularChannelOut) Close() error {
	if co.closed {
		return ErrChannelOutAlreadyClosed
	}
	co.closed = true
	return co.compress.Close()
}

// OutputFrame writes a frame to w with a given max size and returns the frame
// number.
// Use `ReadyBytes`, `Flush`, and `Close` to modify the ready buffer.
// Returns an error if the `maxSize` < FrameV0OverHeadSize.
// Returns io.EOF when the channel is closed & there are no more frames.
// Returns nil if there is still more buffered data.
// Returns an error if it ran into an error during processing.
func (co *SingularChannelOut) OutputFrame(w *bytes.Buffer, maxSize uint64) (uint16, error) {
	// Check that the maxSize is large enough for the frame overhead size.
	if maxSize < FrameV0OverHeadSize {
		return 0, ErrMaxFrameSizeTooSmall
	}

	f := createEmptyFrame(co.id, co.frame, co.ReadyBytes(), co.closed, maxSize)

	if _, err := io.ReadFull(co.compress, f.Data); err != nil {
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

// BlockToSingularBatch transforms a block into a batch object that can easily be RLP encoded.
func BlockToSingularBatch(rollupCfg *rollup.Config, block *types.Block) (*SingularBatch, *L1BlockInfo, error) {
	if len(block.Transactions()) == 0 {
		return nil, nil, fmt.Errorf("block %v has no transactions", block.Hash())
	}

	opaqueTxs := make([]hexutil.Bytes, 0, len(block.Transactions()))
	for i, tx := range block.Transactions() {
		if tx.Type() == types.DepositTxType {
			continue
		}
		otx, err := tx.MarshalBinary()
		if err != nil {
			return nil, nil, fmt.Errorf("could not encode tx %v in block %v: %w", i, tx.Hash(), err)
		}
		opaqueTxs = append(opaqueTxs, otx)
	}

	l1InfoTx := block.Transactions()[0]
	if l1InfoTx.Type() != types.DepositTxType {
		return nil, nil, ErrNotDepositTx
	}
	l1Info, err := L1BlockInfoFromBytes(rollupCfg, block.Time(), l1InfoTx.Data())
	if err != nil {
		return nil, l1Info, fmt.Errorf("could not parse the L1 Info deposit: %w", err)
	}

	return &SingularBatch{
		ParentHash:   block.ParentHash(),
		EpochNum:     rollup.Epoch(l1Info.Number),
		EpochHash:    l1Info.BlockHash,
		Timestamp:    block.Time(),
		Transactions: opaqueTxs,
	}, l1Info, nil
}

// ForceCloseTxData generates the transaction data for a transaction which will force close
// a channel. It should be given every frame of that channel which has been submitted on
// chain. The frames should be given in order that they appear on L1.
func ForceCloseTxData(frames []Frame) ([]byte, error) {
	if len(frames) == 0 {
		return nil, errors.New("must provide at least one frame")
	}
	frameNumbers := make(map[uint16]struct{})
	id := frames[0].ID
	closeNumber := uint16(0)
	closed := false
	for i, frame := range frames {
		if !closed && frame.IsLast {
			closeNumber = frame.FrameNumber
		}
		closed = closed || frame.IsLast
		frameNumbers[frame.FrameNumber] = struct{}{}
		if frame.ID != id {
			return nil, fmt.Errorf("invalid ID in list: first ID: %v, %vth ID: %v", id, i, frame.ID)
		}
	}

	var out bytes.Buffer
	out.WriteByte(params.DerivationVersion0)

	if !closed {
		f := Frame{
			ID:          id,
			FrameNumber: 0,
			Data:        nil,
			IsLast:      true,
		}
		if err := f.MarshalBinary(&out); err != nil {
			return nil, err
		}
	} else {
		for i := uint16(0); i <= closeNumber; i++ {
			if _, ok := frameNumbers[i]; ok {
				continue
			}
			f := Frame{
				ID:          id,
				FrameNumber: i,
				Data:        nil,
				IsLast:      false,
			}
			if err := f.MarshalBinary(&out); err != nil {
				return nil, err
			}
		}
	}

	return out.Bytes(), nil
}

// createEmptyFrame creates new empty Frame with given information. Frame data must be copied from ChannelOut.
func createEmptyFrame(id ChannelID, frame uint64, readyBytes int, closed bool, maxSize uint64) *Frame {
	f := Frame{
		ID:          id,
		FrameNumber: uint16(frame),
	}

	// Copy data from the local buffer into the frame data buffer
	maxDataSize := maxSize - FrameV0OverHeadSize
	if maxDataSize >= uint64(readyBytes) {
		maxDataSize = uint64(readyBytes)
		// If we are closed & will not spill past the current frame
		// mark it as the final frame of the channel.
		if closed {
			f.IsLast = true
		}
	}
	f.Data = make([]byte, maxDataSize)
	return &f
}
