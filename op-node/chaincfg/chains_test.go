package chaincfg

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// TestGetRollupConfig tests that the configs sourced from the superchain-registry match
// the configs that were embedded in the op-node manually before the superchain-registry was utilized.
//
// The superchain-registry repository is a work in progress.
// At a later date, it will be proposed to, and must be approved by, Optimism Governance.
// Until that time, the configuration described in the superchain-registry is subject to change.
//
// This test ensures no op-node config-loading behavior changes before
// the superchain-registry is no longer deemed experimental.
func TestGetRollupConfig(t *testing.T) {
	configsByName := map[string]rollup.Config{
		"mainnet":                       mainnetCfg,
		"sepolia":                       sepoliaCfg,
		"oplabs-devnet-0-sepolia-dev-0": sepoliaDev0Cfg,
	}

	for name, expectedCfg := range configsByName {
		gotCfg, err := GetRollupConfig(name)
		require.NoError(t, err)

		require.Equalf(t, expectedCfg, *gotCfg, "rollup-configs from superchain-registry must match for %v", name)
	}
}

var mainnetCfg = rollup.Config{
	Genesis: rollup.Genesis{
		L1: eth.BlockID{
			Hash:   common.HexToHash("0x438335a20d98863a4c0c97999eb2481921ccd28553eac6f913af7c12aec04108"),
			Number: 17422590,
		},
		L2: eth.BlockID{
			Hash:   common.HexToHash("0xdbf6a80fef073de06add9b0d14026d6e5a86c85f6d102c36d3d8e9cf89c2afd3"),
			Number: 105235063,
		},
		L2Time: 1686068903,
		SystemConfig: eth.SystemConfig{
			BatcherAddr: common.HexToAddress("0x6887246668a3b87f54deb3b94ba47a6f63f32985"),
			Overhead:    eth.Bytes32(common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000bc")),
			Scalar:      eth.Bytes32(common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000a6fe0")),
			GasLimit:    30_000_000,
		},
	},
	BlockTime:              2,
	MaxSequencerDrift:      600,
	SeqWindowSize:          3600,
	ChannelTimeoutBedrock:  300,
	L1ChainID:              big.NewInt(1),
	L2ChainID:              big.NewInt(10),
	BatchInboxAddress:      common.HexToAddress("0xff00000000000000000000000000000000000010"),
	DepositContractAddress: common.HexToAddress("0xbEb5Fc579115071764c7423A4f12eDde41f106Ed"),
	L1SystemConfigAddress:  common.HexToAddress("0x229047fed2591dbec1eF1118d64F7aF3dB9EB290"),
	RegolithTime:           u64Ptr(0),
	CanyonTime:             u64Ptr(1704992401),
	DeltaTime:              u64Ptr(1708560000),
	EcotoneTime:            u64Ptr(1710374401),
	FjordTime:              u64Ptr(1720627201),
	GraniteTime:            u64Ptr(1726070401),
	// HoloceneTime: TBD
	ProtocolVersionsAddress: common.HexToAddress("0x8062AbC286f5e7D9428a0Ccb9AbD71e50d93b935"),
}

var sepoliaCfg = rollup.Config{
	Genesis: rollup.Genesis{
		L1: eth.BlockID{
			Hash:   common.HexToHash("0x48f520cf4ddaf34c8336e6e490632ea3cf1e5e93b0b2bc6e917557e31845371b"),
			Number: 4071408,
		},
		L2: eth.BlockID{
			Hash:   common.HexToHash("0x102de6ffb001480cc9b8b548fd05c34cd4f46ae4aa91759393db90ea0409887d"),
			Number: 0,
		},
		L2Time: 1691802540,
		SystemConfig: eth.SystemConfig{
			BatcherAddr: common.HexToAddress("0x8F23BB38F531600e5d8FDDaAEC41F13FaB46E98c"),
			Overhead:    eth.Bytes32(common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000bc")),
			Scalar:      eth.Bytes32(common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000a6fe0")),
			GasLimit:    30000000,
		},
	},
	BlockTime:               2,
	MaxSequencerDrift:       600,
	SeqWindowSize:           3600,
	ChannelTimeoutBedrock:   300,
	L1ChainID:               big.NewInt(11155111),
	L2ChainID:               big.NewInt(11155420),
	BatchInboxAddress:       common.HexToAddress("0xff00000000000000000000000000000011155420"),
	DepositContractAddress:  common.HexToAddress("0x16fc5058f25648194471939df75cf27a2fdc48bc"),
	L1SystemConfigAddress:   common.HexToAddress("0x034edd2a225f7f429a63e0f1d2084b9e0a93b538"),
	RegolithTime:            u64Ptr(0),
	CanyonTime:              u64Ptr(1699981200),
	DeltaTime:               u64Ptr(1703203200),
	EcotoneTime:             u64Ptr(1708534800),
	FjordTime:               u64Ptr(1716998400),
	GraniteTime:             u64Ptr(1723478400),
	HoloceneTime:            u64Ptr(1732633200),
	ProtocolVersionsAddress: common.HexToAddress("0x79ADD5713B383DAa0a138d3C4780C7A1804a8090"),
}

var sepoliaDev0Cfg = rollup.Config{
	Genesis: rollup.Genesis{
		L1: eth.BlockID{
			Hash:   common.HexToHash("0x5639be97000fec7131a880b19b664cae43f975c773f628a08a9bb658c2a68df0"),
			Number: 5173577,
		},
		L2: eth.BlockID{
			Hash:   common.HexToHash("0x027ae1f4f9a441f9c8a01828f3b6d05803a0f524c07e09263264a38b755f804b"),
			Number: 0,
		},
		L2Time: 1706484048,
		SystemConfig: eth.SystemConfig{
			BatcherAddr: common.HexToAddress("0x19cc7073150d9f5888f09e0e9016d2a39667df14"),
			Overhead:    eth.Bytes32(common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000bc")),
			Scalar:      eth.Bytes32(common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000a6fe0")),
			GasLimit:    30000000,
		},
	},
	BlockTime:               2,
	MaxSequencerDrift:       600,
	SeqWindowSize:           3600,
	ChannelTimeoutBedrock:   300,
	L1ChainID:               big.NewInt(11155111),
	L2ChainID:               big.NewInt(11155421),
	BatchInboxAddress:       common.HexToAddress("0xff00000000000000000000000000000011155421"),
	DepositContractAddress:  common.HexToAddress("0x76114bd29dFcC7a9892240D317E6c7C2A281Ffc6"),
	L1SystemConfigAddress:   common.HexToAddress("0xa6b72407e2dc9EBF84b839B69A24C88929cf20F7"),
	RegolithTime:            u64Ptr(0),
	CanyonTime:              u64Ptr(0),
	DeltaTime:               u64Ptr(0),
	EcotoneTime:             u64Ptr(1706634000),
	FjordTime:               u64Ptr(1715961600),
	GraniteTime:             u64Ptr(1723046400),
	HoloceneTime:            u64Ptr(1731682800),
	ProtocolVersionsAddress: common.HexToAddress("0x252CbE9517F731C618961D890D534183822dcC8d"),
}

func u64Ptr(v uint64) *uint64 {
	return &v
}
