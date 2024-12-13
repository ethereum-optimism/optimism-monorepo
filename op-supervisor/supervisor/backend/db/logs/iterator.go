package logs

import (
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/entrydb"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type IteratorState interface {
	NextIndex() entrydb.EntryIdx
	SealedBlock() (hash common.Hash, num uint64, ok bool)
	SealedTimestamp() (timestamp uint64, ok bool)
	InitMessage() (hash common.Hash, logIndex uint32, ok bool)
	ExecMessage() *types.ExecutingMessage
}

type Iterator interface {
	End() error
	NextInitMsg() error
	NextExecMsg() error
	NextBlock() error
	TraverseConditional(traverseConditionalFn) error
	IteratorState
}

type iterator struct {
	db          *DB
	current     logContext
	entriesRead int64
}

type traverseConditionalFn func(state IteratorState) error

// End traverses the iterator to the end of the DB.
// It does not return io.EOF or ErrFuture.
func (i *iterator) End() error {
	for {
		_, err := i.next()
		if errors.Is(err, types.ErrFuture) {
			return nil
		} else if err != nil {
			return err
		}
	}
}

// NextInitMsg advances the iterator until it reads the next Initiating Message into the current state.
// It scans forward until it finds and fully reads an initiating event, skipping any blocks.
func (i *iterator) NextInitMsg() error {
	seenLog := false
	for {
		typ, err := i.next()
		if err != nil {
			return err
		}
		if typ == TypeInitiatingEvent {
			seenLog = true
		}
		if !i.current.hasCompleteBlock() {
			continue // must know the block we're building on top of
		}
		if i.current.hasIncompleteLog() {
			continue // didn't finish processing the log yet
		}
		if seenLog {
			return nil
		}
	}
}

// NextExecMsg advances the iterator until it reads the next Executing Message into the current state.
// It scans forward until it finds and fully reads an initiating event, skipping any blocks.
func (i *iterator) NextExecMsg() error {
	for {
		err := i.NextInitMsg()
		if err != nil {
			return err
		}
		if i.current.execMsg != nil {
			return nil // found a new executing message!
		}
	}
}

// NextBlock advances the iterator until it reads the next block into the current state.
// It scans forward until it finds and fully reads a block, skipping any events.
func (i *iterator) NextBlock() error {
	seenBlock := false
	for {
		typ, err := i.next()
		if err != nil {
			return err
		}
		if typ == TypeSearchCheckpoint {
			seenBlock = true
		}
		if !i.current.hasCompleteBlock() {
			continue // need the full block content
		}
		if seenBlock {
			return nil
		}
	}
}

func (i *iterator) TraverseConditional(fn traverseConditionalFn) error {
	var snapshot logContext
	for {
		snapshot = i.current // copy the iterator state
		_, err := i.next()
		if err != nil {
			i.current = snapshot
			return err
		}
		if i.current.need != 0 { // skip intermediate states
			continue
		}
		if err := fn(&i.current); err != nil {
			i.current = snapshot
			return err
		}
	}
}

// Read and apply the next entry.
func (i *iterator) next() (EntryType, error) {
	index := i.current.nextEntryIndex
	entry, err := i.db.store.Read(index)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0, types.ErrFuture
		}
		return 0, fmt.Errorf("failed to read entry %d: %w", index, err)
	}
	if err := i.current.ApplyEntry(entry); err != nil {
		return entry.Type(), fmt.Errorf("failed to apply entry %d to iterator state: %w", index, err)
	}

	i.entriesRead++
	return entry.Type(), nil
}

func (i *iterator) NextIndex() entrydb.EntryIdx {
	return i.current.NextIndex()
}

// SealedBlock returns the sealed block that we are appending logs after, if any is available.
// I.e. the block is the parent block of the block containing the logs that are currently appending to it.
func (i *iterator) SealedBlock() (hash common.Hash, num uint64, ok bool) {
	return i.current.SealedBlock()
}

// SealedTimestamp returns the timestamp of SealedBlock
func (i *iterator) SealedTimestamp() (timestamp uint64, ok bool) {
	return i.current.SealedTimestamp()
}

// InitMessage returns the current initiating message, if any is available.
func (i *iterator) InitMessage() (hash common.Hash, logIndex uint32, ok bool) {
	return i.current.InitMessage()
}

// ExecMessage returns the current executing message, if any is available.
func (i *iterator) ExecMessage() *types.ExecutingMessage {
	return i.current.ExecMessage()
}
