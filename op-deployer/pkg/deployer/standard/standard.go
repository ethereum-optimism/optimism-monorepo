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
	ContractsV200Tag        = "op-contracts/v2.0.0-rc.1"
)

var DisputeAbsolutePrestate = common.HexToHash("0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c")

var DefaultL1ContractsTag = ContractsV200Tag

var DefaultL2ContractsTag = ContractsV170Beta1L2Tag

type TaggedRelease struct {
	ArtifactsHash common.Hash
	ContentHash   common.Hash
}

func (t TaggedRelease) URL() string {
	return fmt.Sprintf("https://storage.googleapis.com/oplabs-contract-artifacts/artifacts-v1-%x.tar.gz", t.ContentHash)
}

var taggedReleases = map[string]TaggedRelease{
	ContractsV160Tag: {
		ArtifactsHash: common.HexToHash("d20a930cc0ff204c2d93b7aa60755ec7859ba4f328b881f5090c6a6a2a86dcba"),
		ContentHash:   common.HexToHash("e1f0c4020618c4a98972e7124c39686cab2e31d5d7846f9ce5e0d5eed0f5ff32"),
	},
	ContractsV170Beta1L2Tag: {
		ArtifactsHash: common.HexToHash("9e3ad322ec9b2775d59143ce6874892f9b04781742c603ad59165159e90b00b9"),
		ContentHash:   common.HexToHash("b0fb1f6f674519d637cff39a22187a5993d7f81a6d7b7be6507a0b50a5e38597"),
	},
	ContractsV180Tag: {
		ArtifactsHash: common.HexToHash("78f186df4e9a02a6421bd9c3641b281e297535140967faa428c938286923976a"),
		ContentHash:   common.HexToHash("361ebf1f520c20d932695b00babfff6923ce2530cd05b2776eb74e07038898a6"),
	},
	ContractsV200Tag: {
		ArtifactsHash: common.HexToHash("32e11c96e07b83619f419595facb273368dccfe2439287549e7b436c9b522204"),
		ContentHash:   common.HexToHash("1cec51ed629c0394b8fb17ff2c6fa45c406c30f94ebbd37d4c90ede6c29ad608"),
	},
}

var _ embed.FS

func IsSupportedL1Version(tag string) bool {
	return tag == ContractsV200Tag
}

func IsSupportedL2Version(tag string) bool {
	return tag == ContractsV170Beta1L2Tag
}

func L1VersionsFor(chainID uint64) (validation.Versions, error) {
	switch chainID {
	case 1:
		return validation.StandardVersionsMainnet, nil
	case 11155111:
		return validation.StandardVersionsSepolia, nil
	default:
		return nil, fmt.Errorf("unsupported chain ID: %d", chainID)
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
	versionsData, err := L1VersionsFor(chainID)
	if err != nil {
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}
	versionData, ok := versionsData[validation.Semver(tag)]
	if !ok {
		return common.Address{}, fmt.Errorf("unsupported tag for chain ID %d: %s", chainID, tag)
	}
	return common.Address(*versionData.OPContractsManager.Address), nil
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
	release, ok := taggedReleases[tag]
	if !ok {
		return nil, fmt.Errorf("unsupported tag: %s", tag)
	}

	return url.Parse(release.URL())
}

func ArtifactsHashForTag(tag string) (common.Hash, error) {
	release, ok := taggedReleases[tag]
	if !ok {
		return common.Hash{}, fmt.Errorf("unsupported tag: %s", tag)
	}
	return release.ArtifactsHash, nil
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
