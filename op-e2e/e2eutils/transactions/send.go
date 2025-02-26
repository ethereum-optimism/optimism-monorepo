package transactions

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/errutil"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

type SendTxOpt func(cfg *sendTxCfg)

type sendTxCfg struct {
	receiptStatus       uint64
	ignoreReceiptStatus bool
}

func makeSendTxCfg(opts ...SendTxOpt) *sendTxCfg {
	cfg := &sendTxCfg{
		receiptStatus: types.ReceiptStatusSuccessful,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func WithReceiptFail() SendTxOpt {
	return func(cfg *sendTxCfg) {
		cfg.receiptStatus = types.ReceiptStatusFailed
	}
}

func WithReceiptStatusIgnore() SendTxOpt {
	return func(cfg *sendTxCfg) {
		cfg.ignoreReceiptStatus = true
	}
}

func RequireSendTx(t *testing.T, ctx context.Context, client *ethclient.Client, candidate txmgr.TxCandidate, privKey *ecdsa.PrivateKey, opts ...SendTxOpt) (*types.Transaction, *types.Receipt) {
	tx, rcpt, err := SendTx(ctx, client, candidate, privKey, opts...)
	require.NoError(t, err, "Failed to send transaction")
	return tx, rcpt
}

// RequireSendTxs submits multiple transactions attempting to batch them together in blocks as much as possible.
// There is however no guarantee that all transactions will be included in the same block or that any transactions
// will share a block.
// Note that if the transactions depend on one another, the gas limit may need to be manually set as estimateGas will
// be executed before the earlier transactions have been processed.
func RequireSendTxs(t *testing.T, ctx context.Context, client *ethclient.Client, candidates []txmgr.TxCandidate, privKey *ecdsa.PrivateKey, opts ...SendTxOpt) ([]*types.Transaction, []*types.Receipt) {
	cfg := makeSendTxCfg(opts...)
	// First convert all the candidates to signed transactions so they are most likely to be included in the same block
	// This still isn't guaranteed but minimises the delay between each transaction submission.
	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(privKey.PublicKey))
	require.NoError(t, err, "Failed to get pending nonce")
	txs := make([]*types.Transaction, len(candidates))
	for i, candidate := range candidates {
		tx, err := createTx(ctx, client, candidate, privKey, nonce)
		require.NoErrorf(t, err, "Failed to create transaction %v", i)
		txs[i] = tx
		nonce++
	}

	// Then send all transactions (some may be included in the same block)
	for i, tx := range txs {
		err := client.SendTransaction(ctx, tx)
		require.NoErrorf(t, err, "Failed to send transaction %v", i)
	}

	// Then wait for all receipts
	receipts := make([]*types.Receipt, len(txs))
	for i, tx := range txs {
		receipt, err := wait.ForReceiptMaybe(ctx, client, tx.Hash(), cfg.receiptStatus, cfg.ignoreReceiptStatus)
		if err != nil {
			fmt.Printf("Failed to get receipt for %v: %v", i, err)
		}
		require.NoErrorf(t, err, "Failed to find receipt for tx %v (%s)", i, tx.Hash())
		receipts[i] = receipt
	}
	return txs, receipts
}

func SendTx(ctx context.Context, client *ethclient.Client, candidate txmgr.TxCandidate, privKey *ecdsa.PrivateKey, opts ...SendTxOpt) (*types.Transaction, *types.Receipt, error) {
	cfg := makeSendTxCfg(opts...)
	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(privKey.PublicKey))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get next nonce: %w", err)
	}
	tx, err := createTx(ctx, client, candidate, privKey, nonce)
	if err != nil {
		return nil, nil, err
	}
	err = client.SendTransaction(ctx, tx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send transaction (tx: %s): %w", tx.Hash(), errutil.TryAddRevertReason(err))
	}
	receipt, err := wait.ForReceiptMaybe(ctx, client, tx.Hash(), cfg.receiptStatus, cfg.ignoreReceiptStatus)
	if err != nil {
		return tx, receipt, fmt.Errorf("failed to find OK receipt (tx: %s): %w", tx.Hash(), err)
	}
	return tx, receipt, nil
}

func createTx(ctx context.Context, client *ethclient.Client, candidate txmgr.TxCandidate, privKey *ecdsa.PrivateKey, nonce uint64) (*types.Transaction, error) {
	from := crypto.PubkeyToAddress(privKey.PublicKey)
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	latestBlock, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}
	gasFeeCap := new(big.Int).Mul(latestBlock.BaseFee, big.NewInt(3))
	gasTipCap := big.NewInt(1 * params.GWei)
	if gasFeeCap.Cmp(gasTipCap) < 0 {
		// gasTipCap can't be higher than gasFeeCap
		// Since there's a minimum gasTipCap to be accepted, increase the gasFeeCap. Extra will be refunded anyway.
		gasFeeCap = gasTipCap
	}
	msg := ethereum.CallMsg{
		From:      from,
		To:        candidate.To,
		Value:     candidate.Value,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Data:      candidate.TxData,
	}
	gas := candidate.GasLimit
	if gas == 0 {
		gas, err = client.EstimateGas(ctx, msg)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate gas: %w", errutil.TryAddRevertReason(err))
		}
	}

	tx := types.MustSignNewTx(privKey, types.LatestSignerForChainID(chainID), &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        candidate.To,
		Value:     candidate.Value,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Data:      candidate.TxData,
		Gas:       gas,
	})
	return tx, nil
}
