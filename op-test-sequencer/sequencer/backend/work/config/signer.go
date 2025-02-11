package config

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/signers/localkey"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work/signers/noopsigner"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type SignerEntry struct {
	LocalKey *localkey.Config   `yaml:"local-key,omitempty"`
	Noop     *noopsigner.Config `yaml:"noop,omitempty"`
}

func (b *SignerEntry) Start(ctx context.Context, id seqtypes.SignerID, collection work.Collection, opts *work.StartOpts) (work.Signer, error) {
	switch {
	case b.LocalKey != nil:
		return b.LocalKey.Start(ctx, id, collection, opts)
	case b.Noop != nil:
		return b.Noop.Start(ctx, id, collection, opts)
	default:
		return nil, seqtypes.ErrUnknownKind
	}
}
