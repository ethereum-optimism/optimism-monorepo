package transactions

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// TransactionsBySenderCount returns the number of transactions in the block that were sent by the given sender.
func TransactionsBySenderCount(block *types.Block, sender common.Address) (int64, error) {
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

func TransactionsBySender(block *types.Block, sender common.Address) ([]*types.Transaction, error) {
	txs := make([]*types.Transaction, 0)
	for _, tx := range block.Transactions() {
		signer := types.NewCancunSigner(tx.ChainId())
		txSender, err := types.Sender(signer, tx)
		if err != nil {
			return nil, err
		}
		if txSender == sender {
			txs = append(txs, tx)
		}
	}
	return txs, nil
}
