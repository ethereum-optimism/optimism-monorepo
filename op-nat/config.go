package nat

import (
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
	SenderSecretKey    string `json:"-"`
	ReceiverPublicKeys []string
}

func NewConfig(ctx *cli.Context, validators []Validator) (*Config, error) {
	// Parse flags
	if err := flags.CheckRequired(ctx); err != nil {
		return nil, fmt.Errorf("missing required flags: %w", err)
	}

	rpcURL := ctx.String(flags.ExecutionRPC.Name)
	senderSecretKey := ctx.String(flags.SenderSecretKey.Name)
	receiverPublicKeys := ctx.StringSlice(flags.ReceiverPublicKeys.Name)

	// Parse kurtosis-devnet manifest
	manifest, err := parseManifest(ctx.String(flags.KurtosisDevnetManifest.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to parse kurtosis-devnet manifest: %w", err)
	}

	return &Config{
		SC:                 *manifest,
		RPCURL:             rpcURL,
		SenderSecretKey:    senderSecretKey,
		ReceiverPublicKeys: receiverPublicKeys,
		Validators:         validators,
	}, nil
}

func (c Config) Check() error {
	if c.SenderSecretKey == "" {
		return fmt.Errorf("missing sender secret key")
	}
	if len(c.ReceiverPublicKeys) == 0 {
		return fmt.Errorf("missing receiver public keys")
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
