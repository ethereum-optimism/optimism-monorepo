package derive

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestIsthmusSourcesMatchSpec(t *testing.T) {
	for _, test := range []struct {
		source       UpgradeDepositSource
		expectedHash string
	}{
		{
			source:       blockHashDeployerSource,
			expectedHash: "0xbfb734dae514c5974ddf803e54c1bc43d5cdb4a48ae27e1d9b875a5a150b553a",
		},
		{
			source:       deployIsthmusL1BlockSource,
			expectedHash: "0x3b2d0821ca2411ad5cd3595804d1213d15737188ae4cbd58aa19c821a6c211bf",
		},
		{
			source:       deployIsthmusGasPriceOracleSource,
			expectedHash: "0xfc70b48424763fa3fab9844253b4f8d508f91eb1f7cb11a247c9baec0afb8035",
		},
		{
			source:       deployOperatorFeeVaultSource,
			expectedHash: "0x107a570d3db75e6110817eb024f09f3172657e920634111ce9875d08a16daa96",
		},
		{
			source:       updateIsthmusL1BlockProxySource,
			expectedHash: "0xebe8b5cb10ca47e0d8bda8f5355f2d66711a54ddeb0ef1d30e29418c9bf17a0e",
		},
		{
			source:       updateIsthmusGasPriceOracleSource,
			expectedHash: "0xecf2d9161d26c54eda6b7bfdd9142719b1e1199a6e5641468d1bf705bc531ab0",
		},
		{
			source:       updateOperatorFeeVaultSource,
			expectedHash: "0xad74e1adb877ccbe176b8fa1cc559388a16e090ddbe8b512f5b37d07d887a927",
		},
		{
			source:       enableIsthmusSource,
			expectedHash: "0x3ddf4b1302548dd92939826e970f260ba36167f4c25f18390a5e8b194b295319",
		},
	} {
		require.Equal(t, common.HexToHash(test.expectedHash), test.source.SourceHash())
	}
}

func TestIsthmusNetworkTransactions(t *testing.T) {
	upgradeTxns, err := IsthmusNetworkUpgradeTransactions()
	require.NoError(t, err)
	require.Len(t, upgradeTxns, 8)

	deployBlockHashesSender, deployBlockHashesContract := toDepositTxn(t, upgradeTxns[0])
	require.Equal(t, deployBlockHashesSender, predeploys.EIP2935ContractDeployer)
	require.Equal(t, blockHashDeployerSource.SourceHash(), deployBlockHashesContract.SourceHash())
	require.Nil(t, deployBlockHashesContract.To())
	require.Equal(t, uint64(250_000), deployBlockHashesContract.Gas())
	require.Equal(t, blockHashDeploymentBytecode, deployBlockHashesContract.Data())

	deployL1BlockSender, deployL1Block := toDepositTxn(t, upgradeTxns[0])
	require.Equal(t, deployL1BlockSender, common.HexToAddress("0x4210000000000000000000000000000000000003"))
	require.Equal(t, deployIsthmusL1BlockSource.SourceHash(), deployL1Block.SourceHash())
	require.Nil(t, deployL1Block.To())
	require.Equal(t, uint64(425_000), deployL1Block.Gas()) // TODO
	require.Equal(t, l1BlockIsthmusDeploymentBytecode, deployL1Block.Data())

	deployGasPriceOracleSender, deployGasPriceOracle := toDepositTxn(t, upgradeTxns[1])
	require.Equal(t, deployGasPriceOracleSender, common.HexToAddress("0x4210000000000000000000000000000000000004"))
	require.Equal(t, deployIsthmusGasPriceOracleSource.SourceHash(), deployGasPriceOracle.SourceHash())
	require.Nil(t, deployGasPriceOracle.To())
	require.Equal(t, uint64(1_625_000), deployGasPriceOracle.Gas())
	require.Equal(t, gasPriceOracleIsthmusDeploymentBytecode, deployGasPriceOracle.Data())

	deployOperatorFeeVaultSender, deployOperatorFeeVault := toDepositTxn(t, upgradeTxns[2])
	require.Equal(t, deployOperatorFeeVaultSender, common.HexToAddress("0x4210000000000000000000000000000000000005"))
	require.Equal(t, deployOperatorFeeVaultSource.SourceHash(), deployOperatorFeeVault.SourceHash())
	require.Nil(t, deployOperatorFeeVault.To())
	require.Equal(t, uint64(500_000), deployOperatorFeeVault.Gas())
	require.Equal(t, append(operatorFeeVaultDeploymentBytecode, operatorFeeVaultEncodedArgs...), deployOperatorFeeVault.Data())

	updateL1BlockProxySender, updateL1BlockProxy := toDepositTxn(t, upgradeTxns[3])
	require.Equal(t, updateL1BlockProxySender, common.Address{})
	require.Equal(t, updateIsthmusL1BlockProxySource.SourceHash(), updateL1BlockProxy.SourceHash())
	require.NotNil(t, updateL1BlockProxy.To())
	require.Equal(t, *updateL1BlockProxy.To(), common.HexToAddress("0x4200000000000000000000000000000000000015"))
	require.Equal(t, uint64(50_000), updateL1BlockProxy.Gas())
	require.Equal(t, common.FromHex("3659cfe6000000000000000000000000ff256497d61dcd71a9e9ff43967c13fde1f72d12"), updateL1BlockProxy.Data())

	updateGasPriceOracleSender, updateGasPriceOracle := toDepositTxn(t, upgradeTxns[4])
	require.Equal(t, updateGasPriceOracleSender, common.Address{})
	require.Equal(t, updateIsthmusGasPriceOracleSource.SourceHash(), updateGasPriceOracle.SourceHash())
	require.NotNil(t, updateGasPriceOracle.To())
	require.Equal(t, *updateGasPriceOracle.To(), common.HexToAddress("0x420000000000000000000000000000000000000F"))
	require.Equal(t, uint64(50_000), updateGasPriceOracle.Gas())
	require.Equal(t, common.FromHex("3659cfe600000000000000000000000093e57a196454cb919193fa9946f14943cf733845"), updateGasPriceOracle.Data())

	updateOperatorFeeSender, updateOperatorFee := toDepositTxn(t, upgradeTxns[5])
	require.Equal(t, updateOperatorFeeSender, common.Address{})
	require.Equal(t, updateOperatorFeeVaultSource.SourceHash(), updateOperatorFee.SourceHash())
	require.NotNil(t, updateOperatorFee.To())
	require.Equal(t, *updateOperatorFee.To(), common.HexToAddress("0x420000000000000000000000000000000000001b"))
	require.Equal(t, uint64(50_000), updateOperatorFee.Gas())
	require.Equal(t, common.FromHex("3659cfe60000000000000000000000004fa2be8cd41504037f1838bce3bcc93bc68ff537"), updateOperatorFee.Data())

	gpoSetIsthmusSender, gpoSetIsthmus := toDepositTxn(t, upgradeTxns[6])
	require.Equal(t, gpoSetIsthmusSender, common.HexToAddress("0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAd0001"))
	require.Equal(t, enableIsthmusSource.SourceHash(), gpoSetIsthmus.SourceHash())
	require.NotNil(t, gpoSetIsthmus.To())
	require.Equal(t, *gpoSetIsthmus.To(), common.HexToAddress("0x420000000000000000000000000000000000000F"))
	require.Equal(t, uint64(90_000), gpoSetIsthmus.Gas())
	require.Equal(t, common.FromHex("291b0383"), gpoSetIsthmus.Data())
}
