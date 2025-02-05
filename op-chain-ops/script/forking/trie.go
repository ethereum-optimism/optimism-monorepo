package forking

import (
	"errors"
	"fmt"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
)

type ForkedAccountsTrie struct {
	// stateRoot that this diff is based on top of
	stateRoot common.Hash

	// source to retrieve data from when it's not in the diff
	src ForkSource

	diff *ExportDiff
}

var _ state.Trie = (*ForkedAccountsTrie)(nil)

func (f *ForkedAccountsTrie) Copy() *ForkedAccountsTrie {
	return &ForkedAccountsTrie{
		stateRoot: f.stateRoot,
		diff:      f.diff.Copy(),
	}
}

func (f *ForkedAccountsTrie) ExportDiff() *ExportDiff {
	return f.diff.Copy()
}

func (f *ForkedAccountsTrie) HasDiff() bool {
	return len(f.diff.Code) > 0 || len(f.diff.Account) > 0
}

// ClearDiff clears the flushed changes. This does not clear the warm state changes.
// To fully clear, first Finalise the forked state that uses this trie, and then clear the diff.
func (f *ForkedAccountsTrie) ClearDiff() {
	f.diff.Clear()
}

// ContractCode is not directly part of the vm.State interface,
// but is used by the ForkDB to retrieve the contract code.
func (f *ForkedAccountsTrie) ContractCode(addr common.Address, codeHash common.Hash) ([]byte, error) {
	diffAcc, ok := f.diff.Account[addr]
	if ok {
		if diffAcc.CodeHash != nil && *diffAcc.CodeHash != codeHash {
			return nil, fmt.Errorf("account code changed to %s, cannot get code %s of account %s", *diffAcc.CodeHash, codeHash, addr)
		}
		if code, ok := f.diff.Code[codeHash]; ok {
			return code, nil
		}
		// if not in codeDiff, the actual code has not changed.
	}
	code, err := f.src.Code(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve code: %w", err)
	}
	// sanity-check the retrieved code matches the expected codehash
	if h := crypto.Keccak256Hash(code); h != codeHash {
		return nil, fmt.Errorf("retrieved code of %s hashed to %s, but expected %s", addr, h, codeHash)
	}
	return code, nil
}

// ContractCodeSize is not directly part of the vm.State interface,
// but is used by the ForkDB to retrieve the contract code-size.
func (f *ForkedAccountsTrie) ContractCodeSize(addr common.Address, codeHash common.Hash) (int, error) {
	code, err := f.ContractCode(addr, codeHash)
	if err != nil {
		return 0, fmt.Errorf("cannot get contract code to determine code size: %w", err)
	}
	return len(code), nil
}

func (f *ForkedAccountsTrie) GetKey(bytes []byte) []byte {
	panic("arbitrary key lookups on ForkedAccountsTrie are not supported")
}

func (f *ForkedAccountsTrie) GetAccount(address common.Address) (*types.StateAccount, error) {
	acc := &types.StateAccount{
		Nonce:    0,
		Balance:  nil,
		Root:     fakeRoot,
		CodeHash: nil,
	}
	diffAcc := f.diff.Account[address]
	if diffAcc != nil && diffAcc.Nonce != nil {
		acc.Nonce = *diffAcc.Nonce
	} else {
		v, err := f.src.Nonce(address)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve nonce of account %s: %w", address, err)
		}
		acc.Nonce = v
	}
	if diffAcc != nil && diffAcc.Balance != nil {
		acc.Balance = new(uint256.Int).Set(diffAcc.Balance)
	} else {
		v, err := f.src.Balance(address)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve balance of account %s: %w", address, err)
		}
		acc.Balance = new(uint256.Int).Set(v)
	}
	if diffAcc != nil && diffAcc.CodeHash != nil {
		cpy := *diffAcc.CodeHash
		acc.CodeHash = cpy.Bytes()
	} else {
		v, err := f.src.Code(address)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve code of account %s: %w", address, err)
		}
		acc.CodeHash = crypto.Keccak256Hash(v).Bytes()
	}
	return acc, nil
}

func (f *ForkedAccountsTrie) GetStorage(addr common.Address, key []byte) ([]byte, error) {
	k := common.BytesToHash(key)
	diffAcc, ok := f.diff.Account[addr]
	if ok { // if there is a diff, try and see if it contains a storage diff
		v, ok := diffAcc.Storage[k]
		if ok { // if the storage has changed, return that change
			return v.Bytes(), nil
		}
	}
	v, err := f.src.StorageAt(addr, k)
	if err != nil {
		return nil, err
	}
	return v.Bytes(), nil
}

func (f *ForkedAccountsTrie) UpdateAccount(address common.Address, account *types.StateAccount, codeLen int) error {
	// Ignored, account contains the code details we need.
	// Also see the trie.StateTrie of geth itself, which ignores this arg too.
	_ = codeLen

	nonce := account.Nonce
	b := account.Balance.Clone()
	codeHash := common.BytesToHash(account.CodeHash)
	out := &AccountDiff{
		Nonce:    &nonce,
		Balance:  b,
		Storage:  nil,
		CodeHash: &codeHash,
	}
	// preserve the storage diff
	if diffAcc, ok := f.diff.Account[address]; ok {
		out.Storage = diffAcc.Storage
	}
	f.diff.Account[address] = out
	return nil
}

func (f *ForkedAccountsTrie) UpdateStorage(addr common.Address, key, value []byte) error {
	diffAcc, ok := f.diff.Account[addr]
	if !ok {
		diffAcc = &AccountDiff{}
		f.diff.Account[addr] = diffAcc
	}
	if diffAcc.Storage == nil {
		diffAcc.Storage = make(map[common.Hash]common.Hash)
	}
	k := common.BytesToHash(key)
	v := common.BytesToHash(value)
	diffAcc.Storage[k] = v
	return nil
}

func (f *ForkedAccountsTrie) DeleteAccount(address common.Address) error {
	f.diff.Account[address] = nil
	return nil
}

func (f *ForkedAccountsTrie) DeleteStorage(addr common.Address, key []byte) error {
	return f.UpdateStorage(addr, key, nil)
}

func (f *ForkedAccountsTrie) UpdateContractCode(addr common.Address, codeHash common.Hash, code []byte) error {
	diffAcc, ok := f.diff.Account[addr]
	if !ok {
		diffAcc = &AccountDiff{}
		f.diff.Account[addr] = diffAcc
	}
	diffAcc.CodeHash = &codeHash
	f.diff.Code[codeHash] = code
	return nil
}

func (f *ForkedAccountsTrie) Hash() common.Hash {
	return f.stateRoot
}

func (f *ForkedAccountsTrie) Commit(collectLeaf bool) (common.Hash, *trienode.NodeSet) {
	panic("cannot commit state-changes of a forked trie")
}

func (f *ForkedAccountsTrie) Witness() map[string]struct{} {
	panic("witness generation of a ForkedAccountsTrie is not supported")
}

func (f *ForkedAccountsTrie) NodeIterator(startKey []byte) (trie.NodeIterator, error) {
	return nil, errors.New("node iteration of a ForkedAccountsTrie is not supported")
}

func (f *ForkedAccountsTrie) Prove(key []byte, proofDb ethdb.KeyValueWriter) error {
	return errors.New("proving of a ForkedAccountsTrie is not supported")
}

func (f *ForkedAccountsTrie) IsVerkle() bool {
	return false
}
