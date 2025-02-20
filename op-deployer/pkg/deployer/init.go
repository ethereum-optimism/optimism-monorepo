package deployer

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"

	op_service "github.com/ethereum-optimism/optimism/op-service"

	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

type InitConfig struct {
	IntentType state.IntentType
	L1ChainID  uint64
	Outdir     string
	L2ChainIDs []common.Hash
}

func (c *InitConfig) Check() error {
	if c.L1ChainID == 0 {
		return fmt.Errorf("l1ChainID must be specified")
	}

	if c.Outdir == "" {
		return fmt.Errorf("outdir must be specified")
	}

	if len(c.L2ChainIDs) == 0 {
		return fmt.Errorf("must specify at least one L2 chain ID")
	}

	return nil
}

func InitCLI() func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		l1ChainID := ctx.Uint64(L1ChainIDFlagName)
		outdir := ctx.String(OutdirFlagName)
		l2ChainIDsRaw := ctx.String(L2ChainIDsFlagName)
		intentType := ctx.String(IntentTypeFlagName)

		if len(l2ChainIDsRaw) == 0 {
			return fmt.Errorf("must specify at least one L2 chain ID")
		}

		l2ChainIDsStr := strings.Split(strings.TrimSpace(l2ChainIDsRaw), ",")
		l2ChainIDs := make([]common.Hash, len(l2ChainIDsStr))
		for i, idStr := range l2ChainIDsStr {
			id, err := op_service.Parse256BitChainID(idStr)
			if err != nil {
				return fmt.Errorf("invalid L2 chain ID '%s': %w", idStr, err)
			}
			l2ChainIDs[i] = id
		}

		err := Init(InitConfig{
			IntentType: state.IntentType(intentType),
			L1ChainID:  l1ChainID,
			Outdir:     outdir,
			L2ChainIDs: l2ChainIDs,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Successfully initialized op-deployer intent in directory: %s\n", outdir)
		return nil
	}
}

func Init(cfg InitConfig) error {
	if err := cfg.Check(); err != nil {
		return fmt.Errorf("invalid config for init: %w", err)
	}

	intent, err := state.NewIntent(cfg.IntentType, cfg.L1ChainID, cfg.L2ChainIDs)
	if err != nil {
		return err
	}
	intent.ConfigType = cfg.IntentType

	st := &state.State{
		Version: 1,
	}

	stat, err := os.Stat(cfg.Outdir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(cfg.Outdir, 0755); err != nil {
			return fmt.Errorf("failed to create outdir: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to stat outdir: %w", err)
	} else if !stat.IsDir() {
		return fmt.Errorf("outdir is not a directory")
	}

	if err := intent.WriteToFile(path.Join(cfg.Outdir, "intent.toml")); err != nil {
		return fmt.Errorf("failed to write intent to file: %w", err)
	}
	if err := st.WriteToFile(path.Join(cfg.Outdir, "state.json")); err != nil {
		return fmt.Errorf("failed to write state to file: %w", err)
	}
	return nil
}
