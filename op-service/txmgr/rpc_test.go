package txmgr

import (
	"fmt"
	"math/big"
	"testing"

	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestTxmgrRPC(t *testing.T) {
	minBaseFeeInit := big.NewInt(1000)
	minPriorityFeeInit := big.NewInt(2000)
	minBlobFeeInit := big.NewInt(3000)
	feeThresholdInit := big.NewInt(4000)
	bumpFeeRetryTimeInit := int64(100)

	cfg := Config{}
	cfg.MinBaseFee.Store(minBaseFeeInit)
	cfg.MinTipCap.Store(minPriorityFeeInit)
	cfg.MinBlobTxFee.Store(minBlobFeeInit)
	cfg.FeeLimitThreshold.Store(feeThresholdInit)
	cfg.ResubmissionTimeout.Store(bumpFeeRetryTimeInit)

	h := newTestHarnessWithConfig(t, &cfg)

	appVersion := "test"
	server := oprpc.NewServer(
		"127.0.0.1",
		0,
		appVersion,
	)
	server.AddAPI(h.mgr.API())
	require.NoError(t, server.Start())
	defer func() {
		_ = server.Stop()
	}()

	rpcClient, err := rpc.Dial(fmt.Sprintf("http://%s", server.Endpoint()))
	require.NoError(t, err)

	type tcase struct {
		rpcMethod string
		initValue *big.Int
	}

	cases := []tcase{
		{"MinBaseFee", minBaseFeeInit},
		{"MinPriorityFee", minPriorityFeeInit},
		{"MinBlobFee", minBlobFeeInit},
		{"FeeThreshold", feeThresholdInit},
		{"BumpFeeRetryTime", big.NewInt(bumpFeeRetryTimeInit)},
	}

	for _, tc := range cases {
		t.Run("Get|Set"+tc.rpcMethod, func(t *testing.T) {
			var res *big.Int

			require.NoError(t, rpcClient.Call(&res, "txmgr_get"+tc.rpcMethod))
			require.Equal(t, tc.initValue, res)

			newVal := new(big.Int)
			newVal.Add(tc.initValue, big.NewInt(1))

			require.NoError(t, rpcClient.Call(&res, "txmgr_set"+tc.rpcMethod, newVal))
			require.NoError(t, rpcClient.Call(&res, "txmgr_get"+tc.rpcMethod))
			require.Equal(t, newVal, res)
		})
	}
}
