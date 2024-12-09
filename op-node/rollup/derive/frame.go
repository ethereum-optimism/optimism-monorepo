package derive

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/params"
)

// Frames cannot be larger than 1 MB.
// Data transactions that carry frames are generally not larger than 128 KB due to L1 network conditions,
// but we leave space to grow larger anyway (gas limit allows for more data).
const MaxFrameLen = 1_000_000

// Data Format
//
// frame = channel_id ++ frame_number ++ frame_data_length ++ frame_data ++ is_last
//
// channel_id        = bytes16
// frame_number      = uint16
// frame_data_length = uint32
// frame_data        = bytes
// is_last           = bool

type Frame struct {
	ID          ChannelID `json:"id"`
	FrameNumber uint16    `json:"frame_number"`
	Data        []byte    `json:"data"`
	IsLast      bool      `json:"is_last"`
}

// MarshalBinary writes the frame to `w`.
// It returns any errors encountered while writing, but
// generally expects the writer very rarely fail.
func (f *Frame) MarshalBinary(w io.Writer) error {
	_, err := w.Write(f.ID[:])
	if err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, f.FrameNumber); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(f.Data))); err != nil {
		return err
	}
	_, err = w.Write(f.Data)
	if err != nil {
		return err
	}
	if f.IsLast {
		if _, err = w.Write([]byte{1}); err != nil {
			return err
		}
	} else {
		if _, err = w.Write([]byte{0}); err != nil {
			return err
		}
	}
	return nil
}

type ByteReader interface {
	io.Reader
	io.ByteReader
}

// UnmarshalBinary consumes a full frame from the reader.
// If `r` fails a read, it returns the error from the reader
// The reader will be left in a partially read state.
//
// If r doesn't return any bytes, returns io.EOF.
// If r unexpectedly stops returning data half-way, returns io.ErrUnexpectedEOF.
func (f *Frame) UnmarshalBinary(r ByteReader) error {
	if _, err := io.ReadFull(r, f.ID[:]); err != nil {
		// Forward io.EOF here ok, would mean not a single byte from r.
		return fmt.Errorf("reading channel_id: %w", err)
	}
	if err := binary.Read(r, binary.BigEndian, &f.FrameNumber); err != nil {
		return fmt.Errorf("reading frame_number: %w", eofAsUnexpectedMissing(err))
	}

	var frameLength uint32
	if err := binary.Read(r, binary.BigEndian, &frameLength); err != nil {
		return fmt.Errorf("reading frame_data_length: %w", eofAsUnexpectedMissing(err))
	}

	// Cap frame length to MaxFrameLen (currently 1MB)
	if frameLength > MaxFrameLen {
		return fmt.Errorf("frame_data_length is too large: %d", frameLength)
	}
	f.Data = make([]byte, int(frameLength))
	if _, err := io.ReadFull(r, f.Data); err != nil {
		return fmt.Errorf("reading frame_data: %w", eofAsUnexpectedMissing(err))
	}

	if isLastByte, err := r.ReadByte(); err != nil {
		return fmt.Errorf("reading final byte (is_last): %w", eofAsUnexpectedMissing(err))
	} else if isLastByte == 0 {
		f.IsLast = false
	} else if isLastByte == 1 {
		f.IsLast = true
	} else {
		return errors.New("invalid byte as is_last")
	}
	return nil
}

// eofAsUnexpectedMissing converts an io.EOF in the error chain of err into an
// io.ErrUnexpectedEOF. It should be used to convert intermediate io.EOF errors
// in unmarshalling code to achieve idiomatic error behavior.
// Other errors are passed through unchanged.
func eofAsUnexpectedMissing(err error) error {
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("fully missing: %w", io.ErrUnexpectedEOF)
	}
	return err
}

// Frames are stored in L1 transactions with the following format:
// data = DerivationVersion0 ++ Frame(s)
// Where there is one or more frames concatenated together.

// ParseFrames parse the on chain serialization of frame(s) in
// an L1 transaction. Currently only version 0 of the serialization
// format is supported.
// All frames must be parsed without error and there must not be
// any left over data and there must be at least one frame.
func ParseFrames(data []byte) ([]Frame, error) {
	if len(data) == 0 {
		return nil, errors.New("data array must not be empty")
	}
	if data[0] != params.DerivationVersion0 {
		return nil, fmt.Errorf("invalid derivation format byte: got %d", data[0])
	}
	buf := bytes.NewBuffer(data[1:])
	var frames []Frame
	for buf.Len() > 0 {
		var f Frame
		if err := f.UnmarshalBinary(buf); err != nil {
			return nil, fmt.Errorf("parsing frame %d: %w", len(frames), err)
		}
		frames = append(frames, f)
	}
	if buf.Len() != 0 {
		return nil, fmt.Errorf("did not fully consume data: have %d frames and %d bytes left", len(frames), buf.Len())
	}
	if len(frames) == 0 {
		return nil, errors.New("was not able to find any frames")
	}
	return frames, nil
}
