package system

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	coreTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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

func newWallet(pk types.Key, addr types.Address, chain *chain) *wallet {
	return &wallet{
		privateKey: pk,
		address:    addr,
		chain:      chain,
	}
}

func (w *wallet) PrivateKey() types.Key {
	return strings.TrimPrefix(w.privateKey, "0x")
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
		return types.NewBalance(new(big.Int))
	}

	balance, err := client.BalanceAt(context.Background(), w.address, nil)
	if err != nil {
		return types.NewBalance(new(big.Int))
	}

	return types.NewBalance(balance)
}

type sendImpl struct {
	chain  internalChain
	pk     types.Key
	to     types.Address
	amount types.Balance
}

func (i *sendImpl) Call(ctx context.Context) (any, error) {
	client, err := i.chain.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	pk, err := crypto.HexToECDSA(string(i.pk))
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	from := crypto.PubkeyToAddress(pk.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	// TODO: compute an accurate gas limit
	gasLimit := uint64(210000) // 10x Standard ETH transfer gas limit
	tx := coreTypes.NewTransaction(nonce, i.to, i.amount.Int, gasLimit, gasPrice, nil)

	chainID, err := client.NetworkID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain id: %w", err)
	}

	signedTx, err := coreTypes.SignTx(tx, coreTypes.NewEIP155Signer(chainID), pk)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return signedTx, nil
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
	tx    *coreTypes.Transaction
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

	receipt, err := bind.WaitMined(context.Background(), client, r.tx)
	if err != nil {
		return fmt.Errorf("failed waiting for transaction confirmation: %w", err)
	}

	if receipt.Status == 0 {
		return fmt.Errorf("transaction failed")
	}

	return nil
}

func sendETH(ctx context.Context, chain internalChain, privateKey string, to types.Address, amount types.Balance) (*coreTypes.Transaction, error) {
	client, err := chain.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	pk, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	from := crypto.PubkeyToAddress(pk.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	gasLimit := uint64(210000) // 10x Standard ETH transfer gas limit
	tx := coreTypes.NewTransaction(nonce, to, amount.Int, gasLimit, gasPrice, nil)

	chainID, err := client.NetworkID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain id: %w", err)
	}

	signedTx, err := coreTypes.SignTx(tx, coreTypes.NewEIP155Signer(chainID), pk)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx, nil
}
