package standard

import (
	"embed"
	"fmt"
	"net/url"

	"github.com/ethereum-optimism/superchain-registry/validation"

	"github.com/ethereum/go-ethereum/superchain"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	op_service "github.com/ethereum-optimism/optimism/op-service"

	"github.com/BurntSushi/toml"

	"github.com/ethereum/go-ethereum/common"
)

const (
	GasLimit                        uint64 = 60_000_000
	BasefeeScalar                   uint32 = 1368
	BlobBaseFeeScalar               uint32 = 801949
	WithdrawalDelaySeconds          uint64 = 302400
	MinProposalSizeBytes            uint64 = 126000
	ChallengePeriodSeconds          uint64 = 86400
	ProofMaturityDelaySeconds       uint64 = 604800
	DisputeGameFinalityDelaySeconds uint64 = 302400
	MIPSVersion                     uint64 = 1
	DisputeGameType                 uint32 = 1 // PERMISSIONED game type
	DisputeMaxGameDepth             uint64 = 73
	DisputeSplitDepth               uint64 = 30
	DisputeClockExtension           uint64 = 10800
	DisputeMaxClockDuration         uint64 = 302400
	Eip1559DenominatorCanyon        uint64 = 250
	Eip1559Denominator              uint64 = 50
	Eip1559Elasticity               uint64 = 6

	ContractsV160Tag        = "op-contracts/v1.6.0"
	ContractsV180Tag        = "op-contracts/v1.8.0-rc.4"
	ContractsV170Beta1L2Tag = "op-contracts/v1.7.0-beta.1+l2-contracts"
)

var DisputeAbsolutePrestate = common.HexToHash("0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c")

//go:embed standard-versions-mainnet.toml
var VersionsMainnetData string

//go:embed standard-versions-sepolia.toml
var VersionsSepoliaData string

var L1VersionsSepolia L1Versions

var L1VersionsMainnet L1Versions

var DefaultL1ContractsTag = ContractsV180Tag

var DefaultL2ContractsTag = ContractsV170Beta1L2Tag

type L1Versions map[string]L1VersionsReleases

type L1VersionsReleases struct {
	OptimismPortal               VersionRelease `toml:"optimism_portal"`
	SystemConfig                 VersionRelease `toml:"system_config"`
	AnchorStateRegistry          VersionRelease `toml:"anchor_state_registry"`
	DelayedWETH                  VersionRelease `toml:"delayed_weth"`
	DisputeGameFactory           VersionRelease `toml:"dispute_game_factory"`
	FaultDisputeGame             VersionRelease `toml:"fault_dispute_game"`
	PermissionedDisputeGame      VersionRelease `toml:"permissioned_dispute_game"`
	MIPS                         VersionRelease `toml:"mips"`
	PreimageOracle               VersionRelease `toml:"preimage_oracle"`
	L1CrossDomainMessenger       VersionRelease `toml:"l1_cross_domain_messenger"`
	L1ERC721Bridge               VersionRelease `toml:"l1_erc721_bridge"`
	L1StandardBridge             VersionRelease `toml:"l1_standard_bridge"`
	OptimismMintableERC20Factory VersionRelease `toml:"optimism_mintable_erc20_factory"`
}

type VersionRelease struct {
	Version               string         `toml:"version"`
	ImplementationAddress common.Address `toml:"implementation_address"`
	Address               common.Address `toml:"address"`
}

var _ embed.FS

type OPCMBlueprints struct {
	AddressManager           common.Address
	Proxy                    common.Address
	ProxyAdmin               common.Address
	L1ChugSplashProxy        common.Address
	ResolvedDelegateProxy    common.Address
	AnchorStateRegistry      common.Address
	PermissionedDisputeGame1 common.Address
	PermissionedDisputeGame2 common.Address
}

type OPCMBlueprintsByChain struct {
	Mainnet *OPCMBlueprints
	Sepolia *OPCMBlueprints
}

var opcmBlueprintsByVersion = map[string]OPCMBlueprintsByChain{
	"op-contracts/v1.6.0": {
		Mainnet: &OPCMBlueprints{
			AddressManager:           common.HexToAddress("0x29aA24714c06914d9689e933cae2293C569AfeEa"),
			Proxy:                    common.HexToAddress("0x3626ebD458c7f34FD98789A373593fF2fc227bA0"),
			ProxyAdmin:               common.HexToAddress("0x7170678A5CFFb6872606d251B3CcdB27De962631"),
			L1ChugSplashProxy:        common.HexToAddress("0x538906C8B000D621fd11B7e8642f504dD8730837"),
			ResolvedDelegateProxy:    common.HexToAddress("0xF12bD34d6a1d26d230240ECEA761f77e2013926E"),
			AnchorStateRegistry:      common.HexToAddress("0xbA7Be2bEE016568274a4D1E6c852Bb9a99FaAB8B"),
			PermissionedDisputeGame1: common.HexToAddress("0xb94bF6130Df8BD9a9eA45D8dD8C18957002d1986"),
			PermissionedDisputeGame2: common.HexToAddress("0xe0a642B249CF6cbF0fF7b4dDf41443Ea7a5C8Cc8"),
		},
		Sepolia: &OPCMBlueprints{
			AddressManager:           common.HexToAddress("0x3125a4cB2179E04203D3Eb2b5784aaef9FD64216"),
			Proxy:                    common.HexToAddress("0xe650ADb86a0de96e2c434D0a52E7D5B70980D6f1"),
			ProxyAdmin:               common.HexToAddress("0x3AC6b88F6bC4A5038DB7718dE47a5ab1a9609319"),
			L1ChugSplashProxy:        common.HexToAddress("0x58770FC7ed304c43D2B70248914eb34A741cF411"),
			ResolvedDelegateProxy:    common.HexToAddress("0x0449adB72D489a137d476aB49c6b812161754fD3"),
			AnchorStateRegistry:      common.HexToAddress("0xB98095199437883b7661E0D58256060f3bc730a4"),
			PermissionedDisputeGame1: common.HexToAddress("0xf72Ac5f164cC024DE09a2c249441715b69a16eAb"),
			PermissionedDisputeGame2: common.HexToAddress("0x713dAC5A23728477547b484f9e0D751077E300a2"),
		},
	},
	"op-contracts/v1.8.0-rc.4": {
		Mainnet: &OPCMBlueprints{
			AddressManager:           common.HexToAddress("0x29aA24714c06914d9689e933cae2293C569AfeEa"),
			Proxy:                    common.HexToAddress("0x3626ebD458c7f34FD98789A373593fF2fc227bA0"),
			ProxyAdmin:               common.HexToAddress("0x7170678A5CFFb6872606d251B3CcdB27De962631"),
			L1ChugSplashProxy:        common.HexToAddress("0x538906C8B000D621fd11B7e8642f504dD8730837"),
			ResolvedDelegateProxy:    common.HexToAddress("0xF12bD34d6a1d26d230240ECEA761f77e2013926E"),
			AnchorStateRegistry:      common.HexToAddress("0xbA7Be2bEE016568274a4D1E6c852Bb9a99FaAB8B"),
			PermissionedDisputeGame1: common.HexToAddress("0x596A4334a28056c7943c8bcEf220F38cA5B42dC5"), // updated
			PermissionedDisputeGame2: common.HexToAddress("0x4E3E5C09B07AAA3fe482F5A1f82a19e91944Fffc"), // updated
		},
		Sepolia: &OPCMBlueprints{
			AddressManager:           common.HexToAddress("0x3125a4cB2179E04203D3Eb2b5784aaef9FD64216"),
			Proxy:                    common.HexToAddress("0xe650ADb86a0de96e2c434D0a52E7D5B70980D6f1"),
			ProxyAdmin:               common.HexToAddress("0x3AC6b88F6bC4A5038DB7718dE47a5ab1a9609319"),
			L1ChugSplashProxy:        common.HexToAddress("0x58770FC7ed304c43D2B70248914eb34A741cF411"),
			ResolvedDelegateProxy:    common.HexToAddress("0x0449adB72D489a137d476aB49c6b812161754fD3"),
			AnchorStateRegistry:      common.HexToAddress("0xB98095199437883b7661E0D58256060f3bc730a4"),
			PermissionedDisputeGame1: common.HexToAddress("0x596A4334a28056c7943c8bcEf220F38cA5B42dC5"), // updated
			PermissionedDisputeGame2: common.HexToAddress("0x4E3E5C09B07AAA3fe482F5A1f82a19e91944Fffc"), // updated
		},
	},
}

func OPCMBlueprintsFor(chainID uint64, version string) (OPCMBlueprints, error) {
	switch chainID {
	case 1:
		bps := opcmBlueprintsByVersion[version].Mainnet
		if bps == nil {
			return OPCMBlueprints{}, fmt.Errorf("unsupported version: %s", version)
		}
		return *bps, nil
	case 11155111:
		bps := opcmBlueprintsByVersion[version].Sepolia
		if bps == nil {
			return OPCMBlueprints{}, fmt.Errorf("unsupported version: %s", version)
		}
		return *bps, nil
	default:
		return OPCMBlueprints{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func L1VersionsDataFor(chainID uint64) (string, error) {
	switch chainID {
	case 1:
		return VersionsMainnetData, nil
	case 11155111:
		return VersionsSepoliaData, nil
	default:
		return "", fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func L1VersionsFor(chainID uint64) (L1Versions, error) {
	switch chainID {
	case 1:
		return L1VersionsMainnet, nil
	case 11155111:
		return L1VersionsSepolia, nil
	default:
		return L1Versions{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func GuardianAddressFor(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		return common.Address(validation.StandardConfigRolesMainnet.Guardian), nil
	case 11155111:
		return common.Address(validation.StandardConfigRolesSepolia.Guardian), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func ChallengerAddressFor(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		return common.Address(validation.StandardConfigRolesMainnet.Challenger), nil
	case 11155111:
		return common.Address(validation.StandardConfigRolesSepolia.Challenger), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func SuperchainFor(chainID uint64) (superchain.Superchain, error) {
	switch chainID {
	case 1:
		return superchain.GetSuperchain("mainnet")
	case 11155111:
		return superchain.GetSuperchain("sepolia")
	default:
		return superchain.Superchain{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func ManagerImplementationAddrFor(chainID uint64, tag string) (common.Address, error) {
	switch chainID {
	case 1:
		switch tag {
		case "op-contracts/v1.6.0":
			// Generated using the bootstrap command on 11/18/2024.
			// Verified against compiled bytecode at:
			// https://github.com/ethereum-optimism/optimism/releases/tag/op-contracts-v160-artifacts-opcm-redesign-backport
			return common.HexToAddress("0x9BC0A1eD534BFb31a6Be69e5b767Cba332f14347"), nil
		case "op-contracts/v1.8.0-rc.4":
			// Generated using the bootstrap command on 01/23/2025.
			// Verified against compiled bytecode at:
			// https://github.com/ethereum-optimism/optimism/releases/tag/op-contracts-v180-blueprints-script
			return common.HexToAddress("0x5269eed89b0d04d909a0973439e2587e815ba932"), nil
		default:
			return common.Address{}, fmt.Errorf("unsupported mainnet tag: %s", tag)
		}
	case 11155111:
		switch tag {
		case "op-contracts/v1.6.0":
			// Generated using the bootstrap command on 11/18/2024.
			// Verified against compiled bytecode at:
			// https://github.com/ethereum-optimism/optimism/releases/tag/op-contracts-v160-artifacts-opcm-redesign-backport
			return common.HexToAddress("0x760B1d2Dc68DC51fb6E8B2b8722B8ed08903540c"), nil
		case "op-contracts/v1.8.0-rc.4":
			// Generated using the bootstrap command on 12/19/2024.
			return common.HexToAddress("0xefb0779120d9cc3582747e5eb787d859e3a53a5c"), nil
		default:
			return common.Address{}, fmt.Errorf("unsupported sepolia tag: %s", tag)
		}
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

// SuperchainProxyAdminAddrFor returns the address of the Superchain ProxyAdmin for the given chain ID.
// These have been verified to be the ProxyAdmin addresses on Mainnet and Sepolia.
// DO NOT MODIFY THIS METHOD WITHOUT CLEARING IT WITH THE EVM SAFETY TEAM.
func SuperchainProxyAdminAddrFor(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		return common.HexToAddress("0x543bA4AADBAb8f9025686Bd03993043599c6fB04"), nil
	case 11155111:
		return common.HexToAddress("0x189aBAAaa82DfC015A588A7dbaD6F13b1D3485Bc"), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func L1ProxyAdminOwner(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		return common.Address(validation.StandardConfigRolesMainnet.L1ProxyAdminOwner), nil
	case 11155111:
		return common.Address(validation.StandardConfigRolesSepolia.L1ProxyAdminOwner), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func ProtocolVersionsOwner(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		return common.Address(validation.StandardConfigRolesMainnet.ProtocolVersionsOwner), nil
	case 11155111:
		return common.Address(validation.StandardConfigRolesSepolia.ProtocolVersionsOwner), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func ArtifactsURLForTag(tag string) (*url.URL, error) {
	switch tag {
	case "op-contracts/v1.6.0":
		return url.Parse(standardArtifactsURL("e1f0c4020618c4a98972e7124c39686cab2e31d5d7846f9ce5e0d5eed0f5ff32"))
	case "op-contracts/v1.7.0-beta.1+l2-contracts":
		return url.Parse(standardArtifactsURL("b0fb1f6f674519d637cff39a22187a5993d7f81a6d7b7be6507a0b50a5e38597"))
	case "op-contracts/v1.8.0-rc.4":
		return url.Parse(standardArtifactsURL("361ebf1f520c20d932695b00babfff6923ce2530cd05b2776eb74e07038898a6"))
	default:
		return nil, fmt.Errorf("unsupported tag: %s", tag)
	}
}

func ArtifactsHashForTag(tag string) (common.Hash, error) {
	switch tag {
	case "op-contracts/v1.6.0":
		return common.HexToHash("d20a930cc0ff204c2d93b7aa60755ec7859ba4f328b881f5090c6a6a2a86dcba"), nil
	case "op-contracts/v1.7.0-beta.1+l2-contracts":
		return common.HexToHash("9e3ad322ec9b2775d59143ce6874892f9b04781742c603ad59165159e90b00b9"), nil
	case "op-contracts/v1.8.0-rc.4":
		return common.HexToHash("78f186df4e9a02a6421bd9c3641b281e297535140967faa428c938286923976a"), nil
	default:
		return common.Hash{}, fmt.Errorf("unsupported tag: %s", tag)
	}
}

// DefaultHardforkScheduleForTag is used to determine which hardforks should be activated by default given a
// contract tag. For example, passing in v1.6.0 will return all hardforks up to and including Granite. This allows
// OP Deployer to set sane defaults for hardforks. This is not an ideal solution, but it will have to work until we get
// to MCP L2.
func DefaultHardforkScheduleForTag(tag string) *genesis.UpgradeScheduleDeployConfig {
	sched := &genesis.UpgradeScheduleDeployConfig{
		L2GenesisRegolithTimeOffset: op_service.U64UtilPtr(0),
		L2GenesisCanyonTimeOffset:   op_service.U64UtilPtr(0),
		L2GenesisDeltaTimeOffset:    op_service.U64UtilPtr(0),
		L2GenesisEcotoneTimeOffset:  op_service.U64UtilPtr(0),
		L2GenesisFjordTimeOffset:    op_service.U64UtilPtr(0),
		L2GenesisGraniteTimeOffset:  op_service.U64UtilPtr(0),
	}

	switch tag {
	case ContractsV160Tag, ContractsV170Beta1L2Tag:
		return sched
	default:
		sched.ActivateForkAtGenesis(rollup.Holocene)
	}

	return sched
}

func standardArtifactsURL(checksum string) string {
	return fmt.Sprintf("https://storage.googleapis.com/oplabs-contract-artifacts/artifacts-v1-%s.tar.gz", checksum)
}

func init() {
	L1VersionsMainnet = L1Versions{}
	if err := toml.Unmarshal([]byte(VersionsMainnetData), &L1VersionsMainnet); err != nil {
		panic(err)
	}

	L1VersionsSepolia = L1Versions{}
	if err := toml.Unmarshal([]byte(VersionsSepoliaData), &L1VersionsSepolia); err != nil {
		panic(err)
	}
}
