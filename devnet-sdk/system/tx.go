package system

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

// TxOpts is a struct that holds all transaction options
type TxOpts struct {
	from        common.Address
	to          *common.Address
	value       *big.Int
	data        []byte
	gasLimit    uint64 // Optional: if 0, will be estimated
	accessList  types.AccessList
	blobHashes  []common.Hash
	blobs       []kzg4844.Blob
	commitments []kzg4844.Commitment
	proofs      []kzg4844.Proof
}

var _ TransactionData = (*TxOpts)(nil)

func (opts *TxOpts) From() common.Address {
	return opts.from
}

func (opts *TxOpts) To() *common.Address {
	return opts.to
}

func (opts *TxOpts) Value() *big.Int {
	return opts.value
}

func (opts *TxOpts) Data() []byte {
	return opts.data
}

// Validate checks that all required fields are set and consistent
func (opts *TxOpts) Validate() error {
	// Check mandatory fields
	if opts.from == (common.Address{}) {
		return fmt.Errorf("from address is required")
	}
	if opts.to == nil {
		return fmt.Errorf("to address is required")
	}
	if opts.value == nil || opts.value.Sign() < 0 {
		return fmt.Errorf("value must be non-negative")
	}

	// Check blob-related fields consistency
	hasBlobs := len(opts.blobs) > 0
	hasCommitments := len(opts.commitments) > 0
	hasProofs := len(opts.proofs) > 0
	hasBlobHashes := len(opts.blobHashes) > 0

	// If any blob-related field is set, all must be set
	if hasBlobs || hasCommitments || hasProofs || hasBlobHashes {
		if !hasBlobs {
			return fmt.Errorf("blobs are required when other blob fields are set")
		}
		if !hasCommitments {
			return fmt.Errorf("commitments are required when other blob fields are set")
		}
		if !hasProofs {
			return fmt.Errorf("proofs are required when other blob fields are set")
		}
		if !hasBlobHashes {
			return fmt.Errorf("blob hashes are required when other blob fields are set")
		}

		// Check that all blob-related fields have the same length
		blobCount := len(opts.blobs)
		if len(opts.commitments) != blobCount {
			return fmt.Errorf("number of commitments (%d) does not match number of blobs (%d)", len(opts.commitments), blobCount)
		}
		if len(opts.proofs) != blobCount {
			return fmt.Errorf("number of proofs (%d) does not match number of blobs (%d)", len(opts.proofs), blobCount)
		}
		if len(opts.blobHashes) != blobCount {
			return fmt.Errorf("number of blob hashes (%d) does not match number of blobs (%d)", len(opts.blobHashes), blobCount)
		}
	}

	return nil
}

// TxOption is a function that configures TxOpts
type TxOption func(*TxOpts)

// WithFrom sets the sender address
func WithFrom(from common.Address) TxOption {
	return func(opts *TxOpts) {
		opts.from = from
	}
}

// WithTo sets the recipient address
func WithTo(to common.Address) TxOption {
	return func(opts *TxOpts) {
		opts.to = &to
	}
}

// WithValue sets the transaction value
func WithValue(value *big.Int) TxOption {
	return func(opts *TxOpts) {
		opts.value = value
	}
}

// WithData sets the transaction data
func WithData(data []byte) TxOption {
	return func(opts *TxOpts) {
		opts.data = data
	}
}

// WithGasLimit sets an explicit gas limit
func WithGasLimit(gasLimit uint64) TxOption {
	return func(opts *TxOpts) {
		opts.gasLimit = gasLimit
	}
}

// WithAccessList sets the access list for EIP-2930 transactions
func WithAccessList(accessList types.AccessList) TxOption {
	return func(opts *TxOpts) {
		opts.accessList = accessList
	}
}

// WithBlobs sets the blob transaction fields
func WithBlobs(blobs []kzg4844.Blob) TxOption {
	return func(opts *TxOpts) {
		opts.blobs = blobs
	}
}

// WithBlobCommitments sets the blob commitments
func WithBlobCommitments(commitments []kzg4844.Commitment) TxOption {
	return func(opts *TxOpts) {
		opts.commitments = commitments
	}
}

// WithBlobProofs sets the blob proofs
func WithBlobProofs(proofs []kzg4844.Proof) TxOption {
	return func(opts *TxOpts) {
		opts.proofs = proofs
	}
}

// WithBlobHashes sets the blob hashes
func WithBlobHashes(hashes []common.Hash) TxOption {
	return func(opts *TxOpts) {
		opts.blobHashes = hashes
	}
}

// EthTx is the default implementation of Transaction that wraps types.Transaction
type EthTx struct {
	tx     *types.Transaction
	from   common.Address
	txType uint8
}

func (t *EthTx) Hash() common.Hash {
	return t.tx.Hash()
}

func (t *EthTx) From() common.Address {
	return t.from
}

func (t *EthTx) To() *common.Address {
	return t.tx.To()
}

func (t *EthTx) Value() *big.Int {
	return t.tx.Value()
}

func (t *EthTx) Data() []byte {
	return t.tx.Data()
}

func (t *EthTx) Type() uint8 {
	return t.txType
}

func (t *EthTx) Raw() *types.Transaction {
	return t.tx
}
