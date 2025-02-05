package batcher_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-batcher/batcher"
	"github.com/ethereum-optimism/optimism/op-batcher/compressor"
	"github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/stretchr/testify/require"
)

func validBatcherConfig() batcher.CLIConfig {
	return batcher.CLIConfig{
		L1EthRpc:               "fake",
		L2EthRpc:               "fake",
		RollupRpc:              "fake",
		MaxChannelDuration:     0,
		SubSafetyMargin:        0,
		PollInterval:           time.Second,
		MaxPendingTransactions: 0,
		MaxL1TxSize:            10,
		TargetNumFrames:        1,
		Compressor:             "shadow",
		Stopped:                false,
		BatchType:              0,
		DataAvailabilityType:   flags.CalldataType,
		TxMgrConfig:            txmgr.NewCLIConfig("fake", txmgr.DefaultBatcherFlagValues),
		LogConfig:              log.DefaultCLIConfig(),
		MetricsConfig:          metrics.DefaultCLIConfig(),
		PprofConfig:            oppprof.DefaultCLIConfig(),
		// The compressor config is not checked in config.Check()
		RPC:               rpc.DefaultCLIConfig(),
		CompressionAlgo:   derive.Zlib,
		ThrottleThreshold: 0, // no DA throttling
		ThrottleInterval:  12 * time.Second,
		ThrottleTxSize:    0,
	}
}

func TestValidBatcherConfig(t *testing.T) {
	cfg := validBatcherConfig()
	require.NoError(t, cfg.Check(), "valid config should pass the check function")
}

func TestBatcherConfig(t *testing.T) {
	tests := []struct {
		name      string
		override  func(*batcher.CLIConfig)
		errString string
	}{
		{
			name:      "empty L1",
			override:  func(c *batcher.CLIConfig) { c.L1EthRpc = "" },
			errString: "empty L1 RPC URL",
		},
		{
			name:      "empty L2",
			override:  func(c *batcher.CLIConfig) { c.L2EthRpc = "" },
			errString: "empty L2 RPC URL",
		},
		{
			name:      "empty rollup",
			override:  func(c *batcher.CLIConfig) { c.RollupRpc = "" },
			errString: "empty rollup RPC URL",
		},
		{
			name:      "empty poll interval",
			override:  func(c *batcher.CLIConfig) { c.PollInterval = 0 },
			errString: "must set PollInterval",
		},
		{
			name:      "max L1 tx size too small",
			override:  func(c *batcher.CLIConfig) { c.MaxL1TxSize = 0 },
			errString: "MaxL1TxSize must be greater than 1",
		},
		{
			name:      "invalid batch type close",
			override:  func(c *batcher.CLIConfig) { c.BatchType = 2 },
			errString: "unknown batch type: 2",
		},
		{
			name:      "invalid batch type far",
			override:  func(c *batcher.CLIConfig) { c.BatchType = 100 },
			errString: "unknown batch type: 100",
		},
		{
			name:      "invalid batch submission policy",
			override:  func(c *batcher.CLIConfig) { c.DataAvailabilityType = "foo" },
			errString: "unknown data availability type: \"foo\"",
		},
		{
			name:      "zero TargetNumFrames",
			override:  func(c *batcher.CLIConfig) { c.TargetNumFrames = 0 },
			errString: "TargetNumFrames must be at least 1",
		},
		{
			name: fmt.Sprintf("larger %d TargetNumFrames for blobs", eth.MaxBlobsPerBlobTx),
			override: func(c *batcher.CLIConfig) {
				c.TargetNumFrames = eth.MaxBlobsPerBlobTx + 1
				c.DataAvailabilityType = flags.BlobsType
			},
			errString: fmt.Sprintf("too many frames for blob transactions, max %d", eth.MaxBlobsPerBlobTx),
		},
		{
			name: "invalid compr ratio for ratio compressor",
			override: func(c *batcher.CLIConfig) {
				c.ApproxComprRatio = 4.2
				c.Compressor = compressor.RatioKind
			},
			errString: "invalid ApproxComprRatio 4.2 for ratio compressor",
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBatcherConfig()
			tc.override(&cfg)
			require.ErrorContains(t, cfg.Check(), tc.errString)
		})
	}
}
