package derive

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	// L1Block Parameters
	deployIsthmusL1BlockSource      = UpgradeDepositSource{Intent: "Isthmus: L1 Block Deployment"}
	updateIsthmusL1BlockProxySource = UpgradeDepositSource{Intent: "Isthmus: L1 Block Proxy Update"}
	L1BlockIsthmusDeployerAddress   = common.HexToAddress("0x4210000000000000000000000000000000000003")
	isthmusL1BlockAddress           = crypto.CreateAddress(L1BlockIsthmusDeployerAddress, 0)

	// Gas Price Oracle Parameters
	deployIsthmusGasPriceOracleSource    = UpgradeDepositSource{Intent: "Isthmus: Gas Price Oracle Deployment"}
	updateIsthmusGasPriceOracleSource    = UpgradeDepositSource{Intent: "Isthmus: Gas Price Oracle Proxy Update"}
	GasPriceOracleIsthmusDeployerAddress = common.HexToAddress("0x4210000000000000000000000000000000000004")
	isthmusGasPriceOracleAddress         = crypto.CreateAddress(GasPriceOracleIsthmusDeployerAddress, 0)

	// Operator fee vault Parameters
	deployOperatorFeeVaultSource    = UpgradeDepositSource{Intent: "Isthmus: Operator Fee Vault Deployment"}
	updateOperatorFeeVaultSource    = UpgradeDepositSource{Intent: "Isthmus: Operator Fee Vault Proxy Update"}
	OperatorFeeVaultDeployerAddress = common.HexToAddress("0x4210000000000000000000000000000000000005")
	OperatorFeeVaultAddress         = crypto.CreateAddress(OperatorFeeVaultDeployerAddress, 0)

	// Bytecodes
	l1BlockIsthmusDeploymentBytecode        = common.FromHex("0x0") // TODO
	gasPriceOracleIsthmusDeploymentBytecode = common.FromHex("0x0") // TODO
	operatorFeeVaultDeploymentBytecode      = common.FromHex("0x0") // TODO

	// Enable Isthmus Parameters
	enableIsthmusSource = UpgradeDepositSource{Intent: "Isthmus: Gas Price Oracle Set Isthmus"}
	enableIsthmusInput  = crypto.Keccak256([]byte("setIsthmus()"))[:4]

	BlockHashDeployerAddress    = common.HexToAddress("0xE9f0662359Bb2c8111840eFFD73B9AFA77CbDE10")
	blockHashDeployerSource     = UpgradeDepositSource{Intent: "Isthmus: EIP-2935 Contract Deployment"}
	blockHashDeploymentBytecode = common.FromHex("0x60538060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500")
)

func IsthmusNetworkUpgradeTransactions() ([]hexutil.Bytes, error) {
	upgradeTxns := make([]hexutil.Bytes, 0, 8)

	deployHistoricalBlockHashesContract, err := types.NewTx(&types.DepositTx{
		SourceHash:          blockHashDeployerSource.SourceHash(),
		From:                BlockHashDeployerAddress,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 250_000,
		IsSystemTransaction: false,
		Data:                blockHashDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, deployHistoricalBlockHashesContract)

	// Deploy L1 Block transaction
	deployL1BlockTransaction, err := types.NewTx(&types.DepositTx{
		SourceHash:          deployIsthmusL1BlockSource.SourceHash(),
		From:                L1BlockIsthmusDeployerAddress,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 425_000,
		IsSystemTransaction: false,
		Data:                l1BlockIsthmusDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, deployL1BlockTransaction)

	// Deploy Gas Price Oracle transaction
	deployGasPriceOracle, err := types.NewTx(&types.DepositTx{
		SourceHash:          deployIsthmusGasPriceOracleSource.SourceHash(),
		From:                GasPriceOracleIsthmusDeployerAddress,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 1_625_000,
		IsSystemTransaction: false,
		Data:                gasPriceOracleIsthmusDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, deployGasPriceOracle)

	// Deploy Operator Fee vault transaction
	deployOperatorFeeVault, err := types.NewTx(&types.DepositTx{
		SourceHash:          deployOperatorFeeVaultSource.SourceHash(),
		From:                OperatorFeeVaultDeployerAddress,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 500_000,
		IsSystemTransaction: false,
		Data:                operatorFeeVaultDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, deployOperatorFeeVault)

	// Deploy L1 Block Proxy upgrade transaction
	updateL1BlockProxy, err := types.NewTx(&types.DepositTx{
		SourceHash:          updateIsthmusL1BlockProxySource.SourceHash(),
		From:                common.Address{},
		To:                  &predeploys.L1BlockAddr,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 50_000,
		IsSystemTransaction: false,
		Data:                upgradeToCalldata(isthmusL1BlockAddress),
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, updateL1BlockProxy)

	// Deploy Gas Price Oracle Proxy upgrade transaction
	updateGasPriceOracleProxy, err := types.NewTx(&types.DepositTx{
		SourceHash:          updateIsthmusGasPriceOracleSource.SourceHash(),
		From:                common.Address{},
		To:                  &predeploys.GasPriceOracleAddr,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 50_000,
		IsSystemTransaction: false,
		Data:                upgradeToCalldata(isthmusGasPriceOracleAddress),
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, updateGasPriceOracleProxy)

	// Deploy Operator Fee Vault Proxy upgrade transaction
	updateOperatorFeeVaultProxy, err := types.NewTx(&types.DepositTx{
		SourceHash:          updateOperatorFeeVaultSource.SourceHash(),
		From:                common.Address{},
		To:                  &predeploys.OperatorFeeVaultAddr,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 50_000,
		IsSystemTransaction: false,
		Data:                upgradeToCalldata(OperatorFeeVaultAddress),
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, updateOperatorFeeVaultProxy)

	// Enable Isthmus transaction
	enableIsthmus, err := types.NewTx(&types.DepositTx{
		SourceHash:          enableIsthmusSource.SourceHash(),
		From:                L1InfoDepositerAddress,
		To:                  &predeploys.GasPriceOracleAddr,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 90_000,
		IsSystemTransaction: false,
		Data:                enableIsthmusInput,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, enableIsthmus)

	return upgradeTxns, nil
}
