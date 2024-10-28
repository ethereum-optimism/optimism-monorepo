package opcm

import (
	"embed"
	"fmt"
	"net/url"

	"github.com/BurntSushi/toml"

	"github.com/ethereum-optimism/superchain-registry/superchain"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed standard-versions-mainnet.toml
var StandardVersionsMainnetData string

//go:embed standard-versions-sepolia.toml
var StandardVersionsSepoliaData string

var StandardL1VersionsSepolia StandardL1Versions

var StandardL1VersionsMainnet StandardL1Versions

var DefaultL1ContractsLocator = &ArtifactsLocator{
	Tag: "op-contracts/v1.6.0",
}

var DefaultL2ContractsLocator = &ArtifactsLocator{
	Tag: "op-contracts/v1.7.0-beta.1+l2-contracts",
}

type StandardL1Versions struct {
	Releases map[string]StandardL1VersionsReleases `toml:"releases"`
}

type StandardL1VersionsReleases struct {
	OptimismPortal               StandardVersionRelease `toml:"optimism_portal"`
	SystemConfig                 StandardVersionRelease `toml:"system_config"`
	AnchorStateRegistry          StandardVersionRelease `toml:"anchor_state_registry"`
	DelayedWETH                  StandardVersionRelease `toml:"delayed_weth"`
	DisputeGameFactory           StandardVersionRelease `toml:"dispute_game_factory"`
	FaultDisputeGame             StandardVersionRelease `toml:"fault_dispute_game"`
	PermissionedDisputeGame      StandardVersionRelease `toml:"permissioned_dispute_game"`
	MIPS                         StandardVersionRelease `toml:"mips"`
	PreimageOracle               StandardVersionRelease `toml:"preimage_oracle"`
	L1CrossDomainMessenger       StandardVersionRelease `toml:"l1_cross_domain_messenger"`
	L1ERC721Bridge               StandardVersionRelease `toml:"l1_erc721_bridge"`
	L1StandardBridge             StandardVersionRelease `toml:"l1_standard_bridge"`
	OptimismMintableERC20Factory StandardVersionRelease `toml:"optimism_mintable_erc20_factory"`
}

type StandardVersionRelease struct {
	Version               string         `toml:"version"`
	ImplementationAddress common.Address `toml:"implementation_address"`
	Address               common.Address `toml:"address"`
}

var _ embed.FS

func StandardL1VersionsDataFor(chainID uint64) (string, error) {
	switch chainID {
	case 1:
		return StandardVersionsMainnetData, nil
	case 11155111:
		return StandardVersionsSepoliaData, nil
	default:
		return "", fmt.Errorf("unsupported chain ID: %d", chainID)
	}
}

func StandardL1VersionsFor(chainID uint64) (StandardL1Versions, error) {
	switch chainID {
	case 1:
		return StandardL1VersionsMainnet, nil
	case 11155111:
		return StandardL1VersionsSepolia, nil
	default:
		return StandardL1Versions{}, fmt.Errorf("unsupported chain ID: %d", chainID)
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

func ManagerImplementationAddrFor(chainID uint64) (common.Address, error) {
	switch chainID {
	case 1:
		// Generated using the bootstrap command on 10/18/2024.
		return common.HexToAddress("0x18cec91779995ad14c880e4095456b9147160790"), nil
	case 11155111:
		// Generated using the bootstrap command on 10/18/2024.
		return common.HexToAddress("0xf564eea7960ea244bfebcbbb17858748606147bf"), nil
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

func StandardArtifactsURLForTag(tag string) (*url.URL, error) {
	switch tag {
	case "op-contracts/v1.6.0":
		return url.Parse(standardArtifactsURL("ee07c78c3d8d4cd8f7a933c050f5afeebaa281b57b226cc6f092b19de2a8d61f"))
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
	StandardL1VersionsMainnet = StandardL1Versions{}
	if err := toml.Unmarshal([]byte(StandardVersionsMainnetData), &StandardL1VersionsMainnet); err != nil {
		panic(err)
	}

	StandardL1VersionsSepolia = StandardL1Versions{}
	if err := toml.Unmarshal([]byte(StandardVersionsSepoliaData), &StandardL1VersionsSepolia); err != nil {
		panic(err)
	}
}
