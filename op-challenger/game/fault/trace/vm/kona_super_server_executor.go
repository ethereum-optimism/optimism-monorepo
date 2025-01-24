package vm

import (
	"errors"
	"strings"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum/go-ethereum/common"
)

type KonaSuperExecutor struct {
	nativeMode bool
}

var _ OracleServerExecutor = (*KonaSuperExecutor)(nil)

func NewKonaSuperExecutor() *KonaSuperExecutor {
	return &KonaSuperExecutor{nativeMode: false}
}

func NewNativeKonaSuperExecutor() *KonaSuperExecutor {
	return &KonaSuperExecutor{nativeMode: true}
}

func (s *KonaSuperExecutor) OracleCommand(cfg Config, dataDir string, inputs utils.LocalGameInputs) ([]string, error) {
	if len(inputs.AgreedPreState) == 0 {
		return nil, errors.New("agreed pre-state is not defined")
	}

	args := []string{
		cfg.Server,
		"super",
		"--l1-node-address", cfg.L1,
		"--l1-beacon-address", cfg.L1Beacon,
		"--l2-node-addresses", strings.Join(cfg.L2s, ","),
		"--l1-head", inputs.L1Head.Hex(),
		"--agreed-l2-pre-state", common.Bytes2Hex(inputs.AgreedPreState),
		"--claimed-l2-post-state", inputs.L2Claim.Hex(),
		"--claimed-l2-timestamp", inputs.L2BlockNumber.Text(10),
	}

	if s.nativeMode {
		args = append(args, "--native")
	} else {
		args = append(args, "--server")
		args = append(args, "--data-dir", dataDir)
	}

	if len(cfg.RollupConfigPaths) != 0 {
		args = append(args, "--rollup-config-paths", strings.Join(cfg.RollupConfigPaths, ","))
	}

	return args, nil
}
