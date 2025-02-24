package nooppublisher

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Publisher struct {
	id  seqtypes.PublisherID
	log log.Logger
}

var _ work.Publisher = (*Publisher)(nil)

func NewPublisher(id seqtypes.PublisherID, log log.Logger) *Publisher {
	return &Publisher{id: id, log: log}
}

func (n *Publisher) Close() error {
	return nil
}

func (n *Publisher) String() string {
	return "noop-publisher-" + n.id.String()
}

func (n *Publisher) ID() seqtypes.PublisherID {
	return n.id
}

func (n *Publisher) Publish(ctx context.Context, block work.SignedBlock) error {
	n.log.Info("No-op publish", "block", block)
	return nil
}
