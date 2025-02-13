package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strings"

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

	v := &Verifier{
		apiKey:       apiKey,
		l1ChainID:    l1ChainID,
		st:           st,
		artifactsFS:  artifactsFS,
		log:          l,
		etherscanUrl: etherscanUrl,
		rateLimiter:  rate.NewLimiter(rate.Limit(3), 2),
	}

	return v, nil
}

func VerifyCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
	workdir := cliCtx.String(deployer.WorkdirFlagName)
	etherscanAPIKey := cliCtx.String(deployer.EtherscanAPIKeyFlagName)

	client, err := ethclient.Dial(l1RPCUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L1: %w", err)
	}

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}

	st, err := pipeline.ReadState(workdir)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	artifactsFS, err := artifacts.Download(ctx, st.AppliedIntent.L1ContractsLocator, nil)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}
	l.Info("Downloaded artifacts", "artifacts", artifactsFS)

	v, err := NewVerifier(etherscanAPIKey, chainID.Uint64(), st, artifactsFS, l)
	if err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}

	if cliCtx.Args().Len() > 0 {
		// If a contract name is provided, verify just that contract
		bundleName := cliCtx.String(deployer.ContractBundleFlagName)
		contractName := cliCtx.Args().First()
		if err := v.VerifySingleContract(ctx, contractName, bundleName); err != nil {
			return err
		}
	} else {
		if err := v.VerifyAll(ctx, client, workdir); err != nil {
			return err
		}
	}

	v.log.Info("--- SUCCESS --- all requested contracts verified")
	return nil
}

func (v *Verifier) VerifySingleContract(ctx context.Context, contractName string, bundleName string) error {
	l1Contracts, err := inspect.L1(v.st, v.st.AppliedIntent.Chains[0].ID)
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

func (v *Verifier) VerifyAll(ctx context.Context, client *ethclient.Client, workdir string) error {
	l1Contracts, err := inspect.L1(v.st, v.st.AppliedIntent.Chains[0].ID)
	if err != nil {
		return fmt.Errorf("failed to extract L1 contracts from state: %w", err)
	}

	if err := v.verifyContractBundle(l1Contracts.SuperchainDeployment); err != nil {
		return err
	}
	if err := v.verifyContractBundle(l1Contracts.OpChainDeployment); err != nil {
		return err
	}
	if err := v.verifyContractBundle(l1Contracts.ImplementationsDeployment); err != nil {
		return err
	}
	return nil
}

func (v *Verifier) verifyContractBundle(s interface{}) error {
	val := reflect.ValueOf(s)
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

type ContractArtifact struct {
	ContractName     string
	CompilerVersion  string
	OptimizationUsed bool
	OptimizationRuns int
	EVMVersion       string
	StandardInput    string
}

var exceptions = map[string]string{
	"OptimismPortalImpl":          "OptimismPortal2",
	"L1StandardBridgeProxy":       "L1ChugSplashProxy",
	"L1CrossDomainMessengerProxy": "ResolvedDelegateProxy",
	"Opcm":                        "OPContractsManager",
}

func getArtifactName(name string) string {
	lookupName := strings.TrimSuffix(name, "Address")

	if artifactName, exists := exceptions[lookupName]; exists {
		return artifactName
	}

	// Handle standard cases
	lookupName = strings.TrimSuffix(lookupName, "Proxy")
	lookupName = strings.TrimSuffix(lookupName, "Impl")
	lookupName = strings.TrimSuffix(lookupName, "Singleton")

	// If it was a proxy and not a special case, return "Proxy"
	if strings.HasSuffix(name, "ProxyAddress") {
		return "Proxy"
	}

	return lookupName
}

func (v *Verifier) getContractArtifact(name string) (*ContractArtifact, error) {
	artifactName := getArtifactName(name)
	artifactPath := path.Join(artifactName+".sol", artifactName+".json")

	v.log.Info("Opening artifact", "path", artifactPath, "name", name)
	f, err := v.artifactsFS.Open(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open artifact: %w", err)
	}
	defer f.Close()

	var art foundry.Artifact
	if err := json.NewDecoder(f).Decode(&art); err != nil {
		return nil, fmt.Errorf("failed to decode artifact: %w", err)
	}

	// Add all sources (main contract and dependencies)
	sources := make(map[string]map[string]string)
	for sourcePath, sourceInfo := range art.Metadata.Sources {
		key := sourcePath
		// If the source comes from OpenZeppelin, adjust the key to match the Solidity import.
		if strings.HasPrefix(sourcePath, "lib/openzeppelin-contracts/contracts/") {
			key = strings.Replace(sourcePath, "lib/openzeppelin-contracts/contracts/", "@openzeppelin/contracts/", 1)
		}

		sources[key] = map[string]string{
			"content": sourceInfo.Content,
		}
		v.log.Info("added source", "originalPath", sourcePath, "key", key)
	}

	var optimizer struct {
		Enabled bool `json:"enabled"`
		Runs    int  `json:"runs"`
	}
	if err := json.Unmarshal(art.Metadata.Settings.Optimizer, &optimizer); err != nil {
		return nil, fmt.Errorf("failed to parse optimizer settings: %w", err)
	}

	standardInput := map[string]interface{}{
		"language": "Solidity",
		"sources":  sources,
		"settings": map[string]interface{}{
			"optimizer": map[string]interface{}{
				"enabled": optimizer.Enabled,
				"runs":    optimizer.Runs,
			},
			"evmVersion": art.Metadata.Settings.EVMVersion,
			"metadata": map[string]interface{}{
				"useLiteralContent": true,
				"bytecodeHash":      "none",
			},
			"outputSelection": map[string]interface{}{
				"*": map[string]interface{}{
					"*": []string{
						"abi",
						"evm.bytecode.object",
						"evm.bytecode.sourceMap",
						"evm.deployedBytecode.object",
						"evm.deployedBytecode.sourceMap",
						"metadata",
					},
				},
			},
		},
	}

	standardInputJSON, err := json.Marshal(standardInput)
	if err != nil {
		return nil, fmt.Errorf("failed to generate standard input: %w", err)
	}

	// Get the contract name from the compilation target
	var contractName string
	for contractFile, name := range art.Metadata.Settings.CompilationTarget {
		contractName = contractFile + ":" + name
		break
	}

	v.log.Info("contractName", "name", contractName)

	return &ContractArtifact{
		ContractName:     contractName,
		CompilerVersion:  art.Metadata.Compiler.Version,
		OptimizationUsed: optimizer.Enabled,
		OptimizationRuns: optimizer.Runs,
		EVMVersion:       art.Metadata.Settings.EVMVersion,
		StandardInput:    string(standardInputJSON),
	}, nil
}
