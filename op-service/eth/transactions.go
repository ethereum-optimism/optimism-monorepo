package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
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

// CheckRecentTxs checks the depth of recent blocks for txs from the account with address addr
// and returns either:
//   - blockNum containing the last tx and true if any were found
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
