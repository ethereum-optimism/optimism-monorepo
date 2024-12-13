package fromda

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

const EntrySize = 100

type Entry [EntrySize]byte

func (e Entry) Type() EntryType {
	return EntryType(e[0])
}

type EntryType uint8

const (
	DerivedFromV0 EntryType = 0
)

func (s EntryType) String() string {
	switch s {
	case DerivedFromV0:
		return "v0"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(s))
	}
}

type EntryBinary struct{}

func (EntryBinary) Append(dest []byte, e *Entry) []byte {
	return append(dest, e[:]...)
}

func (EntryBinary) ReadAt(dest *Entry, r io.ReaderAt, at int64) (n int, err error) {
	return r.ReadAt(dest[:], at)
}

func (EntryBinary) EntrySize() int {
	return EntrySize
}

type LinkEntry struct {
	derivedFrom types.BlockSeal
	derived     types.BlockSeal
}

func (d LinkEntry) String() string {
	return fmt.Sprintf("LinkEntry(derivedFrom: %s, derived: %s)", d.derivedFrom, d.derived)
}

func (d *LinkEntry) decode(e Entry) error {
	if e.Type() != DerivedFromV0 {
		return fmt.Errorf("%w: unexpected entry type: %s", types.ErrDataCorruption, e.Type())
	}
	if [3]byte(e[1:4]) != ([3]byte{}) {
		return fmt.Errorf("%w: expected empty data, to pad entry size to round number: %x", types.ErrDataCorruption, e[1:4])
	}
	offset := 4
	d.derivedFrom.Number = binary.BigEndian.Uint64(e[offset : offset+8])
	offset += 8
	d.derivedFrom.Timestamp = binary.BigEndian.Uint64(e[offset : offset+8])
	offset += 8
	d.derived.Number = binary.BigEndian.Uint64(e[offset : offset+8])
	offset += 8
	d.derived.Timestamp = binary.BigEndian.Uint64(e[offset : offset+8])
	offset += 8
	copy(d.derivedFrom.Hash[:], e[offset:offset+32])
	offset += 32
	copy(d.derived.Hash[:], e[offset:offset+32])
	return nil
}

func (d *LinkEntry) encode() Entry {
	var out Entry
	out[0] = uint8(DerivedFromV0)
	offset := 4
	binary.BigEndian.PutUint64(out[offset:offset+8], d.derivedFrom.Number)
	offset += 8
	binary.BigEndian.PutUint64(out[offset:offset+8], d.derivedFrom.Timestamp)
	offset += 8
	binary.BigEndian.PutUint64(out[offset:offset+8], d.derived.Number)
	offset += 8
	binary.BigEndian.PutUint64(out[offset:offset+8], d.derived.Timestamp)
	offset += 8
	copy(out[offset:offset+32], d.derivedFrom.Hash[:])
	offset += 32
	copy(out[offset:offset+32], d.derived.Hash[:])
	return out
}
