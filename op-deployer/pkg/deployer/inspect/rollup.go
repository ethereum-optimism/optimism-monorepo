package inspect

import (
	"fmt"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/urfave/cli/v2"
)

func RollupCLI(cliCtx *cli.Context) error {
	cfg, err := readConfig(cliCtx)
	if err != nil {
		return err
	}

	globalState, err := pipeline.ReadState(cfg.Workdir)
	if err != nil {
		return fmt.Errorf("failed to read intent: %w", err)
	}

	_, rollupConfig, err := GenesisAndRollup(globalState, cfg.ChainID)
	if rollupConfig.HoloceneTime == nil {
		rollupConfig.Genesis.SystemConfig.MarshalPreHolocene = true
	}
	if err != nil {
		return fmt.Errorf("failed to generate rollup config: %w", err)
	}

	if err := jsonutil.WriteJSON(rollupConfig, ioutil.ToStdOutOrFileOrNoop(cfg.Outfile, 0o666)); err != nil {
		return fmt.Errorf("failed to write rollup config: %w", err)
	}

	chainState, err := globalState.Chain(cfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to find chain state: %w", err)
	}
	chainState.Artifacts.RollupConfig = filepath.Join(cfg.Workdir, cfg.Outfile)
	if err = pipeline.WriteState(cfg.Workdir, globalState); err != nil {
		return fmt.Errorf("failed to write updated globalState: %w", err)
	}

	return nil
}
