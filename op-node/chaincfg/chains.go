package chaincfg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/superchain"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
)

// OPSepolia loads the op-sepolia rollup config. This is intended for tests that need an arbitrary, valid rollup config.
func OPSepolia() *rollup.Config {
	return mustLoadRollupConfig("op-sepolia")
}

func mustLoadRollupConfig(name string) *rollup.Config {
	cfg, err := GetRollupConfig(name)
	if err != nil {
		panic(fmt.Errorf("failed to load rollup config %q: %w", name, err))
	}
	return cfg
}

var L2ChainIDToNetworkDisplayName = func() map[string]string {
	out := make(map[string]string)
	for _, netCfg := range superchain.Chains {
		cfg, err := netCfg.Config()
		if err != nil {
			panic(fmt.Errorf("failed to load chain config: %w", err))
		}
		out[fmt.Sprintf("%d", cfg.ChainID)] = netCfg.Name
	}
	return out
}()

// AvailableNetworks returns the selection of network configurations that is available by default.
func AvailableNetworks() []string {
	var networks []string
	for _, cfg := range superchain.Chains {
		networks = append(networks, cfg.Name+"-"+cfg.Network)
	}
	sort.Strings(networks)
	return networks
}

func handleLegacyName(name string) string {
	switch name {
	case "mainnet":
		return "op-mainnet"
	case "sepolia":
		return "op-sepolia"
	default:
		return name
	}
}

// ChainByName returns a chain, from known available configurations, by name.
// ChainByName returns nil when the chain name is unknown.
func ChainByName(name string) *superchain.ChainConfig {
	// Handle legacy name aliases
	name = handleLegacyName(name)
	for _, chainCfg := range superchain.Chains {
		if !strings.EqualFold(chainCfg.Name+"-"+chainCfg.Network, name) {
			continue
		}

		cfg, err := chainCfg.Config()
		if err != nil {
			panic(fmt.Errorf("failed to load chain config: %w", err))
		}
		return cfg
	}
	return nil
}

func GetRollupConfig(name string) (*rollup.Config, error) {
	chainCfg := ChainByName(name)
	if chainCfg == nil {
		return nil, fmt.Errorf("invalid network: %q", name)
	}
	rollupCfg, err := rollup.LoadOPStackRollupConfig(chainCfg.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to load rollup config: %w", err)
	}
	return rollupCfg, nil
}
