package fromda

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func (db *DB) AddDerived(derivedFrom eth.BlockRef, derived eth.BlockRef) error {
	db.rwLock.Lock()
	defer db.rwLock.Unlock()
	return db.addLink(derivedFrom, derived, common.Hash{})
}

// ReplaceInvalidatedBlock replaces the current Invalidated block with the given replacement.
// The to-be invalidated hash must be provided for consistency checks.
func (db *DB) ReplaceInvalidatedBlock(replacementDerived eth.BlockRef, invalidated common.Hash) (types.DerivedBlockSealPair, error) {
	db.rwLock.Lock()
	defer db.rwLock.Unlock()

	db.log.Warn("Replacing invalidated block", "replacement", replacementDerived, "invalidated", invalidated)

	// We take the last occurrence. This is where it started to be considered invalid,
	// and where we thus stopped building additional entries for it.
	lastIndex := db.store.LastEntryIdx()
	if lastIndex < 0 {
		return types.DerivedBlockSealPair{}, types.ErrFuture
	}
	last, err := db.readAt(lastIndex)
	if err != nil {
		return types.DerivedBlockSealPair{}, fmt.Errorf("failed to read last derivation data: %w", err)
	}
	if !last.invalidated {
		return types.DerivedBlockSealPair{}, fmt.Errorf("cannot replace block %d, that was not invalidated, with block %s: %w", last.derived, replacementDerived, types.ErrConflict)
	}
	if last.derived.Hash != invalidated {
		return types.DerivedBlockSealPair{}, fmt.Errorf("cannot replace invalidated %s, DB contains %s: %w", invalidated, last.derived, types.ErrConflict)
	}
	// Find the parent-block of derived-from.
	// We need this to build a block-ref, so the DB can be consistency-checked when the next entry is added.
	// There is always one, since the first entry in the DB should never be an invalidated one.
	prevSource, err := db.previousSource(last.source.ID())
	if err != nil {
		return types.DerivedBlockSealPair{}, err
	}
	// Remove the invalidated placeholder and everything after
	err = db.store.Truncate(lastIndex - 1)
	if err != nil {
		return types.DerivedBlockSealPair{}, err
	}
	replacement := types.DerivedBlockRefPair{
		Source:  last.source.ForceWithParent(prevSource.ID()),
		Derived: replacementDerived,
	}
	// Insert the replacement
	if err := db.addLink(replacement.Source, replacement.Derived, invalidated); err != nil {
		return types.DerivedBlockSealPair{}, fmt.Errorf("failed to add %s as replacement at %s: %w", replacement.Derived, replacement.Source, err)
	}
	return replacement.Seals(), nil
}

// RewindAndInvalidate rolls back the database to just before the invalidated block,
// and then marks the block as invalidated, so that no new data can be added to the DB
// until a Rewind or ReplaceInvalidatedBlock.
func (db *DB) RewindAndInvalidate(invalidated types.DerivedBlockRefPair) error {
	db.rwLock.Lock()
	defer db.rwLock.Unlock()

	invalidatedSeals := types.DerivedBlockSealPair{
		Source:  types.BlockSealFromRef(invalidated.Source),
		Derived: types.BlockSealFromRef(invalidated.Derived),
	}
	if err := db.rewindLocked(invalidatedSeals, true); err != nil {
		return err
	}
	if err := db.addLink(invalidated.Source, invalidated.Derived, invalidated.Derived.Hash); err != nil {
		return fmt.Errorf("failed to add invalidation entry %s: %w", invalidated, err)
	}
	return nil
}

// Rewind rolls back the database to the target, including the target if the including flag is set.
// it locks the DB and calls rewindLocked.
func (db *DB) Rewind(target types.DerivedBlockSealPair, including bool) error {
	db.rwLock.Lock()
	defer db.rwLock.Unlock()
	return db.rewindLocked(target, including)
}

// RewindToScope rewinds the DB to the last entry with
// a derivedFrom value matching the given scope (inclusive, scope is retained in DB).
// Note that this drop L1 blocks that resulted in a previously invalidated local-safe block.
// This returns ErrFuture if the block is newer than the last known block.
// This returns ErrConflict if a different block at the given height is known.
// TODO: rename this "RewindToSource" to match the idea of Source
func (db *DB) RewindToScope(scope eth.BlockID) error {
	db.rwLock.Lock()
	defer db.rwLock.Unlock()
	_, link, err := db.sourceNumToLastDerived(scope.Number)
	if err != nil {
		return fmt.Errorf("failed to find last derived %d: %w", scope.Number, err)
	}
	if link.source.ID() != scope {
		return fmt.Errorf("found derived-from %s but expected %s: %w", link.source, scope, types.ErrConflict)
	}
	return db.rewindLocked(types.DerivedBlockSealPair{
		Source:  link.source,
		Derived: link.derived,
	}, false)
}

// RewindToFirstDerived rewinds to the first time
// when v was derived (inclusive, v is retained in DB).
func (db *DB) RewindToFirstDerived(v eth.BlockID) error {
	db.rwLock.Lock()
	defer db.rwLock.Unlock()
	_, link, err := db.derivedNumToFirstSource(v.Number)
	if err != nil {
		return fmt.Errorf("failed to find when %d was first derived: %w", v.Number, err)
	}
	if link.derived.ID() != v {
		return fmt.Errorf("found derived %s but expected %s: %w", link.derived, v, types.ErrConflict)
	}
	return db.rewindLocked(types.DerivedBlockSealPair{
		Source:  link.source,
		Derived: link.derived,
	}, false)
}

// rewindLocked performs the truncate operation to a specified block seal pair.
// data beyond the specified block seal pair is truncated from the database.
// if including is true, the block seal pair itself is removed as well.
// Note: This function must be called with the rwLock held.
// Callers are responsible for locking and unlocking the Database.
func (db *DB) rewindLocked(t types.DerivedBlockSealPair, including bool) error {
	i, link, err := db.lookup(t.Source.Number, t.Derived.Number)
	if err != nil {
		return err
	}
	if link.source.Hash != t.Source.Hash {
		return fmt.Errorf("found derived-from %s, but expected %s: %w",
			link.source, t.Source, types.ErrConflict)
	}
	if link.derived.Hash != t.Derived.Hash {
		return fmt.Errorf("found derived %s, but expected %s: %w",
			link.derived, t.Derived, types.ErrConflict)
	}
	// adjust the target index to include the block seal pair itself if requested
	target := i
	if including {
		target = i - 1
	}
	if err := db.store.Truncate(target); err != nil {
		return fmt.Errorf("failed to rewind upon block invalidation of %s: %w", t, err)
	}
	db.m.RecordDBDerivedEntryCount(int64(target) + 1)
	return nil
}

// addLink adds a L1/L2 derivation link, with strong consistency checks.
// if the link invalidates a prior L2 block, that was valid in a prior L1,
// the invalidated hash needs to match it, even if a new derived block replaces it.
func (db *DB) addLink(derivedFrom eth.BlockRef, derived eth.BlockRef, invalidated common.Hash) error {
	link := LinkEntry{
		source: types.BlockSeal{
			Hash:      derivedFrom.Hash,
			Number:    derivedFrom.Number,
			Timestamp: derivedFrom.Time,
		},
		derived: types.BlockSeal{
			Hash:      derived.Hash,
			Number:    derived.Number,
			Timestamp: derived.Time,
		},
		invalidated: (invalidated != common.Hash{}) && derived.Hash == invalidated,
	}
	// If we don't have any entries yet, allow any block to start things off
	if db.store.Size() == 0 {
		if link.invalidated {
			return fmt.Errorf("first DB entry %s cannot be an invalidated entry: %w", link, types.ErrConflict)
		}
		e := link.encode()
		if err := db.store.Append(e); err != nil {
			return err
		}
		db.m.RecordDBDerivedEntryCount(db.store.Size())
		return nil
	}

	last, err := db.latest()
	if err != nil {
		return err
	}
	if last.invalidated {
		return fmt.Errorf("cannot build %s on top of invalidated entry %s: %w", link, last, types.ErrConflict)
	}
	lastSource := last.source
	lastDerived := last.derived

	if lastDerived.ID() == derived.ID() && lastSource.ID() == derivedFrom.ID() {
		// it shouldn't be possible, but the ID component of a block ref doesn't include the timestamp
		// so if the timestampt doesn't match, still return no error to the caller, but at least log a warning
		if lastDerived.Timestamp != derived.Time {
			db.log.Warn("Derived block already exists with different timestamp", "derived", derived, "lastDerived", lastDerived)
		}
		if lastSource.Timestamp != derivedFrom.Time {
			db.log.Warn("Derived-from block already exists with different timestamp", "derivedFrom", derivedFrom, "lastSource", lastSource)
		}
		// Repeat of same information. No entries to be written.
		// But we can silently ignore and not return an error, as that brings the caller
		// in a consistent state, after which it can insert the actual new derived-from information.
		return nil
	}

	// Check derived relation: the L2 chain has to be sequential without gaps. An L2 block may repeat if the L1 block is empty.
	if lastDerived.Number == derived.Number {
		// Same block height? Then it must be the same block.
		// I.e. we encountered an empty L1 block, and the same L2 block continues to be the last block that was derived from it.
		if invalidated != (common.Hash{}) {
			if lastDerived.Hash != invalidated {
				return fmt.Errorf("inserting block %s that invalidates %s at height %d, but expected %s", derived.Hash, invalidated, lastDerived.Number, lastDerived.Hash)
			}
		} else {
			if lastDerived.Hash != derived.Hash {
				return fmt.Errorf("derived block %s conflicts with known derived block %s at same height: %w",
					derived, lastDerived, types.ErrConflict)
			}
		}
	} else if lastDerived.Number+1 == derived.Number {
		if lastDerived.Hash != derived.ParentHash {
			return fmt.Errorf("derived block %s (parent %s) does not build on %s: %w",
				derived, derived.ParentHash, lastDerived, types.ErrConflict)
		}
	} else if lastDerived.Number+1 < derived.Number {
		return fmt.Errorf("cannot add block (%s derived from %s), last block (%s derived from %s) is too far behind: (%w)",
			derived, derivedFrom,
			lastDerived, lastSource,
			types.ErrOutOfOrder)
	} else {
		return fmt.Errorf("derived block %s is older than current derived block %s: %w",
			derived, lastDerived, types.ErrOutOfOrder)
	}

	// Check derived-from relation: multiple L2 blocks may be derived from the same L1 block. But everything in sequence.
	if lastSource.Number == derivedFrom.Number {
		// Same block height? Then it must be the same block.
		if lastSource.Hash != derivedFrom.Hash {
			return fmt.Errorf("cannot add block %s as derived from %s, expected to be derived from %s at this block height: %w",
				derived, derivedFrom, lastSource, types.ErrConflict)
		}
	} else if lastSource.Number+1 == derivedFrom.Number {
		// parent hash check
		if lastSource.Hash != derivedFrom.ParentHash {
			return fmt.Errorf("cannot add block %s as derived from %s (parent %s) derived on top of %s: %w",
				derived, derivedFrom, derivedFrom.ParentHash, lastSource, types.ErrConflict)
		}
	} else if lastSource.Number+1 < derivedFrom.Number {
		// adding block that is derived from something too far into the future
		return fmt.Errorf("cannot add block (%s derived from %s), last block (%s derived from %s) is too far behind: (%w)",
			derived, derivedFrom,
			lastDerived, lastSource,
			types.ErrOutOfOrder)
	} else {
		if lastDerived.Hash == derived.Hash {
			// we might see L1 blocks repeat,
			// if the deriver has reset to the latest local-safe block,
			// since we don't reset it to any particular source block.
			// So check if it's canonical, and if it is, we can gracefully accept it, to allow forwards progress.
			_, got, err := db.lookup(derivedFrom.Number, derived.Number)
			if err != nil {
				return fmt.Errorf("failed to check if block %s with old source %s was derived from canonical source chain: %w",
					derived, derivedFrom, err)
			}
			if got.source.Hash != derivedFrom.Hash {
				return fmt.Errorf("cannot add block %s that matches latest derived since it is derived from non-canonical source %s, expected %s: %w",
					derived, derivedFrom, got.source, types.ErrConflict)
			}
			return fmt.Errorf("received latest block %s, derived from known old source %s, latest source is %s: %w",
				derived, derivedFrom, lastSource, types.ErrIneffective)
		}
		// Adding a newer block that is derived from an older source, that cannot be right
		return fmt.Errorf("cannot add block %s as derived from %s, deriving already at %s: %w",
			derived, derivedFrom, lastSource, types.ErrOutOfOrder)
	}

	e := link.encode()
	if err := db.store.Append(e); err != nil {
		return err
	}
	db.m.RecordDBDerivedEntryCount(db.store.Size())
	return nil
}
