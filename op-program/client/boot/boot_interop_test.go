package boot

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestInteropBootstrap_SimpleValues(t *testing.T) {
	expected := &BootInfoInterop{
		L1Head:         common.Hash{0xaa},
		AgreedPrestate: common.Hash{0xbb},
		Claim:          common.Hash{0xcc},
		GameTimestamp:  49829482,
	}
	mockOracle := newMockInteropBootstrapOracle(expected, false)
	actual := BootstrapInterop(mockOracle)
	require.Equal(t, expected.L1Head, actual.L1Head)
	require.Equal(t, expected.AgreedPrestate, actual.AgreedPrestate)
	require.Equal(t, expected.Claim, actual.Claim)
	require.Equal(t, expected.GameTimestamp, actual.GameTimestamp)
}

func TestInteropBootstrap_RollupConfigBuiltIn(t *testing.T) {
	expectedCfg := chaincfg.OPSepolia()
	expected := &BootInfoInterop{
		L1Head:         common.Hash{0xaa},
		AgreedPrestate: common.Hash{0xbb},
		Claim:          common.Hash{0xcc},
		GameTimestamp:  49829482,
	}
	mockOracle := newMockInteropBootstrapOracle(expected, false)
	actual := BootstrapInterop(mockOracle)
	actualCfg, err := actual.Configs.RollupConfig(eth.ChainIDFromBig(expectedCfg.L2ChainID))
	require.NoError(t, err)
	require.Equal(t, expectedCfg, actualCfg)
}

func TestInteropBootstrap_RollupConfigCustom(t *testing.T) {
	config1 := &rollup.Config{L2ChainID: big.NewInt(1111)}
	config2 := &rollup.Config{L2ChainID: big.NewInt(2222)}
	source := &BootInfoInterop{
		L1Head:         common.Hash{0xaa},
		AgreedPrestate: common.Hash{0xbb},
		Claim:          common.Hash{0xcc},
		GameTimestamp:  49829482,
	}
	mockOracle := newMockInteropBootstrapOracle(source, true)
	mockOracle.rollupCfgs = []*rollup.Config{config1, config2}
	actual := BootstrapInterop(mockOracle)
	actualCfg, err := actual.Configs.RollupConfig(eth.ChainIDFromBig(config1.L2ChainID))
	require.NoError(t, err)
	require.Equal(t, config1, actualCfg)

	actualCfg, err = actual.Configs.RollupConfig(eth.ChainIDFromBig(config2.L2ChainID))
	require.NoError(t, err)
	require.Equal(t, config2, actualCfg)
}

func TestInteropBootstrap_ChainConfigBuiltIn(t *testing.T) {
	expectedCfg := chainconfig.OPSepoliaChainConfig()
	expected := &BootInfoInterop{
		L1Head:         common.Hash{0xaa},
		AgreedPrestate: common.Hash{0xbb},
		Claim:          common.Hash{0xcc},
		GameTimestamp:  49829482,
	}
	mockOracle := newMockInteropBootstrapOracle(expected, false)
	actual := BootstrapInterop(mockOracle)
	actualCfg, err := actual.Configs.ChainConfig(eth.ChainIDFromBig(expectedCfg.ChainID))
	require.NoError(t, err)
	require.Equal(t, expectedCfg, actualCfg)
}

func TestInteropBootstrap_ChainConfigCustom(t *testing.T) {
	config1 := &params.ChainConfig{ChainID: big.NewInt(1111)}
	config2 := &params.ChainConfig{ChainID: big.NewInt(2222)}
	expected := &BootInfoInterop{
		L1Head:         common.Hash{0xaa},
		AgreedPrestate: common.Hash{0xbb},
		Claim:          common.Hash{0xcc},
		GameTimestamp:  49829482,
	}
	mockOracle := newMockInteropBootstrapOracle(expected, true)
	mockOracle.chainCfgs = []*params.ChainConfig{config1, config2}
	mockOracle.depset, _ = depset.NewStaticConfigDependencySet(map[eth.ChainID]*depset.StaticConfigDependency{
		eth.ChainIDFromBig(config1.ChainID): {ChainIndex: supervisortypes.ChainIndex(config1.ChainID.Uint64()), ActivationTime: 0, HistoryMinTime: 0},
		eth.ChainIDFromBig(config2.ChainID): {ChainIndex: supervisortypes.ChainIndex(config2.ChainID.Uint64()), ActivationTime: 0, HistoryMinTime: 0},
	})
	actual := BootstrapInterop(mockOracle)

	actualCfg, err := actual.Configs.ChainConfig(eth.ChainIDFromBig(config1.ChainID))
	require.NoError(t, err)
	require.Equal(t, config1, actualCfg)

	actualCfg, err = actual.Configs.ChainConfig(eth.ChainIDFromBig(config2.ChainID))
	require.NoError(t, err)
	require.Equal(t, config2, actualCfg)
}

func TestInteropBootstrap_DependencySetCustom(t *testing.T) {
	config1 := &params.ChainConfig{ChainID: big.NewInt(1111)}
	config2 := &params.ChainConfig{ChainID: big.NewInt(2222)}
	expected := &BootInfoInterop{
		L1Head:         common.Hash{0xaa},
		AgreedPrestate: common.Hash{0xbb},
		Claim:          common.Hash{0xcc},
		GameTimestamp:  49829482,
	}
	mockOracle := newMockInteropBootstrapOracle(expected, true)
	var err error
	mockOracle.depset, err = depset.NewStaticConfigDependencySet(map[eth.ChainID]*depset.StaticConfigDependency{
		eth.ChainIDFromBig(config1.ChainID): {ChainIndex: supervisortypes.ChainIndex(config1.ChainID.Uint64()), ActivationTime: 0, HistoryMinTime: 0},
		eth.ChainIDFromBig(config2.ChainID): {ChainIndex: supervisortypes.ChainIndex(config2.ChainID.Uint64()), ActivationTime: 0, HistoryMinTime: 0},
	})
	require.NoError(t, err)
	actual := BootstrapInterop(mockOracle)

	depset, err := actual.Configs.DependencySet(eth.ChainIDFromBig(config1.ChainID))
	require.NoError(t, err)
	require.Equal(t, mockOracle.depset, depset)
}

func newMockInteropBootstrapOracle(b *BootInfoInterop, custom bool) *mockInteropBootstrapOracle {
	return &mockInteropBootstrapOracle{
		mockBoostrapOracle: mockBoostrapOracle{
			l1Head:             b.L1Head,
			l2OutputRoot:       b.AgreedPrestate,
			l2Claim:            b.Claim,
			l2ClaimBlockNumber: b.GameTimestamp,
		},
		custom: custom,
	}
}

type mockInteropBootstrapOracle struct {
	mockBoostrapOracle
	rollupCfgs []*rollup.Config
	chainCfgs  []*params.ChainConfig
	depset     *depset.StaticConfigDependencySet
	custom     bool
}

func (o *mockInteropBootstrapOracle) Get(key preimage.Key) []byte {
	switch key.PreimageKey() {
	case L2ChainConfigLocalIndex.PreimageKey():
		if !o.custom {
			panic(fmt.Sprintf("unexpected oracle request for preimage key %x", key.PreimageKey()))
		}
		b, _ := json.Marshal(o.chainCfgs)
		return b
	case RollupConfigLocalIndex.PreimageKey():
		if !o.custom {
			panic(fmt.Sprintf("unexpected oracle request for preimage key %x", key.PreimageKey()))
		}
		b, _ := json.Marshal(o.rollupCfgs)
		return b
	case L2ChainIDLocalIndex.PreimageKey():
		panic("unexpected oracle request for l2 chain ID preimage key")
	case DependencySetLocalIndex.PreimageKey():
		if !o.custom {
			panic(fmt.Sprintf("unexpected oracle request for preimage key %x", key.PreimageKey()))
		}
		b, _ := json.Marshal(o.depset)
		return b
	default:
		return o.mockBoostrapOracle.Get(key)
	}
}
