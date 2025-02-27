package service

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

var (
	L1RPCURLFlag = &cli.StringFlag{
		Name:     "l1-rpc-url",
		Usage:    "L1 RPC URL",
		Required: true,
	}
	AbsolutePrestateFlag = &cli.StringFlag{
		Name:     "absolute-prestate",
		Usage:    "Absolute prestate as hex string",
		Required: true,
	}
	ProxyAdminFlag = &cli.StringFlag{
		Name:     "proxy-admin",
		Usage:    "Proxy admin address as hex string",
		Required: true,
	}
	SystemConfigFlag = &cli.StringFlag{
		Name:     "system-config",
		Usage:    "System config address as hex string",
		Required: true,
	}
	L2ChainIDFlag = &cli.StringFlag{
		Name:     "l2-chain-id",
		Usage:    "L2 chain ID",
		Required: true,
	}
	FailOnErrorFlag = &cli.BoolFlag{
		Name:  "fail",
		Usage: "Exit with non-zero code if validation errors are found",
		Value: true,
	}
)

// ValidateFlags contains all the flags needed for validation
var ValidateFlags = []cli.Flag{
	L1RPCURLFlag,
	AbsolutePrestateFlag,
	ProxyAdminFlag,
	SystemConfigFlag,
	L2ChainIDFlag,
	FailOnErrorFlag,
}

// Config represents the configuration for the validator service
type Config struct {
	L1RPCURL         string
	AbsolutePrestate common.Hash
	ProxyAdmin       common.Address
	SystemConfig     common.Address
	L2ChainID        *big.Int
}

// NewConfig creates a new Config from CLI context
func NewConfig(ctx *cli.Context) (*Config, error) {
	absolutePrestate := common.HexToHash(ctx.String(AbsolutePrestateFlag.Name))
	proxyAdmin := common.HexToAddress(ctx.String(ProxyAdminFlag.Name))
	systemConfig := common.HexToAddress(ctx.String(SystemConfigFlag.Name))
	l2ChainID, ok := new(big.Int).SetString(ctx.String(L2ChainIDFlag.Name), 10)
	if !ok {
		return nil, fmt.Errorf("invalid L2 chain ID: %s", ctx.String(L2ChainIDFlag.Name))
	}

	return &Config{
		L1RPCURL:         ctx.String(L1RPCURLFlag.Name),
		AbsolutePrestate: absolutePrestate,
		ProxyAdmin:       proxyAdmin,
		SystemConfig:     systemConfig,
		L2ChainID:        l2ChainID,
	}, nil
}
