package inspect

import (
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"

	"github.com/urfave/cli/v2"
)

func SuperchainRegistryCLI(cliCtx *cli.Context) error {
	cfg, err := readConfig(cliCtx)
	if err != nil {
		return err
	}

	globalIntent, err := pipeline.ReadIntent(cfg.Workdir)
	if err != nil {
		return fmt.Errorf("failed to read intent: %w", err)
	}

	chainIntent, err := globalIntent.Chain(cfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to get chain ID %s: %w", cfg.ChainID.String(), err)
	}

	envVars := map[string]string{}
	envVars["SCR_STANDARD_CHAIN_CANDIDATE"] = "false"

	if err = chainIntent.SuperchainRegistry.Check(); err != nil {
		return fmt.Errorf("must fill all fields in intent's superchainRegistryConfig struct")
	}
	envVars["SCR_CHAIN_NAME"] = chainIntent.SuperchainRegistry.ChainName
	envVars["SCR_CHAIN_SHORT_NAME"] = chainIntent.SuperchainRegistry.ChainShortName
	envVars["SCR_PUBLIC_RPC"] = chainIntent.SuperchainRegistry.PublicRpc
	envVars["SCR_SEQUENCER_RPC"] = chainIntent.SuperchainRegistry.SequencerRpc
	envVars["SCR_EXPLORER"] = chainIntent.SuperchainRegistry.ExplorerUrl

	creationCommit, err := standard.CommitForDeployTag(globalIntent.L2ContractsLocator.Tag)
	if err != nil {
		return fmt.Errorf("failed to get commit for deploy tag: %w", err)
	}
	envVars["SCR_GENESIS_CREATION_COMMIT"] = creationCommit

	l1ChainName, err := standard.ChainNameFor(globalIntent.L1ChainID)
	if err != nil {
		return fmt.Errorf("failed to get l1 chain name: %w", err)
	}
	envVars["SCR_SUPERCHAIN_TARGET"] = l1ChainName

	globalState, err := pipeline.ReadState(cfg.Workdir)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}
	chainState, err := globalState.Chain(cfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to get chain state for ID %s: %w", cfg.ChainID.String(), err)
	}
	if err = chainState.Artifacts.Check(); err != nil {
		return fmt.Errorf("%w: chainId %s", err, cfg.ChainID.String())
	}
	envVars["SCR_DEPLOYMENTS_DIR"] = chainState.Artifacts.ContractAddresses
	envVars["SCR_ROLLUP_CONFIG"] = chainState.Artifacts.RollupConfig
	envVars["SCR_GENESIS"] = chainState.Artifacts.Genesis
	envVars["SCR_DEPLOY_CONFIG"] = chainState.Artifacts.DeployConfig

	err = writeEnvFile(envVars)

	return nil
}

func writeEnvFile(envVars map[string]string) error {
	// Open the file for writing, create it if it doesn't exist
	file, err := os.Create(".env")
	if err != nil {
		return err
	}
	defer file.Close()

	// Write each environment variable to the file
	for key, value := range envVars {
		_, err := file.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		if err != nil {
			return err
		}
	}

	return nil
}
