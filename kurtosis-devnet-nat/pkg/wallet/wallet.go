package wallet

import (
	"crypto/ecdsa"

	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet-nat/pkg/network"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/net/context"
)

type Wallet struct {
	privateKeyESCDA *ecdsa.PrivateKey
	privateKey      string
	publicKey       string
	address         common.Address
	name            string
}

func (w *Wallet) Address() common.Address {
	return w.address
}

// NewWallet creates a new wallet.
func NewWallet(privateKeyHex, name string) (*Wallet, error) {

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	address := crypto.PubkeyToAddress(*publicKey)

	return &Wallet{
		privateKeyESCDA: privateKey,
		privateKey:      privateKeyHex,
		publicKey:       address.String(),
		address:         address,
		name:            name,
	}, nil
}

type WalletInterface interface {
	GetBalance(context.Context, network.Network) (int error)
	Send(network.Network, string) error
}

// GetBalance will get the balance of a wallet given a network
func (w *Wallet) GetBalance(ctx context.Context, network network.Network) (*big.Int, error) {
	return network.RPC.BalanceAt(ctx, w.address, nil)
}

func (w *Wallet) Send(ctx context.Context, network network.Network, to common.Address) (*types.Transaction, error) {

	// 2. Get the nonce (transaction count) for the sender's address:
	nonce, err := network.RPC.PendingNonceAt(ctx, w.address)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}
	log.Info("wallet pending nonce", "name", w.name, "nonce", nonce)

	value := big.NewInt(100000)

	// 5. Suggest a gas price:
	gasPrice, err := network.RPC.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest gas price: %w", err)
	}
	log.Info("current gas price", "network", network.Name, "gasPrice", gasPrice)

	// 6. Estimate the gas limit:
	gasLimit := uint64(21000) // Standard gas limit for a simple ETH transfer

	// 7. Create the transaction:
	tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, nil)

	// 8. Sign the transaction:
	chainID, err := network.RPC.NetworkID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get network ID: %v", err)
	}
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), w.privateKeyESCDA)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	// 9. Send the transaction:
	log.Info("sending transaction")
	err = network.RPC.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	log.Info("transcaction sent successfully",
		"tx_hash", signedTx.Hash().Hex(),
	)

	// 10. Return the transaction hash:
	return signedTx, nil
}

func (w *Wallet) Dump(ctx context.Context, log log.Logger, networks []network.Network) {

	balances := []string{}
	for _, n := range networks {
		bal, err := w.GetBalance(ctx, n)
		if err != nil {
			log.Error("Error dumping wallet", "wallet", w.name, "network", n.Name, "err", err)
		}
		balances = append(balances, fmt.Sprintf("%s     : %s", n.Name, bal.String()))
	}

	log.Info(fmt.Sprintf("-------------- Wallet: %s ---------------", w.name))
	log.Info(fmt.Sprintf("private key: %s", w.privateKey))
	log.Info(fmt.Sprintf("public key : %s", w.publicKey))
	log.Info(fmt.Sprintf("address    : %s", w.address))
	for b := range balances {
		log.Info(balances[b])
	}
	log.Info("----------------------------------------")
}
