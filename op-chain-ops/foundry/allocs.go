package foundry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"os"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

type ForgeAllocs struct {
	Accounts types.GenesisAlloc
}

// FromState takes a geth StateDB, and dumps the accounts into the ForgeAllocs.
// Any previous allocs contents are removed.
// Warning: the state must be committed first, trie-key preimages must be present for iteration,
// and a fresh state around the committed state-root must be presented, for the latest state-contents to be dumped.
func (f *ForgeAllocs) FromState(stateDB StateDB) {
	f.Accounts = make(types.GenesisAlloc)
	stateDB.DumpToCollector((*forgeAllocsDump)(f), &state.DumpConfig{
		OnlyWithAddresses: true,
	})
}

// StateDB is a minimal interface to support dumping of Geth EVM state to ForgeAllocs.
type StateDB interface {
	DumpToCollector(c state.DumpCollector, conf *state.DumpConfig) (nextKey []byte)
}

// Assert that the Geth StateDB implements this interface still.
var _ StateDB = (*state.StateDB)(nil)

// forgeAllocsDump is a wrapper to hide the error-prone state-dumping interface from public API.
// Use ForgeAllocs.FromState to dump a state to forge-allocs.
type forgeAllocsDump ForgeAllocs

// ForgeAllocs implements state.DumpAllocator, such that the EVM state can be dumped into it:
// with a StateDB.DumpToCollector call.
var _ state.DumpCollector = (*forgeAllocsDump)(nil)

func (d *forgeAllocsDump) OnRoot(hash common.Hash) {
	// Unlike the geth raw-state-dump, forge-allocs do not reference the state trie root.
}

func (d *forgeAllocsDump) OnAccount(address *common.Address, account state.DumpAccount) {
	if address == nil {
		return
	}
	if _, ok := d.Accounts[*address]; ok {
		panic(fmt.Errorf("cannot dump account %s twice", *address))
	}
	balance, ok := new(big.Int).SetString(account.Balance, 0)
	if !ok {
		panic("invalid balance")
	}
	var storage map[common.Hash]common.Hash
	if len(account.Storage) > 0 {
		storage = make(map[common.Hash]common.Hash, len(account.Storage))
		for k, v := range account.Storage {
			storage[k] = common.HexToHash(v)
		}
	}
	d.Accounts[*address] = types.Account{
		Code:    account.Code,
		Storage: storage,
		Balance: balance,
		Nonce:   account.Nonce,
	}
}

// Copy returns a deep copy of the ForgeAllocs. We can't
// perform a shallow copy here because some tests modify
// the accounts individually.
func (d *ForgeAllocs) Copy() *ForgeAllocs {
	out := make(types.GenesisAlloc, len(d.Accounts))
	for k, v := range d.Accounts {
		out[k] = types.Account{
			Code:    bytes.Clone(v.Code),
			Storage: maps.Clone(v.Storage),
			Balance: new(big.Int).Set(v.Balance),
			Nonce:   v.Nonce,
		}
	}
	return &ForgeAllocs{Accounts: out}
}

func (d ForgeAllocs) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Accounts)
}

func (d *ForgeAllocs) UnmarshalJSON(b []byte) error {
	// forge, since integrating Alloy, likes to hex-encode everything.
	type forgeAllocAccount struct {
		Balance hexutil.U256                `json:"balance"`
		Nonce   hexutil.Uint64              `json:"nonce"`
		Code    hexutil.Bytes               `json:"code,omitempty"`
		Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
	}
	var allocs map[common.Address]forgeAllocAccount
	if err := json.Unmarshal(b, &allocs); err != nil {
		return err
	}
	d.Accounts = make(types.GenesisAlloc, len(allocs))
	for addr, acc := range allocs {
		acc := acc
		d.Accounts[addr] = types.Account{
			Code:       acc.Code,
			Storage:    acc.Storage,
			Balance:    (*uint256.Int)(&acc.Balance).ToBig(),
			Nonce:      (uint64)(acc.Nonce),
			PrivateKey: nil,
		}
	}
	return nil
}

func LoadForgeAllocs(allocsPath string) (*ForgeAllocs, error) {
	f, err := os.OpenFile(allocsPath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open forge allocs %q: %w", allocsPath, err)
	}
	defer f.Close()
	var out ForgeAllocs
	if err := json.NewDecoder(f).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to json-decode forge allocs %q: %w", allocsPath, err)
	}
	return &out, nil
}
