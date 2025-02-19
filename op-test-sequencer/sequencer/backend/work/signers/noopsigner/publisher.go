package noopsigner

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Signer struct {
	id  seqtypes.SignerID
	log log.Logger
}

var _ work.Signer = (*Signer)(nil)

func NewSigner(id seqtypes.SignerID, log log.Logger) *Signer {
	return &Signer{id: id, log: log}
}

func (n *Signer) Close() error {
	return nil
}

func (n *Signer) String() string {
	return "noop-signer-" + n.id.String()
}

func (n *Signer) ID() seqtypes.SignerID {
	return n.id
}

func (n *Signer) Sign(ctx context.Context, block work.Block) (work.SignedBlock, error) {
	n.log.Info("No-op sign", "block", block)
	return nil, nil
}
