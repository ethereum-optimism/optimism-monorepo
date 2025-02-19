package chainconfig

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-service/superutil"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
)

// OPSepoliaChainConfig loads the op-sepolia chain config. This is intended for tests that need an arbitrary, valid chain config.
func OPSepoliaChainConfig() *params.ChainConfig {
	return mustLoadChainConfig("op-sepolia")
}

//go:embed configs/*json
var customChainConfigFS embed.FS

func RollupConfigByChainID(chainID eth.ChainID) (*rollup.Config, error) {
	config, err := rollup.LoadOPStackRollupConfig(eth.EvilChainIDToUInt64(chainID))
	if err == nil {
		return config, err
	}
	return rollupConfigByChainID(chainID, customChainConfigFS)
}

func rollupConfigByChainID(chainID eth.ChainID, customChainFS embed.FS) (*rollup.Config, error) {
	// Load custom rollup configs from embed FS
	file, err := customChainFS.Open(fmt.Sprintf("configs/%v-rollup.json", chainID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no rollup config available for chain ID: %d", chainID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get rollup config for chain ID %v: %w", chainID, err)
	}
	defer file.Close()

	var customRollupConfig rollup.Config
	return &customRollupConfig, customRollupConfig.ParseRollupConfig(file)
}

func ChainConfigByChainID(chainID eth.ChainID) (*params.ChainConfig, error) {
	config, err := superutil.LoadOPStackChainConfigFromChainID(eth.EvilChainIDToUInt64(chainID))
	if err == nil {
		return config, err
	}
	return chainConfigByChainID(chainID, customChainConfigFS)
}

func chainConfigByChainID(chainID eth.ChainID, customChainFS embed.FS) (*params.ChainConfig, error) {
	// Load from custom chain configs from embed FS
	data, err := customChainFS.ReadFile(fmt.Sprintf("configs/%v-genesis-l2.json", chainID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no chain config available for chain ID: %d", chainID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get chain config for chain ID %v: %w", chainID, err)
	}
	var genesis core.Genesis
	err = json.Unmarshal(data, &genesis)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chain config for chain ID %v: %w", chainID, err)
	}
	return genesis.Config, nil
}

func mustLoadChainConfig(name string) *params.ChainConfig {
	chainCfg := chaincfg.ChainByName(name)
	if chainCfg == nil {
		panic(fmt.Errorf("unknown chain config %q", name))
	}
	cfg, err := ChainConfigByChainID(eth.ChainIDFromUInt64(chainCfg.ChainID))
	if err != nil {
		panic(fmt.Errorf("failed to load rollup config: %q: %w", name, err))
	}
	return cfg
}

func DependencySetByChainID(chainID eth.ChainID) (depset.DependencySet, error) {
	// TODO(#13887): Load from the superchain registry when available.
	return dependencySetByChainID(chainID, customChainConfigFS)
}

func dependencySetByChainID(chainID eth.ChainID, customChainFS embed.FS) (depset.DependencySet, error) {
	// Load custom dependency set configs from embed FS
	data, err := customChainFS.ReadFile("configs/depsets.json")
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no dependency set available for chain ID: %d", chainID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get dependency set for chain ID %v: %w", chainID, err)
	}

	var depSets []*depset.StaticConfigDependencySet

	err = json.Unmarshal(data, &depSets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dependency set for chain ID %v: %w", chainID, err)
	}
	for _, depSet := range depSets {
		if depSet.HasChain(chainID) {
			return depSet, nil
		}
	}
	return nil, fmt.Errorf("no dependency set config includes chain ID: %d", chainID)
}
