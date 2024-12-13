package predeploys

import "github.com/ethereum/go-ethereum/common"

// TODO - we should get a single toml yaml or json file source of truth in @eth-optimism/bedrock package
// This needs to be kept in sync with @eth-optimism/contracts-ts/wagmi.config.ts which also specifies this
// To improve robustness and maintainability contracts-bedrock should export all addresses
const (
	L2ToL1MessagePasser           = "0x4200000000000000000000000000000000000016"
	DeployerWhitelist             = "0x4200000000000000000000000000000000000002"
	WETH                          = "0x4200000000000000000000000000000000000006"
	L2CrossDomainMessenger        = "0x4200000000000000000000000000000000000007"
	L2StandardBridge              = "0x4200000000000000000000000000000000000010"
	SequencerFeeVault             = "0x4200000000000000000000000000000000000011"
	OptimismMintableERC20Factory  = "0x4200000000000000000000000000000000000012"
	L1BlockNumber                 = "0x4200000000000000000000000000000000000013"
	GasPriceOracle                = "0x420000000000000000000000000000000000000F"
	L1Block                       = "0x4200000000000000000000000000000000000015"
	GovernanceToken               = "0x4200000000000000000000000000000000000042"
	LegacyMessagePasser           = "0x4200000000000000000000000000000000000000"
	L2ERC721Bridge                = "0x4200000000000000000000000000000000000014"
	OptimismMintableERC721Factory = "0x4200000000000000000000000000000000000017"
	ProxyAdmin                    = "0x4200000000000000000000000000000000000018"
	BaseFeeVault                  = "0x4200000000000000000000000000000000000019"
	L1FeeVault                    = "0x420000000000000000000000000000000000001a"
	SchemaRegistry                = "0x4200000000000000000000000000000000000020"
	EAS                           = "0x4200000000000000000000000000000000000021"
	CrossL2Inbox                  = "0x4200000000000000000000000000000000000022"
	L2toL2CrossDomainMessenger    = "0x4200000000000000000000000000000000000023"
	SuperchainWETH                = "0x4200000000000000000000000000000000000024"
	ETHLiquidity                  = "0x4200000000000000000000000000000000000025"
	SuperchainTokenBridge         = "0x4200000000000000000000000000000000000028"
	Create2Deployer               = "0x13b0D85CcB8bf860b6b79AF3029fCA081AE9beF2"
	MultiCall3                    = "0xcA11bde05977b3631167028862bE2a173976CA11"
	Safe_v130                     = "0x69f4D1788e39c87893C980c06EdF4b7f686e2938"
	SafeL2_v130                   = "0xfb1bffC9d739B8D520DaF37dF666da4C687191EA"
	MultiSendCallOnly_v130        = "0xA1dabEF33b3B82c7814B6D82A79e50F4AC44102B"
	SafeSingletonFactory          = "0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7"
	DeterministicDeploymentProxy  = "0x4e59b44847b379578588920cA78FbF26c0B4956C"
	MultiSend_v130                = "0x998739BFdAAdde7C933B942a68053933098f9EDa"
	Permit2                       = "0x000000000022D473030F116dDEE9F6B43aC78BA3"
	SenderCreator_v060            = "0x7fc98430eaedbb6070b35b39d798725049088348"
	EntryPoint_v060               = "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"
	SenderCreator_v070            = "0xEFC2c1444eBCC4Db75e7613d20C6a62fF67A167C"
	EntryPoint_v070               = "0x0000000071727De22E5E9d8BAf0edAc6f37da032"
)

var (
	L2ToL1MessagePasserAddr           = common.HexToAddress(L2ToL1MessagePasser)
	DeployerWhitelistAddr             = common.HexToAddress(DeployerWhitelist)
	WETHAddr                          = common.HexToAddress(WETH)
	L2CrossDomainMessengerAddr        = common.HexToAddress(L2CrossDomainMessenger)
	L2StandardBridgeAddr              = common.HexToAddress(L2StandardBridge)
	SequencerFeeVaultAddr             = common.HexToAddress(SequencerFeeVault)
	OptimismMintableERC20FactoryAddr  = common.HexToAddress(OptimismMintableERC20Factory)
	L1BlockNumberAddr                 = common.HexToAddress(L1BlockNumber)
	GasPriceOracleAddr                = common.HexToAddress(GasPriceOracle)
	L1BlockAddr                       = common.HexToAddress(L1Block)
	GovernanceTokenAddr               = common.HexToAddress(GovernanceToken)
	LegacyMessagePasserAddr           = common.HexToAddress(LegacyMessagePasser)
	L2ERC721BridgeAddr                = common.HexToAddress(L2ERC721Bridge)
	OptimismMintableERC721FactoryAddr = common.HexToAddress(OptimismMintableERC721Factory)
	ProxyAdminAddr                    = common.HexToAddress(ProxyAdmin)
	BaseFeeVaultAddr                  = common.HexToAddress(BaseFeeVault)
	L1FeeVaultAddr                    = common.HexToAddress(L1FeeVault)
	SchemaRegistryAddr                = common.HexToAddress(SchemaRegistry)
	EASAddr                           = common.HexToAddress(EAS)
	CrossL2InboxAddr                  = common.HexToAddress(CrossL2Inbox)
	L2toL2CrossDomainMessengerAddr    = common.HexToAddress(L2toL2CrossDomainMessenger)
	SuperchainWETHAddr                = common.HexToAddress(SuperchainWETH)
	ETHLiquidityAddr                  = common.HexToAddress(ETHLiquidity)
	SuperchainTokenBridgeAddr         = common.HexToAddress(SuperchainTokenBridge)
	Create2DeployerAddr               = common.HexToAddress(Create2Deployer)
	MultiCall3Addr                    = common.HexToAddress(MultiCall3)
	Safe_v130Addr                     = common.HexToAddress(Safe_v130)
	SafeL2_v130Addr                   = common.HexToAddress(SafeL2_v130)
	MultiSendCallOnly_v130Addr        = common.HexToAddress(MultiSendCallOnly_v130)
	SafeSingletonFactoryAddr          = common.HexToAddress(SafeSingletonFactory)
	DeterministicDeploymentProxyAddr  = common.HexToAddress(DeterministicDeploymentProxy)
	MultiSend_v130Addr                = common.HexToAddress(MultiSend_v130)
	Permit2Addr                       = common.HexToAddress(Permit2)
	SenderCreator_v060Addr            = common.HexToAddress(SenderCreator_v060)
	EntryPoint_v060Addr               = common.HexToAddress(EntryPoint_v060)
	SenderCreator_v070Addr            = common.HexToAddress(SenderCreator_v070)
	EntryPoint_v070Addr               = common.HexToAddress(EntryPoint_v070)

	Predeploys          = make(map[string]*Predeploy)
	PredeploysByAddress = make(map[common.Address]*Predeploy)
)

func init() {
	Predeploys["L2ToL1MessagePasser"] = &Predeploy{Address: L2ToL1MessagePasserAddr}
	Predeploys["DeployerWhitelist"] = &Predeploy{Address: DeployerWhitelistAddr}
	Predeploys["WETH"] = &Predeploy{Address: WETHAddr, ProxyDisabled: true}
	Predeploys["L2CrossDomainMessenger"] = &Predeploy{Address: L2CrossDomainMessengerAddr}
	Predeploys["L2StandardBridge"] = &Predeploy{Address: L2StandardBridgeAddr}
	Predeploys["SequencerFeeVault"] = &Predeploy{Address: SequencerFeeVaultAddr}
	Predeploys["OptimismMintableERC20Factory"] = &Predeploy{Address: OptimismMintableERC20FactoryAddr}
	Predeploys["L1BlockNumber"] = &Predeploy{Address: L1BlockNumberAddr}
	Predeploys["GasPriceOracle"] = &Predeploy{Address: GasPriceOracleAddr}
	Predeploys["L1Block"] = &Predeploy{Address: L1BlockAddr}
	Predeploys["CrossL2Inbox"] = &Predeploy{Address: CrossL2InboxAddr}
	Predeploys["L2toL2CrossDomainMessenger"] = &Predeploy{Address: L2toL2CrossDomainMessengerAddr}
	Predeploys["SuperchainWETH"] = &Predeploy{Address: SuperchainWETHAddr}
	Predeploys["ETHLiquidity"] = &Predeploy{Address: ETHLiquidityAddr}
	Predeploys["SuperchainTokenBridge"] = &Predeploy{Address: SuperchainTokenBridgeAddr}
	Predeploys["GovernanceToken"] = &Predeploy{
		Address:       GovernanceTokenAddr,
		ProxyDisabled: true,
		Enabled: func(config DeployConfig) bool {
			return config.GovernanceEnabled()
		},
	}
	Predeploys["LegacyMessagePasser"] = &Predeploy{Address: LegacyMessagePasserAddr}
	Predeploys["L2ERC721Bridge"] = &Predeploy{Address: L2ERC721BridgeAddr}
	Predeploys["OptimismMintableERC721Factory"] = &Predeploy{Address: OptimismMintableERC721FactoryAddr}
	Predeploys["ProxyAdmin"] = &Predeploy{Address: ProxyAdminAddr}
	Predeploys["BaseFeeVault"] = &Predeploy{Address: BaseFeeVaultAddr}
	Predeploys["L1FeeVault"] = &Predeploy{Address: L1FeeVaultAddr}
	Predeploys["SchemaRegistry"] = &Predeploy{Address: SchemaRegistryAddr}
	Predeploys["EAS"] = &Predeploy{Address: EASAddr}
	Predeploys["Create2Deployer"] = &Predeploy{
		Address:       Create2DeployerAddr,
		ProxyDisabled: true,
	}
	Predeploys["MultiCall3"] = &Predeploy{
		Address:       MultiCall3Addr,
		ProxyDisabled: true,
	}
	Predeploys["Safe_v130"] = &Predeploy{
		Address:       Safe_v130Addr,
		ProxyDisabled: true,
	}
	Predeploys["SafeL2_v130"] = &Predeploy{
		Address:       SafeL2_v130Addr,
		ProxyDisabled: true,
	}
	Predeploys["MultiSendCallOnly_v130"] = &Predeploy{
		Address:       MultiSendCallOnly_v130Addr,
		ProxyDisabled: true,
	}
	Predeploys["SafeSingletonFactory"] = &Predeploy{
		Address:       SafeSingletonFactoryAddr,
		ProxyDisabled: true,
	}
	Predeploys["DeterministicDeploymentProxy"] = &Predeploy{
		Address:       DeterministicDeploymentProxyAddr,
		ProxyDisabled: true,
	}
	Predeploys["MultiSend_v130"] = &Predeploy{
		Address:       MultiSend_v130Addr,
		ProxyDisabled: true,
	}
	Predeploys["Permit2"] = &Predeploy{
		Address:       Permit2Addr,
		ProxyDisabled: true,
	}
	Predeploys["SenderCreator_v060"] = &Predeploy{
		Address:       SenderCreator_v060Addr,
		ProxyDisabled: true,
	}
	Predeploys["EntryPoint_v060"] = &Predeploy{
		Address:       EntryPoint_v060Addr,
		ProxyDisabled: true,
	}
	Predeploys["SenderCreator_v070"] = &Predeploy{
		Address:       SenderCreator_v070Addr,
		ProxyDisabled: true,
	}
	Predeploys["EntryPoint_v070"] = &Predeploy{
		Address:       EntryPoint_v070Addr,
		ProxyDisabled: true,
	}

	for _, predeploy := range Predeploys {
		PredeploysByAddress[predeploy.Address] = predeploy
	}
}
