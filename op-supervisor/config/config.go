package config

import (
	"errors"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
)

var (
	ErrMissingL2RPC         = errors.New("must specify at least one L2 RPC")
	ErrMissingDependencySet = errors.New("must specify a dependency set source")
	ErrMissingDatadir       = errors.New("must specify datadir")
)

type Config struct {
	Version string

	LogConfig     oplog.CLIConfig
	MetricsConfig opmetrics.CLIConfig
	PprofConfig   oppprof.CLIConfig
	RPC           oprpc.CLIConfig

	DependencySetSource depset.DependencySetSource

	// MockRun runs the service with a mock backend
	MockRun bool

	// SynchronousProcessors disables background-workers,
	// requiring manual triggers for the backend to process anything.
	SynchronousProcessors bool

	L1RPC string

	L2RPCs  []string
	Datadir string
}

func (c *Config) Check() error {
	var result error
	result = errors.Join(result, c.MetricsConfig.Check())
	result = errors.Join(result, c.PprofConfig.Check())
	result = errors.Join(result, c.RPC.Check())
	if len(c.L2RPCs) == 0 {
		result = errors.Join(result, ErrMissingL2RPC)
	}
	if c.DependencySetSource == nil {
		result = errors.Join(result, ErrMissingDependencySet)
	}
	if c.Datadir == "" {
		result = errors.Join(result, ErrMissingDatadir)
	}
	return result
}

// NewConfig creates a new config using default values whenever possible.
// Required options with no suitable default are passed as parameters.
func NewConfig(l1RPC string, l2RPCs []string, depSet depset.DependencySetSource, datadir string) *Config {
	return &Config{
		LogConfig:           oplog.DefaultCLIConfig(),
		MetricsConfig:       opmetrics.DefaultCLIConfig(),
		PprofConfig:         oppprof.DefaultCLIConfig(),
		RPC:                 oprpc.DefaultCLIConfig(),
		DependencySetSource: depSet,
		MockRun:             false,
		L1RPC:               l1RPC,
		L2RPCs:              l2RPCs,
		Datadir:             datadir,
	}
}
