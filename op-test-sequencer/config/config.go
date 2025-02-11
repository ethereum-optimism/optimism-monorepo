package config

import (
	"errors"

	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
)

const (
	DefaultL2CL = "ws://127.0.0.1:9545"
	DefaultL2EL = "ws://127.0.0.1:8545"
	DefaultL1EL = "ws://127.0.0.1:7545"
)

type Config struct {
	Version string

	LogConfig     oplog.CLIConfig
	MetricsConfig opmetrics.CLIConfig
	PprofConfig   oppprof.CLIConfig
	RPC           oprpc.CLIConfig

	JWTSecretPath string

	// L2 consensus-layer RPC endpoint
	L2CL endpoint.RPC

	// L2 execution-layer RPC endpoint
	L2EL endpoint.RPC

	// L1 execution-layer RPC endpoint
	L1EL endpoint.RPC

	MockRun bool
}

func (c *Config) Check() error {
	var result error
	result = errors.Join(result, c.MetricsConfig.Check())
	result = errors.Join(result, c.PprofConfig.Check())
	result = errors.Join(result, c.RPC.Check())
	return result
}

func DefaultCLIConfig() *Config {
	return &Config{
		Version:       "dev",
		LogConfig:     oplog.DefaultCLIConfig(),
		MetricsConfig: opmetrics.DefaultCLIConfig(),
		PprofConfig:   oppprof.DefaultCLIConfig(),
		RPC:           oprpc.DefaultCLIConfig(),
		L2CL:          endpoint.WsURL(DefaultL2CL),
		L2EL:          endpoint.WsURL(DefaultL2EL),
		L1EL:          endpoint.WsURL(DefaultL1EL),
		MockRun:       false,
	}
}
