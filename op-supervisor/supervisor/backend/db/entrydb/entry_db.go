package entrydb

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ethereum/go-ethereum/log"
)

type EntryStore[T EntryType, E Entry[T]] interface {
	Size() int64
	LastEntryIdx() EntryIdx
	Read(idx EntryIdx) (E, error)
	Append(entries ...E) error
	Truncate(idx EntryIdx) error
	Close() error
}

type EntryIdx int64

type EntryType interface {
	String() string
	~uint8
}

type Entry[T EntryType] interface {
	Type() T
	comparable
}

// Binary is the binary interface to encode/decode/size entries.
// This should be a zero-cost abstraction, and is bundled as interface for the EntryDB
// to have generic access to this functionality without const-generics for array size in Go.
type Binary[T EntryType, E Entry[T]] interface {
	Append(dest []byte, e *E) []byte
	ReadAt(dest *E, r io.ReaderAt, at int64) (n int, err error)
	EntrySize() int
}

// dataAccess defines a minimal API required to manipulate the actual stored data.
// It is a subset of the os.File API but could (theoretically) be satisfied by an in-memory implementation for testing.
type dataAccess interface {
	io.ReaderAt
	io.Writer
	io.Closer
	Truncate(size int64) error
}

type EntryDB[T EntryType, E Entry[T], B Binary[T, E]] struct {
	data         dataAccess
	lastEntryIdx EntryIdx

	b B

	cleanupFailedWrite bool
}

// NewEntryDB creates an EntryDB. A new file will be created if the specified path does not exist,
// but parent directories will not be created.
// If the file exists it will be used as the existing data.
// Returns ErrRecoveryRequired if the existing file is not a valid entry db. A EntryDB is still returned but all
// operations will return ErrRecoveryRequired until the Recover method is called.
func NewEntryDB[T EntryType, E Entry[T], B Binary[T, E]](logger log.Logger, path string) (*EntryDB[T, E, B], error) {
	logger.Info("Opening entry database", "path", path)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %v: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat database at %v: %w", path, err)
	}
	var b B
	size := info.Size() / int64(b.EntrySize())
	db := &EntryDB[T, E, B]{
		data:         file,
		lastEntryIdx: EntryIdx(size - 1),
	}
	if size*int64(b.EntrySize()) != info.Size() {
		logger.Warn("File size is not a multiple of entry size. Truncating to last complete entry", "fileSize", size, "entrySize", b.EntrySize())
		if err := db.recover(); err != nil {
			return nil, fmt.Errorf("failed to recover database at %v: %w", path, err)
		}
	}
	return db, nil
}

func (e *EntryDB[T, E, B]) Size() int64 {
	return int64(e.lastEntryIdx) + 1
}

// LastEntryIdx returns the index of the last entry in the DB.
// This returns -1 if the DB is empty.
func (e *EntryDB[T, E, B]) LastEntryIdx() EntryIdx {
	return e.lastEntryIdx
}

// Read an entry from the database by index. Returns io.EOF iff idx is after the last entry.
func (e *EntryDB[T, E, B]) Read(idx EntryIdx) (E, error) {
	var out E
	if idx > e.lastEntryIdx {
		return out, io.EOF
	}
	read, err := e.b.ReadAt(&out, e.data, int64(idx)*int64(e.b.EntrySize()))
	// Ignore io.EOF if we read the entire last entry as ReadAt may return io.EOF or nil when it reads the last byte
	if err != nil && !(errors.Is(err, io.EOF) && read == e.b.EntrySize()) {
		return out, fmt.Errorf("failed to read entry %v: %w", idx, err)
	}
	return out, nil
}

// Append entries to the database.
// The entries are combined in memory and passed to a single Write invocation.
// If the write fails, it will attempt to truncate any partially written data.
// Subsequent writes to this instance will fail until partially written data is truncated.
func (e *EntryDB[T, E, B]) Append(entries ...E) error {
	if e.cleanupFailedWrite {
		// Try to rollback partially written data from a previous Append
		if truncateErr := e.Truncate(e.lastEntryIdx); truncateErr != nil {
			return fmt.Errorf("failed to recover from previous write error: %w", truncateErr)
		}
	}
	data := make([]byte, 0, len(entries)*e.b.EntrySize())
	for i := range entries {
		data = e.b.Append(data, &entries[i])
	}
	if n, err := e.data.Write(data); err != nil {
		if n == 0 {
			// Didn't write any data, so no recovery required
			return err
		}
		// Try to rollback the partially written data
		if truncateErr := e.Truncate(e.lastEntryIdx); truncateErr != nil {
			// Failed to rollback, set a flag to attempt the clean up on the next write
			e.cleanupFailedWrite = true
			return errors.Join(err, fmt.Errorf("failed to remove partially written data: %w", truncateErr))
		}
		// Successfully rolled back the changes, still report the failed write
		return err
	}
	e.lastEntryIdx += EntryIdx(len(entries))
	return nil
}

// Truncate the database so that the last retained entry is idx. Any entries after idx are deleted.
func (e *EntryDB[T, E, B]) Truncate(idx EntryIdx) error {
	if err := e.data.Truncate((int64(idx) + 1) * int64(e.b.EntrySize())); err != nil {
		return fmt.Errorf("failed to truncate to entry %v: %w", idx, err)
	}
	// Update the lastEntryIdx cache
	e.lastEntryIdx = idx
	e.cleanupFailedWrite = false
	return nil
}

// recover an invalid database by truncating back to the last complete event.
func (e *EntryDB[T, E, B]) recover() error {
	if err := e.data.Truncate(e.Size() * int64(e.b.EntrySize())); err != nil {
		return fmt.Errorf("failed to truncate trailing partial entries: %w", err)
	}
	return nil
}

func (e *EntryDB[T, E, B]) Close() error {
	return e.data.Close()
}
