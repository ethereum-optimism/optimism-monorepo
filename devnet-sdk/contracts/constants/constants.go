package constants

import (
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

var (
	LegacyMessagePasser           types.Address = common.HexToAddress("0x4200000000000000000000000000000000000000")
	DeployerWhitelist             types.Address = common.HexToAddress("0x4200000000000000000000000000000000000002")
	WETH                          types.Address = common.HexToAddress("0x4200000000000000000000000000000000000006")
	L2CrossDomainMessenger        types.Address = common.HexToAddress("0x4200000000000000000000000000000000000007")
	GasPriceOracle                types.Address = common.HexToAddress("0x420000000000000000000000000000000000000F")
	L2StandardBridge              types.Address = common.HexToAddress("0x4200000000000000000000000000000000000010")
	SequencerFeeVault             types.Address = common.HexToAddress("0x4200000000000000000000000000000000000011")
	OptimismMintableERC20Factory  types.Address = common.HexToAddress("0x4200000000000000000000000000000000000012")
	L1BlockNumber                 types.Address = common.HexToAddress("0x4200000000000000000000000000000000000013")
	L1Block                       types.Address = common.HexToAddress("0x4200000000000000000000000000000000000015")
	L2ToL1MessagePasser           types.Address = common.HexToAddress("0x4200000000000000000000000000000000000016")
	L2ERC721Bridge                types.Address = common.HexToAddress("0x4200000000000000000000000000000000000014")
	OptimismMintableERC721Factory types.Address = common.HexToAddress("0x4200000000000000000000000000000000000017")
	ProxyAdmin                    types.Address = common.HexToAddress("0x4200000000000000000000000000000000000018")
	BaseFeeVault                  types.Address = common.HexToAddress("0x4200000000000000000000000000000000000019")
	L1FeeVault                    types.Address = common.HexToAddress("0x420000000000000000000000000000000000001a")
	SchemaRegistry                types.Address = common.HexToAddress("0x4200000000000000000000000000000000000020")
	EAS                           types.Address = common.HexToAddress("0x4200000000000000000000000000000000000021")
	CrossL2Inbox                  types.Address = common.HexToAddress("0x4200000000000000000000000000000000000022")
	L2ToL2CrossDomainMessenger    types.Address = common.HexToAddress("0x4200000000000000000000000000000000000023")
	SuperchainWETH                types.Address = common.HexToAddress("0x4200000000000000000000000000000000000024")
	ETHLiquidity                  types.Address = common.HexToAddress("0x4200000000000000000000000000000000000025")
	SuperchainTokenBridge         types.Address = common.HexToAddress("0x4200000000000000000000000000000000000028")
	GovernanceToken               types.Address = common.HexToAddress("0x4200000000000000000000000000000000000042")
	Create2Deployer               types.Address = common.HexToAddress("0x13b0D85CcB8bf860b6b79AF3029fCA081AE9beF2")
	MultiCall3                    types.Address = common.HexToAddress("0xcA11bde05977b3631167028862bE2a173976CA11")
	Safe_v130                     types.Address = common.HexToAddress("0x69f4D1788e39c87893C980c06EdF4b7f686e2938")
	SafeL2_v130                   types.Address = common.HexToAddress("0xfb1bffC9d739B8D520DaF37dF666da4C687191EA")
	MultiSendCallOnly_v130        types.Address = common.HexToAddress("0xA1dabEF33b3B82c7814B6D82A79e50F4AC44102B")
	SafeSingletonFactory          types.Address = common.HexToAddress("0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7")
	DeterministicDeploymentProxy  types.Address = common.HexToAddress("0x4e59b44847b379578588920cA78FbF26c0B4956C")
	MultiSend_v130                types.Address = common.HexToAddress("0x998739BFdAAdde7C933B942a68053933098f9EDa")
	Permit2                       types.Address = common.HexToAddress("0x000000000022D473030F116dDEE9F6B43aC78BA3")
	SenderCreator_v060            types.Address = common.HexToAddress("0x7fc98430eaedbb6070b35b39d798725049088348")
	EntryPoint_v060               types.Address = common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789")
	SenderCreator_v070            types.Address = common.HexToAddress("0xEFC2c1444eBCC4Db75e7613d20C6a62fF67A167C")
	EntryPoint_v070               types.Address = common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032")
)

const (
	ETH  = 1e18
	Gwei = 1e9
)
