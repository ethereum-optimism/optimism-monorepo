package batcher

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-batcher/compressor"
	"github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

type CLIConfig struct {
	// L1EthRpc is the HTTP provider URL for L1.
	L1EthRpc string

	// L2EthRpc is the HTTP provider URL for the L2 execution engine. A comma-separated list enables the active L2 provider. Such a list needs to match the number of RollupRpcs provided.
	L2EthRpc string

	// RollupRpc is the HTTP provider URL for the L2 rollup node. A comma-separated list enables the active L2 provider. Such a list needs to match the number of L2EthRpcs provided.
	RollupRpc string

	// MaxChannelDuration is the maximum duration (in #L1-blocks) to keep a
	// channel open. This allows to more eagerly send batcher transactions
	// during times of low L2 transaction volume. Note that the effective
	// L1-block distance between batcher transactions is then MaxChannelDuration
	// + NumConfirmations because the batcher waits for NumConfirmations blocks
	// after sending a batcher tx and only then starts a new channel.
	//
	// If 0, duration checks are disabled.
	MaxChannelDuration uint64

	// The batcher tx submission safety margin (in #L1-blocks) to subtract from
	// a channel's timeout and sequencing window, to guarantee safe inclusion of
	// a channel on L1.
	SubSafetyMargin uint64

	// PollInterval is the delay between querying L2 for more transaction
	// and creating a new batch.
	PollInterval time.Duration

	// MaxPendingTransactions is the maximum number of concurrent pending
	// transactions sent to the transaction manager (0 == no limit).
	MaxPendingTransactions uint64

	// MaxL1TxSize is the maximum size of a batch tx submitted to L1.
	// If using blobs, this setting is ignored and the max blob size is used.
	MaxL1TxSize uint64

	// Maximum number of blocks to add to a span batch. Default is 0 - no maximum.
	MaxBlocksPerSpanBatch int

	// The target number of frames to create per channel. Controls number of blobs
	// per blob tx, if using Blob DA.
	TargetNumFrames int

	// ApproxComprRatio to assume (only [compressor.RatioCompressor]).
	// Should be slightly smaller than average from experiments to avoid the
	// chances of creating a small additional leftover frame.
	ApproxComprRatio float64

	// Type of compressor to use. Must be one of [compressor.KindKeys].
	Compressor string

	// Type of compression algorithm to use. Must be one of [zlib, brotli, brotli[9-11]]
	CompressionAlgo derive.CompressionAlgo

	// If Stopped is true, the batcher starts stopped and won't start batching right away.
	// Batching needs to be started via an admin RPC.
	Stopped bool

	// Whether to wait for the sequencer to sync to a recent block at startup.
	WaitNodeSync bool

	// How many blocks back to look for recent batcher transactions during node sync at startup.
	// If 0, the batcher will just use the current head.
	CheckRecentTxsDepth int

	BatchType uint

	// DataAvailabilityType is one of the values defined in op-batcher/flags/types.go and dictates
	// the data availability type to use for posting batches, e.g. blobs vs calldata, or auto
	// for choosing the most economic type dynamically at the start of each channel.
	DataAvailabilityType flags.DataAvailabilityType

	// ActiveSequencerCheckDuration is the duration between checks to determine the active sequencer endpoint.
	ActiveSequencerCheckDuration time.Duration

	// ThrottleInterval is the interval between notifying the block builder of the latest DA throttling state, or 0 to
	// disable notifications entirely (only recommended for testing).
	ThrottleInterval time.Duration
	// ThrottleThreshold is the number of pending bytes beyond which the batcher will start throttling future bytes
	// written to DA.
	ThrottleThreshold uint64
	// ThrottleTxSize is the DA size of a transaction to start throttling when we are over the throttling threshold.
	ThrottleTxSize uint64
	// ThrottleBlockSize is the total per-block DA limit to start imposing on block building when we are over the throttling threshold.
	ThrottleBlockSize uint64
	// ThrottleAlwaysBlockSize is the total per-block DA limit to always imposing on block building.
	ThrottleAlwaysBlockSize uint64

	// TestUseMaxTxSizeForBlobs allows to set the blob size with MaxL1TxSize.
	// Should only be used for testing purposes.
	TestUseMaxTxSizeForBlobs bool

	TxMgrConfig   txmgr.CLIConfig
	LogConfig     oplog.CLIConfig
	MetricsConfig opmetrics.CLIConfig
	PprofConfig   oppprof.CLIConfig
	RPC           oprpc.CLIConfig
	AltDA         altda.CLIConfig
}

func (c *CLIConfig) Check() error {
	if c.L1EthRpc == "" {
		return errors.New("empty L1 RPC URL")
	}
	if c.L2EthRpc == "" {
		return errors.New("empty L2 RPC URL")
	}
	if c.RollupRpc == "" {
		return errors.New("empty rollup RPC URL")
	}
	if strings.Count(c.RollupRpc, ",") != strings.Count(c.L2EthRpc, ",") {
		return errors.New("number of rollup and eth URLs must match")
	}
	if c.PollInterval == 0 {
		return errors.New("must set PollInterval")
	}
	if c.MaxL1TxSize <= 1 {
		return errors.New("MaxL1TxSize must be greater than 1")
	}
	if c.TargetNumFrames < 1 {
		return errors.New("TargetNumFrames must be at least 1")
	}
	if c.Compressor == compressor.RatioKind && (c.ApproxComprRatio <= 0 || c.ApproxComprRatio > 1) {
		return fmt.Errorf("invalid ApproxComprRatio %v for ratio compressor", c.ApproxComprRatio)
	}
	if !derive.ValidCompressionAlgo(c.CompressionAlgo) {
		return fmt.Errorf("invalid compression algo %v", c.CompressionAlgo)
	}
	if c.BatchType > derive.SpanBatchType {
		return fmt.Errorf("unknown batch type: %v", c.BatchType)
	}
	if c.CheckRecentTxsDepth > 128 {
		return fmt.Errorf("CheckRecentTxsDepth cannot be set higher than 128: %v", c.CheckRecentTxsDepth)
	}
	if !flags.ValidDataAvailabilityType(c.DataAvailabilityType) {
		return fmt.Errorf("unknown data availability type: %q", c.DataAvailabilityType)
	}
	// we want to enforce it for both blobs and auto
	if c.DataAvailabilityType != flags.CalldataType && c.TargetNumFrames > eth.MaxBlobsPerBlobTx {
		return fmt.Errorf("too many frames for blob transactions, max %d", eth.MaxBlobsPerBlobTx)
	}
	if err := c.MetricsConfig.Check(); err != nil {
		return err
	}
	if err := c.PprofConfig.Check(); err != nil {
		return err
	}
	if err := c.TxMgrConfig.Check(); err != nil {
		return err
	}
	if err := c.RPC.Check(); err != nil {
		return err
	}
	return nil
}

// NewConfig parses the Config from the provided flags or environment variables.
func NewConfig(ctx *cli.Context) *CLIConfig {
	return &CLIConfig{
		/* Required Flags */
		L1EthRpc:        ctx.String(flags.L1EthRpcFlag.Name),
		L2EthRpc:        ctx.String(flags.L2EthRpcFlag.Name),
		RollupRpc:       ctx.String(flags.RollupRpcFlag.Name),
		SubSafetyMargin: ctx.Uint64(flags.SubSafetyMarginFlag.Name),
		PollInterval:    ctx.Duration(flags.PollIntervalFlag.Name),

		/* Optional Flags */
		MaxPendingTransactions:       ctx.Uint64(flags.MaxPendingTransactionsFlag.Name),
		MaxChannelDuration:           ctx.Uint64(flags.MaxChannelDurationFlag.Name),
		MaxL1TxSize:                  ctx.Uint64(flags.MaxL1TxSizeBytesFlag.Name),
		MaxBlocksPerSpanBatch:        ctx.Int(flags.MaxBlocksPerSpanBatch.Name),
		TargetNumFrames:              ctx.Int(flags.TargetNumFramesFlag.Name),
		ApproxComprRatio:             ctx.Float64(flags.ApproxComprRatioFlag.Name),
		Compressor:                   ctx.String(flags.CompressorFlag.Name),
		CompressionAlgo:              derive.CompressionAlgo(ctx.String(flags.CompressionAlgoFlag.Name)),
		Stopped:                      ctx.Bool(flags.StoppedFlag.Name),
		WaitNodeSync:                 ctx.Bool(flags.WaitNodeSyncFlag.Name),
		CheckRecentTxsDepth:          ctx.Int(flags.CheckRecentTxsDepthFlag.Name),
		BatchType:                    ctx.Uint(flags.BatchTypeFlag.Name),
		DataAvailabilityType:         flags.DataAvailabilityType(ctx.String(flags.DataAvailabilityTypeFlag.Name)),
		ActiveSequencerCheckDuration: ctx.Duration(flags.ActiveSequencerCheckDurationFlag.Name),
		TxMgrConfig:                  txmgr.ReadCLIConfig(ctx),
		LogConfig:                    oplog.ReadCLIConfig(ctx),
		MetricsConfig:                opmetrics.ReadCLIConfig(ctx),
		PprofConfig:                  oppprof.ReadCLIConfig(ctx),
		RPC:                          oprpc.ReadCLIConfig(ctx),
		AltDA:                        altda.ReadCLIConfig(ctx),
		ThrottleThreshold:            ctx.Uint64(flags.ThrottleThresholdFlag.Name),
		ThrottleInterval:             ctx.Duration(flags.ThrottleIntervalFlag.Name),
		ThrottleTxSize:               ctx.Uint64(flags.ThrottleTxSizeFlag.Name),
		ThrottleBlockSize:            ctx.Uint64(flags.ThrottleBlockSizeFlag.Name),
		ThrottleAlwaysBlockSize:      ctx.Uint64(flags.ThrottleAlwaysBlockSizeFlag.Name),
	}
}
