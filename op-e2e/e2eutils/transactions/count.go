package transactions

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TransactionsBySender(block *types.Block, sender common.Address) (int64, error) {
	txCount := int64(0)
	for _, tx := range block.Transactions() {
		signer := types.NewCancunSigner(tx.ChainId())
		txSender, err := types.Sender(signer, tx)
		if err != nil {
			return 0, err
		}
		if txSender == sender {
			txCount++
		}
	}
	return txCount, nil
}
