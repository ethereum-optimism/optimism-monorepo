package env

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
)

type DevnetEnv struct {
	config descriptors.DevnetEnvironment
	fname  string
}

func LoadDevnetEnv(devnetFile string) (*DevnetEnv, error) {
	data, err := os.ReadFile(devnetFile)
	if err != nil {
		return nil, fmt.Errorf("error reading devnet file: %w", err)
	}

	var config descriptors.DevnetEnvironment
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	return &DevnetEnv{
		config: config,
		fname:  devnetFile,
	}, nil
}

func (d *DevnetEnv) GetChain(chainName string) (*ChainConfig, error) {
	var chain *descriptors.Chain
	if d.config.L1.Name == chainName {
		chain = d.config.L1
	} else {
		for _, l2Chain := range d.config.L2 {
			if l2Chain.Name == chainName {
				chain = l2Chain
				break
			}
		}
	}

	if chain == nil {
		return nil, fmt.Errorf("chain '%s' not found in devnet config", chainName)
	}

	return &ChainConfig{
		chain:      chain,
		devnetFile: d.fname,
		name:       chainName,
	}, nil
}
