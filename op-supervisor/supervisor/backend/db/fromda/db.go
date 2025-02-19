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

// First returns the first known values, alike to Latest.
func (db *DB) First() (pair types.DerivedBlockSealPair, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	lastIndex := db.store.LastEntryIdx()
	if lastIndex < 0 {
		return types.DerivedBlockSealPair{}, types.ErrFuture
	}
	last, err := db.readAt(0)
	if err != nil {
		return types.DerivedBlockSealPair{}, fmt.Errorf("failed to read first derivation data: %w", err)
	}
	return last.sealOrErr()
}

func (db *DB) PreviousDerived(derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	// last is always the latest view, and thus canonical.
	_, lastCanonical, err := db.derivedNumToLastSource(derived.Number)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to find last derived %d: %w", derived.Number, err)
	}
	// get the first time this L2 block was seen.
	selfIndex, self, err := db.derivedNumToFirstSource(derived.Number)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to find first derived %d: %w", derived.Number, err)
	}
	// The first entry might not match, since it may have been invalidated with a later L1 scope.
	// But the last entry should always match.
	if lastCanonical.derived.ID() != derived {
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
// source: the L1 block that the L2 block is safe for (not necessarily the first, multiple L2 blocks may be derived from the same L1 block).
// derived: the L2 block that was derived (not necessarily the first, the L1 block may have been empty and repeated the last safe L2 block).
// If the last entry is invalidated, this returns a types.ErrAwaitReplacementBlock error.
func (db *DB) Last() (pair types.DerivedBlockSealPair, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	link, err := db.latest()
	if err != nil {
		return types.DerivedBlockSealPair{}, err
	}
	return link.sealOrErr()
}

// latest is like Latest, but without lock, for internal use.
func (db *DB) latest() (link LinkEntry, err error) {
	lastIndex := db.store.LastEntryIdx()
	if lastIndex < 0 {
		return LinkEntry{}, types.ErrFuture
	}
	last, err := db.readAt(lastIndex)
	if err != nil {
		return LinkEntry{}, fmt.Errorf("failed to read last derivation data: %w", err)
	}
	return last, nil
}

func (db *DB) Invalidated() (pair types.DerivedBlockSealPair, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	link, err := db.latest()
	if err != nil {
		return types.DerivedBlockSealPair{}, err
	}
	if !link.invalidated {
		return types.DerivedBlockSealPair{}, fmt.Errorf("last entry %s is not invalidated: %w", link, types.ErrConflict)
	}
	return types.DerivedBlockSealPair{
		Source:  link.source,
		Derived: link.derived,
	}, nil
}

// LastDerivedAt returns the last L2 block derived from the given L1 block.
// This may return types.ErrAwaitReplacementBlock if the entry was invalidated and needs replacement.
func (db *DB) SourceToLastDerived(source eth.BlockID) (derived types.BlockSeal, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	_, link, err := db.sourceNumToLastDerived(source.Number)
	if err != nil {
		return types.BlockSeal{}, err
	}
	if link.source.ID() != source {
		return types.BlockSeal{}, fmt.Errorf("searched for last derived-from %s but found %s: %w",
			source, link.source, types.ErrConflict)
	}
	if link.invalidated {
		return types.BlockSeal{}, types.ErrAwaitReplacementBlock
	}
	return link.derived, nil
}

// NextDerived finds the next L2 block after derived, and what it was derived from.
// This may return types.ErrAwaitReplacementBlock if the entry was invalidated and needs replacement.
func (db *DB) NextDerived(derived eth.BlockID) (pair types.DerivedBlockSealPair, err error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	// get the last time this L2 block was seen.
	selfIndex, self, err := db.derivedNumToLastSource(derived.Number)
	if err != nil {
		return types.DerivedBlockSealPair{}, fmt.Errorf("failed to find derived %d: %w", derived.Number, err)
	}
	if self.derived.ID() != derived {
		return types.DerivedBlockSealPair{}, fmt.Errorf("found %s, but expected %s: %w", self.derived, derived, types.ErrConflict)
	}
	next, err := db.readAt(selfIndex + 1)
	if err != nil {
		return types.DerivedBlockSealPair{}, fmt.Errorf("cannot find next derived after %s: %w", derived, err)
	}
	return next.sealOrErr()
}

// ContainsDerived checks if the given block is canonical for the given chain.
// This returns an ErrFuture if the block is not known yet.
// An ErrConflict if there is a different block.
// Or an ErrAwaitReplacementBlock if it was invalidated.
func (db *DB) ContainsDerived(derived eth.BlockID) error {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	// Take the last entry: this will be the latest canonical view,
	// if the block was previously invalidated.
	_, link, err := db.derivedNumToLastSource(derived.Number)
	if err != nil {
		return err
	}
	if link.derived.ID() != derived {
		return fmt.Errorf("searched if derived %s but found %s: %w",
			derived, link.derived, types.ErrConflict)
	}
	if link.invalidated {
		return fmt.Errorf("derived %s, but invalidated it: %w", derived, types.ErrAwaitReplacementBlock)
	}
	return nil
}

// DerivedToFirstSource determines where a L2 block was first derived from.
// (a L2 block may repeat if the following L1 blocks are empty and don't produce additional L2 blocks)
func (db *DB) DerivedToFirstSource(derived eth.BlockID) (types.BlockSeal, error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	_, link, err := db.derivedNumToFirstSource(derived.Number)
	if err != nil {
		return types.BlockSeal{}, err
	}
	if link.derived.ID() != derived {
		return types.BlockSeal{}, fmt.Errorf("searched for first derived %s but found %s: %w",
			derived, link.derived, types.ErrConflict)
	}
	return link.source, nil
}

func (db *DB) PreviousSource(source eth.BlockID) (types.BlockSeal, error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	return db.previousSource(source)
}

func (db *DB) previousSource(source eth.BlockID) (types.BlockSeal, error) {
	// get the last time this L1 block was seen.
	selfIndex, self, err := db.sourceNumToFirstDerived(source.Number)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to find derived %d: %w", source.Number, err)
	}
	if self.source.ID() != source {
		return types.BlockSeal{}, fmt.Errorf("found %s, but expected %s: %w", self.source, source, types.ErrConflict)
	}
	if selfIndex == 0 {
		// genesis block has a zeroed block as parent block
		if self.source.Number == 0 {
			return types.BlockSeal{}, nil
		} else {
			return types.BlockSeal{},
				fmt.Errorf("cannot find previous derived before start of database: %s (%w)", source, types.ErrPreviousToFirst)
		}
	}
	prev, err := db.readAt(selfIndex - 1)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("cannot find previous derived before %s: %w", source, err)
	}
	return prev.source, nil
}

// NextSource finds the next source after the given source
func (db *DB) NextSource(source eth.BlockID) (types.BlockSeal, error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	selfIndex, self, err := db.sourceNumToLastDerived(source.Number)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("failed to find derived-from %d: %w", source.Number, err)
	}
	if self.source.ID() != source {
		return types.BlockSeal{}, fmt.Errorf("found %s, but expected %s: %w", self.source, source, types.ErrConflict)
	}
	next, err := db.readAt(selfIndex + 1)
	if err != nil {
		return types.BlockSeal{}, fmt.Errorf("cannot find next derived-from after %s: %w", source, err)
	}
	return next.source, nil
}

// Next returns the next Derived Block Pair after the given pair.
// This may return types.ErrAwaitReplacementBlock if the entry was invalidated and needs replacement.
func (db *DB) Next(pair types.DerivedIDPair) (types.DerivedBlockSealPair, error) {
	db.rwLock.RLock()
	defer db.rwLock.RUnlock()
	selfIndex, selfLink, err := db.lookup(pair.Source.Number, pair.Derived.Number)
	if err != nil {
		return types.DerivedBlockSealPair{}, err
	}
	if selfLink.source.ID() != pair.Source {
		return types.DerivedBlockSealPair{}, fmt.Errorf("DB has derived-from %s but expected %s: %w", selfLink.source, pair.Source, types.ErrConflict)
	}
	if selfLink.derived.ID() != pair.Derived {
		return types.DerivedBlockSealPair{}, fmt.Errorf("DB has derived %s but expected %s: %w", selfLink.derived, pair.Derived, types.ErrConflict)
	}
	next, err := db.readAt(selfIndex + 1)
	if err != nil {
		return types.DerivedBlockSealPair{}, err
	}
	return next.sealOrErr()
}

func (db *DB) derivedNumToFirstSource(derivedNum uint64) (entrydb.EntryIdx, LinkEntry, error) {
	// Forward: prioritize the first entry.
	return db.find(false, func(link LinkEntry) int {
		return cmp.Compare(link.derived.Number, derivedNum)
	})
}

func (db *DB) derivedNumToLastSource(derivedNum uint64) (entrydb.EntryIdx, LinkEntry, error) {
	// Reverse: prioritize the last entry.
	return db.find(true, func(link LinkEntry) int {
		return cmp.Compare(derivedNum, link.derived.Number)
	})
}

func (db *DB) sourceNumToFirstDerived(sourceNum uint64) (entrydb.EntryIdx, LinkEntry, error) {
	// Forward: prioritize the first entry.
	return db.find(false, func(link LinkEntry) int {
		return cmp.Compare(link.source.Number, sourceNum)
	})
}

func (db *DB) sourceNumToLastDerived(sourceNum uint64) (entrydb.EntryIdx, LinkEntry, error) {
	// Reverse: prioritize the last entry.
	return db.find(true, func(link LinkEntry) int {
		return cmp.Compare(sourceNum, link.source.Number)
	})
}

func (db *DB) lookup(source, derived uint64) (entrydb.EntryIdx, LinkEntry, error) {
	return db.find(false, func(link LinkEntry) int {
		res := cmp.Compare(link.derived.Number, derived)
		if res == 0 {
			return cmp.Compare(link.source.Number, source)
		}
		return res
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
	// i.e. find the earliest entry that is bigger or equal than the needle.
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
	// If we did not find anything, then we got the length of the input.
	if result == int(n) {
		if reverse {
			// If searching in reverse, then the last entry is the start.
			// I.e. the needle must be before the db start.
			return -1, LinkEntry{}, fmt.Errorf("no entry found: %w", types.ErrSkipped)
		} else {
			// If searing regularly, then the last entry is the end.
			// I.e. the needle must be after the db end.
			return -1, LinkEntry{}, fmt.Errorf("no entry found: %w", types.ErrFuture)
		}
	}
	// If the very first entry matched, then we might be missing prior data.
	firstTry := result == 0
	// Transform back the index, if we were searching in reverse
	if reverse {
		result = int(n) - 1 - result
	}
	// Whatever we found as first entry to be bigger or equal, must be checked for equality.
	// We don't want it if it's bigger, we were searching for the equal-case.
	link, err := db.readAt(entrydb.EntryIdx(result))
	if err != nil {
		return -1, LinkEntry{}, fmt.Errorf("failed to read final result entry %d: %w", result, err)
	}
	if cmpFn(link) != 0 {
		if firstTry { // if the first found entry already is bigger, then we are missing the real data.
			if reverse {
				return -1, LinkEntry{}, fmt.Errorf("query is past last entry %s: %w", link, types.ErrFuture)
			} else {
				return -1, LinkEntry{}, fmt.Errorf("query is before first entry %s: %w", link, types.ErrSkipped)
			}
		} else {
			return -1, LinkEntry{}, fmt.Errorf("traversed data, no exact match found, but hit %s: %w", link, types.ErrDataCorruption)
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
