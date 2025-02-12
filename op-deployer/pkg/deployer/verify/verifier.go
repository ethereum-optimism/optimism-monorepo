package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

	client, err := ethclient.Dial(l1RPCUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L1: %w", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}

	st, err := pipeline.ReadState(workdir)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	progressor := func(curr, total int64) {
		l.Info("artifacts download progress", "current", curr, "total", total)
	}

	// Get artifacts filesystem (either local or downloaded)
	artifactsFS, err := artifacts.Download(ctx, st.AppliedIntent.L1ContractsLocator, progressor)
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
	SourceCode       string
	ContractName     string
	CompilerVersion  string
	OptimizationUsed bool
	OptimizationRuns int
	EVMVersion       string
}

func (v *Verifier) getContractArtifact(name string) (*ContractArtifact, error) {
	// Remove suffix if present
	lookupName := strings.TrimSuffix(name, "ProxyAddress")
	lookupName = strings.TrimSuffix(lookupName, "Address")

	artifactPath := path.Join(lookupName+".sol", lookupName+".json")

	v.log.Info("Opening artifact", "path", artifactPath, "lookupName", lookupName)
	f, err := v.artifactsFS.Open(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open artifact: %w", err)
	}
	defer f.Close()

	var art foundry.Artifact
	if err := json.NewDecoder(f).Decode(&art); err != nil {
		return nil, fmt.Errorf("failed to decode artifact: %w", err)
	}

	var optimizer struct {
		Enabled bool `json:"enabled"`
		Runs    int  `json:"runs"`
	}
	if err := json.Unmarshal(art.Metadata.Settings.Optimizer, &optimizer); err != nil {
		return nil, fmt.Errorf("failed to parse optimizer settings: %w", err)
	}

	// Get compiler version from the main artifact and clean it
	compilerVersion := art.Metadata.Compiler.Version
	compilerVersion = strings.Split(compilerVersion, "+")[0] // Remove the "+commit..." part
	v.log.Info("Using compiler version", "version", compilerVersion)

	// Combine all sources into one flat file
	var combinedSource strings.Builder
	for sourcePath := range art.Metadata.Sources {
		// Extract just the filename from the path
		baseName := strings.TrimSuffix(path.Base(sourcePath), ".sol")
		contractDir := baseName + ".sol"

		baseName = strings.TrimPrefix(baseName, "draft-")

		// Try to find the JSON file with matching compiler version
		sourceArtifactPath := path.Join(contractDir, fmt.Sprintf("%s.%s.json",
			baseName,
			compilerVersion))

		f, err := v.artifactsFS.Open(sourceArtifactPath)
		if err != nil {
			// Fallback to non-versioned file if version-specific one doesn't exist
			sourceArtifactPath = path.Join(contractDir, baseName+".json")
			f, err = v.artifactsFS.Open(sourceArtifactPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read source file %s: %w", baseName, err)
			}
		}
		content, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read source content %s: %w", sourceArtifactPath, err)
		}
		combinedSource.Write(content)
		combinedSource.WriteString("\n\n")

		v.log.Info("added source", "contract", baseName)
	}

	return &ContractArtifact{
		SourceCode:       combinedSource.String(),
		ContractName:     lookupName,
		CompilerVersion:  art.Metadata.Compiler.Version,
		OptimizationUsed: optimizer.Enabled,
		OptimizationRuns: optimizer.Runs,
		EVMVersion:       art.Metadata.Settings.EVMVersion,
	}, nil
}
