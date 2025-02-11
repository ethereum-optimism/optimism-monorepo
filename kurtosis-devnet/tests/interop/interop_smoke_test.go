package interop

import (
	"context"
	"log/slog"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func smokeTestScenario(chainIdx uint64, userSentinel interface{}) systest.SystemTestFunc {
	return func(t systest.T, sys system.System) {
		ctx := t.Context()
		logger := slog.With("test", "TestMinimal", "devnet", sys.Identifier())

		chain := sys.L2(chainIdx)
		logger = logger.With("chain", chain.ID())
		logger.InfoContext(ctx, "starting test")

		funds := types.NewBalance(big.NewInt(0.5 * constants.ETH))
		user := ctx.Value(userSentinel).(types.Wallet)

		scw0Addr := constants.SuperchainWETH
		scw0, err := chain.ContractsRegistry().SuperchainWETH(scw0Addr)
		require.NoError(t, err)
		logger.InfoContext(ctx, "using SuperchainWETH", "contract", scw0Addr)

		initialBalance, err := scw0.BalanceOf(user.Address()).Call(ctx)
		require.NoError(t, err)
		logger = logger.With("user", user.Address())
		logger.InfoContext(ctx, "initial balance retrieved", "balance", initialBalance)

		logger.InfoContext(ctx, "sending ETH to contract", "amount", funds)
		require.NoError(t, user.SendETH(scw0Addr, funds).Send(ctx).Wait())

		balance, err := scw0.BalanceOf(user.Address()).Call(ctx)
		require.NoError(t, err)
		logger.InfoContext(ctx, "final balance retrieved", "balance", balance)

		require.Equal(t, balance, initialBalance.Add(funds))
	}
}

func TestSystemWrapETH(t *testing.T) {
	chainIdx := uint64(0)         // We'll use the first L2 chain for this test
	testUserMarker := &struct{}{} // Sentinel for the user context value

	systest.SystemTest(t,
		smokeTestScenario(chainIdx, testUserMarker),
		walletFundsValidator(chainIdx, types.NewBalance(big.NewInt(1.0*constants.ETH)), testUserMarker),
	)
}

func TestInteropSystemNoop(t *testing.T) {
	systest.InteropSystemTest(t, func(t systest.T, sys system.InteropSystem) {
		slog.Info("noop")
	})
}

func TestSmokeTestFailure(t *testing.T) {
	// Create mock failing system
	mockAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	mockWallet := &mockFailingWallet{
		addr: mockAddr,
		key:  "mock-key",
		bal:  types.NewBalance(big.NewInt(1000000)),
	}
	mockChain := &mockFailingChain{
		id:     types.ChainID(big.NewInt(1234)),
		wallet: mockWallet,
		reg:    &mockRegistry{},
	}
	mockSys := &mockFailingSystem{chain: mockChain}

	// Run the smoke test logic and capture failures
	sentinel := &struct{}{}
	rt := NewRecordingT(context.WithValue(context.TODO(), sentinel, mockWallet))
	rt.TestScenario(
		smokeTestScenario(0, sentinel),
		mockSys,
	)

	// Verify that the test failed due to SendETH error
	require.True(t, rt.Failed(), "test should have failed")
	require.Contains(t, rt.Logs(), "transaction failure", "unexpected failure message")
}
