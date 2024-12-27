package derive

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// UserDeposits transforms the L2 block-height and L1 receipts into the transaction inputs for a full L2 block
func UserDeposits(receipts []*types.Receipt, depositContractAddr common.Address, currentNonce uint64) ([]*types.DepositTx, uint64, error) {
	var out []*types.DepositTx
	var result error
	for i, rec := range receipts {
		if rec.Status != types.ReceiptStatusSuccessful {
			continue
		}
		for j, log := range rec.Logs {
			if log.Address == depositContractAddr && len(log.Topics) > 0 && log.Topics[0] == DepositEventABIHash {
				dep, newNonce, err := UnmarshalDepositLogEvent(log, currentNonce)
				currentNonce = newNonce
				if err != nil {
					result = multierror.Append(result, fmt.Errorf("malformatted L1 deposit log in receipt %d, log %d: %w", i, j, err))
				} else {
					out = append(out, dep)
				}
			}
		}
	}
	return out, currentNonce, result
}

func DeriveDeposits(receipts []*types.Receipt, depositContractAddr common.Address, currentNonce uint64) ([]hexutil.Bytes, uint64, error) {
	var result error
	userDeposits, newNonce, err := UserDeposits(receipts, depositContractAddr, currentNonce)
	currentNonce = newNonce
	if err != nil {
		result = multierror.Append(result, err)
	}
	encodedTxs := make([]hexutil.Bytes, 0, len(userDeposits))
	for i, tx := range userDeposits {
		opaqueTx, err := types.NewTx(tx).MarshalBinary()
		if err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to encode user tx %d", i))
		} else {
			encodedTxs = append(encodedTxs, opaqueTx)
		}
	}
	return encodedTxs, currentNonce, result
}
