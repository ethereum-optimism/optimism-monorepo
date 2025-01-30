package boot

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestBootstrapClient(t *testing.T) {
	rollupCfg := chaincfg.OPSepolia()
	bootInfo := &BootInfo{
		L1Head:             common.HexToHash("0x1111"),
		L2OutputRoot:       common.HexToHash("0x2222"),
		L2Claim:            common.HexToHash("0x3333"),
		L2ClaimBlockNumber: 1,
		L2ChainID:          eth.ChainIDFromBig(rollupCfg.L2ChainID),
		L2ChainConfig:      chainconfig.OPSepoliaChainConfig(),
		RollupConfig:       rollupCfg,
	}
	mockOracle := newMockPreinteropBootstrapOracle(bootInfo, false)
	readBootInfo := NewBootstrapClient(mockOracle).BootInfo()
	require.EqualValues(t, bootInfo, readBootInfo)
}

func TestBootstrapClient_CustomChain(t *testing.T) {
	bootInfo := &BootInfo{
		L1Head:             common.HexToHash("0x1111"),
		L2OutputRoot:       common.HexToHash("0x2222"),
		L2Claim:            common.HexToHash("0x3333"),
		L2ClaimBlockNumber: 1,
		L2ChainID:          CustomChainIDIndicator,
		L2ChainConfig:      chainconfig.OPSepoliaChainConfig(),
		RollupConfig:       chaincfg.OPSepolia(),
	}
	mockOracle := newMockPreinteropBootstrapOracle(bootInfo, true)
	readBootInfo := NewBootstrapClient(mockOracle).BootInfo()
	require.EqualValues(t, bootInfo, readBootInfo)
}

func TestBootstrapClient_UnknownChainPanics(t *testing.T) {
	bootInfo := &BootInfo{
		L1Head:             common.HexToHash("0x1111"),
		L2OutputRoot:       common.HexToHash("0x2222"),
		L2Claim:            common.HexToHash("0x3333"),
		L2ClaimBlockNumber: 1,
		L2ChainID:          eth.ChainID{0xdead},
	}
	mockOracle := newMockPreinteropBootstrapOracle(bootInfo, false)
	client := NewBootstrapClient(mockOracle)
	require.Panics(t, func() { client.BootInfo() })
}

func newMockPreinteropBootstrapOracle(info *BootInfo, custom bool) *mockPreinteropBoostrapOracle {
	return &mockPreinteropBoostrapOracle{
		mockBoostrapOracle: mockBoostrapOracle{
			l1Head:             info.L1Head,
			l2OutputRoot:       info.L2OutputRoot,
			l2Claim:            info.L2Claim,
			l2ClaimBlockNumber: info.L2ClaimBlockNumber,
		},
		b:      info,
		custom: custom,
	}
}

type mockPreinteropBoostrapOracle struct {
	mockBoostrapOracle
	b      *BootInfo
	custom bool
}

func (o *mockPreinteropBoostrapOracle) Get(key preimage.Key) []byte {
	switch key.PreimageKey() {
	case L2ChainIDLocalIndex.PreimageKey():
		return binary.BigEndian.AppendUint64(nil, eth.EvilChainIDToUInt64(o.b.L2ChainID))
	case L2ChainConfigLocalIndex.PreimageKey():
		if !o.custom {
			panic(fmt.Sprintf("unexpected oracle request for preimage key %x", key.PreimageKey()))
		}
		b, _ := json.Marshal(o.b.L2ChainConfig)
		return b
	case RollupConfigLocalIndex.PreimageKey():
		if !o.custom {
			panic(fmt.Sprintf("unexpected oracle request for preimage key %x", key.PreimageKey()))
		}
		b, _ := json.Marshal(o.b.RollupConfig)
		return b
	default:
		return o.mockBoostrapOracle.Get(key)
	}
}
