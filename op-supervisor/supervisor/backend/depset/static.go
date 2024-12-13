package depset

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type StaticConfigDependency struct {
	// ChainIndex is the unique short identifier of this chain.
	ChainIndex types.ChainIndex `json:"chainIndex"`

	// ActivationTime is when the chain becomes part of the dependency set.
	// This is the minimum timestamp of the inclusion of an executing message.
	ActivationTime uint64 `json:"activationTime"`

	// HistoryMinTime is what the lower bound of data is to store.
	// This is the minimum timestamp of an initiating message to be accessible to others.
	// This is set to 0 when all data since genesis is executable.
	HistoryMinTime uint64 `json:"historyMinTime"`
}

// StaticConfigDependencySet statically declares a DependencySet.
// It can be used as a DependencySetSource itself, by simply returning the itself when loading the set.
type StaticConfigDependencySet struct {
	// dependency info per chain
	dependencies map[types.ChainID]*StaticConfigDependency
	// cached mapping of chain index to chain ID
	indexToID map[types.ChainIndex]types.ChainID
	// cached list of chain IDs, sorted by ID value
	chainIDs []types.ChainID
}

func NewStaticConfigDependencySet(dependencies map[types.ChainID]*StaticConfigDependency) (*StaticConfigDependencySet, error) {
	out := &StaticConfigDependencySet{dependencies: dependencies}
	if err := out.hydrate(); err != nil {
		return nil, err
	}
	return out, nil
}

// jsonStaticConfigDependencySet is a util for JSON encoding/decoding,
// to encode/decode just the attributes that matter,
// while wrapping the decoding functionality with additional hydration step.
type jsonStaticConfigDependencySet struct {
	Dependencies map[types.ChainID]*StaticConfigDependency `json:"dependencies"`
}

func (ds *StaticConfigDependencySet) MarshalJSON() ([]byte, error) {
	out := &jsonStaticConfigDependencySet{
		Dependencies: ds.dependencies,
	}
	return json.Marshal(out)
}

func (ds *StaticConfigDependencySet) UnmarshalJSON(data []byte) error {
	var v jsonStaticConfigDependencySet
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	ds.dependencies = v.Dependencies
	return ds.hydrate()
}

// hydrate sets all the cached values, based on the dependencies attribute
func (ds *StaticConfigDependencySet) hydrate() error {
	ds.indexToID = make(map[types.ChainIndex]types.ChainID)
	ds.chainIDs = make([]types.ChainID, 0, len(ds.dependencies))
	for id, dep := range ds.dependencies {
		if existing, ok := ds.indexToID[dep.ChainIndex]; ok {
			return fmt.Errorf("chain %s cannot have the same index (%d) as chain %s", id, dep.ChainIndex, existing)
		}
		ds.indexToID[dep.ChainIndex] = id
		ds.chainIDs = append(ds.chainIDs, id)
	}
	sort.Slice(ds.chainIDs, func(i, j int) bool {
		return ds.chainIDs[i].Cmp(ds.chainIDs[j]) < 0
	})
	return nil
}

var _ DependencySetSource = (*StaticConfigDependencySet)(nil)

var _ DependencySet = (*StaticConfigDependencySet)(nil)

func (ds *StaticConfigDependencySet) LoadDependencySet(ctx context.Context) (DependencySet, error) {
	return ds, nil
}

func (ds *StaticConfigDependencySet) CanExecuteAt(chainID types.ChainID, execTimestamp uint64) (bool, error) {
	dep, ok := ds.dependencies[chainID]
	if !ok {
		return false, nil
	}
	return execTimestamp >= dep.ActivationTime, nil
}

func (ds *StaticConfigDependencySet) CanInitiateAt(chainID types.ChainID, initTimestamp uint64) (bool, error) {
	dep, ok := ds.dependencies[chainID]
	if !ok {
		return false, nil
	}
	return initTimestamp >= dep.HistoryMinTime, nil
}

func (ds *StaticConfigDependencySet) Chains() []types.ChainID {
	return slices.Clone(ds.chainIDs)
}

func (ds *StaticConfigDependencySet) HasChain(chainID types.ChainID) bool {
	_, ok := ds.dependencies[chainID]
	return ok
}

func (ds *StaticConfigDependencySet) ChainIndexFromID(id types.ChainID) (types.ChainIndex, error) {
	dep, ok := ds.dependencies[id]
	if !ok {
		return 0, types.ErrUnknownChain
	}
	return dep.ChainIndex, nil
}

func (ds *StaticConfigDependencySet) ChainIDFromIndex(index types.ChainIndex) (types.ChainID, error) {
	id, ok := ds.indexToID[index]
	if !ok {
		return types.ChainID{}, types.ErrUnknownChain
	}
	return id, nil
}
