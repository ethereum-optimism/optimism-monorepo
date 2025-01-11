package wallet

import (
	"crypto/ecdsa"

	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet-nat/pkg/network"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/net/context"
)

type Wallet struct {
	privateKey string
	publicKey  string
	address    common.Address
	name       string
}

// NewWallet creates a new wallet.
func NewWallet(privateKeyHex, name string) (*Wallet, error) {

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %v", err)
	}

	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	address := crypto.PubkeyToAddress(*publicKey)

	return &Wallet{
		privateKey: privateKeyHex,
		publicKey:  address.String(),
		address:    address,
		name:       name,
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

func (w *Wallet) Send() string {
	return w.privateKey
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
