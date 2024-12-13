package batcher

import (
	"context"
	"errors"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/stretchr/testify/require"
)

type mockGasPricer struct {
	err         error
	tipCap      int64
	baseFee     int64
	blobBaseFee int64
}

func (gp *mockGasPricer) SuggestGasPriceCaps(context.Context) (tipCap *big.Int, baseFee *big.Int, blobBaseFee *big.Int, err error) {
	if gp.err != nil {
		return nil, nil, nil, gp.err
	}
	return big.NewInt(gp.tipCap), big.NewInt(gp.baseFee), big.NewInt(gp.blobBaseFee), nil
}

func TestDynamicEthChannelConfig_ChannelConfig(t *testing.T) {
	calldataCfg := ChannelConfig{
		MaxFrameSize:    120_000 - 1,
		TargetNumFrames: 1,
		DaType:          DaTypeCalldata,
	}
	blobCfg := ChannelConfig{
		MaxFrameSize:    eth.MaxBlobDataSize - 1,
		TargetNumFrames: 3, // gets closest to amortized fixed tx costs
		DaType:          DaTypeBlob,
	}

	tests := []struct {
		name         string
		tipCap       int64
		baseFee      int64
		blobBaseFee  int64
		wantCalldata bool
	}{
		{
			name:        "much-cheaper-blobs",
			tipCap:      1e3,
			baseFee:     1e6,
			blobBaseFee: 1,
		},
		{
			name:        "close-cheaper-blobs",
			tipCap:      1e3,
			baseFee:     1e6,
			blobBaseFee: 16e6, // because of amortized fixed 21000 tx cost, blobs are still cheaper here...
		},
		{
			name:         "close-cheaper-calldata",
			tipCap:       1e3,
			baseFee:      1e6,
			blobBaseFee:  161e5, // ...but then increasing the fee just a tiny bit makes blobs more expensive
			wantCalldata: true,
		},
		{
			name:         "much-cheaper-calldata",
			tipCap:       1e3,
			baseFee:      1e6,
			blobBaseFee:  1e9,
			wantCalldata: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lgr, ch := testlog.CaptureLogger(t, slog.LevelInfo)
			gp := &mockGasPricer{
				tipCap:      tt.tipCap,
				baseFee:     tt.baseFee,
				blobBaseFee: tt.blobBaseFee,
			}
			dec := NewDynamicEthChannelConfig(lgr, 1*time.Second, gp, blobCfg, calldataCfg)
			cc := dec.ChannelConfig()
			if tt.wantCalldata {
				require.Equal(t, cc, calldataCfg)
				require.NotNil(t, ch.FindLog(testlog.NewMessageContainsFilter("calldata")))
				require.Same(t, &dec.calldataConfig, dec.lastConfig)
			} else {
				require.Equal(t, cc, blobCfg)
				require.NotNil(t, ch.FindLog(testlog.NewMessageContainsFilter("blob")))
				require.Same(t, &dec.blobConfig, dec.lastConfig)
			}
		})
	}

	t.Run("error-latest", func(t *testing.T) {
		lgr, ch := testlog.CaptureLogger(t, slog.LevelInfo)
		gp := &mockGasPricer{
			tipCap:      1,
			baseFee:     1e3,
			blobBaseFee: 1e6, // should return calldata cfg without error
			err:         errors.New("gp-error"),
		}
		dec := NewDynamicEthChannelConfig(lgr, 1*time.Second, gp, blobCfg, calldataCfg)
		require.Equal(t, dec.ChannelConfig(), blobCfg)
		require.NotNil(t, ch.FindLog(
			testlog.NewLevelFilter(slog.LevelWarn),
			testlog.NewMessageContainsFilter("returning last config"),
		))

		gp.err = nil
		require.Equal(t, dec.ChannelConfig(), calldataCfg)
		require.NotNil(t, ch.FindLog(
			testlog.NewLevelFilter(slog.LevelInfo),
			testlog.NewMessageContainsFilter("calldata"),
		))

		gp.err = errors.New("gp-error-2")
		require.Equal(t, dec.ChannelConfig(), calldataCfg)
		require.NotNil(t, ch.FindLog(
			testlog.NewLevelFilter(slog.LevelWarn),
			testlog.NewMessageContainsFilter("returning last config"),
		))
	})
}
