package db

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func (db *ChainsDB) FindSealedBlock(chain types.ChainID, number uint64) (seal types.BlockSeal, err error) {
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return types.BlockSeal{}, fmt.Errorf("%w: %v", types.ErrUnknownChain, chain)
	}
	return logDB.FindSealedBlock(number)
}

// LatestBlockNum returns the latest fully-sealed block number that has been recorded to the logs db
// for the given chain. It does not contain safety guarantees.
// The block number might not be available (empty database, or non-existent chain).
func (db *ChainsDB) LatestBlockNum(chain types.ChainID) (num uint64, ok bool) {
	logDB, knownChain := db.logDBs.Get(chain)
	if !knownChain {
		return 0, false
	}
	return logDB.LatestSealedBlockNum()
}

func (db *ChainsDB) IsCrossUnsafe(chainID types.ChainID, block eth.BlockID) error {
	v, ok := db.crossUnsafe.Get(chainID)
	if !ok {
		return types.ErrUnknownChain
	}
	crossUnsafe := v.Get()
	if crossUnsafe == (types.BlockSeal{}) {
		return types.ErrFuture
	}
	if block.Number > crossUnsafe.Number {
		return types.ErrFuture
	}
	// TODO(#11693): make cross-unsafe reorg safe
	return nil
}

func (db *ChainsDB) ParentBlock(chainID types.ChainID, parentOf eth.BlockID) (parent eth.BlockID, err error) {
	logDB, ok := db.logDBs.Get(chainID)
	if !ok {
		return eth.BlockID{}, types.ErrUnknownChain
	}
	if parentOf.Number == 0 {
		return eth.BlockID{}, nil
	}
	// TODO(#11693): make parent-lookup reorg safe
	got, err := logDB.FindSealedBlock(parentOf.Number - 1)
	if err != nil {
		return eth.BlockID{}, err
	}
	return got.ID(), nil
}

func (db *ChainsDB) IsLocalUnsafe(chainID types.ChainID, block eth.BlockID) error {
	logDB, ok := db.logDBs.Get(chainID)
	if !ok {
		return types.ErrUnknownChain
	}
	got, err := logDB.FindSealedBlock(block.Number)
	if err != nil {
		return err
	}
	if got.ID() != block {
		return fmt.Errorf("found %s but was looking for unsafe block %s: %w", got, block, types.ErrConflict)
	}
	return nil
}

func (db *ChainsDB) LocalUnsafe(chainID types.ChainID) (types.BlockSeal, error) {
	eventsDB, ok := db.logDBs.Get(chainID)
	if !ok {
		return types.BlockSeal{}, types.ErrUnknownChain
	}
	n, ok := eventsDB.LatestSealedBlockNum()
	if !ok {
		return types.BlockSeal{}, types.ErrFuture
	}
	return eventsDB.FindSealedBlock(n)
}

func (db *ChainsDB) CrossUnsafe(chainID types.ChainID) (types.BlockSeal, error) {
	result, ok := db.crossUnsafe.Get(chainID)
	if !ok {
		return types.BlockSeal{}, types.ErrUnknownChain
	}
	crossUnsafe := result.Get()
	// Fall back to cross-safe if cross-unsafe is not known yet
	if crossUnsafe == (types.BlockSeal{}) {
		_, crossSafe, err := db.CrossSafe(chainID)
		if err != nil {
			return types.BlockSeal{}, fmt.Errorf("no cross-unsafe known for chain %s, and failed to fall back to cross-safe value: %w", chainID, err)
		}
		return crossSafe, nil
	}
	return crossUnsafe, nil
}

func (db *ChainsDB) LocalSafe(chainID types.ChainID) (derivedFrom types.BlockSeal, derived types.BlockSeal, err error) {
	localDB, ok := db.localDBs.Get(chainID)
	if !ok {
		return types.BlockSeal{}, types.BlockSeal{}, types.ErrUnknownChain
	}
	return localDB.Latest()
}

func (db *ChainsDB) CrossSafe(chainID types.ChainID) (derivedFrom types.BlockSeal, derived types.BlockSeal, err error) {
	crossDB, ok := db.crossDBs.Get(chainID)
	if !ok {
		return types.BlockSeal{}, types.BlockSeal{}, types.ErrUnknownChain
	}
	return crossDB.Latest()
}

func (db *ChainsDB) Finalized(chainID types.ChainID) (types.BlockSeal, error) {
	finalizedL1 := db.finalizedL1.Get()
	if finalizedL1 == (eth.L1BlockRef{}) {
		return types.BlockSeal{}, errors.New("no finalized L1 signal, cannot determine L2 finality yet")
	}
	derived, err := db.LastDerivedFrom(chainID, finalizedL1.ID())
	if err != nil {
		return types.BlockSeal{}, errors.New("could not find what was last derived from the finalized L1 block")
	}
	return derived, nil
}

func (db *ChainsDB) LastDerivedFrom(chainID types.ChainID, derivedFrom eth.BlockID) (derived types.BlockSeal, err error) {
	crossDB, ok := db.crossDBs.Get(chainID)
	if !ok {
		return types.BlockSeal{}, types.ErrUnknownChain
	}
	return crossDB.LastDerivedAt(derivedFrom)
}

// CrossDerivedFromBlockRef returns the block that the given block was derived from, if it exists in the cross derived-from storage.
// This includes the parent-block lookup. Use CrossDerivedFrom if no parent-block info is needed.
func (db *ChainsDB) CrossDerivedFromBlockRef(chainID types.ChainID, derived eth.BlockID) (derivedFrom eth.BlockRef, err error) {
	xdb, ok := db.crossDBs.Get(chainID)
	if !ok {
		return eth.BlockRef{}, types.ErrUnknownChain
	}
	res, err := xdb.DerivedFrom(derived)
	if err != nil {
		return eth.BlockRef{}, err
	}
	parent, err := xdb.PreviousDerivedFrom(res.ID())
	// if we are working with the first item in the database, PreviousDerivedFrom will return ErrPreviousToFirst
	// in which case we can attach a zero parent to the cross-derived-from block, as the parent block is unknown
	if errors.Is(err, types.ErrPreviousToFirst) {
		return res.ForceWithParent(eth.BlockID{}), nil
	} else if err != nil {
		return eth.BlockRef{}, err
	}
	return res.MustWithParent(parent.ID()), nil
}

// Check calls the underlying logDB to determine if the given log entry exists at the given location.
// If the block-seal of the block that includes the log is known, it is returned. It is fully zeroed otherwise, if the block is in-progress.
func (db *ChainsDB) Check(chain types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (includedIn types.BlockSeal, err error) {
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return types.BlockSeal{}, fmt.Errorf("%w: %v", types.ErrUnknownChain, chain)
	}
	return logDB.Contains(blockNum, logIdx, logHash)
}

// OpenBlock returns the Executing Messages for the block at the given number on the given chain.
// it routes the request to the appropriate logDB.
func (db *ChainsDB) OpenBlock(chainID types.ChainID, blockNum uint64) (seal eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	logDB, ok := db.logDBs.Get(chainID)
	if !ok {
		return eth.BlockRef{}, 0, nil, types.ErrUnknownChain
	}
	return logDB.OpenBlock(blockNum)
}

// LocalDerivedFrom returns the block that the given block was derived from, if it exists in the local derived-from storage.
// it routes the request to the appropriate localDB.
func (db *ChainsDB) LocalDerivedFrom(chain types.ChainID, derived eth.BlockID) (derivedFrom types.BlockSeal, err error) {
	lDB, ok := db.localDBs.Get(chain)
	if !ok {
		return types.BlockSeal{}, types.ErrUnknownChain
	}
	return lDB.DerivedFrom(derived)
}

// CrossDerivedFrom returns the block that the given block was derived from, if it exists in the cross derived-from storage.
// it routes the request to the appropriate crossDB.
func (db *ChainsDB) CrossDerivedFrom(chain types.ChainID, derived eth.BlockID) (derivedFrom types.BlockSeal, err error) {
	xDB, ok := db.crossDBs.Get(chain)
	if !ok {
		return types.BlockSeal{}, types.ErrUnknownChain
	}
	return xDB.DerivedFrom(derived)
}

// CandidateCrossSafe returns the candidate local-safe block that may become cross-safe.
//
// This returns ErrFuture if no block is known yet.
//
// Or ErrConflict if there is an inconsistency between the local-safe and cross-safe DB.
//
// Or ErrOutOfScope, with non-zero derivedFromScope,
// if additional L1 data is needed to cross-verify the candidate L2 block.
func (db *ChainsDB) CandidateCrossSafe(chain types.ChainID) (derivedFromScope, crossSafe eth.BlockRef, err error) {
	xDB, ok := db.crossDBs.Get(chain)
	if !ok {
		return eth.BlockRef{}, eth.BlockRef{}, types.ErrUnknownChain
	}

	lDB, ok := db.localDBs.Get(chain)
	if !ok {
		return eth.BlockRef{}, eth.BlockRef{}, types.ErrUnknownChain
	}

	// Example:
	// A B C D      <- L1
	// 1     2      <- L2
	// return:
	// (A, 0) -> initial scope, no L2 block yet. Genesis found to be cross-safe
	// (A, 1) -> 1 is determined cross-safe, won't be a candidate anymore after. 2 is the new candidate
	// (B, 2) -> 2 is out of scope, go to B
	// (C, 2) -> 2 is out of scope, go to C
	// (D, 2) -> 2 is in scope, stay on D, promote candidate to cross-safe
	// (D, 3) -> look at 3 next, see if we have to bump L1 yet, try with same L1 scope first

	crossDerivedFrom, crossDerived, err := xDB.Latest()
	if err != nil {
		if errors.Is(err, types.ErrFuture) {
			// If we do not have any cross-safe block yet, then return the first local-safe block.
			derivedFrom, derived, err := lDB.First()
			if err != nil {
				return eth.BlockRef{}, eth.BlockRef{}, fmt.Errorf("failed to find first local-safe block: %w", err)
			}
			// the first derivedFrom (L1 block) is unlikely to be the genesis block,
			derivedFromRef, err := derivedFrom.WithParent(eth.BlockID{})
			if err != nil {
				// if the first derivedFrom isn't the genesis block, just warn and continue anyway
				db.logger.Warn("First DerivedFrom is not genesis block")
				derivedFromRef = derivedFrom.ForceWithParent(eth.BlockID{})
			}
			// the first derived must be the genesis block, panic otherwise
			derivedRef := derived.MustWithParent(eth.BlockID{})
			return derivedFromRef, derivedRef, nil
		}
		return eth.BlockRef{}, eth.BlockRef{}, err
	}
	// Find the local-safe block that comes right after the last seen cross-safe block.
	// Just L2 block by block traversal, conditional on being local-safe.
	// This will be the candidate L2 block to promote.

	// While the local-safe block isn't cross-safe given limited L1 scope, we'll keep bumping the L1 scope,
	// And update cross-safe accordingly.
	// This method will keep returning the latest known scope that has been verified to be cross-safe.
	candidateFrom, candidate, err := lDB.NextDerived(crossDerived.ID())
	if err != nil {
		return eth.BlockRef{}, eth.BlockRef{}, err
	}

	candidateRef := candidate.MustWithParent(crossDerived.ID())

	parentDerivedFrom, err := lDB.PreviousDerivedFrom(candidateFrom.ID())
	// if we are working with the first item in the database, PreviousDerivedFrom will return ErrPreviousToFirst
	// in which case we can attach a zero parent to the cross-derived-from block, as the parent block is unknown
	if errors.Is(err, types.ErrPreviousToFirst) {
		parentDerivedFrom = types.BlockSeal{}
	} else if err != nil {
		return eth.BlockRef{}, eth.BlockRef{}, fmt.Errorf("failed to find parent-block of derived-from %s: %w", candidateFrom, err)
	}
	candidateFromRef := candidateFrom.MustWithParent(parentDerivedFrom.ID())

	// Allow increment of DA by 1, if we know the floor (due to local safety) is 1 ahead of the current cross-safe L1 scope.
	if candidateFrom.Number > crossDerivedFrom.Number+1 {
		// If we are not ready to process the candidate block,
		// then we need to stick to the current scope, so the caller can bump up from there.
		var crossDerivedFromRef eth.BlockRef
		parent, err := lDB.PreviousDerivedFrom(crossDerivedFrom.ID())
		// if we are working with the first item in the database, PreviousDerivedFrom will return ErrPreviousToFirst
		// in which case we can attach a zero parent to the cross-derived-from block, as the parent block is unknown
		if errors.Is(err, types.ErrPreviousToFirst) {
			crossDerivedFromRef = crossDerivedFrom.ForceWithParent(eth.BlockID{})
		} else if err != nil {
			return eth.BlockRef{}, eth.BlockRef{},
				fmt.Errorf("failed to find parent-block of cross-derived-from %s: %w", crossDerivedFrom, err)
		} else {
			crossDerivedFromRef = crossDerivedFrom.MustWithParent(parent.ID())
		}
		return crossDerivedFromRef, eth.BlockRef{},
			fmt.Errorf("candidate is from %s, while current scope is %s: %w",
				candidateFrom, crossDerivedFrom, types.ErrOutOfScope)
	}
	return candidateFromRef, candidateRef, nil
}

func (db *ChainsDB) PreviousDerived(chain types.ChainID, derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
	lDB, ok := db.localDBs.Get(chain)
	if !ok {
		return types.BlockSeal{}, types.ErrUnknownChain
	}
	return lDB.PreviousDerived(derived)
}

func (db *ChainsDB) PreviousDerivedFrom(chain types.ChainID, derivedFrom eth.BlockID) (prevDerivedFrom types.BlockSeal, err error) {
	lDB, ok := db.localDBs.Get(chain)
	if !ok {
		return types.BlockSeal{}, types.ErrUnknownChain
	}
	return lDB.PreviousDerivedFrom(derivedFrom)
}

func (db *ChainsDB) NextDerivedFrom(chain types.ChainID, derivedFrom eth.BlockID) (after eth.BlockRef, err error) {
	lDB, ok := db.localDBs.Get(chain)
	if !ok {
		return eth.BlockRef{}, types.ErrUnknownChain
	}
	v, err := lDB.NextDerivedFrom(derivedFrom)
	if err != nil {
		return eth.BlockRef{}, err
	}
	return v.MustWithParent(derivedFrom), nil
}

// Safest returns the strongest safety level that can be guaranteed for the given log entry.
// it assumes the log entry has already been checked and is valid, this function only checks safety levels.
// Safety levels are assumed to graduate from LocalUnsafe to LocalSafe to CrossUnsafe to CrossSafe, with Finalized as the strongest.
func (db *ChainsDB) Safest(chainID types.ChainID, blockNum uint64, index uint32) (safest types.SafetyLevel, err error) {
	if finalized, err := db.Finalized(chainID); err == nil {
		if finalized.Number >= blockNum {
			return types.Finalized, nil
		}
	}
	_, crossSafe, err := db.CrossSafe(chainID)
	if err != nil {
		return types.Invalid, err
	}
	if crossSafe.Number >= blockNum {
		return types.CrossSafe, nil
	}
	crossUnsafe, err := db.CrossUnsafe(chainID)
	if err != nil {
		return types.Invalid, err
	}
	// TODO(#12425): API: "index" for in-progress block building shouldn't be exposed from DB.
	//  For now we're not counting anything cross-safe until the block is sealed.
	if blockNum <= crossUnsafe.Number {
		return types.CrossUnsafe, nil
	}
	_, localSafe, err := db.LocalSafe(chainID)
	if err != nil {
		return types.Invalid, err
	}
	if blockNum <= localSafe.Number {
		return types.LocalSafe, nil
	}
	return types.LocalUnsafe, nil
}

func (db *ChainsDB) IteratorStartingAt(chain types.ChainID, sealedNum uint64, logIndex uint32) (logs.Iterator, error) {
	logDB, ok := db.logDBs.Get(chain)
	if !ok {
		return nil, fmt.Errorf("%w: %v", types.ErrUnknownChain, chain)
	}
	return logDB.IteratorStartingAt(sealedNum, logIndex)
}
