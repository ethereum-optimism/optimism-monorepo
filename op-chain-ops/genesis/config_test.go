package genesis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestConfigDataMarshalUnmarshal(t *testing.T) {
	b, err := os.ReadFile("testdata/test-deploy-config-full.json")
	require.NoError(t, err)

	dec := json.NewDecoder(bytes.NewReader(b))
	decoded := new(DeployConfig)
	require.NoError(t, dec.Decode(decoded))
	require.NoError(t, decoded.Check(testlog.Logger(t, log.LevelDebug)))

	encoded, err := json.MarshalIndent(decoded, "", "  ")
	require.NoError(t, err)
	require.JSONEq(t, string(b), string(encoded))
}

func TestUnmarshalL1StartingBlockTag(t *testing.T) {
	decoded := new(DeployConfig)
	require.NoError(t, json.Unmarshal([]byte(`{"l1StartingBlockTag": "earliest"}`), decoded))
	require.EqualValues(t, rpc.EarliestBlockNumber, *decoded.L1StartingBlockTag.BlockNumber)
	h := "0x86c7263d87140ca7cd9bf1bc9e95a435a7a0efc0ae2afaf64920c5b59a6393d4"
	require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"l1StartingBlockTag": "%s"}`, h)), decoded))
	require.EqualValues(t, common.HexToHash(h), *decoded.L1StartingBlockTag.BlockHash)
}

func TestRegolithTimeZero(t *testing.T) {
	regolithOffset := hexutil.Uint64(0)
	config := &DeployConfig{
		L2InitializationConfig: L2InitializationConfig{
			UpgradeScheduleDeployConfig: UpgradeScheduleDeployConfig{
				L2GenesisRegolithTimeOffset: &regolithOffset,
			},
		},
	}
	require.Equal(t, uint64(0), *config.RegolithTime(1234))
}

func TestRegolithTimeAsOffset(t *testing.T) {
	regolithOffset := hexutil.Uint64(1500)
	config := &DeployConfig{
		L2InitializationConfig: L2InitializationConfig{
			UpgradeScheduleDeployConfig: UpgradeScheduleDeployConfig{
				L2GenesisRegolithTimeOffset: &regolithOffset,
			},
		},
	}
	require.Equal(t, uint64(1500+5000), *config.RegolithTime(5000))
}

func TestCanyonTimeZero(t *testing.T) {
	canyonOffset := hexutil.Uint64(0)
	config := &DeployConfig{
		L2InitializationConfig: L2InitializationConfig{
			UpgradeScheduleDeployConfig: UpgradeScheduleDeployConfig{
				L2GenesisCanyonTimeOffset: &canyonOffset,
			},
		},
	}
	require.Equal(t, uint64(0), *config.CanyonTime(1234))
}

func TestCanyonTimeOffset(t *testing.T) {
	canyonOffset := hexutil.Uint64(1500)
	config := &DeployConfig{
		L2InitializationConfig: L2InitializationConfig{
			UpgradeScheduleDeployConfig: UpgradeScheduleDeployConfig{
				L2GenesisCanyonTimeOffset: &canyonOffset,
			},
		},
	}
	require.Equal(t, uint64(1234+1500), *config.CanyonTime(1234))
}

// TestCopy will copy a DeployConfig and ensure that the copy is equal to the original.
func TestCopy(t *testing.T) {
	b, err := os.ReadFile("testdata/test-deploy-config-full.json")
	require.NoError(t, err)

	decoded := new(DeployConfig)
	require.NoError(t, json.NewDecoder(bytes.NewReader(b)).Decode(decoded))

	cpy := decoded.Copy()
	require.EqualValues(t, decoded, cpy)

	offset := hexutil.Uint64(100)
	cpy.L2GenesisRegolithTimeOffset = &offset
	require.NotEqual(t, decoded, cpy)
}

// TestL1Deployments ensures that NewL1Deployments can read a JSON file
// from disk and deserialize all of the key/value pairs correctly.
func TestL1Deployments(t *testing.T) {
	deployments, err := NewL1Deployments("testdata/l1-deployments.json")
	require.NoError(t, err)

	require.NotEqual(t, deployments.AddressManager, common.Address{})
	require.NotEqual(t, deployments.DisputeGameFactory, common.Address{})
	require.NotEqual(t, deployments.DisputeGameFactoryProxy, common.Address{})
	require.NotEqual(t, deployments.L1CrossDomainMessenger, common.Address{})
	require.NotEqual(t, deployments.L1CrossDomainMessengerProxy, common.Address{})
	require.NotEqual(t, deployments.L1ERC721Bridge, common.Address{})
	require.NotEqual(t, deployments.L1ERC721BridgeProxy, common.Address{})
	require.NotEqual(t, deployments.L1StandardBridge, common.Address{})
	require.NotEqual(t, deployments.L1StandardBridgeProxy, common.Address{})
	require.NotEqual(t, deployments.L2OutputOracle, common.Address{})
	require.NotEqual(t, deployments.L2OutputOracleProxy, common.Address{})
	require.NotEqual(t, deployments.OptimismMintableERC20Factory, common.Address{})
	require.NotEqual(t, deployments.OptimismMintableERC20FactoryProxy, common.Address{})
	require.NotEqual(t, deployments.OptimismPortal, common.Address{})
	require.NotEqual(t, deployments.OptimismPortalProxy, common.Address{})
	require.NotEqual(t, deployments.ProxyAdmin, common.Address{})
	require.NotEqual(t, deployments.SystemConfig, common.Address{})
	require.NotEqual(t, deployments.SystemConfigProxy, common.Address{})
	require.NotEqual(t, deployments.ProtocolVersions, common.Address{})
	require.NotEqual(t, deployments.ProtocolVersionsProxy, common.Address{})

	require.Equal(t, "AddressManager", deployments.GetName(deployments.AddressManager))
	require.Equal(t, "OptimismPortalProxy", deployments.GetName(deployments.OptimismPortalProxy))
	// One that doesn't exist returns empty string
	require.Equal(t, "", deployments.GetName(common.Address{19: 0xff}))
}

// This test guarantees that getters and setters for all forks are present.
func TestUpgradeScheduleDeployConfig_ForkGettersAndSetters(t *testing.T) {
	var d UpgradeScheduleDeployConfig
	for i, fork := range rollup.ForksFrom(rollup.Regolith) {
		require.Nil(t, d.ForkTimeOffset(fork))
		offset := uint64(i * 42)
		d.SetForkTimeOffset(fork, &offset)
		require.Equal(t, offset, *d.ForkTimeOffset(fork))
	}
}

func TestUpgradeScheduleDeployConfig_ActivateForkAtOffset(t *testing.T) {
	var d UpgradeScheduleDeployConfig
	ts := uint64(42)
	t.Run("invalid", func(t *testing.T) {
		require.Panics(t, func() { d.ActivateForkAtOffset(rollup.Bedrock, ts) })
	})

	t.Run("regolith", func(t *testing.T) {
		d.ActivateForkAtOffset(rollup.Regolith, ts)
		require.EqualValues(t, &ts, d.L2GenesisRegolithTimeOffset)
		for _, fork := range scheduleableForks[1:] {
			require.Nil(t, d.ForkTimeOffset(fork))
		}
	})

	t.Run("ecotone", func(t *testing.T) {
		d.ActivateForkAtOffset(rollup.Ecotone, ts)
		require.EqualValues(t, &ts, d.L2GenesisEcotoneTimeOffset)
		for _, fork := range scheduleableForks[:3] {
			require.Zero(t, *d.ForkTimeOffset(fork))
		}
		for _, fork := range scheduleableForks[4:] {
			require.Nil(t, d.ForkTimeOffset(fork))
		}
	})
}
