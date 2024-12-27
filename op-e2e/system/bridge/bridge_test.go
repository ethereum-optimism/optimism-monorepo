package bridge

import (
	"context"
	"math"
	"math/big"
	"testing"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"

	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/receipts"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	op_e2e.RunMain(m)
}

// TestERC20BridgeDeposits tests the the L1StandardBridge bridge ERC20
// functionality.
func TestERC20BridgeDeposits(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	log := testlog.Logger(t, log.LevelInfo)
	log.Info("genesis", "l2", sys.RollupConfig.Genesis.L2, "l1", sys.RollupConfig.Genesis.L1, "l2_time", sys.RollupConfig.Genesis.L2Time)

	l1Client := sys.NodeClient("l1")
	l2Client := sys.NodeClient("sequencer")

	opts, err := bind.NewKeyedTransactorWithChainID(sys.Cfg.Secrets.Alice, cfg.L1ChainIDBig())
	require.Nil(t, err)

	// Deploy WETH
	wethAddress, tx, WETH, err := bindings.DeployWETH(opts, l1Client)
	require.NoError(t, err)
	_, err = wait.ForReceiptOK(context.Background(), l1Client, tx.Hash())
	require.NoError(t, err, "Waiting for deposit tx on L1")

	// Get some WETH
	opts.Value = big.NewInt(params.Ether)
	tx, err = WETH.Deposit(opts)
	require.NoError(t, err)
	_, err = wait.ForReceiptOK(context.Background(), l1Client, tx.Hash())
	require.NoError(t, err)
	opts.Value = nil
	wethBalance, err := WETH.BalanceOf(&bind.CallOpts{}, opts.From)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(params.Ether), wethBalance)

	// Deploy L2 WETH
	l2Opts, err := bind.NewKeyedTransactorWithChainID(sys.Cfg.Secrets.Alice, cfg.L2ChainIDBig())
	require.NoError(t, err)
	optimismMintableTokenFactory, err := bindings.NewOptimismMintableERC20Factory(predeploys.OptimismMintableERC20FactoryAddr, l2Client)
	require.NoError(t, err)
	tx, err = optimismMintableTokenFactory.CreateOptimismMintableERC20(l2Opts, wethAddress, "L2-WETH", "L2-WETH")
	require.NoError(t, err)
	rcpt, err := wait.ForReceiptOK(context.Background(), l2Client, tx.Hash())
	require.NoError(t, err)

	event, err := receipts.FindLog(rcpt.Logs, optimismMintableTokenFactory.ParseOptimismMintableERC20Created)
	require.NoError(t, err, "Should emit ERC20Created event")

	// Approve WETH with the bridge
	tx, err = WETH.Approve(opts, cfg.L1Deployments.L1StandardBridgeProxy, new(big.Int).SetUint64(math.MaxUint64))
	require.NoError(t, err)
	_, err = wait.ForReceiptOK(context.Background(), l1Client, tx.Hash())
	require.NoError(t, err)

	// Bridge the WETH
	l1StandardBridge, err := bindings.NewL1StandardBridge(cfg.L1Deployments.L1StandardBridgeProxy, l1Client)
	require.NoError(t, err)
	tx, err = transactions.PadGasEstimate(opts, 1.1, func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return l1StandardBridge.BridgeERC20(opts, wethAddress, event.LocalToken, big.NewInt(100), 100000, []byte{})
	})
	require.NoError(t, err)
	depositReceipt, err := wait.ForReceiptOK(context.Background(), l1Client, tx.Hash())
	require.NoError(t, err)

	t.Log("Deposit through L1StandardBridge", "gas used", depositReceipt.GasUsed)

	// compute the deposit transaction hash + poll for it
	portal, err := bindings.NewOptimismPortal(cfg.L1Deployments.OptimismPortalProxy, l1Client)
	require.NoError(t, err)

	depositEvent, err := receipts.FindLog(depositReceipt.Logs, portal.ParseTransactionDeposited)
	require.NoError(t, err, "Should emit deposit event")

	depositTx, err := derive.UnmarshalDepositLogEventIgnoreNonce(&depositEvent.Raw)
	require.NoError(t, err)
	_, err = wait.ForReceiptOK(context.Background(), l2Client, types.NewTx(depositTx).Hash())
	require.NoError(t, err)

	// Ensure that the deposit went through
	optimismMintableToken, err := bindings.NewOptimismMintableERC20(event.LocalToken, l2Client)
	require.NoError(t, err)

	// Should have balance on L2
	l2Balance, err := optimismMintableToken.BalanceOf(&bind.CallOpts{}, opts.From)
	require.NoError(t, err)
	require.Equal(t, l2Balance, big.NewInt(100))
}
