package noopseq

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Sequencer struct {
	id  seqtypes.SequencerID
	log log.Logger
}

var _ work.Sequencer = (*Sequencer)(nil)

func NewSequencer(id seqtypes.SequencerID, log log.Logger) *Sequencer {
	return &Sequencer{id: id, log: log}
}

func (n *Sequencer) Close() error {
	return nil
}

func (n *Sequencer) String() string {
	return "noop-sequencer-" + n.id.String()
}

func (n *Sequencer) ID() seqtypes.SequencerID {
	return n.id
}

func (n *Sequencer) Open(ctx context.Context) error {
	return nil
}

func (n *Sequencer) BuildJob() work.BuildJob {
	return nil
}

func (n *Sequencer) Seal(ctx context.Context) error {
	return nil
}

func (n *Sequencer) Prebuilt(ctx context.Context, block work.Block) error {
	return nil
}

func (n *Sequencer) Sign(ctx context.Context) error {
	return nil
}

func (n *Sequencer) Commit(ctx context.Context) error {
	return nil
}

func (n *Sequencer) Publish(ctx context.Context) error {
	return nil
}

func (n *Sequencer) Next(ctx context.Context) error {
	return nil
}

func (n *Sequencer) Start(ctx context.Context, head common.Hash) error {
	return seqtypes.ErrSequencerInactive
}

func (n *Sequencer) Stop(ctx context.Context) (last common.Hash, err error) {
	return common.Hash{}, seqtypes.ErrSequencerInactive
}

func (n *Sequencer) Active() bool {
	return false
}
