package inspect

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/urfave/cli/v2"
)

func DeployConfigCLI(cliCtx *cli.Context) error {
	cliCfg, err := readConfig(cliCtx)
	if err != nil {
		return err
	}

	globalState, err := pipeline.ReadState(cliCfg.Workdir)
	if err != nil {
		return fmt.Errorf("failed to read intent: %w", err)
	}
	chainState, err := globalState.Chain(cliCfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to find chain state: %w", err)
	}

	intent := globalState.AppliedIntent
	if intent == nil {
		return fmt.Errorf("can only run this command following a full apply")
	}
	chainIntent, err := intent.Chain(cliCfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to find chain intent: %w", err)
	}

	config, err := state.CombineDeployConfig(intent, chainIntent, globalState, chainState)
	if err != nil {
		return fmt.Errorf("failed to generate deploy config: %w", err)
	}
	if err := jsonutil.WriteJSON(config, ioutil.ToStdOutOrFileOrNoop(cliCfg.Outfile, 0o666)); err != nil {
		return fmt.Errorf("failed to write deploy config: %w", err)
	}

	return nil
}
