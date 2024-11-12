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
		return "", fmt.Errorf("unrecognized chain ID: %d", chainID)
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
		// Generated using the bootstrap command on 10/18/2024.
		// TODO: @blmalone this needs re-bootstrapped because it's still proxied
		return common.HexToAddress(""), nil
	case 11155111:
		// Generated using the bootstrap command on 11/15/2024.
		return common.HexToAddress("0xde9eacb994a6eb12997445f8a63a22772c5c4313"), nil
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

func ArtifactsURLForTag(tag string) (*url.URL, error) {
	switch tag {
	case "op-contracts/v1.6.0":
		return url.Parse(standardArtifactsURL("e1f0c4020618c4a98972e7124c39686cab2e31d5d7846f9ce5e0d5eed0f5ff32"))
	case "op-contracts/v1.7.0-beta.1+l2-contracts":
		return url.Parse(standardArtifactsURL("b0fb1f6f674519d637cff39a22187a5993d7f81a6d7b7be6507a0b50a5e38597"))
	default:
		return nil, fmt.Errorf("unsupported tag: %s", tag)
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
