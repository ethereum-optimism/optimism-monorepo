package client

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
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
		L2ChainID:          rollupCfg.L2ChainID.Uint64(),
		L2ChainConfig:      chainconfig.OPSepoliaChainConfig(),
		RollupConfig:       rollupCfg,
	}
	mockOracle := &mockBoostrapOracle{bootInfo, false}
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
	mockOracle := &mockBoostrapOracle{bootInfo, true}
	readBootInfo := NewBootstrapClient(mockOracle).BootInfo()
	require.EqualValues(t, bootInfo, readBootInfo)
}

func TestBootstrapClient_UnknownChainPanics(t *testing.T) {
	bootInfo := &BootInfo{
		L1Head:             common.HexToHash("0x1111"),
		L2OutputRoot:       common.HexToHash("0x2222"),
		L2Claim:            common.HexToHash("0x3333"),
		L2ClaimBlockNumber: 1,
		L2ChainID:          uint64(0xdead),
	}
	mockOracle := &mockBoostrapOracle{bootInfo, false}
	client := NewBootstrapClient(mockOracle)
	require.Panics(t, func() { client.BootInfo() })
}

type mockBoostrapOracle struct {
	b      *BootInfo
	custom bool
}

func (o *mockBoostrapOracle) Get(key preimage.Key) []byte {
	switch key.PreimageKey() {
	case L1HeadLocalIndex.PreimageKey():
		return o.b.L1Head[:]
	case L2OutputRootLocalIndex.PreimageKey():
		return o.b.L2OutputRoot[:]
	case L2ClaimLocalIndex.PreimageKey():
		return o.b.L2Claim[:]
	case L2ClaimBlockNumberLocalIndex.PreimageKey():
		return binary.BigEndian.AppendUint64(nil, o.b.L2ClaimBlockNumber)
	case L2ChainIDLocalIndex.PreimageKey():
		return binary.BigEndian.AppendUint64(nil, o.b.L2ChainID)
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
		panic("unknown key")
	}
}
