package sources

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

// Implements the RPCCheck interface for validating RPC responses
type L1RPCChecker struct {
}

func NewL1RPCChecker() *L1RPCChecker {
	return &L1RPCChecker{}
}

func (c *L1RPCChecker) ValidateWithdrawals(withdrawals *types.Withdrawals, withdrawalsRoot *common.Hash) error {
	if withdrawalsRoot != nil {
		if withdrawals == nil {
			return errors.New("expected withdrawals")
		}
		for i, w := range *withdrawals {
			if w == nil {
				return fmt.Errorf("block withdrawal %d is null", i)
			}
		}
		if computed := types.DeriveSha(*withdrawals, trie.NewStackTrie(nil)); *withdrawalsRoot != computed {
			return fmt.Errorf("failed to verify withdrawals list: computed %s but RPC said %s", computed, withdrawalsRoot)
		}
	} else {
		if withdrawals != nil {
			return fmt.Errorf("expected no withdrawals due to missing withdrawals-root, but got %d", len(*withdrawals))
		}
	}
	return nil
}
