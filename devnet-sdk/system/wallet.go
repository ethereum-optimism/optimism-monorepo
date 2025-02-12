package system

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	coreTypes "github.com/ethereum/go-ethereum/core/types"
)

// internalChain provides access to internal chain functionality
type internalChain interface {
	Chain
	getClient() (*ethclient.Client, error)
}

type wallet struct {
	privateKey types.Key
	address    types.Address
	chain      internalChain
}

func newWallet(pk string, addr types.Address, chain *chain) (*wallet, error) {
	pk = strings.TrimPrefix(pk, "0x")
	pkBytes, err := hex.DecodeString(pk)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}
	privateKey, err := crypto.ToECDSA(pkBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert private key to ECDSA: %w", err)
	}

	return &wallet{
		privateKey: privateKey,
		address:    addr,
		chain:      chain,
	}, nil
}

func (w *wallet) PrivateKey() types.Key {
	return w.privateKey
}

func (w *wallet) Address() types.Address {
	return w.address
}

func (w *wallet) SendETH(to types.Address, amount types.Balance) types.WriteInvocation[any] {
	return &sendImpl{
		chain:  w.chain,
		pk:     w.PrivateKey(),
		to:     to,
		amount: amount,
	}
}

func (w *wallet) Balance() types.Balance {
	client, err := w.chain.getClient()
	if err != nil {
		return types.Balance{}
	}

	balance, err := client.BalanceAt(context.Background(), w.address, nil)
	if err != nil {
		return types.Balance{}
	}

	return types.NewBalance(balance)
}

func (w *wallet) Nonce() uint64 {
	client, err := w.chain.getClient()
	if err != nil {
		return 0
	}

	nonce, err := client.PendingNonceAt(context.Background(), w.address)
	if err != nil {
		return 0
	}

	return nonce
}

func (w *wallet) Signer() types.Signer {
	pk := w.PrivateKey()

	auth, err := bind.NewKeyedTransactorWithChainID(pk, w.chain.ID())
	if err != nil {
		panic(err)
	}

	return auth
}

func (w *wallet) Sign(tx Transaction) (Transaction, error) {
	pk := w.privateKey

	var signer coreTypes.Signer
	switch tx.Type() {
	case coreTypes.DynamicFeeTxType:
		signer = coreTypes.NewLondonSigner(w.chain.ID())
	case coreTypes.AccessListTxType:
		signer = coreTypes.NewEIP2930Signer(w.chain.ID())
	default:
		signer = coreTypes.NewEIP155Signer(w.chain.ID())
	}

	if rt, ok := tx.(RawTransaction); ok {
		signedTx, err := coreTypes.SignTx(rt.Raw(), signer, pk)
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

func (w *wallet) Send(ctx context.Context, tx Transaction) error {
	if st, ok := tx.(RawTransaction); ok {
		client, err := w.chain.getClient()
		if err != nil {
			return fmt.Errorf("failed to get client: %w", err)
		}
		if err := client.SendTransaction(ctx, st.Raw()); err != nil {
			return fmt.Errorf("failed to send transaction: %w", err)
		}
		return nil
	}

	return fmt.Errorf("transaction is not signed")
}

type sendImpl struct {
	chain  internalChain
	pk     types.Key
	to     types.Address
	amount types.Balance
}

func (i *sendImpl) Call(ctx context.Context) (any, error) {
	pk := i.pk

	from := crypto.PubkeyToAddress(pk.PublicKey)
	toAddr := i.to

	builder := NewTxBuilder(ctx, i.chain)
	tx, err := builder.BuildTx(
		WithFrom(from),
		WithTo(toAddr),
		WithValue(i.amount.Int),
		WithData(nil),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}

	processor, err := i.chain.TransactionProcessor()
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction processor: %w", err)
	}
	tx, err = processor.Sign(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return tx, nil
}

func (i *sendImpl) Send(ctx context.Context) types.InvocationResult {
	tx, err := sendETH(ctx, i.chain, i.pk, i.to, i.amount)
	return &sendResult{
		chain: i.chain,
		tx:    tx,
		err:   err,
	}
}

type sendResult struct {
	chain internalChain
	tx    Transaction
	err   error
}

func (r *sendResult) Error() error {
	return r.err
}

func (r *sendResult) Wait() error {
	client, err := r.chain.getClient()
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}

	if r.err != nil {
		return r.err
	}
	if r.tx == nil {
		return fmt.Errorf("no transaction to wait for")
	}

	if tx, ok := r.tx.(RawTransaction); ok {
		receipt, err := bind.WaitMined(context.Background(), client, tx.Raw())
		if err != nil {
			return fmt.Errorf("failed waiting for transaction confirmation: %w", err)
		}

		if receipt.Status == 0 {
			return fmt.Errorf("transaction failed")
		}
	}

	return nil
}

func sendETH(ctx context.Context, chain internalChain, pk types.Key, to types.Address, amount types.Balance) (Transaction, error) {
	from := crypto.PubkeyToAddress(pk.PublicKey)

	builder := NewTxBuilder(ctx, chain)
	tx, err := builder.BuildTx(
		WithFrom(from),
		WithTo(to),
		WithValue(amount.Int),
		WithData(nil),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}

	processor, err := chain.TransactionProcessor()
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction processor: %w", err)
	}
	tx, err = processor.Sign(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	if err := processor.Send(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	return tx, nil
}
