package entrydb

import (
	"io"
)

type MemEntryStore[T EntryType, E Entry[T]] struct {
	entries []E
}

func (s *MemEntryStore[T, E]) Size() int64 {
	return int64(len(s.entries))
}

func (s *MemEntryStore[T, E]) LastEntryIdx() EntryIdx {
	return EntryIdx(s.Size() - 1)
}

func (s *MemEntryStore[T, E]) Read(idx EntryIdx) (E, error) {
	if idx < EntryIdx(len(s.entries)) {
		return s.entries[idx], nil
	}
	var out E
	return out, io.EOF
}

func (s *MemEntryStore[T, E]) Append(entries ...E) error {
	s.entries = append(s.entries, entries...)
	return nil
}

func (s *MemEntryStore[T, E]) Truncate(idx EntryIdx) error {
	s.entries = s.entries[:min(s.Size()-1, int64(idx+1))]
	return nil
}

func (s *MemEntryStore[T, E]) Close() error {
	return nil
}
