package forking

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/triedb/pathdb"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/ethereum/go-ethereum/triedb"
)

// ForkDB is a virtual state database: it wraps a forked accounts trie,
// and can maintain a state diff, so we can mutate the forked state,
// and even finalize state changes (so we can accurately measure things like cold storage gas cost).
type ForkDB struct {
	active *ForkedAccountsTrie
}

// Reader for read-only access to a known state. All cold reads go through this.
// So the state-DB creates one initially, and then holds on to it.
// The diff will be overlayed on the reader still. To get rid of the diff, it has to be explicitly cleared.
// Warning: diffs applied to the original state that the reader wraps will be visible.
// Geth StateDB is meant to be reinitialized after committing state.
func (f *ForkDB) Reader(root common.Hash) (state.Reader, error) {
	if root != f.active.stateRoot {
		return nil, fmt.Errorf("current state is at %s, cannot open state at %s", f.active.stateRoot, root)
	}
	return &forkStateReader{
		f.active,
	}, nil
}

func (f *ForkDB) Snapshot() *snapshot.Tree {
	return nil
}

var _ state.Database = (*ForkDB)(nil)

func NewForkDB(source ForkSource) *ForkDB {
	return &ForkDB{active: &ForkedAccountsTrie{
		stateRoot: source.StateRoot(),
		src:       source,
		diff:      NewExportDiff(),
	}}
}

// fakeRoot is just a marker; every account we load into the fork-db has this storage-root.
// When opening a storage-trie, we sanity-check we have this root, or an empty trie.
// And then just return the same global trie view for storage reads/writes.
// It needs to be set to EmptyRootHash to avoid contract collision errors when
// deploying contracts, since Geth checks the storage root prior to deployment.
var fakeRoot = types.EmptyRootHash

func (f *ForkDB) OpenTrie(root common.Hash) (state.Trie, error) {
	if f.active.stateRoot != root {
		return nil, fmt.Errorf("active fork is at %s, but tried to open %s", f.active.stateRoot, root)
	}
	return f.active, nil
}

func (f *ForkDB) OpenStorageTrie(stateRoot common.Hash, address common.Address, root common.Hash, trie state.Trie) (state.Trie, error) {
	if f.active.stateRoot != stateRoot {
		return nil, fmt.Errorf("active fork is at %s, but tried to open account %s of state %s", f.active.stateRoot, address, stateRoot)
	}
	if _, ok := trie.(*ForkedAccountsTrie); !ok {
		return nil, fmt.Errorf("ForkDB tried to open non-fork storage-trie %v", trie)
	}
	if root != fakeRoot && root != types.EmptyRootHash {
		return nil, fmt.Errorf("ForkDB unexpectedly was queried with real looking storage root: %s", root)
	}
	return f.active, nil
}

func (f *ForkDB) CopyTrie(trie state.Trie) state.Trie {
	if st, ok := trie.(*ForkedAccountsTrie); ok {
		return st.Copy()
	}
	panic(fmt.Errorf("ForkDB tried to copy non-fork trie %v", trie))
}

func (f *ForkDB) ContractCode(addr common.Address, codeHash common.Hash) ([]byte, error) {
	return f.active.ContractCode(addr, codeHash)
}

func (f *ForkDB) ContractCodeSize(addr common.Address, codeHash common.Hash) (int, error) {
	return f.active.ContractCodeSize(addr, codeHash)
}

func (f *ForkDB) DiskDB() ethdb.KeyValueStore {
	panic("DiskDB() during active Fork is not supported")
}

func (f *ForkDB) PointCache() *utils.PointCache {
	panic("PointCache() is not supported")
}

func (f *ForkDB) TrieDB() *triedb.Database {
	// The TrieDB is unused, but geth does use to check if Verkle is activated.
	// So we have to create a read-only dummy one, to communicate that verkle really is disabled.
	diskDB := rawdb.NewMemoryDatabase()
	tdb := triedb.NewDatabase(diskDB, &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB:    nil,
		PathDB: &pathdb.Config{
			StateHistory:   0,
			CleanCacheSize: 0,
			DirtyCacheSize: 0,
			ReadOnly:       true,
		},
	})
	return tdb
}
