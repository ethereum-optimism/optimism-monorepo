package upgrade

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

type UpgradeConfig struct {
	L1RPCUrl         string
	Workdir          string
	PrivateKey       string
	DeploymentTarget deployer.DeploymentTarget
	Logger           log.Logger

	privateKeyECDSA *ecdsa.PrivateKey
}

// OpChainConfig mirrors the Solidity struct for type safety
type OpChainConfig struct {
	SystemConfigProxy common.Address
	ProxyAdmin        common.Address
	AbsolutePrestate  common.Hash
}

// GenerateOpChainConfigs converts ChainStates to the format expected by the OPContractsManager
func GenerateOpChainConfigs(states []*state.ChainState) []*OpChainConfig {
	opChainConfigs := make([]*OpChainConfig, 0, len(states))
	for _, st := range states {
		opChainConfigs = append(opChainConfigs, &OpChainConfig{
			SystemConfigProxy: st.SystemConfigProxyAddress,
			ProxyAdmin:        st.ProxyAdminAddress,
			AbsolutePrestate:  common.Hash{}, // Not used by OPCMUpgrade.s.sol
		})
	}
	return opChainConfigs
}

func (a *UpgradeConfig) Check() error {
	if a.Workdir == "" {
		return fmt.Errorf("workdir must be specified")
	}

	if a.PrivateKey != "" {
		privECDSA, err := crypto.HexToECDSA(strings.TrimPrefix(a.PrivateKey, "0x"))
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		a.privateKeyECDSA = privECDSA
	}

	if a.Logger == nil {
		return fmt.Errorf("logger must be specified")
	}

	if a.DeploymentTarget == deployer.DeploymentTargetLive {
		if a.L1RPCUrl == "" {
			return fmt.Errorf("l1 RPC URL must be specified for live deployment")
		}

		if a.privateKeyECDSA == nil {
			return fmt.Errorf("private key must be specified for live deployment")
		}
	}

	return nil
}

func UpgradeCLI() cli.ActionFunc {
	return func(cliCtx *cli.Context) error {
		logCfg := oplog.ReadCLIConfig(cliCtx)
		l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
		oplog.SetGlobalLogHandler(l.Handler())

		l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
		workdir := cliCtx.String(deployer.WorkdirFlagName)
		privateKey := cliCtx.String(deployer.PrivateKeyFlagName)

		ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

		return UpgradeContracts(ctx, UpgradeConfig{
			L1RPCUrl:   l1RPCUrl,
			Workdir:    workdir,
			PrivateKey: privateKey,
			Logger:     l,
		})
	}
}

func UpgradeContracts(
	ctx context.Context,
	cfg UpgradeConfig,
) error {
	if err := cfg.Check(); err != nil {
		return fmt.Errorf("invalid config for upgrade: %w", err)
	}

	st, err := pipeline.ReadState(cfg.Workdir)
	if err != nil {
		return err
	}

	opChainConfigs := GenerateOpChainConfigs(st.Chains)

	opcmAddr := st.ImplementationsDeployment.OpcmAddress
	if opcmAddr == (common.Address{}) {
		return fmt.Errorf("OPCM address not found in state")
	}

	if err := RunUpgradeScript(
		nil,
		opcmAddr,
		opChainConfigs,
		"OPCMUpgrade.s.sol",
		"OPCMUpgrade",
	); err != nil {
		return fmt.Errorf("failed to run upgrade script: %w", err)
	}

	return nil
}
