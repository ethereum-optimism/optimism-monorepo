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
	SourceV0          EntryType = 0
	InvalidatedFromV0 EntryType = 1
)

func (s EntryType) String() string {
	switch s {
	case SourceV0:
		return "sourceV0"
	case InvalidatedFromV0:
		return "invalidatedFromV0"
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

// LinkEntry is a DerivedFromV0 or a InvalidatedFromV0 kind
type LinkEntry struct {
	source  types.BlockSeal
	derived types.BlockSeal
	// when it exists as local-safe, but cannot be cross-safe.
	// If false: this link is a DerivedFromV0
	// If true: this link is a InvalidatedFromV0
	invalidated bool
}

func (d LinkEntry) String() string {
	return fmt.Sprintf("LinkEntry(derivedFrom: %s, derived: %s, invalidated: %v)", d.source, d.derived, d.invalidated)
}

func (d *LinkEntry) decode(e Entry) error {
	if t := e.Type(); t != SourceV0 && t != InvalidatedFromV0 {
		return fmt.Errorf("%w: unexpected entry type: %s", types.ErrDataCorruption, e.Type())
	}
	if [3]byte(e[1:4]) != ([3]byte{}) {
		return fmt.Errorf("%w: expected empty data, to pad entry size to round number: %x", types.ErrDataCorruption, e[1:4])
	}
	d.invalidated = e.Type() == InvalidatedFromV0
	// Format:
	// l1-number(8) l1-timestamp(8) l2-number(8) l2-timestamp(8) l1-hash(32) l2-hash(32)
	// Note: attributes are ordered for lexical sorting to nicely match chronological sorting.
	offset := 4
	d.source.Number = binary.BigEndian.Uint64(e[offset : offset+8])
	offset += 8
	d.source.Timestamp = binary.BigEndian.Uint64(e[offset : offset+8])
	offset += 8
	d.derived.Number = binary.BigEndian.Uint64(e[offset : offset+8])
	offset += 8
	d.derived.Timestamp = binary.BigEndian.Uint64(e[offset : offset+8])
	offset += 8
	copy(d.source.Hash[:], e[offset:offset+32])
	offset += 32
	copy(d.derived.Hash[:], e[offset:offset+32])
	return nil
}

func (d *LinkEntry) encode() Entry {
	var out Entry
	if d.invalidated {
		out[0] = uint8(InvalidatedFromV0)
	} else {
		out[0] = uint8(SourceV0)
	}
	offset := 4
	binary.BigEndian.PutUint64(out[offset:offset+8], d.source.Number)
	offset += 8
	binary.BigEndian.PutUint64(out[offset:offset+8], d.source.Timestamp)
	offset += 8
	binary.BigEndian.PutUint64(out[offset:offset+8], d.derived.Number)
	offset += 8
	binary.BigEndian.PutUint64(out[offset:offset+8], d.derived.Timestamp)
	offset += 8
	copy(out[offset:offset+32], d.source.Hash[:])
	offset += 32
	copy(out[offset:offset+32], d.derived.Hash[:])
	return out
}

func (d *LinkEntry) sealOrErr() (types.DerivedBlockSealPair, error) {
	if d.invalidated {
		return types.DerivedBlockSealPair{}, types.ErrAwaitReplacementBlock
	}
	return types.DerivedBlockSealPair{
		Source:  d.source,
		Derived: d.derived,
	}, nil
}
