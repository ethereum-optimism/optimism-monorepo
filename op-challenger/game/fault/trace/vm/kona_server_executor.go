package vm

import (
	"errors"
	"strconv"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
)

type KonaExecutor struct {
	nativeMode bool
}

var _ OracleServerExecutor = (*KonaExecutor)(nil)

func NewKonaExecutor() *KonaExecutor {
	return &KonaExecutor{nativeMode: false}
}

func NewNativeKonaExecutor() *KonaExecutor {
	return &KonaExecutor{nativeMode: true}
}

func (s *KonaExecutor) OracleCommand(cfg Config, dataDir string, inputs utils.LocalGameInputs) ([]string, error) {
	if len(cfg.L2s) != 1 || len(cfg.RollupConfigPaths) > 1 || len(cfg.Networks) > 1 {
		return nil, errors.New("multiple L2s specified but only one supported")
	}
	args := []string{
		cfg.Server,
		"single",
		"--l1-node-address", cfg.L1,
		"--l1-beacon-address", cfg.L1Beacon,
		"--l2-node-address", cfg.L2s[0],
		"--l1-head", inputs.L1Head.Hex(),
		"--l2-head", inputs.L2Head.Hex(),
		"--l2-output-root", inputs.L2OutputRoot.Hex(),
		"--l2-claim", inputs.L2Claim.Hex(),
		"--l2-block-number", inputs.L2BlockNumber.Text(10),
	}

	if s.nativeMode {
		args = append(args, "--native")
	} else {
		args = append(args, "--server")
		args = append(args, "--data-dir", dataDir)
	}

	if len(cfg.RollupConfigPaths) > 0 {
		args = append(args, "--rollup-config-path", cfg.RollupConfigPaths[0])
	} else {
		if len(cfg.Networks) == 0 {
			return nil, errors.New("network is not defined")
		}

		chainCfg := chaincfg.ChainByName(cfg.Networks[0])
		args = append(args, "--l2-chain-id", strconv.FormatUint(chainCfg.ChainID, 10))
	}

	return args, nil
}
