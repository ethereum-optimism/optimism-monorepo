package eth

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type L1Client interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
}

// EncodeTransactions encodes a list of transactions into opaque transactions.
func EncodeTransactions(elems []*types.Transaction) ([]hexutil.Bytes, error) {
	out := make([]hexutil.Bytes, len(elems))
	for i, el := range elems {
		dat, err := el.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tx %d: %w", i, err)
		}
		out[i] = dat
	}
	return out, nil
}

// DecodeTransactions decodes a list of opaque transactions into transactions.
func DecodeTransactions(data []hexutil.Bytes) ([]*types.Transaction, error) {
	dest := make([]*types.Transaction, len(data))
	for i := range dest {
		var x types.Transaction
		if err := x.UnmarshalBinary(data[i]); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tx %d: %w", i, err)
		}
		dest[i] = &x
	}
	return dest, nil
}

// TransactionsToHashes computes the transaction-hash for every transaction in the input.
func TransactionsToHashes(elems []*types.Transaction) []common.Hash {
	out := make([]common.Hash, len(elems))
	for i, el := range elems {
		out[i] = el.Hash()
	}
	return out
}

// CheckRecentTxs checks the depth recent blocks for txs from the account with address addr
// and returns either:
//   - blockNum containing the last tx and true if any was found
//   - the oldest block checked and false if no nonce change was found
func CheckRecentTxs(
	ctx context.Context,
	l1 L1Client,
	depth int,
	addr common.Address,
) (blockNum uint64, found bool, err error) {
	blockHeader, err := l1.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve current block header: %w", err)
	}

	currentBlock := blockHeader.Number
	currentNonce, err := l1.NonceAt(ctx, addr, currentBlock)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve current nonce: %w", err)
	}

	oldestBlock := new(big.Int).Sub(currentBlock, big.NewInt(int64(depth)))
	previousNonce, err := l1.NonceAt(ctx, addr, oldestBlock)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve previous nonce: %w", err)
	}

	if currentNonce == previousNonce {
		// Most recent tx is older than the given depth
		return oldestBlock.Uint64(), false, nil
	}

	// Use binary search to find the block where the nonce changed
	low := oldestBlock.Uint64()
	high := currentBlock.Uint64()

	for low < high {
		mid := (low + high) / 2
		midNonce, err := l1.NonceAt(ctx, addr, new(big.Int).SetUint64(mid))
		if err != nil {
			return 0, false, fmt.Errorf("failed to retrieve nonce at block %d: %w", mid, err)
		}

		if midNonce > currentNonce {
			// Catch a reorg that causes inconsistent nonce
			return CheckRecentTxs(ctx, l1, depth, addr)
		} else if midNonce == currentNonce {
			high = mid
		} else {
			// midNonce < currentNonce: check the next block to see if we've found the
			// spot where the nonce transitions to the currentNonce
			nextBlockNum := mid + 1
			nextBlockNonce, err := l1.NonceAt(ctx, addr, new(big.Int).SetUint64(nextBlockNum))
			if err != nil {
				return 0, false, fmt.Errorf("failed to retrieve nonce at block %d: %w", mid, err)
			}

			if nextBlockNonce == currentNonce {
				return nextBlockNum, true, nil
			}
			low = mid + 1
		}
	}
	return oldestBlock.Uint64(), false, nil
}

type GenericTx interface {
	// Transaction tries to interpret into a typed tx.
	// This will return types.ErrTxTypeNotSupported if the transaction-type is not supported.
	Transaction() (*types.Transaction, error)

	// TxType returns the EIP-2718 type.
	TxType() uint8

	// TxHash returns the transaction hash.
	TxHash() common.Hash

	// MarshalJSON into RPC tx definition.
	// Block metadata may or may not be included.
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON into RPC tx definition.
	// Block metadata is ignored.
	UnmarshalJSON([]byte) error

	// MarshalBinary as EIP-2718 opaque tx (including version byte).
	MarshalBinary() ([]byte, error)
	// UnmarshalBinary as EIP-2718 opaque tx (including version byte).
	UnmarshalBinary(b []byte) error
}

type OpaqueTransaction struct {
	raw  []byte
	hash common.Hash
}

func (o *OpaqueTransaction) Transaction() (*types.Transaction, error) {
	panic("not implemented")
}

// TxType returns the EIP-2718 TransactionType. https://eips.ethereum.org/EIPS/eip-2718
func (o *OpaqueTransaction) TxType() uint8 {
	firstByte := o.raw[0]
	switch {
	case 0xc0 <= firstByte && firstByte <= 0xfe:
		// legacy tx
		return 0
	case firstByte <= 0x7f:
		// EIP-2718 tx
		return firstByte
	default:
		panic("invalid tx type")
	}
}

func (o *OpaqueTransaction) TxHash() common.Hash {
	if (o.hash == common.Hash{}) {
		o.hash = crypto.Keccak256Hash(o.raw[:])
	}
	return o.hash
}

func (o *OpaqueTransaction) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("cannot marshal opaque transaction to JSON")
}

func (o *OpaqueTransaction) UnmarshalJSON([]byte) error {
	return fmt.Errorf("cannot unmarshal opaque transaction from JSON")
}

func (o *OpaqueTransaction) MarshalBinary() ([]byte, error) {
	return o.raw, nil
}

func (o *OpaqueTransaction) UnmarshalBinary(b []byte) error {
	o.raw = bytes.Clone(b)
	o.TxType()
	return nil
}

// Compile-time check that OpqueTransaction implements GenericTx.
var _ GenericTx = (*OpaqueTransaction)(nil)
