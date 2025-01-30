package fees

import (
	"context"
	"math"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	legacybindings "github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/stretchr/testify/require"
)

// TestGasPriceOracleFeeUpdates checks that the gas price oracle cannot be locked by mis-configuring parameters.
func TestGasPriceOracleFeeUpdates(t *testing.T) {
	op_e2e.InitParallel(t)

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	maxScalars := eth.EcotoneScalars{
		BaseFeeScalar:     math.MaxUint32,
		BlobBaseFeeScalar: math.MaxUint32,
	}
	var cancel context.CancelFunc

	// Create our system configuration for L1/L2 and start it
	cfg := e2esys.DefaultSystemConfig(t)
	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")

	// Obtain our sequencer, verifier, and transactor keypair.
	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")
	// l2Verif := sys.NodeClient("verifier")
	ethPrivKey := cfg.Secrets.Deployer

	// Bind to the SystemConfig & GasPriceOracle contracts
	sysconfig, err := legacybindings.NewSystemConfig(cfg.L1Deployments.SystemConfigProxy, l1Client)
	require.NoError(t, err)
	gpoContract, err := legacybindings.NewGasPriceOracleCaller(predeploys.GasPriceOracleAddr, l2Seq)
	require.NoError(t, err)

	// Obtain our signer.
	opts, err := bind.NewKeyedTransactorWithChainID(ethPrivKey, cfg.L1ChainIDBig())
	require.NoError(t, err)

	// Define our L1 transaction timeout duration.
	txTimeoutDuration := 10 * time.Duration(cfg.DeployConfig.L1BlockTime) * time.Second

	// Update the gas config, wait for it to show up on L2, & verify that it was set as intended
	opts.Context, cancel = context.WithTimeout(ctx, txTimeoutDuration)
	tx, err := sysconfig.SetGasConfigEcotone(opts, maxScalars.BaseFeeScalar, maxScalars.BlobBaseFeeScalar)
	cancel()
	require.NoError(t, err, "SetGasConfigEcotone update tx")

	receipt, err := wait.ForReceiptOK(ctx, l1Client, tx.Hash())
	require.NoError(t, err, "Waiting for sysconfig set gas config update tx")

	_, err = geth.WaitForL1OriginOnL2(sys.RollupConfig, receipt.BlockNumber.Uint64(), l2Seq, txTimeoutDuration)
	require.NoError(t, err, "waiting for L2 block to include the sysconfig update")

	baseFeeScalar, err := gpoContract.BaseFeeScalar(&bind.CallOpts{})
	require.NoError(t, err, "reading base fee scalar")
	require.Equal(t, baseFeeScalar, maxScalars.BaseFeeScalar)

	blobBaseFeeScalar, err := gpoContract.BlobBaseFeeScalar(&bind.CallOpts{})
	require.NoError(t, err, "reading blob base fee scalar")
	require.Equal(t, blobBaseFeeScalar, maxScalars.BlobBaseFeeScalar)

	// Now modify the scalar value & ensure that the gas params can be modified
	normalScalars := eth.EcotoneScalars{
		BaseFeeScalar:     1e6,
		BlobBaseFeeScalar: 1e6,
	}

	opts.Context, cancel = context.WithTimeout(context.Background(), txTimeoutDuration)
	tx, err = sysconfig.SetGasConfigEcotone(opts, normalScalars.BaseFeeScalar, normalScalars.BlobBaseFeeScalar)
	cancel()
	require.NoError(t, err, "SetGasConfigEcotone update tx")

	receipt, err = wait.ForReceiptOK(ctx, l1Client, tx.Hash())
	require.NoError(t, err, "Waiting for sysconfig set gas config update tx")

	_, err = geth.WaitForL1OriginOnL2(sys.RollupConfig, receipt.BlockNumber.Uint64(), l2Seq, txTimeoutDuration)
	require.NoError(t, err, "waiting for L2 block to include the sysconfig update")

	baseFeeScalar, err = gpoContract.BaseFeeScalar(&bind.CallOpts{})
	require.NoError(t, err, "reading base fee scalar")
	require.Equal(t, baseFeeScalar, normalScalars.BaseFeeScalar)

	blobBaseFeeScalar, err = gpoContract.BlobBaseFeeScalar(&bind.CallOpts{})
	require.NoError(t, err, "reading blob base fee scalar")
	require.Equal(t, blobBaseFeeScalar, normalScalars.BlobBaseFeeScalar)
}
