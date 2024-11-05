package fromda

import (
	"cmp"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/entrydb"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type EntryStore interface {
	Size() int64
	LastEntryIdx() entrydb.EntryIdx
	Read(idx entrydb.EntryIdx) (Entry, error)
	Append(entries ...Entry) error
	Truncate(idx entrydb.EntryIdx) error
	Close() error
}

// DB implements an append only database for log data and cross-chain dependencies.
// Each entry is fixed size, and denotes an increment in L1 (derived-from) and/or L2 (derived) block.
// Data is an append-only log, that can be binary searched for any necessary derivation-link data.
type DB struct {
	log    log.Logger
	m      Metrics
	store  EntryStore
	rwLock sync.RWMutex
}

func NewFromFile(logger log.Logger, m Metrics, path string) (*DB, error) {
	store, err := entrydb.NewEntryDB[EntryType, Entry, EntryBinary](logger, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %w", err)
	}
	return NewFromEntryStore(logger, m, store)
}

func NewFromEntryStore(logger log.Logger, m Metrics, store EntryStore) (*DB, error) {
	db := &DB{
		log:   logger,
		m:     m,
		store: store,
	}
	db.m.RecordDBDerivedEntryCount(db.store.Size())
	return db, nil
}

// Rewind to the last entry that was derived from a L1 block with the given block number.
func (db *DB) Rewind(derivedFrom uint64) error {
	index, _, err := db.lastDerivedAt(derivedFrom)
	if err != nil {
		return fmt.Errorf("failed to find point to rewind to: %w", err)
	}
	err = db.store.Truncate(index)
	if err != nil {
		return err
	}
	db.m.RecordDBDerivedEntryCount(int64(index) + 1)
	return nil
}

// First returns the first known values, alike to Latest.
func (db *DB) First() (derivedFrom types.BlockSeal, derived types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	lastIndex := db.store.LastEntryIdx()
	if lastIndex < 0 {
		return types.BlockSeal{}, types.BlockSeal{}, types.ErrFuture
	}
	last, err := db.readAt(0)
	if err != nil {
		return types.BlockSeal{}, types.BlockSeal{}, fmt.Errorf("failed to read first derivation data: %w", err)
	}
	return last.derivedFrom, last.derived, nil
}

func (db *DB) PreviousDerived(derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	// get the last time this L2 block was seen.
	selfIndex, self, err := db.firstDerivedFrom(derived.Number)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to find derived %d: %w", derived.Number, err)
	}
	if self.derived.ID() != derived {
		return types.BlockSeal{}, fmt.Errorf("found %s, but expected %s: %w", self.derived, derived, types.ErrConflict)
	}
	if selfIndex == 0 { // genesis block has a zeroed block as parent block
		return types.BlockSeal{}, nil
	}
	prev, err := db.readAt(selfIndex - 1)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("cannot find previous derived before %s: %w", derived, err)
	}
	return prev.derived, nil
}

// Latest returns the last known values:
// derivedFrom: the L1 block that the L2 block is safe for (not necessarily the first, multiple L2 blocks may be derived from the same L1 block).
// derived: the L2 block that was derived (not necessarily the first, the L1 block may have been empty and repeated the last safe L2 block).
func (db *DB) Latest() (derivedFrom types.BlockSeal, derived types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	return db.latest()
}

// latest is like Latest, but without lock, for internal use.
func (db *DB) latest() (derivedFrom types.BlockSeal, derived types.BlockSeal, err error) {
	lastIndex := db.store.LastEntryIdx()
	if lastIndex < 0 {
		return types.BlockSeal{}, types.BlockSeal{}, types.ErrFuture
	}
	last, err := db.readAt(lastIndex)
	if err != nil {
		return types.BlockSeal{}, types.BlockSeal{}, fmt.Errorf("failed to read last derivation data: %w", err)
	}
	return last.derivedFrom, last.derived, nil
}

// LastDerivedAt returns the last L2 block derived from the given L1 block.
func (db *DB) LastDerivedAt(derivedFrom eth.BlockID) (derived types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	_, link, err := db.lastDerivedAt(derivedFrom.Number)
	if err != nil {
		return types.BlockSeal{}, err
	}
	if link.derivedFrom.ID() != derivedFrom {
		return types.BlockSeal{}, fmt.Errorf("searched for last derived-from %s but found %s: %w",
			derivedFrom, link.derivedFrom, types.ErrConflict)
	}
	return link.derived, nil
}

// NextDerived finds the next L2 block after derived, and what it was derived from
func (db *DB) NextDerived(derived eth.BlockID) (derivedFrom types.BlockSeal, nextDerived types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	// get the last time this L2 block was seen.
	selfIndex, self, err := db.lastDerivedFrom(derived.Number)
	if err != nil {
		return types.BlockSeal{}, types.BlockSeal{}, fmt.Errorf("failed to find derived %d: %w", derived.Number, err)
	}
	if self.derived.ID() != derived {
		return types.BlockSeal{}, types.BlockSeal{}, fmt.Errorf("found %s, but expected %s: %w", self.derived, derived, types.ErrConflict)
	}
	next, err := db.readAt(selfIndex + 1)
	if err != nil {
		return types.BlockSeal{}, types.BlockSeal{}, fmt.Errorf("cannot find next derived after %s: %w", derived, err)
	}
	return next.derivedFrom, next.derived, nil
}

// DerivedFrom determines where a L2 block was first derived from.
// (a L2 block may repeat if the following L1 blocks are empty and don't produce additional L2 blocks)
func (db *DB) DerivedFrom(derived eth.BlockID) (derivedFrom types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	_, link, err := db.firstDerivedFrom(derived.Number)
	if err != nil {
		return types.BlockSeal{}, err
	}
	if link.derived.ID() != derived {
		return types.BlockSeal{}, fmt.Errorf("searched for first derived %s but found %s: %w",
			derived, link.derived, types.ErrConflict)
	}
	return link.derivedFrom, nil
}

func (db *DB) PreviousDerivedFrom(derivedFrom eth.BlockID) (prevDerivedFrom types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	// get the last time this L1 block was seen.
	selfIndex, self, err := db.firstDerivedAt(derivedFrom.Number)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to find derived %d: %w", derivedFrom.Number, err)
	}
	if self.derivedFrom.ID() != derivedFrom {
		return types.BlockSeal{}, fmt.Errorf("found %s, but expected %s: %w", self.derivedFrom, derivedFrom, types.ErrConflict)
	}
	if selfIndex == 0 {
		// genesis block has a zeroed block as parent block
		if self.derivedFrom.Number == 0 {
			return types.BlockSeal{}, nil
		} else {
			return types.BlockSeal{},
				fmt.Errorf("cannot find previous derived before start of database: %s (%w)", derivedFrom, types.ErrPreviousToFirst)
		}
	}
	prev, err := db.readAt(selfIndex - 1)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("cannot find previous derived before %s: %w", derivedFrom, err)
	}
	return prev.derivedFrom, nil
}

// NextDerivedFrom finds the next L1 block after derivedFrom
func (db *DB) NextDerivedFrom(derivedFrom eth.BlockID) (nextDerivedFrom types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	selfIndex, self, err := db.lastDerivedAt(derivedFrom.Number)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to find derived-from %d: %w", derivedFrom.Number, err)
	}
	if self.derivedFrom.ID() != derivedFrom {
		return types.BlockSeal{}, fmt.Errorf("found %s, but expected %s: %w", self.derivedFrom, derivedFrom, types.ErrConflict)
	}
	next, err := db.readAt(selfIndex + 1)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("cannot find next derived-from after %s: %w", derivedFrom, err)
	}
	return next.derivedFrom, nil
}

// FirstAfter determines the next entry after the given pair of derivedFrom, derived.
// Either one or both of the two entries will be an increment by 1
func (db *DB) FirstAfter(derivedFrom, derived eth.BlockID) (nextDerivedFrom, nextDerived types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	selfIndex, selfLink, err := db.lookup(derivedFrom.Number, derived.Number)
	if err != nil {
		return types.BlockSeal{}, types.BlockSeal{}, err
	}
	if selfLink.derivedFrom.ID() != derivedFrom {
		return types.BlockSeal{}, types.BlockSeal{}, fmt.Errorf("DB has derived-from %s but expected %s: %w", selfLink.derivedFrom, derivedFrom, types.ErrConflict)
	}
	if selfLink.derived.ID() != derived {
		return types.BlockSeal{}, types.BlockSeal{}, fmt.Errorf("DB has derived %s but expected %s: %w", selfLink.derived, derived, types.ErrConflict)
	}
	next, err := db.readAt(selfIndex + 1)
	if err != nil {
		return types.BlockSeal{}, types.BlockSeal{}, err
	}
	return next.derivedFrom, next.derived, nil
}

func (db *DB) lastDerivedFrom(derived uint64) (entrydb.EntryIdx, LinkEntry, error) {
	return db.find(true, func(link LinkEntry) int {
		return cmp.Compare(derived, link.derived.Number)
	})
}

func (db *DB) firstDerivedFrom(derived uint64) (entrydb.EntryIdx, LinkEntry, error) {
	return db.find(false, func(link LinkEntry) int {
		return cmp.Compare(link.derived.Number, derived)
	})
}

func (db *DB) lookup(derivedFrom, derived uint64) (entrydb.EntryIdx, LinkEntry, error) {
	return db.find(false, func(link LinkEntry) int {
		res := cmp.Compare(link.derived.Number, derived)
		if res == 0 {
			return cmp.Compare(link.derivedFrom.Number, derivedFrom)
		}
		return res
	})
}

func (db *DB) lastDerivedAt(derivedFrom uint64) (entrydb.EntryIdx, LinkEntry, error) {
	// Reverse: prioritize the last entry.
	return db.find(true, func(link LinkEntry) int {
		return cmp.Compare(derivedFrom, link.derivedFrom.Number)
	})
}

func (db *DB) firstDerivedAt(derivedFrom uint64) (entrydb.EntryIdx, LinkEntry, error) {
	return db.find(false, func(link LinkEntry) int {
		return cmp.Compare(link.derivedFrom.Number, derivedFrom)
	})
}

// find finds the first entry for which cmpFn(link) returns 0.
// The cmpFn entries to the left should return -1, entries to the right 1.
// If reverse, the cmpFn should be flipped too, and the last entry for which cmpFn(link) is 0 will be found.
func (db *DB) find(reverse bool, cmpFn func(link LinkEntry) int) (entrydb.EntryIdx, LinkEntry, error) {
	n := db.store.Size()
	if n == 0 {
		return -1, LinkEntry{}, types.ErrFuture
	}
	var searchErr error
	// binary-search for the smallest index i for which cmp(i) >= 0
	result := sort.Search(int(n), func(i int) bool {
		at := entrydb.EntryIdx(i)
		if reverse {
			at = entrydb.EntryIdx(n) - 1 - at
		}
		entry, err := db.readAt(at)
		if err != nil {
			searchErr = err
			return false
		}
		return cmpFn(entry) >= 0
	})
	if searchErr != nil {
		return -1, LinkEntry{}, fmt.Errorf("failed to search: %w", searchErr)
	}
	if result == int(n) {
		if reverse {
			return -1, LinkEntry{}, fmt.Errorf("no entry found: %w", types.ErrSkipped)
		} else {
			return -1, LinkEntry{}, fmt.Errorf("no entry found: %w", types.ErrFuture)
		}
	}
	if reverse {
		result = int(n) - 1 - result
	}
	link, err := db.readAt(entrydb.EntryIdx(result))
	if err != nil {
		return -1, LinkEntry{}, fmt.Errorf("failed to read final result entry %d: %w", result, err)
	}
	if cmpFn(link) != 0 {
		if reverse {
			return -1, LinkEntry{}, fmt.Errorf("lowest entry %s is too high: %w", link, types.ErrFuture)
		} else {
			return -1, LinkEntry{}, fmt.Errorf("lowest entry %s is too high: %w", link, types.ErrSkipped)
		}
	}
	if cmpFn(link) != 0 {
		// Search should have returned lowest entry >= the target.
		// And we already checked it's not > the target
		panic(fmt.Errorf("invalid search result %s, did not match equality check", link))
	}
	return entrydb.EntryIdx(result), link, nil
}

func (db *DB) readAt(i entrydb.EntryIdx) (LinkEntry, error) {
	entry, err := db.store.Read(i)
	if err != nil {
		if err == io.EOF {
			return LinkEntry{}, types.ErrFuture
		}
		return LinkEntry{}, err
	}
	var out LinkEntry
	err = out.decode(entry)
	return out, err
}

func (db *DB) Close() error {
	db.rwLock.Lock()
	defer db.rwLock.Unlock()
	return db.store.Close()
}
