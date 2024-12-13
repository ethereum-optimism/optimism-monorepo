package derive

import (
	"context"
	"errors"
	"io"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

// ChannelAssembler assembles frames into a raw channel. It replaces the ChannelBank since Holocene.
type ChannelAssembler struct {
	log     log.Logger
	spec    ChannelStageSpec
	metrics Metrics

	channel *Channel

	prev NextFrameProvider
}

var _ RawChannelProvider = (*ChannelAssembler)(nil)

type ChannelStageSpec interface {
	ChannelTimeout(t uint64) uint64
	MaxRLPBytesPerChannel(t uint64) uint64
}

// NewChannelAssembler creates the Holocene channel stage.
// It must only be used for derivation from Holocene origins.
func NewChannelAssembler(log log.Logger, spec ChannelStageSpec, prev NextFrameProvider, m Metrics) *ChannelAssembler {
	return &ChannelAssembler{
		log:     log,
		spec:    spec,
		metrics: m,
		prev:    prev,
	}
}

func (ca *ChannelAssembler) Log() log.Logger {
	return ca.log.New("stage", "channel", "origin", ca.Origin())
}

func (ca *ChannelAssembler) Origin() eth.L1BlockRef {
	return ca.prev.Origin()
}

func (ca *ChannelAssembler) Reset(context.Context, eth.L1BlockRef, eth.SystemConfig) error {
	ca.resetChannel()
	return io.EOF
}

func (ca *ChannelAssembler) FlushChannel() {
	ca.resetChannel()
}

func (ca *ChannelAssembler) resetChannel() {
	ca.channel = nil
}

// Returns whether the current staging channel is timed out. Panics if there's no current channel.
func (ca *ChannelAssembler) channelTimedOut() bool {
	return ca.channel.OpenBlockNumber()+ca.spec.ChannelTimeout(ca.Origin().Time) < ca.Origin().Number
}

func (ca *ChannelAssembler) NextRawChannel(ctx context.Context) ([]byte, error) {
	if ca.channel != nil && ca.channelTimedOut() {
		ca.metrics.RecordChannelTimedOut()
		ca.resetChannel()
	}

	lgr := ca.Log()
	origin := ca.Origin()

	// Note that if the current channel was already completed, we would have forwarded its data
	// already. So we start by reading in frames.
	if ca.channel != nil && ca.channel.IsReady() {
		return nil, NewCriticalError(errors.New("unexpected ready channel"))
	}

	// Ingest frames until we either hit an error (including io.EOF and NotEnoughData) or complete a
	// channel.
	// Note that we ingest the frame queue in a loop instead of returning NotEnoughData after a
	// single frame ingestion, because it is guaranteed that the total size of new frames ingested
	// per L1 origin block is limited by the size of batcher transactions in that block and it
	// doesn't make a difference in computational effort if these are many small frames or one large
	// frame of that size. Plus, this is really just moving data around, no decompression etc. yet.
	for {
		frame, err := ca.prev.NextFrame(ctx)
		if err != nil { // includes io.EOF; a last frame broke the loop already
			return nil, err
		}

		// first frames always start a new channel, discarding an existing one
		if frame.FrameNumber == 0 {
			ca.metrics.RecordHeadChannelOpened()
			ca.channel = NewChannel(frame.ID, origin, true)
		}
		if frame.FrameNumber > 0 && ca.channel == nil {
			lgr.Warn("dropping non-first frame without channel",
				"frame_channel", frame.ID, "frame_number", frame.FrameNumber)
			continue // read more frames
		}

		// Catches Holocene ordering rules. Note that even though the frame queue is guaranteed to
		// only hold ordered frames in the current queue, it cannot guarantee this w.r.t. frames
		// that already got dequeued. So ordering has to be checked here again.
		if err := ca.channel.AddFrame(frame, origin); err != nil {
			lgr.Warn("failed to add frame to channel",
				"channel", ca.channel.ID(), "frame_channel", frame.ID,
				"frame_number", frame.FrameNumber, "err", err)
			continue // read more frames
		}
		if ca.channel.Size() > ca.spec.MaxRLPBytesPerChannel(ca.Origin().Time) {
			lgr.Warn("dropping oversized channel",
				"channel", ca.channel.ID(), "frame_number", frame.FrameNumber)
			ca.resetChannel()
			continue // read more frames
		}
		ca.metrics.RecordFrame()

		if frame.IsLast {
			break // forward current complete channel
		}
	}

	ch := ca.channel
	// Note that if we exit the frame ingestion loop, we're guaranteed to have a ready channel.
	if ch == nil || !ch.IsReady() {
		return nil, NewCriticalError(errors.New("unexpected non-ready channel"))
	}

	ca.resetChannel()
	r := ch.Reader()
	return io.ReadAll(r)
}
