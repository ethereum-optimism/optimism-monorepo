package sources

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Implements the RPCCheck interface for validating RPC responses
type L2RPCChecker struct {
}

func NewL2RPCChecker() *L2RPCChecker {
	return &L2RPCChecker{}
}

func (c *L2RPCChecker) ValidateWithdrawals(withdrawals *types.Withdrawals, withdrawalsRoot *common.Hash) error {
	if withdrawalsRoot != nil {
		if !(withdrawals != nil && len(*withdrawals) == 0) {
			return fmt.Errorf("expected empty withdrawals, but got %d", len(*withdrawals))
		}
	}
	return nil
}
