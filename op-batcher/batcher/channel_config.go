package batcher

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-batcher/compressor"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
)

type ChannelConfig struct {
	// Number of epochs (L1 blocks) per sequencing window, including the epoch
	// L1 origin block itself
	SeqWindowSize uint64
	// The maximum number of L1 blocks that the inclusion transactions of a
	// channel's frames can span.
	ChannelTimeout uint64

	// Builder Config

	// MaxChannelDuration is the maximum duration (in #L1-blocks) to keep the
	// channel open. This allows control over how long a channel is kept open
	// during times of low transaction volume.
	//
	// If 0, duration checks are disabled.
	MaxChannelDuration uint64
	// The batcher tx submission safety margin (in #L1-blocks) to subtract from
	// a channel's timeout and sequencing window, to guarantee safe inclusion of
	// a channel on L1.
	SubSafetyMargin uint64
	// The maximum byte-size a frame can have.
	MaxFrameSize uint64
	// MaxBlocksPerSpanBatch is the maximum number of blocks to add to a span batch.
	// A value of 0 disables a maximum.
	MaxBlocksPerSpanBatch int

	// Target number of frames to create per channel.
	// For blob transactions, this controls the number of blobs to target adding
	// to each blob tx.
	TargetNumFrames int

	// CompressorConfig contains the configuration for creating new compressors.
	// It should not be set directly, but via the Init*Compressor methods after
	// creating the ChannelConfig to guarantee a consistent configuration.
	CompressorConfig compressor.Config

	// BatchType indicates whether the channel uses SingularBatch or SpanBatch.
	BatchType uint

	// UseBlobs indicates that this channel should be sent as a multi-blob
	// transaction with one blob per frame.
	UseBlobs bool
}

// ChannelConfig returns a copy of the receiver.
// This allows the receiver to be a static ChannelConfigProvider of itself.
func (cc ChannelConfig) ChannelConfig(isPectra bool) ChannelConfig {
	return cc
}

// InitCompressorConfig (re)initializes the channel configuration's compressor
// configuration using the given values. The TargetOutputSize will be set to a
// value consistent with cc.TargetNumFrames and cc.MaxFrameSize.
// comprKind can be the empty string, in which case the default compressor will
// be used.
func (cc *ChannelConfig) InitCompressorConfig(approxComprRatio float64, comprKind string, compressionAlgo derive.CompressionAlgo) {
	cc.CompressorConfig = compressor.Config{
		// Compressor output size needs to account for frame encoding overhead
		TargetOutputSize: MaxDataSize(cc.TargetNumFrames, cc.MaxFrameSize),
		ApproxComprRatio: approxComprRatio,
		Kind:             comprKind,
		CompressionAlgo:  compressionAlgo,
	}
}

func (cc *ChannelConfig) InitRatioCompressor(approxComprRatio float64, compressionAlgo derive.CompressionAlgo) {
	cc.InitCompressorConfig(approxComprRatio, compressor.RatioKind, compressionAlgo)
}

func (cc *ChannelConfig) InitShadowCompressor(compressionAlgo derive.CompressionAlgo) {
	cc.InitCompressorConfig(0, compressor.ShadowKind, compressionAlgo)
}

func (cc *ChannelConfig) InitNoneCompressor() {
	cc.InitCompressorConfig(0, compressor.NoneKind, derive.Zlib)
}

func (cc *ChannelConfig) ReinitCompressorConfig() {
	cc.InitCompressorConfig(
		cc.CompressorConfig.ApproxComprRatio,
		cc.CompressorConfig.Kind,
		cc.CompressorConfig.CompressionAlgo,
	)
}

func (cc *ChannelConfig) MaxFramesPerTx() int {
	if !cc.UseBlobs {
		return 1
	}
	return cc.TargetNumFrames
}

// Check validates the [ChannelConfig] parameters.
func (cc *ChannelConfig) Check() error {
	// The [ChannelTimeout] must be larger than the [SubSafetyMargin].
	// Otherwise, new blocks would always be considered timed out.
	if cc.ChannelTimeout < cc.SubSafetyMargin {
		return fmt.Errorf("%w: %d < %d", ErrInvalidChannelTimeout, cc.ChannelTimeout, cc.SubSafetyMargin)
	}

	// The max frame size must at least be able to accommodate the constant
	// frame overhead.
	if cc.MaxFrameSize < derive.FrameV0OverHeadSize {
		return fmt.Errorf("max frame size %d is less than the minimum %d",
			cc.MaxFrameSize, derive.FrameV0OverHeadSize)
	}

	if cc.BatchType > derive.SpanBatchType {
		return fmt.Errorf("unrecognized batch type: %d", cc.BatchType)
	}

	if nf := cc.TargetNumFrames; nf < 1 {
		return fmt.Errorf("invalid number of frames %d", nf)
	}

	return nil
}

// MaxDataSize returns the maximum byte size of output data that can be packed
// into a channel with numFrames frames and frames of max size maxFrameSize.
// It accounts for the constant frame overhead. It panics if the maxFrameSize
// is smaller than [derive.FrameV0OverHeadSize].
func MaxDataSize(numFrames int, maxFrameSize uint64) uint64 {
	if maxFrameSize < derive.FrameV0OverHeadSize {
		panic("max frame size smaller than frame overhead")
	}
	return uint64(numFrames) * (maxFrameSize - derive.FrameV0OverHeadSize)
}
