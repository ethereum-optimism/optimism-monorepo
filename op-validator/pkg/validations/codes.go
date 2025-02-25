package validations

var descriptions = map[string]string{
	// SuperchainConfig validations
	"SPRCFG-10": "SuperchainConfig is paused",

	// ProxyAdmin validations
	"PROXYA-10": "ProxyAdmin owner is not set to L1 PAO multisig",

	// SystemConfig validations
	"SYSCON-10":  "SystemConfig version mismatch",
	"SYSCON-20":  "SystemConfig gas limit is not set to 60,000,000",
	"SYSCON-30":  "SystemConfig scalar is not set to 1",
	"SYSCON-40":  "SystemConfig implementation address mismatch",
	"SYSCON-50":  "SystemConfig maxResourceLimit is not set to 20,000,000",
	"SYSCON-60":  "SystemConfig elasticityMultiplier is not set to 10",
	"SYSCON-70":  "SystemConfig baseFeeMaxChangeDenominator is not set to 8",
	"SYSCON-80":  "SystemConfig systemTxMaxGas is not set to 1,000,000",
	"SYSCON-90":  "SystemConfig minimumBaseFee is not set to 1 gwei",
	"SYSCON-100": "SystemConfig maximumBaseFee is not set to max uint128",

	// L1 Cross Domain Messenger validations
	"L1xDM-10": "L1CrossDomainMessenger version mismatch",
	"L1xDM-20": "L1CrossDomainMessenger implementation address mismatch",
	"L1xDM-30": "L1CrossDomainMessenger OTHER_MESSENGER address mismatch",
	"L1xDM-40": "L1CrossDomainMessenger otherMessenger address mismatch",
	"L1xDM-50": "L1CrossDomainMessenger PORTAL address mismatch",
	"L1xDM-60": "L1CrossDomainMessenger portal address mismatch",
	"L1xDM-70": "L1CrossDomainMessenger superchainConfig address mismatch",

	// L1 Standard Bridge validations
	"L1SB-10": "L1StandardBridge version mismatch",
	"L1SB-20": "L1StandardBridge implementation address mismatch",
	"L1SB-30": "L1StandardBridge MESSENGER address mismatch",
	"L1SB-40": "L1StandardBridge messenger address mismatch",
	"L1SB-50": "L1StandardBridge OTHER_BRIDGE address mismatch",
	"L1SB-60": "L1StandardBridge otherBridge address mismatch",
	"L1SB-70": "L1StandardBridge superchainConfig address mismatch",

	// Optimism Mintable ERC20 Factory validations
	"MERC20F-10": "OptimismMintableERC20Factory version mismatch",
	"MERC20F-20": "OptimismMintableERC20Factory implementation address mismatch",
	"MERC20F-30": "OptimismMintableERC20Factory BRIDGE address mismatch",
	"MERC20F-40": "OptimismMintableERC20Factory bridge address mismatch",

	// L1 ERC721 Bridge validations
	"L721B-10": "L1ERC721Bridge version mismatch",
	"L721B-20": "L1ERC721Bridge implementation address mismatch",
	"L721B-30": "L1ERC721Bridge OTHER_BRIDGE address mismatch",
	"L721B-40": "L1ERC721Bridge otherBridge address mismatch",
	"L721B-50": "L1ERC721Bridge MESSENGER address mismatch",
	"L721B-60": "L1ERC721Bridge messenger address mismatch",
	"L721B-70": "L1ERC721Bridge superchainConfig address mismatch",

	// Optimism Portal validations
	"PORTAL-10": "OptimismPortal version mismatch",
	"PORTAL-20": "OptimismPortal implementation address mismatch",
	"PORTAL-30": "OptimismPortal disputeGameFactory address mismatch",
	"PORTAL-40": "OptimismPortal systemConfig address mismatch",
	"PORTAL-50": "OptimismPortal superchainConfig address mismatch",
	"PORTAL-60": "OptimismPortal guardian address mismatch",
	"PORTAL-70": "OptimismPortal paused state mismatch with superchainConfig",
	"PORTAL-80": "OptimismPortal l2Sender not set to default value",

	// Dispute Factory validations
	"DF-10": "DisputeGameFactory version mismatch",
	"DF-20": "DisputeGameFactory implementation address mismatch",
	"DF-30": "DisputeGameFactory owner is not set to L1 PAO multisig",

	// Permissioned Dispute Game validations
	"PDDG-10":  "Permissioned dispute game implementation not found",
	"PDDG-20":  "Permissioned dispute game version mismatch",
	"PDDG-30":  "Permissioned dispute game type mismatch",
	"PDDG-40":  "Permissioned dispute game absolute prestate mismatch",
	"PDDG-50":  "Permissioned dispute game VM address mismatch",
	"PDDG-60":  "Permissioned dispute game L2 chain ID mismatch",
	"PDDG-70":  "Permissioned dispute game L2 block number not set to 0",
	"PDDG-80":  "Permissioned dispute game clock extension not set to 10800",
	"PDDG-90":  "Permissioned dispute game split depth not set to 30",
	"PDDG-100": "Permissioned dispute game max game depth not set to 73",
	"PDDG-110": "Permissioned dispute game max clock duration not set to 302400",
	"PDDG-120": "Permissioned dispute game challenger address mismatch",

	// Permissionless Dispute Game validations
	"PLDG-10":  "Permissionless dispute game implementation not found",
	"PLDG-20":  "Permissionless dispute game version mismatch",
	"PLDG-30":  "Permissionless dispute game type mismatch",
	"PLDG-40":  "Permissionless dispute game absolute prestate mismatch",
	"PLDG-50":  "Permissionless dispute game VM address mismatch",
	"PLDG-60":  "Permissionless dispute game L2 chain ID mismatch",
	"PLDG-70":  "Permissionless dispute game L2 block number not set to 0",
	"PLDG-80":  "Permissionless dispute game clock extension not set to 10800",
	"PLDG-90":  "Permissionless dispute game split depth not set to 30",
	"PLDG-100": "Permissionless dispute game max game depth not set to 73",
	"PLDG-110": "Permissionless dispute game max clock duration not set to 302400",

	// Delayed WETH validations (for both PDDG and PLDG)
	"PDDG-DWETH-10": "Permissioned dispute game delayed WETH version mismatch",
	"PDDG-DWETH-20": "Permissioned dispute game delayed WETH implementation address mismatch",
	"PDDG-DWETH-30": "Permissioned dispute game delayed WETH owner mismatch",
	"PDDG-DWETH-40": "Permissioned dispute game delayed WETH delay not set to 1 week",
	"PLDG-DWETH-10": "Permissionless dispute game delayed WETH version mismatch",
	"PLDG-DWETH-20": "Permissionless dispute game delayed WETH implementation address mismatch",
	"PLDG-DWETH-30": "Permissionless dispute game delayed WETH owner mismatch",
	"PLDG-DWETH-40": "Permissionless dispute game delayed WETH delay not set to 1 week",

	// Anchor State Registry validations (for both PDDG and PLDG)
	"PDDG-ANCHORP-10": "Permissioned dispute game anchor state registry version mismatch",
	"PDDG-ANCHORP-20": "Permissioned dispute game anchor state registry implementation address mismatch",
	"PDDG-ANCHORP-30": "Permissioned dispute game anchor state registry dispute game factory address mismatch",
	"PDDG-ANCHORP-40": "Permissioned dispute game anchor state registry root hash mismatch",
	"PDDG-ANCHORP-50": "Permissioned dispute game anchor state registry superchain config address mismatch",
	"PLDG-ANCHORP-10": "Permissionless dispute game anchor state registry version mismatch",
	"PLDG-ANCHORP-20": "Permissionless dispute game anchor state registry implementation address mismatch",
	"PLDG-ANCHORP-30": "Permissionless dispute game anchor state registry dispute game factory address mismatch",
	"PLDG-ANCHORP-40": "Permissionless dispute game anchor state registry root hash mismatch",
	"PLDG-ANCHORP-50": "Permissionless dispute game anchor state registry superchain config address mismatch",

	// Preimage Oracle validations (for both PDDG and PLDG)
	"PDDG-PIMGO-10": "Permissioned dispute game preimage oracle version mismatch",
	"PDDG-PIMGO-20": "Permissioned dispute game preimage oracle challenge period not set to 86400",
	"PDDG-PIMGO-30": "Permissioned dispute game preimage oracle min proposal size not set to 126000",
	"PLDG-PIMGO-10": "Permissionless dispute game preimage oracle version mismatch",
	"PLDG-PIMGO-20": "Permissionless dispute game preimage oracle challenge period not set to 86400",
	"PLDG-PIMGO-30": "Permissionless dispute game preimage oracle min proposal size not set to 126000",
}

func ErrorDescription(code string) string {
	return descriptions[code]
}
