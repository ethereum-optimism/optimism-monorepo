package forking

import (
	"bytes"
	"maps"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
)

// AccountDiff represents changes to an account. Unchanged values of the account are not included.
type AccountDiff struct {
	// Nonce change.
	// No diff if nil.
	Nonce *uint64 `json:"nonce"`

	// Balance change.
	// No diff if nil.
	Balance *uint256.Int `json:"balance"`

	// Storage changes.
	// No diff if not present in map. Deletions are zero-value entries.
	Storage map[common.Hash]common.Hash `json:"storage"`

	// CodeHash, for lookup of contract bytecode in the code diff map.
	// No code-diff if nil.
	CodeHash *common.Hash `json:"codeHash"`
}

func (d *AccountDiff) Copy() *AccountDiff {
	var out AccountDiff
	if d.Nonce != nil {
		v := *d.Nonce // copy the value
		out.Nonce = &v
	}
	if d.Balance != nil {
		out.Balance = d.Balance.Clone()
	}
	if d.Storage != nil {
		out.Storage = maps.Clone(d.Storage)
	}
	if d.CodeHash != nil {
		h := *d.CodeHash
		out.CodeHash = &h
	}
	return &out
}

type ExportDiff struct {
	// Accounts diff. Deleted accounts are set to nil.
	// Warning: this only contains finalized state changes.
	// The state itself holds on to non-flushed changes.
	Account map[common.Address]*AccountDiff `json:"account"`

	// Stores new contract codes by code-hash
	Code map[common.Hash][]byte `json:"code"`
}

func NewExportDiff() *ExportDiff {
	return &ExportDiff{
		Account: make(map[common.Address]*AccountDiff),
		Code:    make(map[common.Hash][]byte),
	}
}

func (ed *ExportDiff) Copy() *ExportDiff {
	out := &ExportDiff{
		Account: make(map[common.Address]*AccountDiff),
		Code:    make(map[common.Hash][]byte),
	}
	for addr, acc := range ed.Account {
		out.Account[addr] = acc.Copy()
	}
	for addr, code := range ed.Code {
		out.Code[addr] = bytes.Clone(code)
	}
	return out
}

func (ed *ExportDiff) Any() bool {
	return len(ed.Code) > 0 || len(ed.Account) > 0
}

func (ed *ExportDiff) Clear() {
	ed.Account = make(map[common.Address]*AccountDiff)
	ed.Code = make(map[common.Hash][]byte)
}
