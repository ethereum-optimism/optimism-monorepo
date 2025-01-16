package sources

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type RawJsonTransaction struct {
	txHash common.Hash
	txType uint8
	raw    json.RawMessage
}

var _ eth.GenericTx = (*RawJsonTransaction)(nil)

// Transaction tries to interpret into a typed tx.
// This will return types.ErrTxTypeNotSupported if the transaction-type is not supported.
func (m *RawJsonTransaction) Transaction() (*types.Transaction, error) {
	switch m.txType {
	case types.LegacyTxType, types.AccessListTxType,
		types.DynamicFeeTxType, types.BlobTxType,
		types.DepositTxType:
		var tx types.Transaction
		// Note: this json unmarshal may still return ErrTxTypeNotSupported
		// if the linked-in geth library doesn't support the expected tx types.
		if err := json.Unmarshal(m.raw, &tx); err != nil {
			return nil, err
		}
		return &tx, nil
	default:
		return nil, types.ErrTxTypeNotSupported
	}
}

func (m *RawJsonTransaction) TxType() uint8 {
	return m.txType
}

func (m *RawJsonTransaction) TxHash() common.Hash {
	return m.txHash
}

func (m *RawJsonTransaction) UnmarshalJSON(data []byte) error {
	var x struct {
		Hash common.Hash    `json:"hash"`
		Type hexutil.Uint64 `json:"type"`
	}
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x.Hash == (common.Hash{}) {
		return errors.New("expected hash attribute")
	}
	if x.Type >= 0xff || (x.Type < 0xc0 && x.Type > 0x7f) {
		return fmt.Errorf("cannot decode into an EIP-2718 transaction: TransactionType: %d", uint64(x.Type))
	}
	m.raw = bytes.Clone(data)
	m.txHash = x.Hash
	m.txType = uint8(x.Type)
	return nil
}

func (m *RawJsonTransaction) MarshalJSON() ([]byte, error) {
	return m.raw, nil
}

func (m *RawJsonTransaction) MarshalBinary() ([]byte, error) {
	tx, err := m.Transaction()
	if err != nil {
		return nil, err
	}
	return tx.MarshalBinary()
}

func (m *RawJsonTransaction) UnmarshalBinary(b []byte) error {
	var tx types.Transaction
	if err := tx.UnmarshalBinary(b); err != nil {
		return err
	}
	data, err := json.Marshal(&tx)
	if err != nil {
		return err
	}
	m.raw = data
	m.txType = tx.Type()
	m.txHash = tx.Hash()
	return nil
}
