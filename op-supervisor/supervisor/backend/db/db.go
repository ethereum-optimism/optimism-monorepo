package db

import (
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type LogStorage interface {
	io.Closer

	AddLog(logHash common.Hash, parentBlock eth.BlockID,
		logIdx uint32, execMsg *types.ExecutingMessage) error

	SealBlock(parentHash common.Hash, block eth.BlockID, timestamp uint64) error

	Rewind(newHead eth.BlockID) error

	LatestSealedBlock() (id eth.BlockID, ok bool)

	// FindSealedBlock finds the requested block by number, to check if it exists,
	// returning the block seal if it was found.
	// returns ErrFuture if the block is too new to be able to tell.
	FindSealedBlock(number uint64) (block types.BlockSeal, err error)

	IteratorStartingAt(sealedNum uint64, logsSince uint32) (logs.Iterator, error)

	// Contains returns no error iff the specified logHash is recorded in the specified blockNum and logIdx.
	// If the log is out of reach, then ErrFuture is returned.
	// If the log is determined to conflict with the canonical chain, then ErrConflict is returned.
	// logIdx is the index of the log in the array of all logs in the block.
	// This can be used to check the validity of cross-chain interop events.
	// The block-seal of the blockNum block, that the log was included in, is returned.
	// This seal may be fully zeroed, without error, if the block isn't fully known yet.
	Contains(types.ContainsQuery) (includedIn types.BlockSeal, err error)

	// OpenBlock accumulates the ExecutingMessage events for a block and returns them
	OpenBlock(blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

type DerivationStorage interface {
	// basic info
	First() (pair types.DerivedBlockSealPair, err error)
	Last() (pair types.DerivedBlockSealPair, err error)

	// mapping from source<>derived
	DerivedToFirstSource(derived eth.BlockID) (source types.BlockSeal, err error)
	SourceToLastDerived(source eth.BlockID) (derived types.BlockSeal, err error)

	// traversal
	Next(pair types.DerivedIDPair) (next types.DerivedBlockSealPair, err error)
	NextSource(source eth.BlockID) (nextSource types.BlockSeal, err error)
	NextDerived(derived eth.BlockID) (next types.DerivedBlockSealPair, err error)
	PreviousSource(source eth.BlockID) (prevSource types.BlockSeal, err error)
	PreviousDerived(derived eth.BlockID) (prevDerived types.BlockSeal, err error)

	// type-specific
	Invalidated() (pair types.DerivedBlockSealPair, err error)
	ContainsDerived(derived eth.BlockID) error

	// writing
	AddDerived(source eth.BlockRef, derived eth.BlockRef) error
	ReplaceInvalidatedBlock(replacementDerived eth.BlockRef, invalidated common.Hash) (types.DerivedBlockSealPair, error)

	// rewining
	RewindAndInvalidate(invalidated types.DerivedBlockRefPair) error
	RewindToScope(scope eth.BlockID) error
	RewindToFirstDerived(v eth.BlockID) error
}

var _ DerivationStorage = (*fromda.DB)(nil)

var _ LogStorage = (*logs.DB)(nil)

type Metrics interface {
	RecordCrossUnsafeRef(chainID eth.ChainID, ref eth.BlockRef)
	RecordCrossSafeRef(chainID eth.ChainID, ref eth.BlockRef)
}

// ChainsDB is a database that stores logs and derived-from data for multiple chains.
// it implements the LogStorage interface, as well as several DB interfaces needed by the cross package.
type ChainsDB struct {
	// unsafe info: the sequence of block seals and events
	logDBs locks.RWMap[eth.ChainID, LogStorage]

	// cross-unsafe: how far we have processed the unsafe data.
	// If present but set to a zeroed value the cross-unsafe will fallback to cross-safe.
	crossUnsafe locks.RWMap[eth.ChainID, *locks.RWValue[types.BlockSeal]]

	// local-safe: index of what we optimistically know about L2 blocks being derived from L1
	localDBs locks.RWMap[eth.ChainID, DerivationStorage]

	// cross-safe: index of L2 blocks we know to only have cross-L2 valid dependencies
	crossDBs locks.RWMap[eth.ChainID, DerivationStorage]

	// finalized: the L1 finality progress. This can be translated into what may be considered as finalized in L2.
	// It is initially zeroed, and the L2 finality query will return
	// an error until it has this L1 finality to work with.
	finalizedL1 locks.RWValue[eth.L1BlockRef]

	// depSet is the dependency set, used to determine what may be tracked,
	// what is missing, and to provide it to DB users.
	depSet depset.DependencySet

	logger log.Logger

	// emitter used to signal when the DB changes, for other modules to react to
	emitter event.Emitter

	m Metrics
}

var _ event.AttachEmitter = (*ChainsDB)(nil)

func NewChainsDB(l log.Logger, depSet depset.DependencySet, m Metrics) *ChainsDB {
	if m == nil {
		m = metrics.NoopMetrics
	}

	return &ChainsDB{
		logger: l,
		depSet: depSet,
		m:      m,
	}
}

func (db *ChainsDB) AttachEmitter(em event.Emitter) {
	db.emitter = em
}

func (db *ChainsDB) OnEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case superevents.AnchorEvent:
		db.logger.Info("Received chain anchor information",
			"chain", x.ChainID, "derived", x.Anchor.Derived, "source", x.Anchor.Source)
		db.maybeInitEventsDB(x.ChainID, x.Anchor)
		db.maybeInitSafeDB(x.ChainID, x.Anchor)
	case superevents.LocalDerivedEvent:
		db.UpdateLocalSafe(x.ChainID, x.Derived.Source, x.Derived.Derived)
	case superevents.FinalizedL1RequestEvent:
		db.onFinalizedL1(x.FinalizedL1)
	case superevents.ReplaceBlockEvent:
		db.onReplaceBlock(x.ChainID, x.Replacement.Replacement, x.Replacement.Invalidated)
	default:
		return false
	}
	return true
}

func (db *ChainsDB) AddLogDB(chainID eth.ChainID, logDB LogStorage) {
	if db.logDBs.Has(chainID) {
		db.logger.Warn("overwriting existing log DB for chain", "chain", chainID)
	}

	db.logDBs.Set(chainID, logDB)
}

func (db *ChainsDB) AddLocalDerivationDB(chainID eth.ChainID, dfDB DerivationStorage) {
	if db.localDBs.Has(chainID) {
		db.logger.Warn("overwriting existing local derived-from DB for chain", "chain", chainID)
	}

	db.localDBs.Set(chainID, dfDB)
}

func (db *ChainsDB) AddCrossDerivationDB(chainID eth.ChainID, dfDB DerivationStorage) {
	if db.crossDBs.Has(chainID) {
		db.logger.Warn("overwriting existing cross derived-from DB for chain", "chain", chainID)
	}

	db.crossDBs.Set(chainID, dfDB)
}

func (db *ChainsDB) AddCrossUnsafeTracker(chainID eth.ChainID) {
	if db.crossUnsafe.Has(chainID) {
		db.logger.Warn("overwriting existing cross-unsafe tracker for chain", "chain", chainID)
	}
	db.crossUnsafe.Set(chainID, &locks.RWValue[types.BlockSeal]{})
}

// ResumeFromLastSealedBlock prepares the chains db to resume recording events after a restart.
// It rewinds the database to the last block that is guaranteed to have been fully recorded to the database,
// to ensure it can resume recording from the first log of the next block.
func (db *ChainsDB) ResumeFromLastSealedBlock() error {
	var result error
	db.logDBs.Range(func(chain eth.ChainID, logStore LogStorage) bool {
		head, ok := logStore.LatestSealedBlock()
		if !ok {
			// db must be empty, nothing to rewind to
			db.logger.Info("Resuming, but found no DB contents", "chain", chain)
			return true
		}
		db.logger.Info("Resuming, starting from last sealed block", "head", head)
		if err := logStore.Rewind(head); err != nil {
			result = fmt.Errorf("failed to rewind chain %s to sealed block %d", chain, head)
			return false
		}
		return true
	})
	return result
}

func (db *ChainsDB) DependencySet() depset.DependencySet {
	return db.depSet
}

func (db *ChainsDB) Close() error {
	var combined error
	db.logDBs.Range(func(id eth.ChainID, logDB LogStorage) bool {
		if err := logDB.Close(); err != nil {
			combined = errors.Join(combined, fmt.Errorf("failed to close log db for chain %v: %w", id, err))
		}
		return true
	})
	return combined
}
