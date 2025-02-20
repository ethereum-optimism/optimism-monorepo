package super

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"
)

func TestRollupConfigs(t *testing.T) {
	t.Run("LoadNamedNetworks", func(t *testing.T) {
		vmCfg := vm.Config{
			Networks: []string{"op-mainnet", "op-sepolia"},
		}
		configs, err := NewRollupConfigs(vmCfg)
		require.NoError(t, err)
		require.Len(t, configs.cfgs, 2)
		expectedMainnet, err := chaincfg.GetRollupConfig("op-mainnet")
		require.NoError(t, err)
		expectedSepolia, err := chaincfg.GetRollupConfig("op-sepolia")
		require.NoError(t, err)
		actual, ok := configs.Get(eth.ChainIDFromBig(expectedMainnet.L2ChainID))
		require.True(t, ok, "did not load mainnet config")
		require.EqualValues(t, expectedMainnet, actual)
		actual, ok = configs.Get(eth.ChainIDFromBig(expectedSepolia.L2ChainID))
		require.True(t, ok, "did not load sepolia config")
		require.EqualValues(t, expectedSepolia, actual)
	})

	t.Run("LoadConfigFiles", func(t *testing.T) {
		expectedMainnet, err := chaincfg.GetRollupConfig("op-mainnet")
		require.NoError(t, err)
		expectedSepolia, err := chaincfg.GetRollupConfig("op-sepolia")
		require.NoError(t, err)

		dir := t.TempDir()
		writeConfig := func(cfg *rollup.Config) string {
			data, err := json.Marshal(cfg)
			require.NoError(t, err)
			path := filepath.Join(dir, cfg.L2ChainID.String()+".json")
			err = os.WriteFile(path, data, 0600)
			require.NoError(t, err)
			return path
		}

		mainnetFile := writeConfig(expectedMainnet)
		sepoliaFile := writeConfig(expectedSepolia)

		vmCfg := vm.Config{
			RollupConfigPaths: []string{mainnetFile, sepoliaFile},
		}
		configs, err := NewRollupConfigs(vmCfg)
		require.NoError(t, err)
		require.Len(t, configs.cfgs, 2)
		actual, ok := configs.Get(eth.ChainIDFromBig(expectedMainnet.L2ChainID))
		require.True(t, ok, "did not load mainnet config")
		require.EqualValues(t, expectedMainnet, actual)
		actual, ok = configs.Get(eth.ChainIDFromBig(expectedSepolia.L2ChainID))
		require.True(t, ok, "did not load sepolia config")
		require.EqualValues(t, expectedSepolia, actual)
	})

	t.Run("CombineLoadedConfigFiles", func(t *testing.T) {
		expectedMainnet, err := chaincfg.GetRollupConfig("op-mainnet")
		require.NoError(t, err)
		expectedSepolia, err := chaincfg.GetRollupConfig("op-sepolia")
		require.NoError(t, err)

		mainnetFile := writeConfig(t, expectedMainnet)

		vmCfg := vm.Config{
			RollupConfigPaths: []string{mainnetFile},
			Networks:          []string{"op-sepolia"},
		}
		configs, err := NewRollupConfigs(vmCfg)
		require.NoError(t, err)
		require.Len(t, configs.cfgs, 2)
		actual, ok := configs.Get(eth.ChainIDFromBig(expectedMainnet.L2ChainID))
		require.True(t, ok, "did not load mainnet config")
		require.EqualValues(t, expectedMainnet, actual)
		actual, ok = configs.Get(eth.ChainIDFromBig(expectedSepolia.L2ChainID))
		require.True(t, ok, "did not load sepolia config")
		require.EqualValues(t, expectedSepolia, actual)
	})

	t.Run("UnknownConfig", func(t *testing.T) {
		cfg, err := NewRollupConfigs(vm.Config{})
		require.NoError(t, err)
		_, ok := cfg.Get(eth.ChainIDFromUInt64(4))
		require.False(t, ok)
	})

	t.Run("ErrorOnDuplicateConfig-Named", func(t *testing.T) {
		_, err := NewRollupConfigs(vm.Config{Networks: []string{"op-mainnet", "op-mainnet"}})
		require.ErrorIs(t, err, ErrDuplicateChain)
	})

	t.Run("ErrorOnDuplicateConfig-NameAndFile", func(t *testing.T) {
		expectedMainnet, err := chaincfg.GetRollupConfig("op-mainnet")
		require.NoError(t, err)
		mainnetPath := writeConfig(t, expectedMainnet)
		_, err = NewRollupConfigs(vm.Config{Networks: []string{"op-mainnet"}, RollupConfigPaths: []string{mainnetPath}})
		require.ErrorIs(t, err, ErrDuplicateChain)
	})

	t.Run("ErrorOnDuplicateConfig-Parsed", func(t *testing.T) {
		expectedMainnet, err := chaincfg.GetRollupConfig("op-mainnet")
		require.NoError(t, err)
		_, err = NewRollupConfigsFromParsed(expectedMainnet, expectedMainnet)
		require.ErrorIs(t, err, ErrDuplicateChain)
	})
}

func writeConfig(t *testing.T, cfg *rollup.Config) string {
	dir := t.TempDir()
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	path := filepath.Join(dir, cfg.L2ChainID.String()+".json")
	err = os.WriteFile(path, data, 0600)
	require.NoError(t, err)
	return path
}
