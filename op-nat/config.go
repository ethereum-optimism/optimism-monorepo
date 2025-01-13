package nat

import (
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-nat/flags"
)

type Config struct {
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

	if !strings.Contains(rpcURL, "http") {
		return nil, errors.New("RPC URL is malformed")
	}

	return &Config{
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
