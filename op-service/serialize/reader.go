package serialize

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
)

// BinaryReader provides methods to decode content written by BinaryWriter.
type BinaryReader struct {
	in io.Reader
}

func NewBinaryReader(in io.Reader) *BinaryReader {
	return &BinaryReader{in: in}
}

func (r *BinaryReader) ReadUInt(target any) error {
	return binary.Read(r.in, binary.BigEndian, target)
}

func (r *BinaryReader) ReadHash(target *common.Hash) error {
	_, err := io.ReadFull(r.in, target[:])
	return err
}

func (r *BinaryReader) ReadBool(target *bool) error {
	var v uint8
	if err := r.ReadUInt(&v); err != nil {
		return err
	}
	switch v {
	case 0:
		*target = false
	case 1:
		*target = true
	default:
		return fmt.Errorf("invalid boolean value: %v", v)
	}
	return nil
}

func (r *BinaryReader) ReadBytes(target *[]byte) error {
	var size uint32
	if err := r.ReadUInt(&size); err != nil {
		return err
	}
	if size == 0 {
		*target = nil
		return nil
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r.in, data); err != nil {
		return err
	}
	*target = data
	return nil
}
