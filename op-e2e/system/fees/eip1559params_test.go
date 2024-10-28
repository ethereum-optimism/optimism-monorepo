package fees

import (
	"context"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	legacybindings "github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/stretchr/testify/require"
)

// TestEIP1599Params checks that we can successfully change EIP-1559 parameters via SysConfig with
// the Holocene upgrade.
func TestEIP1559Params(t *testing.T) {
	op_e2e.InitParallel(t)

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	// Create our system configuration for L1/L2 and start it
	cfg := e2esys.HoloceneSystemConfig(t, new(hexutil.Uint64))
	cfg.DeployConfig.L2GenesisBlockBaseFeePerGas = (*hexutil.Big)(big.NewInt(100_000_000))
	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")

	// Obtain our sequencer, verifier, and transactor keypair.
	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")
	ethPrivKey := cfg.Secrets.SysCfgOwner

	_, err = l2Seq.HeaderByNumber(context.Background(), big.NewInt(0))
	require.NoError(t, err)

	// Bind to the SystemConfig contract
	sysconfig, err := legacybindings.NewSystemConfig(cfg.L1Deployments.SystemConfigProxy, l1Client)
	require.NoError(t, err)

	// Obtain our signer.
	opts, err := bind.NewKeyedTransactorWithChainID(ethPrivKey, cfg.L1ChainIDBig())
	require.NoError(t, err)

	// Define our L1 transaction timeout duration.
	txTimeoutDuration := 10 * time.Duration(cfg.DeployConfig.L1BlockTime) * time.Second

	var cancel context.CancelFunc

	// confirm eip-1559 parameters are initialized to 0
	denom, err := sysconfig.Eip1559Denominator(&bind.CallOpts{})
	require.NoError(t, err, "reading denominator")
	require.Equal(t, uint32(0), denom)

	elasticity, err := sysconfig.Eip1559Elasticity(&bind.CallOpts{})
	require.NoError(t, err, "reading elasticity")
	require.Equal(t, uint32(0), elasticity)

	// update the EIP-1559 params, wait for it to show up on L2, & verify that it was set as intended
	expectedDenom := uint32(10)
	expectedElasticity := uint32(2) // implies gas target will be 15M since block limit is 30M
	const gasTarget = 15_000_000
	opts.Context, cancel = context.WithTimeout(ctx, txTimeoutDuration)
	tx, err := sysconfig.SetEIP1559Params(opts, expectedDenom, expectedElasticity)
	cancel()
	require.NoError(t, err, "SetEIP1559Params update tx")

	receipt, err := wait.ForReceiptOK(ctx, l1Client, tx.Hash())
	require.NoError(t, err, "Waiting for sysconfig set gas config update tx")

	denom, err = sysconfig.Eip1559Denominator(&bind.CallOpts{})
	require.NoError(t, err, "reading denominator")
	require.Equal(t, expectedDenom, denom)

	elasticity, err = sysconfig.Eip1559Elasticity(&bind.CallOpts{})
	require.NoError(t, err, "reading elasticity")
	require.Equal(t, expectedElasticity, elasticity)

	_, err = geth.WaitForL1OriginOnL2(sys.RollupConfig, receipt.BlockNumber.Uint64(), l2Seq, txTimeoutDuration)
	require.NoError(t, err, "waiting for L2 block to include the sysconfig update")

	h, err := l2Seq.HeaderByNumber(context.Background(), nil)
	require.NoError(t, err)

	// confirm the extraData is being set as expected
	require.Equal(t, eip1559.EncodeHoloceneExtraData(uint64(expectedDenom), uint64(expectedElasticity)), h.Extra)

	// confirm the next base fee will be as expected with the new 1559 parameters
	delta := ((gasTarget - int64(h.GasUsed)) * h.BaseFee.Int64() / gasTarget / int64(expectedDenom))
	expectedNextFee := h.BaseFee.Int64() - delta

	b, err := geth.WaitForBlock(big.NewInt(h.Number.Int64()+1), l2Seq)
	require.NoError(t, err, "waiting for next L2 block")
	require.Equal(t, expectedNextFee, b.Header().BaseFee.Int64())

	// confirm the extraData is still being set as expected
	require.Equal(t, eip1559.EncodeHoloceneExtraData(uint64(expectedDenom), uint64(expectedElasticity)), b.Header().Extra)
}
