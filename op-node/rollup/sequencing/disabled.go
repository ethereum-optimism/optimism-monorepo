package sequencing

import (
	"context"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
)

var ErrSequencerNotEnabled = errors.New("sequencer is not enabled")

type DisabledSequencer struct{}

var _ SequencerIface = DisabledSequencer{}

func (ds DisabledSequencer) OnEvent(ev event.Event) bool {
	return false
}

func (ds DisabledSequencer) NextAction() (t time.Time, ok bool) {
	return time.Time{}, false
}

func (ds DisabledSequencer) Active() bool {
	return false
}

func (ds DisabledSequencer) Init(ctx context.Context, active bool) error {
	return ErrSequencerNotEnabled
}

func (ds DisabledSequencer) Start(ctx context.Context, head common.Hash) error {
	return ErrSequencerNotEnabled
}

func (ds DisabledSequencer) Stop(ctx context.Context) (hash common.Hash, err error) {
	return common.Hash{}, ErrSequencerNotEnabled
}

func (ds DisabledSequencer) SetMaxSafeLag(ctx context.Context, v uint64) error {
	return ErrSequencerNotEnabled
}

func (ds DisabledSequencer) OverrideLeader(ctx context.Context) error {
	return ErrSequencerNotEnabled
}

func (ds DisabledSequencer) ConductorEnabled(ctx context.Context) bool {
	return false
}

func (ds DisabledSequencer) SetRecoverMode(mode bool) {}

func (ds DisabledSequencer) Close() {}
