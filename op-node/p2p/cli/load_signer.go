package cli

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-node/flags"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	opsigner "github.com/ethereum-optimism/optimism/op-service/signer"
)

// LoadSignerSetup loads a configuration for a Signer to be set up later
func LoadSignerSetup(ctx *cli.Context, logger log.Logger) (p2p.SignerSetup, error) {
	key := ctx.String(flags.SequencerP2PKeyName)
	signerCfg := opsigner.ReadCLIConfig(ctx)
	if key != "" {
		// Mnemonics are bad because they leak *all* keys when they leak.
		// Unencrypted keys from file are bad because they are easy to leak (and we are not checking file permissions).
		priv, err := crypto.HexToECDSA(strings.TrimPrefix(key, "0x"))
		if err != nil {
			return nil, fmt.Errorf("failed to read batch submitter key: %w", err)
		}

		return &p2p.PreparedSigner{Signer: p2p.NewLocalSigner(priv)}, nil
	} else if signerCfg.Enabled() {
		remoteSigner, err := p2p.NewRemoteSigner(logger, signerCfg)
		if err != nil {
			return nil, err
		}
		return &p2p.PreparedSigner{Signer: remoteSigner}, nil
	}

	return nil, nil
}
