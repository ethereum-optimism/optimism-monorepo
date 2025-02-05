package upgrades

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

var (
	l1BlockCodeHash        = common.HexToHash("0xc88a313aa75dc4fbf0b6850d9f9ae41e04243b7008cf3eadb29256d4a71c1dfd")
	gasPriceOracleCodeHash = common.HexToHash("0x8b71360ea773b4cfaf1ae6d2bd15464a4e1e2e360f786e475f63aeaed8da0ae5")
)

// verifyCodeHashMatches checks that the has of the code at the given address matches the expected code-hash.
// It also sanity-checks that the code is not empty: we should never deploy empty contract codes.
// Returns the contract code
func verifyCodeHashMatches(t helpers.Testing, client *ethclient.Client, address common.Address, expectedCodeHash common.Hash) []byte {
	code, err := client.CodeAt(context.Background(), address, nil)
	require.NoError(t, err)
	require.NotEmpty(t, code)
	codeHash := crypto.Keccak256Hash(code)
	require.Equal(t, expectedCodeHash, codeHash)
	return code
}

func TestEcotoneNetworkUpgradeTransactions(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	ecotoneOffset := hexutil.Uint64(4)

	log := testlog.Logger(t, log.LevelDebug)

	require.Zero(t, *dp.DeployConfig.L1CancunTimeOffset)
	// Activate all forks at genesis, and schedule Ecotone the block after
	dp.DeployConfig.L2GenesisEcotoneTimeOffset = &ecotoneOffset
	dp.DeployConfig.L2GenesisFjordTimeOffset = nil
	dp.DeployConfig.L2GenesisGraniteTimeOffset = nil
	dp.DeployConfig.L2GenesisHoloceneTimeOffset = nil
	// New forks have to be added here...
	require.NoError(t, dp.DeployConfig.Check(log), "must have valid config")

	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	_, _, miner, sequencer, engine, verifier, _, _ := helpers.SetupReorgTestActors(t, dp, sd, log)
	ethCl := engine.EthClient()

	// build a single block to move away from the genesis with 0-values in L1Block contract
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)

	// start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Get gas price from oracle
	gasPriceOracle, err := bindings.NewGasPriceOracleCaller(predeploys.GasPriceOracleAddr, ethCl)
	require.NoError(t, err)

	scalar, err := gasPriceOracle.Scalar(nil)
	require.NoError(t, err)
	require.True(t, scalar.Cmp(big.NewInt(0)) > 0, "scalar must start non-zero")
	feeScalar := dp.DeployConfig.FeeScalar()
	require.Equal(t, scalar, new(big.Int).SetBytes(feeScalar[:]), "must match deploy config")

	// Get current implementations addresses (by slot) for L1Block + GasPriceOracle
	initialGasPriceOracleAddress, err := ethCl.StorageAt(context.Background(), predeploys.GasPriceOracleAddr, genesis.ImplementationSlot, nil)
	require.NoError(t, err)
	initialL1BlockAddress, err := ethCl.StorageAt(context.Background(), predeploys.L1BlockAddr, genesis.ImplementationSlot, nil)
	require.NoError(t, err)

	// Build to the ecotone block
	sequencer.ActBuildL2ToEcotone(t)

	// get latest block
	latestBlock, err := ethCl.BlockByNumber(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, sequencer.L2Unsafe().Number, latestBlock.Number().Uint64())

	transactions := latestBlock.Transactions()
	// L1Block: 1 set-L1-info + 2 deploys + 2 upgradeTo + 1 enable ecotone on GPO + 1 4788 deploy
	// See [derive.EcotoneNetworkUpgradeTransactions]
	require.Equal(t, 7, len(transactions))

	l1Info, err := derive.L1BlockInfoFromBytes(sd.RollupCfg, latestBlock.Time(), transactions[0].Data())
	require.NoError(t, err)
	require.Equal(t, derive.L1InfoBedrockLen, len(transactions[0].Data()))
	require.Nil(t, l1Info.BlobBaseFee)

	// All transactions are successful
	for i := 1; i < 7; i++ {
		txn := transactions[i]
		receipt, err := ethCl.TransactionReceipt(context.Background(), txn.Hash())
		require.NoError(t, err)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %d must pass", i)
		require.NotEmpty(t, txn.Data(), "upgrade tx must provide input data")
	}

	expectedL1BlockAddress := crypto.CreateAddress(derive.L1BlockDeployerAddress, 0)
	expectedGasPriceOracleAddress := crypto.CreateAddress(derive.GasPriceOracleDeployerAddress, 0)

	// Gas Price Oracle Proxy is updated
	updatedGasPriceOracleAddress, err := ethCl.StorageAt(context.Background(), predeploys.GasPriceOracleAddr, genesis.ImplementationSlot, latestBlock.Number())
	require.NoError(t, err)
	require.Equal(t, expectedGasPriceOracleAddress, common.BytesToAddress(updatedGasPriceOracleAddress))
	require.NotEqualf(t, initialGasPriceOracleAddress, updatedGasPriceOracleAddress, "Gas Price Oracle Proxy address should have changed")
	verifyCodeHashMatches(t, ethCl, expectedGasPriceOracleAddress, gasPriceOracleCodeHash)

	// L1Block Proxy is updated
	updatedL1BlockAddress, err := ethCl.StorageAt(context.Background(), predeploys.L1BlockAddr, genesis.ImplementationSlot, latestBlock.Number())
	require.NoError(t, err)
	require.Equal(t, expectedL1BlockAddress, common.BytesToAddress(updatedL1BlockAddress))
	require.NotEqualf(t, initialL1BlockAddress, updatedL1BlockAddress, "L1Block Proxy address should have changed")
	verifyCodeHashMatches(t, ethCl, expectedL1BlockAddress, l1BlockCodeHash)

	_, err = gasPriceOracle.Scalar(nil)
	require.ErrorContains(t, err, "scalar() is deprecated")

	cost, err := gasPriceOracle.GetL1Fee(nil, []byte{0, 1, 2, 3, 4})
	require.NoError(t, err)
	// The L1 info tx does not get included until after the Ecotone upgrade.
	// The scalars are thus empty during activation, and only deposits are included, so the L1 fee is unused.
	require.True(t, cost.IsUint64())
	require.Equal(t, cost.Uint64(), uint64(0), "expecting zero scalars within activation block")

	// Check that Ecotone was activated
	isEcotone, err := gasPriceOracle.IsEcotone(nil)
	require.NoError(t, err)
	require.True(t, isEcotone)

	// 4788 contract is deployed
	expected4788Address := crypto.CreateAddress(derive.EIP4788From, 0)
	require.Equal(t, predeploys.EIP4788ContractAddr, expected4788Address)
	code := verifyCodeHashMatches(t, ethCl, predeploys.EIP4788ContractAddr, predeploys.EIP4788ContractCodeHash)
	require.Equal(t, predeploys.EIP4788ContractCode, code)
	// Test that the beacon-block-root has been set
	checkBeaconBlockRoot := func(timestamp uint64, expectedHash common.Hash, expectedTime uint64, msg string) {
		historyBufferLength := uint64(8191)
		rootIdx := common.BigToHash(new(big.Int).SetUint64((timestamp % historyBufferLength) + historyBufferLength))
		timeIdx := common.BigToHash(new(big.Int).SetUint64(timestamp % historyBufferLength))

		rootValue, err := ethCl.StorageAt(context.Background(), predeploys.EIP4788ContractAddr, rootIdx, nil)
		require.NoError(t, err)
		require.Equal(t, expectedHash, common.BytesToHash(rootValue), msg)

		timeValue, err := ethCl.StorageAt(context.Background(), predeploys.EIP4788ContractAddr, timeIdx, nil)
		require.NoError(t, err)
		timeBig := new(big.Int).SetBytes(timeValue)
		require.True(t, timeBig.IsUint64())
		require.Equal(t, expectedTime, timeBig.Uint64(), msg)
	}
	// The header will always have the beacon-block-root, at the very start.
	require.NotNil(t, latestBlock.BeaconRoot())
	require.Equal(t, *latestBlock.BeaconRoot(), common.Hash{},
		"L1 genesis block has zeroed parent-beacon-block-root, since it has no parent block, and that propagates into L2")
	// Legacy check:
	// > The first block is an exception in upgrade-networks,
	// > since the beacon-block root contract isn't there at Ecotone activation,
	// > and the beacon-block-root insertion is processed at the start of the block before deposit txs.
	// > If the contract was permissionlessly deployed before, the contract storage will be updated however.
	// > checkBeaconBlockRoot(latestBlock.Time(), common.Hash{}, 0, "ecotone activation block has no data yet (since contract wasn't there)")
	// Note: 4788 is now installed as preinstall, and thus always there.
	checkBeaconBlockRoot(latestBlock.Time(), common.Hash{}, latestBlock.Time(), "4788 lookup of first cancun block is 0 hash")

	// Build empty L2 block, to pass ecotone activation
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)

	// Test the L2 block after activation: it should have data in the contract storage now
	latestBlock, err = ethCl.BlockByNumber(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, latestBlock.BeaconRoot())
	firstBeaconBlockRoot := *latestBlock.BeaconRoot()
	checkBeaconBlockRoot(latestBlock.Time(), *latestBlock.BeaconRoot(), latestBlock.Time(), "post-activation")

	// require.again, now that we are past activation
	_, err = gasPriceOracle.Scalar(nil)
	require.ErrorContains(t, err, "scalar() is deprecated")

	// test if the migrated scalar matches the deploy config
	basefeeScalar, err := gasPriceOracle.BaseFeeScalar(nil)
	require.NoError(t, err)
	require.Equal(t, uint64(basefeeScalar), dp.DeployConfig.GasPriceOracleScalar, "must match deploy config")

	cost, err = gasPriceOracle.GetL1Fee(nil, []byte{0, 1, 2, 3, 4})
	require.NoError(t, err)
	// The GPO getL1Fee contract returns the L1 fee with approximate signature overhead pre-included,
	// like the pre-regolith L1 fee. We do the full fee check below. Just sanity check it is not zero anymore first.
	require.Greater(t, cost.Uint64(), uint64(0), "expecting non-zero scalars after activation block")

	// Get L1Block info
	l1Block, err := bindings.NewL1BlockCaller(predeploys.L1BlockAddr, ethCl)
	require.NoError(t, err)
	l1BlockInfo, err := l1Block.Timestamp(nil)
	require.NoError(t, err)
	require.Greater(t, l1BlockInfo, uint64(0))

	l1OriginBlock, err := miner.EthClient().BlockByHash(context.Background(), sequencer.L2Unsafe().L1Origin.Hash)
	require.NoError(t, err)
	l1Basefee, err := l1Block.Basefee(nil)
	require.NoError(t, err)
	require.Equal(t, l1OriginBlock.BaseFee().Uint64(), l1Basefee.Uint64(), "basefee must match")

	// calldataGas*(l1BaseFee*16*l1BaseFeeScalar + l1BlobBaseFee*l1BlobBaseFeeScalar)/16e6
	// _getCalldataGas in GPO adds the cost of 68 non-zero bytes for signature/rlp overhead.
	calldataGas := big.NewInt(4*16 + 1*4 + 68*16)
	expectedL1Fee := new(big.Int).Mul(calldataGas, l1Basefee)
	expectedL1Fee = expectedL1Fee.Mul(expectedL1Fee, big.NewInt(16))
	expectedL1Fee = expectedL1Fee.Mul(expectedL1Fee, new(big.Int).SetUint64(uint64(basefeeScalar)))
	expectedL1Fee = expectedL1Fee.Div(expectedL1Fee, big.NewInt(16e6))
	require.Equal(t, expectedL1Fee, cost, "expecting cost based on regular base fee scalar alone")

	// build forward, incorporate new L1 data
	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)

	// Contract storage should be updated now, different than before
	latestBlock, err = ethCl.BlockByNumber(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, latestBlock.BeaconRoot())
	require.NotEqual(t, firstBeaconBlockRoot, *latestBlock.BeaconRoot())
	checkBeaconBlockRoot(latestBlock.Time(), *latestBlock.BeaconRoot(), latestBlock.Time(), "updates on new L1 data")
}

// TestEcotoneBeforeL1 tests that the L2 Ecotone fork can activate before L1 Dencun does
func TestEcotoneBeforeL1(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	offset := hexutil.Uint64(0)
	farOffset := hexutil.Uint64(10000)
	dp.DeployConfig.L2GenesisRegolithTimeOffset = &offset
	dp.DeployConfig.L1CancunTimeOffset = &farOffset // L1 Dencun will not be active at genesis
	dp.DeployConfig.L2GenesisCanyonTimeOffset = &offset
	dp.DeployConfig.L2GenesisDeltaTimeOffset = &offset
	dp.DeployConfig.L2GenesisEcotoneTimeOffset = &offset

	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	_, _, _, sequencer, engine, verifier, _, _ := helpers.SetupReorgTestActors(t, dp, sd, log)

	// start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Genesis block has ecotone properties
	verifyEcotoneBlock(gt, engine.L2Chain().CurrentBlock())

	// Blocks post fork have Ecotone properties
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)
	verifyEcotoneBlock(gt, engine.L2Chain().CurrentBlock())
}
