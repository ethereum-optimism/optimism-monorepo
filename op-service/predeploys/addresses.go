package predeploys

import (
	"github.com/BurntSushi/toml"
	"github.com/ethereum/go-ethereum/common"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Contracts    map[string]string `toml:"contracts"`
	ProxyDisabled map[string]bool  `toml:"proxy_disabled"`
}

var (
	Predeploys          = make(map[string]*Predeploy)
	PredeploysByAddress = make(map[common.Address]*Predeploy)
)

func init() {
	// Get the directory of the current file
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// Load the TOML config file
	configPath := filepath.Join(dir, "..", "..", "predeploys.toml")
	var config Config
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		panic(err)
	}

	// Initialize predeploys from config
	for name, addr := range config.Contracts {
		address := common.HexToAddress(addr)
		predeploy := &Predeploy{
			Address:       address,
			ProxyDisabled: config.ProxyDisabled[name],
		}

		// Special case for GovernanceToken
		if name == "GovernanceToken" {
			predeploy.Enabled = func(config DeployConfig) bool {
				return config.GovernanceEnabled()
			}
		}

		Predeploys[name] = predeploy
		PredeploysByAddress[address] = predeploy
	}
}
