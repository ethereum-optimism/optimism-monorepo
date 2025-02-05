package fjord

import (
	"context"
	"fmt"
	"testing"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/stretchr/testify/require"

	fjordChecks "github.com/ethereum-optimism/optimism/op-chain-ops/cmd/check-fjord/checks"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

// TestCheckFjordScript ensures the op-chain-ops/cmd/check-fjord script runs successfully
// against a test chain with the fjord hardfork activated/unactivated
func TestCheckFjordScript(t *testing.T) {
	op_e2e.InitParallel(t)
	genesisActivation := hexutil.Uint64(0)
	tests := []struct {
		fjord bool
	}{
		{fjord: true},
		{fjord: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("fjord=%t", tt.fjord), func(t *testing.T) {
			t.Parallel()

			log := testlog.Logger(t, log.LevelInfo)
			cfg := e2esys.EcotoneSystemConfig(t, &genesisActivation)
			if tt.fjord {
				cfg.DeployConfig.L2GenesisFjordTimeOffset = ptr(hexutil.Uint64(cfg.DeployConfig.L2BlockTime))
			} else {
				cfg.DeployConfig.L2GenesisFjordTimeOffset = nil
			}

			sys, err := cfg.Start(t)
			require.NoError(t, err, "Error starting up system")

			require.NoError(t, wait.ForNextBlock(context.Background(), sys.NodeClient(e2esys.RoleSeq)))

			checkFjordConfig := &fjordChecks.CheckFjordConfig{
				Log:  log,
				L2:   sys.NodeClient(e2esys.RoleSeq),
				Key:  sys.Cfg.Secrets.Alice,
				Addr: sys.Cfg.Secrets.Addresses().Alice,
			}
			if !tt.fjord {
				err = fjordChecks.CheckRIP7212(context.Background(), checkFjordConfig)
				require.Error(t, err, "expected error for CheckRIP7212")
				err = fjordChecks.CheckGasPriceOracle(context.Background(), checkFjordConfig)
				require.Error(t, err, "expected error for CheckGasPriceOracle")
				err = fjordChecks.CheckTxEmpty(context.Background(), checkFjordConfig)
				require.Error(t, err, "expected error for CheckTxEmpty")
				err = fjordChecks.CheckTxAllZero(context.Background(), checkFjordConfig)
				require.Error(t, err, "expected error for CheckTxAllZero")
				err = fjordChecks.CheckTxAll42(context.Background(), checkFjordConfig)
				require.Error(t, err, "expected error for CheckTxAll42")
				err = fjordChecks.CheckTxRandom(context.Background(), checkFjordConfig)
				require.Error(t, err, "expected error for CheckTxRandom")
			} else {
				err = fjordChecks.CheckAll(context.Background(), checkFjordConfig)
				require.NoError(t, err, "should not error on CheckAll")
			}
		})
	}
}

func ptr[T any](t T) *T { return &t }
