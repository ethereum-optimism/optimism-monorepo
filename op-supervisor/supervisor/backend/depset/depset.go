package depset

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type DependencySetSource interface {
	LoadDependencySet(ctx context.Context) (DependencySet, error)
}

// DependencySet is an initialized dependency set, ready to answer queries
// of what is and what is not part of the dependency set.
type DependencySet interface {

	// CanExecuteAt determines if an executing message is valid at all.
	// I.e. if the chain may be executing messages at the given timestamp.
	// This may return an error if the query temporarily cannot be answered.
	// E.g. if the DependencySet is syncing new changes.
	CanExecuteAt(chainID types.ChainID, execTimestamp uint64) (bool, error)

	// CanInitiateAt determines if an initiating message is valid to pull in.
	// I.e. if the message of the given chain is readable or not.
	// This may return an error if the query temporarily cannot be answered.
	// E.g. if the DependencySet is syncing new changes.
	CanInitiateAt(chainID types.ChainID, initTimestamp uint64) (bool, error)

	// Chains returns the list of chains that are part of the dependency set.
	Chains() []types.ChainID

	// HasChain determines if a chain is being tracked for interop purposes.
	// See CanExecuteAt and CanInitiateAt to check if a chain may message at a given time.
	HasChain(chainID types.ChainID) bool

	// ChainIndexFromID converts a ChainID to a ChainIndex.
	ChainIndexFromID(id types.ChainID) (types.ChainIndex, error)

	// ChainIDFromIndex converts a ChainIndex to a ChainID.
	ChainIDFromIndex(index types.ChainIndex) (types.ChainID, error)
}
