package upgrades

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

var (
	fjordGasPriceOracleCodeHash = common.HexToHash("0xa88fa50a2745b15e6794247614b5298483070661adacb8d32d716434ed24c6b2")
	// https://basescan.org/tx/0x8debb2fe54200183fb8baa3c6dbd8e6ec2e4f7a4add87416cd60336b8326d16a
	txHex = "02f875822105819b8405709fb884057d460082e97f94273ca93a52b817294830ed7572aa591ccfa647fd80881249c58b0021fb3fc080a05bb08ccfd68f83392e446dac64d88a2d28e7072c06502dfabc4a77e77b5c7913a05878d53dd4ebba4f6367e572d524dffcabeec3abb1d8725ee3ac5dc32e1852e3"
)

func TestFjordNetworkUpgradeTransactions(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	genesisBlock := hexutil.Uint64(0)
	fjordOffset := hexutil.Uint64(2)

	log := testlog.Logger(t, log.LvlDebug)

	dp.DeployConfig.L1CancunTimeOffset = &genesisBlock // can be removed once Cancun on L1 is the default

	// Activate all forks at genesis, and schedule Fjord the block after
	dp.DeployConfig.L2GenesisRegolithTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisCanyonTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisDeltaTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisEcotoneTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisFjordTimeOffset = &fjordOffset
	dp.DeployConfig.L2GenesisGraniteTimeOffset = nil
	require.NoError(t, dp.DeployConfig.Check(log), "must have valid config")

	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	_, _, _, sequencer, engine, verifier, _, _ := helpers.SetupReorgTestActors(t, dp, sd, log)
	ethCl := engine.EthClient()

	// start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Get gas price from oracle
	gasPriceOracle, err := bindings.NewGasPriceOracleCaller(predeploys.GasPriceOracleAddr, ethCl)
	require.NoError(t, err)

	// Get current implementations addresses (by slot) for L1Block + GasPriceOracle
	initialGasPriceOracleAddress, err := ethCl.StorageAt(context.Background(), predeploys.GasPriceOracleAddr, genesis.ImplementationSlot, nil)
	require.NoError(t, err)

	sequencer.ActBuildL2ToFjord(t)

	// get latest block
	latestBlock, err := ethCl.BlockByNumber(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, sequencer.L2Unsafe().Number, latestBlock.Number().Uint64())

	transactions := latestBlock.Transactions()
	// L1Block: 1 set-L1-info + 1 deploys + 1 upgradeTo + 1 enable fjord on GPO
	// See [derive.FjordNetworkUpgradeTransactions]
	require.Equal(t, 4, len(transactions))

	// All transactions are successful
	for i := 1; i < 4; i++ {
		txn := transactions[i]
		receipt, err := ethCl.TransactionReceipt(context.Background(), txn.Hash())
		require.NoError(t, err)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
		require.NotEmpty(t, txn.Data(), "upgrade tx must provide input data")
	}

	expectedGasPriceOracleAddress := crypto.CreateAddress(derive.GasPriceOracleFjordDeployerAddress, 0)

	// Gas Price Oracle Proxy is updated
	updatedGasPriceOracleAddress, err := ethCl.StorageAt(context.Background(), predeploys.GasPriceOracleAddr, genesis.ImplementationSlot, latestBlock.Number())
	require.NoError(t, err)
	require.Equal(t, expectedGasPriceOracleAddress, common.BytesToAddress(updatedGasPriceOracleAddress))
	require.NotEqualf(t, initialGasPriceOracleAddress, updatedGasPriceOracleAddress, "Gas Price Oracle Proxy address should have changed")
	verifyCodeHashMatches(t, ethCl, expectedGasPriceOracleAddress, fjordGasPriceOracleCodeHash)

	// Check that Fjord was activated
	isFjord, err := gasPriceOracle.IsFjord(nil)
	require.NoError(t, err)
	require.True(t, isFjord)

	// Check GetL1GasUsed is updated
	txData, err := hex.DecodeString(txHex)
	require.NoError(t, err)

	gpoL1GasUsed, err := gasPriceOracle.GetL1GasUsed(&bind.CallOpts{}, txData)
	require.NoError(t, err)
	require.Equal(gt, uint64(1_888), gpoL1GasUsed.Uint64())

	// Check that GetL1Fee takes into account fast LZ
	gpoFee, err := gasPriceOracle.GetL1Fee(&bind.CallOpts{}, txData)
	require.NoError(t, err)

	gethFee := fjordL1Cost(t, gasPriceOracle, types.RollupCostData{
		FastLzSize: uint64(types.FlzCompressLen(txData) + 68),
	})
	require.Equal(t, gethFee.Uint64(), gpoFee.Uint64())

	// Check that L1FeeUpperBound works
	upperBound, err := gasPriceOracle.GetL1FeeUpperBound(&bind.CallOpts{}, big.NewInt(int64(len(txData))))
	require.NoError(t, err)

	txLen := len(txData) + 68
	flzUpperBound := uint64(txLen + txLen/255 + 16)

	upperBoundCost := fjordL1Cost(t, gasPriceOracle, types.RollupCostData{FastLzSize: flzUpperBound})
	require.Equal(t, upperBoundCost.Uint64(), upperBound.Uint64())
}

func fjordL1Cost(t require.TestingT, gasPriceOracle *bindings.GasPriceOracleCaller, rollupCostData types.RollupCostData) *big.Int {
	baseFeeScalar, err := gasPriceOracle.BaseFeeScalar(nil)
	require.NoError(t, err)
	l1BaseFee, err := gasPriceOracle.L1BaseFee(nil)
	require.NoError(t, err)
	blobBaseFeeScalar, err := gasPriceOracle.BlobBaseFeeScalar(nil)
	require.NoError(t, err)
	blobBaseFee, err := gasPriceOracle.BlobBaseFee(nil)
	require.NoError(t, err)

	costFunc := types.NewL1CostFuncFjord(
		l1BaseFee,
		blobBaseFee,
		new(big.Int).SetUint64(uint64(baseFeeScalar)),
		new(big.Int).SetUint64(uint64(blobBaseFeeScalar)))

	fee, _ := costFunc(rollupCostData)
	return fee
}
