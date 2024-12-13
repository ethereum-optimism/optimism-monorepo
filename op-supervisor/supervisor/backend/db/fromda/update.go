package fromda

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func (db *DB) AddDerived(derivedFrom eth.BlockRef, derived eth.BlockRef) error {
	db.rwLock.Lock()
	defer db.rwLock.Unlock()

	// If we don't have any entries yet, allow any block to start things off
	if db.store.Size() == 0 {
		link := LinkEntry{
			derivedFrom: types.BlockSeal{
				Hash:      derivedFrom.Hash,
				Number:    derivedFrom.Number,
				Timestamp: derivedFrom.Time,
			},
			derived: types.BlockSeal{
				Hash:      derived.Hash,
				Number:    derived.Number,
				Timestamp: derived.Time,
			},
		}
		e := link.encode()
		if err := db.store.Append(e); err != nil {
			return err
		}
		db.m.RecordDBDerivedEntryCount(db.store.Size())
		return nil
	}

	lastDerivedFrom, lastDerived, err := db.latest()
	if err != nil {
		return err
	}

	if lastDerived.ID() == derived.ID() && lastDerivedFrom.ID() == derivedFrom.ID() {
		// it shouldn't be possible, but the ID component of a block ref doesn't include the timestamp
		// so if the timestampt doesn't match, still return no error to the caller, but at least log a warning
		if lastDerived.Timestamp != derived.Time {
			db.log.Warn("Derived block already exists with different timestamp", "derived", derived, "lastDerived", lastDerived)
		}
		if lastDerivedFrom.Timestamp != derivedFrom.Time {
			db.log.Warn("Derived-from block already exists with different timestamp", "derivedFrom", derivedFrom, "lastDerivedFrom", lastDerivedFrom)
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
		if lastDerived.Hash != derived.Hash {
			return fmt.Errorf("derived block %s conflicts with known derived block %s at same height: %w",
				derived, lastDerived, types.ErrConflict)
		}
	} else if lastDerived.Number+1 == derived.Number {
		if lastDerived.Hash != derived.ParentHash {
			return fmt.Errorf("derived block %s (parent %s) does not build on %s: %w",
				derived, derived.ParentHash, lastDerived, types.ErrConflict)
		}
	} else if lastDerived.Number+1 < derived.Number {
		return fmt.Errorf("cannot add block (%s derived from %s), last block (%s derived from %s) is too far behind: (%w)",
			derived, derivedFrom,
			lastDerived, lastDerivedFrom,
			types.ErrOutOfOrder)
	} else {
		return fmt.Errorf("derived block %s is older than current derived block %s: %w",
			derived, lastDerived, types.ErrOutOfOrder)
	}

	// Check derived-from relation: multiple L2 blocks may be derived from the same L1 block. But everything in sequence.
	if lastDerivedFrom.Number == derivedFrom.Number {
		// Same block height? Then it must be the same block.
		if lastDerivedFrom.Hash != derivedFrom.Hash {
			return fmt.Errorf("cannot add block %s as derived from %s, expected to be derived from %s at this block height: %w",
				derived, derivedFrom, lastDerivedFrom, types.ErrConflict)
		}
	} else if lastDerivedFrom.Number+1 == derivedFrom.Number {
		// parent hash check
		if lastDerivedFrom.Hash != derivedFrom.ParentHash {
			return fmt.Errorf("cannot add block %s as derived from %s (parent %s) derived on top of %s: %w",
				derived, derivedFrom, derivedFrom.ParentHash, lastDerivedFrom, types.ErrConflict)
		}
	} else if lastDerivedFrom.Number+1 < derivedFrom.Number {
		// adding block that is derived from something too far into the future
		return fmt.Errorf("cannot add block (%s derived from %s), last block (%s derived from %s) is too far behind: (%w)",
			derived, derivedFrom,
			lastDerived, lastDerivedFrom,
			types.ErrOutOfOrder)
	} else {
		// adding block that is derived from something too old
		return fmt.Errorf("cannot add block %s as derived from %s, deriving already at %s: %w",
			derived, derivedFrom, lastDerivedFrom, types.ErrOutOfOrder)
	}

	link := LinkEntry{
		derivedFrom: types.BlockSeal{
			Hash:      derivedFrom.Hash,
			Number:    derivedFrom.Number,
			Timestamp: derivedFrom.Time,
		},
		derived: types.BlockSeal{
			Hash:      derived.Hash,
			Number:    derived.Number,
			Timestamp: derived.Time,
		},
	}
	e := link.encode()
	if err := db.store.Append(e); err != nil {
		return err
	}
	db.m.RecordDBDerivedEntryCount(db.store.Size())
	return nil
}
