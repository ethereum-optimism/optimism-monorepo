package nat

import (
	// "context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-nat/flags"
)

type Config struct {
	SC         SuperchainManifest
	RPCURL     string
	Validators []Validator

	// tx-fuzz
	SenderSecretKey     string `json:"-"`
	ReceiverPublicKeys  []string
	ReceiverPrivateKeys []string
}

func NewConfig(ctx *cli.Context, validators []Validator) (*Config, error) {
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
	rpcURL := fmt.Sprintf("http://%s:%d", firstL2.Nodes[0].Services.EL.Endpoints["rpc"].Host, firstL2.Nodes[0].Services.EL.Endpoints["rpc"].Port)
	senderSecretKey := firstL2.Wallets["l2Faucet"].PrivateKey
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

	return &Config{
		SC:                  *manifest,
		RPCURL:              rpcURL,
		SenderSecretKey:     senderSecretKey,
		ReceiverPublicKeys:  receiverPublicKeys,
		ReceiverPrivateKeys: receiverPrivateKeys,
		Validators:          validators,
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
