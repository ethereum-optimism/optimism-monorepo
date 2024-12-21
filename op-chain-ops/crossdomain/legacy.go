package crossdomain

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
)

type LegacyReceipt struct {
	// Consensus fields: These fields are defined by the Yellow Paper
	PostState         []byte       `json:"root"`
	Status            uint64       `json:"status"`
	CumulativeGasUsed uint64       `json:"cumulativeGasUsed" gencodec:"required"`
	Bloom             types.Bloom  `json:"logsBloom"         gencodec:"required"`
	Logs              []*types.Log `json:"logs"              gencodec:"required"`

	// Implementation fields: These fields are added by geth when processing a transaction.
	// They are stored in the chain database.
	TxHash          common.Hash    `json:"transactionHash" gencodec:"required"`
	ContractAddress common.Address `json:"contractAddress"`
	GasUsed         uint64         `json:"gasUsed" gencodec:"required"`

	// Inclusion information: These fields provide information about the inclusion of the
	// transaction corresponding to this receipt.
	BlockHash        common.Hash `json:"blockHash,omitempty"`
	BlockNumber      *big.Int    `json:"blockNumber,omitempty"`
	TransactionIndex uint        `json:"transactionIndex"`

	// UsingOVM
	L1GasPrice *big.Int   `json:"l1GasPrice" gencodec:"required"`
	L1GasUsed  *big.Int   `json:"l1GasUsed" gencodec:"required"`
	L1Fee      *big.Int   `json:"l1Fee" gencodec:"required"`
	FeeScalar  *big.Float `json:"l1FeeScalar" gencodec:"required"`
}

// DecodeRLP implements rlp.Decoder, and loads both consensus and implementation
// fields of a receipt from an RLP stream.
func (r *LegacyReceipt) DecodeRLP(s *rlp.Stream) error {
	// Retrieve the entire receipt blob as we need to try multiple decoders
	blob, err := s.Raw()
	if err != nil {
		return err
	}
	// Try decoding from the newest format for future proofness, then the older one
	// for old nodes that just upgraded. V4 was an intermediate unreleased format so
	// we do need to decode it, but it's not common (try last).
	if err := decodeStoredReceiptRLP(r, blob); err == nil {
		return nil
	}

	return errors.New("invalid receipt")
}

type storedReceiptRLP struct {
	PostStateOrStatus []byte
	CumulativeGasUsed uint64
	Logs              []*types.LogForStorage
	// UsingOVM
	L1GasUsed  *big.Int
	L1GasPrice *big.Int
	L1Fee      *big.Int
	FeeScalar  string
}

func decodeStoredReceiptRLP(r *LegacyReceipt, blob []byte) error {
	var stored storedReceiptRLP
	if err := rlp.DecodeBytes(blob, &stored); err != nil {
		return err
	}
	r.Logs = make([]*types.Log, len(stored.Logs))
	for i, log := range stored.Logs {
		r.Logs[i] = (*types.Log)(log)
	}
	return nil
}

func ReadLegacyReceipts(db ethdb.Reader, hash common.Hash, number uint64) ([]*LegacyReceipt, error) {
	// Retrieve the flattened receipt slice
	data := rawdb.ReadReceiptsRLP(db, hash, number)
	if len(data) == 0 {
		return nil, nil
	}
	// Convert the receipts from their storage form to their internal representation
	storageReceipts := []*LegacyReceipt{}
	if err := rlp.DecodeBytes(data, &storageReceipts); err != nil {
		return nil, fmt.Errorf("error decoding legacy receipts: %w", err)
	}
	return storageReceipts, nil
}
