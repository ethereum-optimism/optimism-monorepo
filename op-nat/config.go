package nat

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"math/big"

	"github.com/ethereum-optimism/optimism/op-nat/flags"
	"github.com/ethereum-optimism/optimism/op-nat/network"
	"github.com/ethereum-optimism/optimism/op-nat/wallet"
	"github.com/ethereum/go-ethereum/log"
)

type Config struct {
	// Network config
	SC         SuperchainManifest
	L1RPCUrl   string
	RPCURL     string
	Validators []Validator

	// mix of chain config and tx-fuzz params - needs cleanup
	SenderSecretKey     string `json:"-"`
	ReceiverPublicKeys  []string
	ReceiverPrivateKeys []string

	Wallets []*wallet.Wallet

	// Networks
	L1  *network.Network
	L2A *network.Network
	// Save L2B for interop
	// L2B *network.Network
}

func NewConfig(ctx *cli.Context, log log.Logger, validators []Validator) (*Config, error) {
	// Parse flags
	if err := flags.CheckRequired(ctx); err != nil {
		return nil, fmt.Errorf("missing required flags: %w", err)
	}

	// Parse kurtosis-devnet manifest
	manifest, err := parseManifest(ctx.String(flags.KurtosisDevnetManifest.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to parse kurtosis-devnet manifest: %w", err)
	}

	firstL2 := manifest.L2[0]
	receiverPublicKeys := []string{
		manifest.L1.Wallets["user-key-0"].Address,
		manifest.L1.Wallets["user-key-1"].Address,
		manifest.L1.Wallets["user-key-2"].Address,
	}

	receiverPrivateKeys := []string{
		manifest.L1.Wallets["user-key-0"].PrivateKey,
		manifest.L1.Wallets["user-key-1"].PrivateKey,
		manifest.L1.Wallets["user-key-2"].PrivateKey,
	}

	senderSecretKey := receiverPrivateKeys[2]

	wallets := []*wallet.Wallet{}
	for i, pKey := range receiverPrivateKeys {
		w, err := wallet.NewWallet(pKey, fmt.Sprintf("user-%d", i))
		if err != nil {
			log.Warn("error creating wallet: %w", err)
		}
		wallets = append(wallets, w)
	}

	l1RpcUrl := fmt.Sprintf("https://%s", manifest.L1.Nodes[0].Services.EL.Endpoints["rpc"].Host)

	l2ARpc := fmt.Sprintf("https://%s", firstL2.Nodes[0].Services.EL.Endpoints["rpc"].Host)

	l1, err := network.NewNetwork(ctx.Context, log, l1RpcUrl, "sepolia", big.NewInt(11155111))
	if err != nil {
		return nil, fmt.Errorf("failed to setup l1 network")
	}

	l2A, err := network.NewNetwork(ctx.Context, log, l2ARpc, "alpaca-l2", big.NewInt(11155111100000))
	if err != nil {
		return nil, fmt.Errorf("failed to setup l2a network")
	}

	return &Config{
		SC:                  *manifest,
		L1RPCUrl:            l1RpcUrl,
		RPCURL:              l2ARpc,
		SenderSecretKey:     senderSecretKey,
		ReceiverPublicKeys:  receiverPublicKeys,
		ReceiverPrivateKeys: receiverPrivateKeys,
		Validators:          validators,
		L1:                  l1,
		L2A:                 l2A,
		Wallets:             wallets,
	}, nil
}

func (c Config) Check() error {
	if c.SenderSecretKey == "" {
		return fmt.Errorf("missing sender secret key")
	}
	if len(c.ReceiverPublicKeys) == 0 {
		return fmt.Errorf("missing receiver public keys")
	}
	if len(c.ReceiverPrivateKeys) == 0 {
		return fmt.Errorf("missing receiver private keys")
	}
	return nil
}

// GetNetworks returns all of the networks in an array
func (c *Config) GetNetworks() []*network.Network {
	return []*network.Network{
		c.L1,
		c.L2A,
	}
}

func parseManifest(manifestPath string) (*SuperchainManifest, error) {
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var superchainManifest SuperchainManifest
	if err := json.Unmarshal(manifest, &superchainManifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	return &superchainManifest, nil
}

// GetWallet will return a specifc wallet from the wallets array that matches a public key
func (c *Config) GetWallet(pubKey string) *wallet.Wallet {
	for _, w := range c.Wallets {
		if w.Address().String() == pubKey {
			return w
		}
	}
	return nil
}

func (c *Config) GetRandomWallet() *wallet.Wallet {
	randomIndex := rand.Intn(len(c.Wallets))
	return c.Wallets[randomIndex]
}

// GetWalletWithBalance is used to find a wallet with a balance on a network
func (c *Config) GetWalletWithBalance(ctx context.Context, network *network.Network, amount *big.Int) (*wallet.Wallet, error) {
	for _, w := range c.Wallets {
		balance, err := w.GetBalance(ctx, network)
		if err != nil {
			log.Error("error getting wallet balance",
				"wallet", w.Address(),
				"network", network.Name,
			)
			return nil, err
		}
		log.Info("",
			"wallet", w.Address(),
			"balance", balance,
			"network", network.Name,
		)
		if balance.Cmp(amount) == 1 {
			return w, nil
		}
	}
	return nil, errors.Errorf("no wallet found with balance %d on network %s",
		amount,
		network.Name,
	)
}
