package sync

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// FileAliases maps a file alias to its actual path
var FileAliases = map[string]string{
	"localsafe": "local_safe.db",
	"crosssafe": "cross_safe.db",
}

// Config contains all configuration for the Server or Client.
type Config struct {
	DataDir string
	Chains  []types.ChainID
	Logger  log.Logger
}
