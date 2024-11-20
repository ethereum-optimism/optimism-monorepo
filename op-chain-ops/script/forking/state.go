package forking

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/holiman/uint256"
)

type forkStateEntry struct {
	state *state.StateDB
}

func (fe *forkStateEntry) DB() *ForkDB {
	return fe.state.Database().(*ForkDB)
}

// ForkableState implements the vm.StateDB interface,
// and a few other methods as defined in the VMStateDB interface.
// This state can be forked in-place,
// swapping over operations to route to in-memory states that wrap fork sources.
type ForkableState struct {
	selected VMStateDB

	activeFork ForkID
	forks      map[ForkID]*forkStateEntry

	// persistent accounts will override any interactions
	// to be directly with the forkID that was active at the time it was made persistent,
	// rather than whatever fork is currently active.
	persistent map[common.Address]ForkID

	fallback VMStateDB

	idCounter uint64
}

var _ VMStateDB = (*ForkableState)(nil)

func NewForkableState(base VMStateDB) *ForkableState {
	return &ForkableState{
		selected:   base,
		activeFork: ForkID{},
		forks:      make(map[ForkID]*forkStateEntry),
		persistent: map[common.Address]ForkID{
			addresses.DefaultSenderAddr: ForkID{},
			addresses.VMAddr:            ForkID{},
			addresses.ConsoleAddr:       ForkID{},
		},
		fallback:  base,
		idCounter: 0,
	}
}

// ExportDiff exports a state diff. Warning: diffs are like flushed states.
// So we flush the state, making all the contents cold, losing transient storage, etc.
func (fst *ForkableState) ExportDiff(id ForkID) (*ExportDiff, error) {
	if id == (ForkID{}) {
		return nil, errors.New("default no-fork state does not have an exportable diff")
	}
	f, ok := fst.forks[id]
	if !ok {
		return nil, fmt.Errorf("unknown fork %q", id)
	}
	// Finalize the state content, so we can get an accurate diff.
	f.state.IntermediateRoot(true)
	tr := f.state.GetTrie()
	ft, ok := tr.(*ForkedAccountsTrie)
	if !ok {
		return nil, fmt.Errorf("forked state trie is unexpectedly not a ForkedAccountsTrie: %T", tr)
	}
	diff := ft.ExportDiff()
	// Now re-init the state, so we can use it again (albeit it cold).
	forkDB := &ForkDB{active: ft}
	st, err := state.New(forkDB.active.stateRoot, forkDB)
	if err != nil {
		return nil, fmt.Errorf("failed to construct fork state: %w", err)
	}
	fst.forks[id].state = st
	if fst.activeFork == id {
		fst.selected = st
	}
	return diff, nil
}

// CreateSelectFork is like vm.createSelectFork, it creates a fork, and selects it immediately.
func (fst *ForkableState) CreateSelectFork(source ForkSource) (ForkID, error) {
	id, err := fst.CreateFork(source)
	if err != nil {
		return id, err
	}
	return id, fst.SelectFork(id)
}

// CreateFork is like vm.createFork, it creates a fork, but does not select it yet.
func (fst *ForkableState) CreateFork(source ForkSource) (ForkID, error) {
	fst.idCounter += 1 // increment first, don't use ID 0
	id := ForkID(*uint256.NewInt(fst.idCounter))
	_, ok := fst.forks[id]
	if ok { // sanity check our ID counter is consistent with the tracked forks
		return id, fmt.Errorf("cannot create fork, fork %q already exists", id)
	}
	forkDB := NewForkDB(source)
	st, err := state.New(forkDB.active.stateRoot, forkDB)
	if err != nil {
		return id, fmt.Errorf("failed to construct fork state: %w", err)
	}
	fst.forks[id] = &forkStateEntry{
		state: st,
	}
	return id, nil
}

// SelectFork is like vm.selectFork, it activates the usage of a previously created fork.
func (fst *ForkableState) SelectFork(id ForkID) error {
	if id == (ForkID{}) {
		fst.selected = fst.fallback
		fst.activeFork = ForkID{}
		return nil
	}
	f, ok := fst.forks[id]
	if !ok {
		return fmt.Errorf("cannot select fork, fork %q is unknown", id)
	}
	fst.selected = f.state
	fst.activeFork = id
	return nil
}

// ResetFork resets the fork to be coupled to the given fork-source.
// Any ephemeral state changes (transient storage, warm s-loads, etc.)
// as well as any uncommitted state, as well as any previously flushed diffs, will be lost.
func (fst *ForkableState) ResetFork(id ForkID, src ForkSource) error {
	if id == (ForkID{}) {
		return errors.New("default no-fork state cannot change its ForkSource")
	}
	f, ok := fst.forks[id]
	if !ok {
		return fmt.Errorf("unknown fork %q", id)
	}
	// Now create a new state
	forkDB := NewForkDB(src)
	st, err := state.New(src.StateRoot(), forkDB)
	if err != nil {
		return fmt.Errorf("failed to construct fork state: %w", err)
	}
	f.state = st
	if fst.activeFork == id {
		fst.selected = st
	}
	return nil
}

// ActiveFork returns the ID current active fork, or active == false if no fork is active.
func (fst *ForkableState) ActiveFork() (id ForkID, active bool) {
	return fst.activeFork, fst.activeFork != (ForkID{})
}

// ForkURLOrAlias returns the URL or alias that the fork was configured with as source.
// Returns an error if no fork is active
func (fst *ForkableState) ForkURLOrAlias(id ForkID) (string, error) {
	if id == (ForkID{}) {
		return "", errors.New("default no-fork state does not have an URL or Alias")
	}
	f, ok := fst.forks[id]
	if !ok {
		return "", fmt.Errorf("unknown fork %q", id)
	}
	return f.DB().active.src.URLOrAlias(), nil
}

// SubstituteBaseState substitutes in a fallback state.
func (fst *ForkableState) SubstituteBaseState(base VMStateDB) {
	fst.fallback = base
	// If the fallback is currently selected, also updated the fallback.
	if fst.activeFork == (ForkID{}) {
		fst.selected = base
	}
}

// MakePersistent is like vm.makePersistent, it maintains this account context across all forks.
// It does not make the account of a fork persistent, it makes an account override what might be in a fork.
func (fst *ForkableState) MakePersistent(addr common.Address) {
	fst.persistent[addr] = fst.activeFork
}

// MakeExcluded excludes an account from forking. This is useful for things like scripts, which
// should always use the fallback state.
func (fst *ForkableState) MakeExcluded(addr common.Address) {
	fst.persistent[addr] = ForkID{}
}

// RevokePersistent is like vm.revokePersistent, it undoes a previous vm.makePersistent.
func (fst *ForkableState) RevokePersistent(addr common.Address) {
	delete(fst.persistent, addr)
}

// RevokeExcluded undoes MakeExcluded. It will panic if the account was marked as
// persistent in a different fork.
func (fst *ForkableState) RevokeExcluded(addr common.Address) {
	forkID, ok := fst.persistent[addr]
	if ok && forkID != (ForkID{}) {
		panic(fmt.Sprintf("cannot revoke excluded account %s since it was made persistent in fork %q", addr, forkID))
	}
	delete(fst.persistent, addr)
}

// IsPersistent is like vm.isPersistent, it checks if an account persists across forks.
func (fst *ForkableState) IsPersistent(addr common.Address) bool {
	_, ok := fst.persistent[addr]
	return ok
}

func (fst *ForkableState) stateFor(addr common.Address) VMStateDB {
	// if forked, check if we persisted this account across forks
	persistedForkID, ok := fst.persistent[addr]
	if ok {
		if persistedForkID == (ForkID{}) {
			return fst.fallback
		}
		return fst.forks[persistedForkID].state
	}
	// This may be the fallback state, if no fork is active.
	return fst.selected
}

// Finalise finalises the state by removing the destructed objects and clears
// the journal as well as the refunds. Finalise, however, will not push any updates
// into the tries just yet.
//
// The changes will be flushed to the underlying DB.
// A *ForkDB if the state is currently forked.
func (fst *ForkableState) Finalise(deleteEmptyObjects bool) {
	fst.selected.Finalise(deleteEmptyObjects)
}

func (fst *ForkableState) CreateAccount(address common.Address) {
	fst.stateFor(address).CreateAccount(address)
}

func (fst *ForkableState) CreateContract(address common.Address) {
	fst.stateFor(address).CreateContract(address)
}

func (fst *ForkableState) SubBalance(address common.Address, u *uint256.Int, reason tracing.BalanceChangeReason) {
	fst.stateFor(address).SubBalance(address, u, reason)
}

func (fst *ForkableState) AddBalance(address common.Address, u *uint256.Int, reason tracing.BalanceChangeReason) {
	fst.stateFor(address).AddBalance(address, u, reason)
}

func (fst *ForkableState) GetBalance(address common.Address) *uint256.Int {
	return fst.stateFor(address).GetBalance(address)
}

func (fst *ForkableState) GetNonce(address common.Address) uint64 {
	return fst.stateFor(address).GetNonce(address)
}

func (fst *ForkableState) SetNonce(address common.Address, u uint64) {
	fst.stateFor(address).SetNonce(address, u)
}

func (fst *ForkableState) GetCodeHash(address common.Address) common.Hash {
	return fst.stateFor(address).GetCodeHash(address)
}

func (fst *ForkableState) GetCode(address common.Address) []byte {
	return fst.stateFor(address).GetCode(address)
}

func (fst *ForkableState) SetCode(address common.Address, bytes []byte) {
	fst.stateFor(address).SetCode(address, bytes)
}

func (fst *ForkableState) GetCodeSize(address common.Address) int {
	return fst.stateFor(address).GetCodeSize(address)
}

func (fst *ForkableState) AddRefund(u uint64) {
	fst.selected.AddRefund(u)
}

func (fst *ForkableState) SubRefund(u uint64) {
	fst.selected.SubRefund(u)
}

func (fst *ForkableState) GetRefund() uint64 {
	return fst.selected.GetRefund()
}

func (fst *ForkableState) GetCommittedState(address common.Address, hash common.Hash) common.Hash {
	return fst.stateFor(address).GetCommittedState(address, hash)
}

func (fst *ForkableState) GetState(address common.Address, k common.Hash) common.Hash {
	return fst.stateFor(address).GetState(address, k)
}

func (fst *ForkableState) SetState(address common.Address, k common.Hash, v common.Hash) {
	fst.stateFor(address).SetState(address, k, v)
}

func (fst *ForkableState) GetStorageRoot(addr common.Address) common.Hash {
	return fst.stateFor(addr).GetStorageRoot(addr)
}

func (fst *ForkableState) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return fst.stateFor(addr).GetTransientState(addr, key)
}

func (fst *ForkableState) SetTransientState(addr common.Address, key, value common.Hash) {
	fst.stateFor(addr).SetTransientState(addr, key, value)
}

func (fst *ForkableState) SelfDestruct(address common.Address) {
	fst.stateFor(address).SelfDestruct(address)
}

func (fst *ForkableState) HasSelfDestructed(address common.Address) bool {
	return fst.stateFor(address).HasSelfDestructed(address)
}

func (fst *ForkableState) Selfdestruct6780(address common.Address) {
	fst.stateFor(address).Selfdestruct6780(address)
}

func (fst *ForkableState) Exist(address common.Address) bool {
	return fst.stateFor(address).Exist(address)
}

func (fst *ForkableState) Empty(address common.Address) bool {
	return fst.stateFor(address).Empty(address)
}

func (fst *ForkableState) AddressInAccessList(addr common.Address) bool {
	return fst.stateFor(addr).AddressInAccessList(addr)
}

func (fst *ForkableState) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return fst.stateFor(addr).SlotInAccessList(addr, slot)
}

func (fst *ForkableState) AddAddressToAccessList(addr common.Address) {
	fst.stateFor(addr).AddAddressToAccessList(addr)
}

func (fst *ForkableState) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	fst.stateFor(addr).AddSlotToAccessList(addr, slot)
}

func (fst *ForkableState) PointCache() *utils.PointCache {
	return fst.selected.PointCache()
}

func (fst *ForkableState) Prepare(rules params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	fst.selected.Prepare(rules, sender, coinbase, dest, precompiles, txAccesses)
}

func (fst *ForkableState) RevertToSnapshot(i int) {
	fst.selected.RevertToSnapshot(i)
}

func (fst *ForkableState) Snapshot() int {
	return fst.selected.Snapshot()
}

func (fst *ForkableState) AddLog(log *types.Log) {
	fst.selected.AddLog(log)
}

func (fst *ForkableState) AddPreimage(hash common.Hash, img []byte) {
	fst.selected.AddPreimage(hash, img)
}

func (fst *ForkableState) Witness() *stateless.Witness {
	return fst.selected.Witness()
}

func (fst *ForkableState) SetBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) {
	fst.stateFor(addr).SetBalance(addr, amount, reason)
}
