package serialize

import (
	"encoding/binary"
	"io"

	"github.com/ethereum/go-ethereum/common"
)

// BinaryWriter writes a simple binary format which can be read again using BinaryReader.
// The format is a simple concatenation of values, with prefixed length for variable length items.
// All numbers are encoded using big endian.
type BinaryWriter struct {
	out io.Writer
}

func NewBinaryWriter(out io.Writer) *BinaryWriter {
	return &BinaryWriter{out: out}
}

func (w *BinaryWriter) WriteUInt(v any) error {
	return binary.Write(w.out, binary.BigEndian, v)
}

func (w *BinaryWriter) WriteHash(v common.Hash) error {
	_, err := w.out.Write(v[:])
	return err
}

func (w *BinaryWriter) WriteBool(v bool) error {
	if v {
		return w.WriteUInt(uint8(1))
	} else {
		return w.WriteUInt(uint8(0))
	}
}

func (w *BinaryWriter) WriteBytes(v []byte) error {
	if err := w.WriteUInt(uint32(len(v))); err != nil {
		return err
	}
	_, err := w.out.Write(v)
	return err
}
