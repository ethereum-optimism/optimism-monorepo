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
	}
	blobCfg := ChannelConfig{
		MaxFrameSize:    eth.MaxBlobDataSize - 1,
		TargetNumFrames: 3, // gets closest to amortized fixed tx costs
		UseBlobs:        true,
	}

	tests := []struct {
		name         string
		tipCap       int64
		baseFee      int64
		blobBaseFee  int64
		wantCalldata bool
		isL1Pectra   bool
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
		{
			name:        "much-cheaper-blobs-l1-pectra",
			tipCap:      1e3,
			baseFee:     1e6,
			blobBaseFee: 1,
			isL1Pectra:  true,
		},
		{
			name:        "close-cheaper-blobs-l1-pectra",
			tipCap:      1e3,
			baseFee:     1e6,
			blobBaseFee: 398e5, // this value just under the equilibrium point for 3 blobs
			isL1Pectra:  true,
		},
		{
			name:         "close-cheaper-calldata-l1-pectra",
			tipCap:       1e3,
			baseFee:      1e6,
			blobBaseFee:  399e5, // this value just over the equilibrium point for 3 blobs
			wantCalldata: true,
			isL1Pectra:   true,
		},
		{
			name:         "much-cheaper-calldata-l1-pectra",
			tipCap:       1e3,
			baseFee:      1e6,
			blobBaseFee:  1e9,
			wantCalldata: true,
			isL1Pectra:   true,
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
			cc := dec.ChannelConfig(tt.isL1Pectra)
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
		require.Equal(t, dec.ChannelConfig(false), blobCfg)
		require.NotNil(t, ch.FindLog(
			testlog.NewLevelFilter(slog.LevelWarn),
			testlog.NewMessageContainsFilter("returning last config"),
		))

		gp.err = nil
		require.Equal(t, dec.ChannelConfig(false), calldataCfg)
		require.NotNil(t, ch.FindLog(
			testlog.NewLevelFilter(slog.LevelInfo),
			testlog.NewMessageContainsFilter("calldata"),
		))

		gp.err = errors.New("gp-error-2")
		require.Equal(t, dec.ChannelConfig(false), calldataCfg)
		require.NotNil(t, ch.FindLog(
			testlog.NewLevelFilter(slog.LevelWarn),
			testlog.NewMessageContainsFilter("returning last config"),
		))
	})
}

func TestComputeSingleCalldataTxCost(t *testing.T) {
	// 30KB of data
	got := computeSingleCalldataTxCost(120_000, big.NewInt(1), big.NewInt(1), false)
	require.Equal(t, big.NewInt(1_002_000), got) // (21_000 + 4*120_000) * (1+1)

	got = computeSingleCalldataTxCost(120_000, big.NewInt(1), big.NewInt(1), true)
	require.Equal(t, big.NewInt(2_442_000), got) // (21_000 + 10*120_000) * (1+1)
}

func TestComputeSingleBlobTxCost(t *testing.T) {
	// This tx submits 655KB of data (21x the calldata example above)
	// Setting blobBaseFee to 16x (baseFee + tipCap) gives a cost which is ~21x higher
	// than the calldata example, showing the rough equilibrium point
	// of the two DA markets.
	got := computeSingleBlobTxCost(5, big.NewInt(1), big.NewInt(1), big.NewInt(32))
	require.Equal(t, big.NewInt(21_013_520), got) // 21_000 * (1+1) + 131_072*5*32
}
