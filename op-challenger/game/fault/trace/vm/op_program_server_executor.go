package vm

import (
	"context"
	"strings"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum/go-ethereum/log"
)

type OpProgramServerExecutor struct {
	logger log.Logger
}

var _ OracleServerExecutor = (*OpProgramServerExecutor)(nil)

func NewOpProgramServerExecutor(logger log.Logger) *OpProgramServerExecutor {
	return &OpProgramServerExecutor{logger: logger}
}

func (s *OpProgramServerExecutor) OracleCommand(cfg Config, dataDir string, inputs utils.LocalGameInputs) ([]string, error) {
	args := []string{
		cfg.Server, "--server",
		"--l1", cfg.L1,
		"--l1.beacon", cfg.L1Beacon,
		"--l2", strings.Join(cfg.L2s, ","),
		"--datadir", dataDir,
		"--l1.head", inputs.L1Head.Hex(),
		"--l2.head", inputs.L2Head.Hex(),
		"--l2.outputroot", inputs.L2OutputRoot.Hex(),
		"--l2.claim", inputs.L2Claim.Hex(),
		"--l2.blocknumber", inputs.L2BlockNumber.Text(10),
	}
	if len(cfg.Networks) != 0 {
		args = append(args, "--network", strings.Join(cfg.Networks, ","))
	}
	if len(cfg.RollupConfigPaths) != 0 {
		args = append(args, "--rollup.config", strings.Join(cfg.RollupConfigPaths, ","))
	}
	if len(cfg.L2GenesisPaths) != 0 {
		args = append(args, "--l2.genesis", strings.Join(cfg.L2GenesisPaths, ","))
	}
	if cfg.L2Experimental != "" {
		args = append(args, "--l2.experimental", cfg.L2Experimental)
	}
	var logLevel string
	if s.logger.Enabled(context.Background(), log.LevelTrace) {
		logLevel = "TRACE"
	} else if s.logger.Enabled(context.Background(), log.LevelDebug) {
		logLevel = "DEBUG"
	} else if s.logger.Enabled(context.Background(), log.LevelInfo) {
		logLevel = "INFO"
	} else if s.logger.Enabled(context.Background(), log.LevelWarn) {
		logLevel = "WARN"
	} else if s.logger.Enabled(context.Background(), log.LevelError) {
		logLevel = "ERROR"
	} else {
		logLevel = "CRIT"
	}
	args = append(args, "--log.level", logLevel)
	if cfg.L2Custom {
		args = append(args, "--l2.custom")
	}
	return args, nil
}
