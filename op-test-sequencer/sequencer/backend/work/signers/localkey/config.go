package localkey

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Config struct {
	ChainID eth.ChainID    `yaml:"chainID"`
	KeyPath string         `yaml:"path,omitempty"`
	RawKey  *hexutil.Bytes `yaml:"raw,omitempty"`
}

func readHexBytes(keyPath string) ([]byte, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %q: %w", keyPath, err)
	}
	// Remove optional 0x prefix
	key := strings.TrimPrefix(string(data), "0x")
	// Geth always wants the 0x prefix
	return hexutil.Decode("0x" + key)
}

func (c *Config) Start(ctx context.Context, id seqtypes.SignerID, collection work.Collection, opts *work.StartOpts) (work.Signer, error) {
	var keyBytes []byte
	if c.KeyPath != "" && c.RawKey != nil {
		return nil, errors.New("cannot specify both keyPath and rawKey")
	} else if c.KeyPath != "" {
		x, err := readHexBytes(c.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("")
		}
		keyBytes = x
	} else if c.RawKey != nil {
		keyBytes = *c.RawKey
	} else {
		return nil, errors.New("no key specified")
	}
	priv, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}
	signer := p2p.NewLocalSigner(priv)

	bu := &Signer{
		id:      id,
		log:     opts.Log,
		chainID: eth.ChainID{},
		signer:  signer,
	}
	return bu, nil
}
