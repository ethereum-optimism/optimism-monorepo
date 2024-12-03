package standard

import (
	"embed"
	"fmt"
	"net/url"

	"github.com/BurntSushi/toml"

	"github.com/ethereum-optimism/superchain-registry/superchain"
	"github.com/ethereum/go-ethereum/common"
)

const (
	GasLimit                        uint64 = 60_000_000
	BasefeeScalar                   uint32 = 1368
	BlobBaseFeeScalar               uint32 = 801949
	WithdrawalDelaySeconds          uint64 = 604800
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
	ContractsV170Beta1L2Tag = "op-contracts/v1.7.0-beta.1+l2-contracts"
)

var DisputeAbsolutePrestate = common.HexToHash("0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c")

//go:embed standard-versions-mainnet.toml
var VersionsMainnetData string

//go:embed standard-versions-sepolia.toml
var VersionsSepoliaData string

var L1VersionsSepolia L1Versions

var L1VersionsMainnet L1Versions

var DefaultL1ContractsTag = ContractsV160Tag

var DefaultL2ContractsTag = ContractsV170Beta1L2Tag

type L1Versions struct {
	Releases map[string]L1VersionsReleases `toml:"releases"`
}

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

var sepoliaBlueprints = OPCMBlueprints{
	AddressManager:           common.HexToAddress("0x3125a4cB2179E04203D3Eb2b5784aaef9FD64216"),
	Proxy:                    common.HexToAddress("0xe650ADb86a0de96e2c434D0a52E7D5B70980D6f1"),
	ProxyAdmin:               common.HexToAddress("0x3AC6b88F6bC4A5038DB7718dE47a5ab1a9609319"),
	L1ChugSplashProxy:        common.HexToAddress("0x58770FC7ed304c43D2B70248914eb34A741cF411"),
	ResolvedDelegateProxy:    common.HexToAddress("0x0449adB72D489a137d476aB49c6b812161754fD3"),
	AnchorStateRegistry:      common.HexToAddress("0xB98095199437883b7661E0D58256060f3bc730a4"),
	PermissionedDisputeGame1: common.HexToAddress("0xf72Ac5f164cC024DE09a2c249441715b69a16eAb"),
	PermissionedDisputeGame2: common.HexToAddress("0x713dAC5A23728477547b484f9e0D751077E300a2"),
}

var mainnetBlueprints = OPCMBlueprints{
	AddressManager:           common.HexToAddress("0x29aA24714c06914d9689e933cae2293C569AfeEa"),
	Proxy:                    common.HexToAddress("0x3626ebD458c7f34FD98789A373593fF2fc227bA0"),
	ProxyAdmin:               common.HexToAddress("0x7170678A5CFFb6872606d251B3CcdB27De962631"),
	L1ChugSplashProxy:        common.HexToAddress("0x538906C8B000D621fd11B7e8642f504dD8730837"),
	ResolvedDelegateProxy:    common.HexToAddress("0xF12bD34d6a1d26d230240ECEA761f77e2013926E"),
	AnchorStateRegistry:      common.HexToAddress("0xbA7Be2bEE016568274a4D1E6c852Bb9a99FaAB8B"),
	PermissionedDisputeGame1: common.HexToAddress("0xb94bF6130Df8BD9a9eA45D8dD8C18957002d1986"),
	PermissionedDisputeGame2: common.HexToAddress("0xe0a642B249CF6cbF0fF7b4dDf41443Ea7a5C8Cc8"),
}

func OPCMBlueprintsFor(chainID uint64) (OPCMBlueprints, error) {
	switch chainID {
	case 1:
		return mainnetBlueprints, nil
	case 11155111:
		return sepoliaBlueprints, nil
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
		return common.HexToAddress("0x09f7150D8c019BeF34450d6920f6B3608ceFdAf2"), nil
	case 11155111:
		return common.HexToAddress("0x7a50f00e8D05b95F98fE38d8BeE366a7324dCf7E"), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func ChallengerAddressFor(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		return common.HexToAddress("0x9BA6e03D8B90dE867373Db8cF1A58d2F7F006b3A"), nil
	case 11155111:
		return common.HexToAddress("0xfd1D2e729aE8eEe2E146c033bf4400fE75284301"), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func SuperchainFor(chainID uint64) (*superchain.Superchain, error) {
	switch chainID {
	case 1:
		return superchain.Superchains["mainnet"], nil
	case 11155111:
		return superchain.Superchains["sepolia"], nil
	default:
		return nil, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func ChainNameFor(chainID uint64) (string, error) {
	switch chainID {
	case 1:
		return "mainnet", nil
	case 11155111:
		return "sepolia", nil
	default:
		return "", fmt.Errorf("unrecognized l1 chain ID: %d", chainID)
	}
}

func CommitForDeployTag(tag string) (string, error) {
	switch tag {
	case "op-contracts/v1.6.0":
		return "33f06d2d5e4034125df02264a5ffe84571bd0359", nil
	case "op-contracts/v1.7.0-beta.1+l2-contracts":
		return "5e14a61547a45eef2ebeba677aee4a049f106ed8", nil
	default:
		return "", fmt.Errorf("unsupported tag: %s", tag)
	}
}

func ManagerImplementationAddrFor(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		// Generated using the bootstrap command on 11/18/2024.
		// Verified against compiled bytecode at:
		// https://github.com/ethereum-optimism/optimism/releases/tag/op-contracts-v160-artifacts-opcm-redesign-backport
		return common.HexToAddress("0x9BC0A1eD534BFb31a6Be69e5b767Cba332f14347"), nil
	case 11155111:
		// Generated using the bootstrap command on 11/18/2024.
		// Verified against compiled bytecode at:
		// https://github.com/ethereum-optimism/optimism/releases/tag/op-contracts-v160-artifacts-opcm-redesign-backport
		return common.HexToAddress("0x760B1d2Dc68DC51fb6E8B2b8722B8ed08903540c"), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func ManagerOwnerAddrFor(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		// Set to superchain proxy admin
		return common.HexToAddress("0x543bA4AADBAb8f9025686Bd03993043599c6fB04"), nil
	case 11155111:
		// Set to development multisig
		return common.HexToAddress("0xDEe57160aAfCF04c34C887B5962D0a69676d3C8B"), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func SystemOwnerAddrFor(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		// Set to owner of superchain proxy admin
		return common.HexToAddress("0x5a0Aae59D09fccBdDb6C6CcEB07B7279367C3d2A"), nil
	case 11155111:
		// Set to development multisig
		return common.HexToAddress("0xDEe57160aAfCF04c34C887B5962D0a69676d3C8B"), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func L1ProxyAdminOwner(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		return common.HexToAddress("0x5a0Aae59D09fccBdDb6C6CcEB07B7279367C3d2A"), nil
	case 11155111:
		return common.HexToAddress("0x1Eb2fFc903729a0F03966B917003800b145F56E2"), nil
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
	case "op-contracts/v1.8.0-rc.3":
		return url.Parse(standardArtifactsURL("3bcff2944953862596d5fd0125d166a04af2ba6426dc693983291d3cb86b2e2e"))
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
	case "op-contracts/v1.8.0-rc.3":
		return common.HexToHash("7c133142165fbbdba28ced5d9a04af8bea68baf58b19a07cdd8ae531b01fbe9d"), nil
	default:
		return common.Hash{}, fmt.Errorf("unsupported tag: %s", tag)
	}
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
