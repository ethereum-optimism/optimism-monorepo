package system

import (
	"context"
	"fmt"
	"math/big"

	sdkTypes "github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthClient defines the interface for interacting with Ethereum node
type EthClient interface {
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

// TransactionProcessor handles signing and sending transactions
type transactionProcessor struct {
	client     EthClient
	chainID    *big.Int
	privateKey sdkTypes.Key
}

// NewTransactionProcessor creates a new transaction processor
func NewTransactionProcessor(client EthClient, chainID *big.Int) TransactionProcessor {
	return &transactionProcessor{
		client:  client,
		chainID: chainID,
	}
}

// NewEthTransactionProcessor creates a new transaction processor with an ethclient
func NewEthTransactionProcessor(client *ethclient.Client, chainID *big.Int) TransactionProcessor {
	return NewTransactionProcessor(client, chainID)
}

// Sign signs a transaction with the given private key
func (p *transactionProcessor) Sign(tx Transaction) (Transaction, error) {
	pk := p.privateKey
	if pk == nil {
		return nil, fmt.Errorf("private key is nil")
	}

	var signer types.Signer
	switch tx.Type() {
	case types.DynamicFeeTxType:
		signer = types.NewLondonSigner(p.chainID)
	case types.AccessListTxType:
		signer = types.NewEIP2930Signer(p.chainID)
	default:
		signer = types.NewEIP155Signer(p.chainID)
	}

	if rt, ok := tx.(RawTransaction); ok {
		signedTx, err := types.SignTx(rt.Raw(), signer, pk)
		if err != nil {
			return nil, fmt.Errorf("failed to sign transaction: %w", err)
		}

		return &EthTx{
			tx:     signedTx,
			from:   tx.From(),
			txType: tx.Type(),
		}, nil
	}

	return nil, fmt.Errorf("transaction does not support signing")
}

// Send sends a signed transaction to the network
func (p *transactionProcessor) Send(ctx context.Context, tx Transaction) error {
	if st, ok := tx.(RawTransaction); ok {
		if err := p.client.SendTransaction(ctx, st.Raw()); err != nil {
			return fmt.Errorf("failed to send transaction: %w", err)
		}
		return nil
	}

	return fmt.Errorf("transaction is not signed")
}
