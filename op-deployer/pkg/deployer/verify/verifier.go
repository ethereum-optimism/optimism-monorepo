package verify

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
	"golang.org/x/time/rate"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/inspect"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

type Verifier struct {
	apiKey       string
	l1ChainID    uint64
	st           *state.State
	artifactsFS  foundry.StatDirFs
	log          log.Logger
	etherscanUrl string
	rateLimiter  *rate.Limiter
}

func NewVerifier(apiKey string, l1ChainID uint64, st *state.State, artifactsFS foundry.StatDirFs, l log.Logger) (*Verifier, error) {
	etherscanUrl := getAPIEndpoint(l1ChainID)
	if etherscanUrl == "" {
		return nil, fmt.Errorf("unsupported L1 chain ID: %d", l1ChainID)
	}

	return &Verifier{
		apiKey:       apiKey,
		l1ChainID:    l1ChainID,
		st:           st,
		artifactsFS:  artifactsFS,
		log:          l,
		etherscanUrl: etherscanUrl,
		rateLimiter:  rate.NewLimiter(rate.Limit(3), 2),
	}, nil
}

func VerifyCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
	workdir := cliCtx.String(deployer.WorkdirFlagName)
	etherscanAPIKey := cliCtx.String(deployer.EtherscanAPIKeyFlagName)
	l2ChainIndex := cliCtx.Int(deployer.L2ChainIndexFlagName)

	client, err := ethclient.Dial(l1RPCUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L1: %w", err)
	}

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)
	l1ChainId, err := client.ChainID(ctx)
	if err != nil {
		return err
	}

	st, err := pipeline.ReadState(workdir)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	if l1ChainId.Uint64() != st.AppliedIntent.L1ChainID {
		return fmt.Errorf("rpc l1 chain ID does not match state l1 chain ID: %d != %d", l1ChainId, st.AppliedIntent.L1ChainID)
	}

	artifactsFS, err := artifacts.Download(ctx, st.AppliedIntent.L1ContractsLocator, nil)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}
	l.Info("Downloaded artifacts", "path", artifactsFS)

	v, err := NewVerifier(etherscanAPIKey, l1ChainId.Uint64(), st, artifactsFS, l)
	if err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}

	// Retrieve the CLI flags for the contract bundle and contract name.
	bundleName := cliCtx.String(deployer.ContractBundleFlagName)
	contractName := cliCtx.String(deployer.ContractNameFlagName)

	if bundleName == "" && contractName == "" {
		if err := v.verifyAll(ctx, l2ChainIndex); err != nil {
			return err
		}
	} else if bundleName != "" && contractName == "" {
		if err := v.verifyContractBundle(bundleName, l2ChainIndex); err != nil {
			return err
		}
	} else if bundleName != "" && contractName != "" {
		if err := v.verifySingleContract(ctx, contractName, bundleName, l2ChainIndex); err != nil {
			return err
		}
	} else {
		// If a contract name is provided without a contract bundle, report an error.
		return fmt.Errorf("contract-name flag provided without contract-bundle flag")
	}

	v.log.Info("--- SUCCESS ---")
	return nil
}

func (v *Verifier) verifyAll(ctx context.Context, l2ChainIndex int) error {
	for _, bundleName := range inspect.ContractBundles {
		if err := v.verifyContractBundle(bundleName, l2ChainIndex); err != nil {
			return fmt.Errorf("failed to verify bundle %s: %w", bundleName, err)
		}
	}
	return nil
}

func (v *Verifier) verifyContractBundle(bundleName string, l2ChainIndex int) error {
	// Retrieve the L1 contracts from state.
	l1Contracts, err := inspect.L1(v.st, v.st.AppliedIntent.Chains[l2ChainIndex].ID)
	if err != nil {
		return fmt.Errorf("failed to extract L1 contracts from state: %w", err)
	}

	// Select the appropriate bundle based on the input bundleName.
	var bundle interface{}
	switch bundleName {
	case inspect.SuperchainBundle:
		bundle = l1Contracts.SuperchainDeployment
	case inspect.ImplementationsBundle:
		bundle = l1Contracts.ImplementationsDeployment
	case inspect.OpChainBundle:
		bundle = l1Contracts.OpChainDeployment
	default:
		return fmt.Errorf("invalid contract bundle: %s", bundleName)
	}

	// Use reflection to iterate over fields of the bundle.
	val := reflect.ValueOf(bundle)
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Type() == reflect.TypeOf(common.Address{}) {
			addr := field.Interface().(common.Address)
			if addr != (common.Address{}) { // Skip zero addresses
				name := typ.Field(i).Name
				if err := v.verifyContract(addr, name); err != nil {
					return fmt.Errorf("failed to verify %s: %w", name, err)
				}
			}
		}
	}
	return nil
}

func (v *Verifier) verifySingleContract(ctx context.Context, contractName string, bundleName string, l2ChainIndex int) error {
	l1Contracts, err := inspect.L1(v.st, v.st.AppliedIntent.Chains[l2ChainIndex].ID)
	if err != nil {
		return fmt.Errorf("failed to extract L1 contracts from state: %w", err)
	}

	v.log.Info("Looking up contract address", "name", contractName, "bundle", bundleName)
	addr, err := l1Contracts.GetContractAddress(contractName, bundleName)
	if err != nil {
		return fmt.Errorf("failed to find address for contract %s: %w", contractName, err)
	}

	return v.verifyContract(addr, contractName)
}
