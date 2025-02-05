package bridge

import (
	"testing"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/stretchr/testify/require"
)

func TestWithdrawals_L2OO(t *testing.T) {
	testWithdrawals(t, config.AllocTypeL2OO)
}

func TestWithdrawals_Standard(t *testing.T) {
	testWithdrawals(t, config.AllocTypeStandard)
}

// testWithdrawals checks that a deposit and then withdrawal execution succeeds. It verifies the
// balance changes on L1 and L2 and has to include gas fees in the balance checks.
// It does not check that the withdrawal can be executed prior to the end of the finality period.
func testWithdrawals(t *testing.T, allocType config.AllocType) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t, e2esys.WithAllocType(allocType))
	cfg.DeployConfig.FinalizationPeriodSeconds = 2 // 2s finalization period
	cfg.L1FinalizedDistance = 2                    // Finalize quick, don't make the proposer wait too long

	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")

	RunWithdrawalsTest(t, sys)
}
