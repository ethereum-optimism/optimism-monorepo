package inspect

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"

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

	envVars := map[string]string{}
	envVars["SCR_CHAIN_NAME"] = ""
	envVars["SCR_CHAIN_SHORT_NAME"] = ""
	envVars["SCR_PUBLIC_RPC"] = ""
	envVars["SCR_SEQUENCER_RPC"] = ""
	envVars["SCR_EXPLORER"] = ""
	envVars["SCR_STANDARD_CHAIN_CANDIDATE"] = "false"

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

	genesis, rollup, err := GenesisAndRollup(globalState, cfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to generate genesis and rollup: %w", err)
	}
	genesisFilepath := filepath.Join(cfg.Workdir, "genesis.json")
	if err := jsonutil.WriteJSON(genesis, ioutil.ToStdOutOrFileOrNoop(genesisFilepath, 0o666)); err != nil {
		return fmt.Errorf("failed to write genesis: %w", err)
	}
	rollupFilepath := filepath.Join(cfg.Workdir, "rollup.json")
	if err := jsonutil.WriteJSON(rollup, ioutil.ToStdOutOrFileOrNoop(rollupFilepath, 0o666)); err != nil {
		return fmt.Errorf("failed to write rollup: %w", err)
	}

	deployConfig, err := DeployConfig(globalState, cfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to generate deploy config: %w", err)
	}
	deployConfigFilepath := filepath.Join(cfg.Workdir, "deploy-config.json")
	if err := jsonutil.WriteJSON(deployConfig, ioutil.ToStdOutOrFileOrNoop(deployConfigFilepath, 0o666)); err != nil {
		return fmt.Errorf("failed to write rollup: %w", err)
	}

	l1Contracts, err := L1(globalState, cfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to generate l1 contracts: %w", err)
	}
	l1ContractsFilepath := filepath.Join(cfg.Workdir, "l1-contracts.json")
	if err := jsonutil.WriteJSON(l1Contracts, ioutil.ToStdOutOrFileOrNoop(l1ContractsFilepath, 0o666)); err != nil {
		return fmt.Errorf("failed to write rollup: %w", err)
	}

	envVars["SCR_GENESIS"] = genesisFilepath
	envVars["SCR_ROLLUP_CONFIG"] = rollupFilepath
	envVars["SCR_DEPLOY_CONFIG"] = deployConfigFilepath
	envVars["SCR_DEPLOYMENTS_DIR"] = l1ContractsFilepath

	envFilepath := filepath.Join(cfg.Workdir, "superchain-registry.env")
	err = writeEnvFile(envFilepath, envVars)
	if err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	fmt.Printf("---------------------------------------------------\n"+
		"Please populate any empty values in your .env file\n"+
		"before creating your pull-request to add this chain\n"+
		"to the superchain-registry repo.\n\n"+
		"  * %s\n"+
		"---------------------------------------------------\n", envFilepath,
	)

	return nil
}

func writeEnvFile(filepath string, envVars map[string]string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	for key, value := range envVars {
		_, err := file.WriteString(fmt.Sprintf("%s=\"%s\"\n", key, value))
		if err != nil {
			return err
		}
	}

	return nil
}
