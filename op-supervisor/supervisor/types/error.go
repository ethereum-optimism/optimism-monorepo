package types

import "errors"

var (
	// ErrOutOfOrder happens when you try to add data to the DB,
	// but it does not actually fit onto the latest data (by being too old or new).
	ErrOutOfOrder = errors.New("data out of order")
	// ErrDataCorruption happens when the underlying DB has some I/O issue
	ErrDataCorruption = errors.New("data corruption")
	// ErrSkipped happens when we try to retrieve data that is not available (pruned)
	// It may also happen if we erroneously skip data, that was not considered a conflict, if the DB is corrupted.
	ErrSkipped = errors.New("skipped data")
	// ErrFuture happens when data is just not yet available
	ErrFuture = errors.New("future data")
	// ErrConflict happens when we know for sure that there is different canonical data
	ErrConflict = errors.New("conflicting data")
	// ErrStop can be used in iterators to indicate iteration has to stop
	ErrStop = errors.New("iter stop")
	// ErrOutOfScope is when data is accessed, but access is not allowed, because of a limited scope.
	// E.g. when limiting scope to L2 blocks derived from a specific subset of the L1 chain.
	ErrOutOfScope = errors.New("out of scope")
	// ErrPreviousToFirst is when you try to get the previous block of the first block
	// E.g. when calling PreviousDerivedFrom on the first L1 block in the DB.
	ErrPreviousToFirst = errors.New("cannot get parent of first block in the database")
	// ErrUnknownChain is when a chain is unknown, not in the dependency set.
	ErrUnknownChain = errors.New("unknown chain")
	// ErrNoRPCSource happens when a sub-service needs an RPC data source, but is not configured with one.
	ErrNoRPCSource = errors.New("no RPC client configured")
)
