package forking

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
)

type TestForkSource struct {
	urlOrAlias string
	stateRoot  common.Hash
	nonces     map[common.Address]uint64
	balances   map[common.Address]*uint256.Int
	storage    map[common.Address]map[common.Hash]common.Hash
	code       map[common.Address][]byte
}

func (t TestForkSource) URLOrAlias() string {
	return t.urlOrAlias
}

func (t TestForkSource) StateRoot() common.Hash {
	return t.stateRoot
}

func (t TestForkSource) Nonce(addr common.Address) (uint64, error) {
	return t.nonces[addr], nil
}

func (t TestForkSource) Balance(addr common.Address) (*uint256.Int, error) {
	b, ok := t.balances[addr]
	if !ok {
		return uint256.NewInt(0), nil
	}
	return b.Clone(), nil
}

func (t TestForkSource) StorageAt(addr common.Address, key common.Hash) (common.Hash, error) {
	storage, ok := t.storage[addr]
	if !ok {
		return common.Hash{}, nil
	}
	return storage[key], nil
}

func (t TestForkSource) Code(addr common.Address) ([]byte, error) {
	return t.code[addr], nil
}

var _ ForkSource = (*TestForkSource)(nil)

func TestForking(t *testing.T) {
	// create regular DB
	rawDB := rawdb.NewMemoryDatabase()
	stateDB := state.NewDatabase(triedb.NewDatabase(rawDB, &triedb.Config{
		Preimages: true, // To be able to iterate the state we need the Preimages
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
		PathDB:    nil,
	}), nil)
	baseState, err := state.New(types.EmptyRootHash, stateDB)
	if err != nil {
		panic(fmt.Errorf("failed to create memory state db: %w", err))
	}
	forkState := NewForkableState(baseState)

	// No active fork yet
	id, active := forkState.ActiveFork()
	require.False(t, active)
	require.Equal(t, ForkID{}, id)

	name, err := forkState.ForkURLOrAlias(ForkID{})
	require.ErrorContains(t, err, "default")
	require.Equal(t, "", name)

	alice := common.Address(bytes.Repeat([]byte{0xaa}, 20))
	bob := common.Address(bytes.Repeat([]byte{0xbb}, 20))

	forkState.CreateAccount(alice)
	forkState.SetNonce(alice, 3)
	forkState.AddBalance(alice, uint256.NewInt(123), tracing.BalanceChangeUnspecified)
	// Check if writes worked
	require.Equal(t, uint64(123), forkState.GetBalance(alice).Uint64())
	require.Equal(t, uint64(3), forkState.GetNonce(alice))
	// No active fork yet, balance change should be applied to underlying base-state
	require.Equal(t, uint64(123), baseState.GetBalance(alice).Uint64())
	require.Equal(t, uint64(3), baseState.GetNonce(alice))

	src1 := &TestForkSource{
		urlOrAlias: "src 1",
		stateRoot:  crypto.Keccak256Hash([]byte("test fork state 1")),
		nonces: map[common.Address]uint64{
			alice: uint64(42),
			bob:   uint64(1000),
		},
		balances: make(map[common.Address]*uint256.Int),
		storage:  make(map[common.Address]map[common.Hash]common.Hash),
		code:     make(map[common.Address][]byte),
	}
	forkA, err := forkState.CreateSelectFork(src1)
	require.NoError(t, err)
	// Check that we selected A
	id, active = forkState.ActiveFork()
	require.True(t, active)
	require.Equal(t, forkA, id)
	name, err = forkState.ForkURLOrAlias(forkA)
	require.NoError(t, err)
	require.Equal(t, "src 1", name)

	// the fork has a different nonce for alice
	require.Equal(t, uint64(42), forkState.GetNonce(alice))
	// the fork has Bob, which didn't exist thus far
	require.Equal(t, uint64(1000), forkState.GetNonce(bob))

	// Apply a diff change on top of the fork
	forkState.SetNonce(bob, 99999)

	// Now unselect the fork, going back to the default again.
	require.NoError(t, forkState.SelectFork(ForkID{}))
	// No longer active fork
	id, active = forkState.ActiveFork()
	require.False(t, active)
	require.Equal(t, ForkID{}, id)

	// Check that things are back to normal
	require.Equal(t, uint64(3), forkState.GetNonce(alice))
	require.Equal(t, uint64(0), forkState.GetNonce(bob))

	// Make a change to the base-state, to see if it survives going back to the fork.
	forkState.SetNonce(bob, 5)

	// Re-select the fork, see if the changes come back, including the diff we made
	require.NoError(t, forkState.SelectFork(forkA))
	require.Equal(t, uint64(42), forkState.GetNonce(alice))
	require.Equal(t, uint64(99999), forkState.GetNonce(bob))

	// This change will continue to be visible across forks,
	// alice is going to be persistent.
	forkState.SetNonce(alice, 777)

	// Now make Alice persistent, see if we can get the original value
	forkState.MakePersistent(alice)

	// Activate a fork, to see if alice is really persistent
	src2 := &TestForkSource{
		urlOrAlias: "src 2",
		stateRoot:  crypto.Keccak256Hash([]byte("test fork state 2")),
		nonces: map[common.Address]uint64{
			alice: uint64(2222),
			bob:   uint64(222),
		},
		balances: make(map[common.Address]*uint256.Int),
		storage:  make(map[common.Address]map[common.Hash]common.Hash),
		code:     make(map[common.Address][]byte),
	}
	tmpFork, err := forkState.CreateSelectFork(src2)
	require.NoError(t, err)
	require.Equal(t, uint64(777), forkState.GetNonce(alice), "persistent original value")
	// While bob is still read from the fork
	require.Equal(t, uint64(222), forkState.GetNonce(bob), "bob is forked")

	// Mutate both, and undo the fork, to test if the persistent change is still there in non-fork mode
	forkState.SetNonce(alice, 1001) // this mutates forkA, because alice was made persistent there
	forkState.SetNonce(bob, 1002)
	require.NoError(t, forkState.SelectFork(ForkID{}))
	require.Equal(t, uint64(1001), forkState.GetNonce(alice), "alice is persistent")
	require.Equal(t, uint64(5), forkState.GetNonce(bob), "bob is not persistent")

	// Stop alice persistence. Forks can now override it again.
	forkState.RevokePersistent(alice)
	// This foundry behavior is unspecified/undocumented.
	// Not sure if correctly doing it by dropping the previously persisted state if it comes from another fork.
	require.Equal(t, uint64(3), forkState.GetNonce(alice))
	require.Equal(t, uint64(3), baseState.GetNonce(alice))
	require.Equal(t, uint64(5), forkState.GetNonce(bob))

	// Create another fork, don't select it immediately
	src3 := &TestForkSource{
		urlOrAlias: "src 3",
		stateRoot:  crypto.Keccak256Hash([]byte("test fork state 3")),
		nonces: map[common.Address]uint64{
			alice: uint64(3333),
		},
		balances: make(map[common.Address]*uint256.Int),
		storage:  make(map[common.Address]map[common.Hash]common.Hash),
		code:     make(map[common.Address][]byte),
	}
	forkB, err := forkState.CreateFork(src3)
	require.NoError(t, err)

	id, active = forkState.ActiveFork()
	require.False(t, active)
	require.Equal(t, ForkID{}, id)

	// forkA is still bound to src 1
	name, err = forkState.ForkURLOrAlias(forkA)
	require.NoError(t, err)
	require.Equal(t, "src 1", name)
	// tmpFork is still bound to src 2
	name, err = forkState.ForkURLOrAlias(tmpFork)
	require.NoError(t, err)
	require.Equal(t, "src 2", name)
	// forkB is on src 3
	name, err = forkState.ForkURLOrAlias(forkB)
	require.NoError(t, err)
	require.Equal(t, "src 3", name)

	require.Equal(t, uint64(3), forkState.GetNonce(alice), "not forked yet")
	require.NoError(t, forkState.SelectFork(forkB))
	id, active = forkState.ActiveFork()
	require.True(t, active)
	require.Equal(t, forkB, id)

	// check if successfully forked now
	require.Equal(t, uint64(3333), forkState.GetNonce(alice), "fork B active now")
	// Bob is not in this fork. But that doesn't mean the base-state should be used.
	require.Equal(t, uint64(0), forkState.GetNonce(bob))

	// See if we can go from B straight to A
	require.NoError(t, forkState.SelectFork(forkA))
	require.Equal(t, uint64(1001), forkState.GetNonce(alice), "alice from A says hi")
	// And back to B
	require.NoError(t, forkState.SelectFork(forkB))
	require.Equal(t, uint64(3333), forkState.GetNonce(alice), "alice from B says hi")

	// And a fork on top of a fork; forks don't stack, they are their own individual contexts.
	src4 := &TestForkSource{
		urlOrAlias: "src 4",
		stateRoot:  crypto.Keccak256Hash([]byte("test fork state 4")),
		nonces: map[common.Address]uint64{
			bob: uint64(9000),
		},
		balances: make(map[common.Address]*uint256.Int),
		storage:  make(map[common.Address]map[common.Hash]common.Hash),
		code:     make(map[common.Address][]byte),
	}
	forkC, err := forkState.CreateSelectFork(src4)
	require.NoError(t, err)
	// No alice in this fork.
	require.Equal(t, uint64(0), forkState.GetNonce(alice))
	// But bob is set
	require.Equal(t, uint64(9000), forkState.GetNonce(bob))

	// Put in some mutations, for the fork-diff testing
	forkState.SetNonce(alice, 1234)
	forkState.SetBalance(alice, uint256.NewInt(100_000), tracing.BalanceChangeUnspecified)
	forkState.SetState(alice, common.Hash{4}, common.Hash{42})
	forkState.SetState(alice, common.Hash{5}, common.Hash{100})
	forkState.SetCode(alice, []byte("hello world"))

	// Check the name
	name, err = forkState.ForkURLOrAlias(forkC)
	require.NoError(t, err)
	require.Equal(t, "src 4", name)

	// Now test our fork-diff exporting:
	// it needs to reflect the changes we made to the fork, but not other fork contents.
	forkADiff, err := forkState.ExportDiff(forkA)
	require.NoError(t, err)
	require.NotNil(t, forkADiff.Account[alice])
	require.Equal(t, uint64(1001), *forkADiff.Account[alice].Nonce)
	require.Equal(t, uint64(99999), *forkADiff.Account[bob].Nonce)

	forkBDiff, err := forkState.ExportDiff(forkB)
	require.NoError(t, err)
	require.Len(t, forkBDiff.Account, 0, "no changes to fork B")

	forkCDiff, err := forkState.ExportDiff(forkC)
	require.NoError(t, err)
	require.Contains(t, forkCDiff.Account, alice)
	require.NotContains(t, forkCDiff.Account, bob)
	require.Equal(t, uint64(1234), *forkCDiff.Account[alice].Nonce)
	require.Equal(t, uint64(100_000), forkCDiff.Account[alice].Balance.Uint64())
	require.Equal(t, common.Hash{42}, forkCDiff.Account[alice].Storage[common.Hash{4}])
	require.Equal(t, common.Hash{100}, forkCDiff.Account[alice].Storage[common.Hash{5}])
	require.Equal(t, crypto.Keccak256Hash([]byte("hello world")), *forkCDiff.Account[alice].CodeHash)
	require.Equal(t, []byte("hello world"), forkCDiff.Code[*forkCDiff.Account[alice].CodeHash])
}
