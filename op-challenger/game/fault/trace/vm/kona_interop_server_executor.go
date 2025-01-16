package vm

import (
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum/go-ethereum/common"
)

type KonaInteropExecutor struct {
	nativeMode     bool
	agreedPreState []byte
}

var _ OracleServerExecutor = (*KonaInteropExecutor)(nil)

func NewKonaInteropExecutor(agreedPreState []byte) *KonaInteropExecutor {
	return &KonaInteropExecutor{nativeMode: false, agreedPreState: agreedPreState}
}

func NewNativeKonaInteropExecutor(agreedPreState []byte) *KonaInteropExecutor {
	return &KonaInteropExecutor{nativeMode: true, agreedPreState: agreedPreState}
}

func (s *KonaInteropExecutor) OracleCommand(cfg Config, dataDir string, inputs utils.LocalGameInputs) ([]string, error) {
	args := []string{
		cfg.Server,
		"super",
		"--l1-node-address", cfg.L1,
		"--l1-beacon-address", cfg.L1Beacon,
		"--l2-node-addresses", cfg.L2,
		"--l1-head", inputs.L1Head.Hex(),
		"--agreed-l2-pre-state", common.Bytes2Hex(s.agreedPreState),
		"--claimed-l2-post-state", inputs.L2Claim.Hex(),
		"--claimed-l2-timestamp", inputs.L2BlockNumber.Text(10),
	}

	if s.nativeMode {
		args = append(args, "--native")
	} else {
		args = append(args, "--server")
		args = append(args, "--data-dir", dataDir)
	}

	if cfg.RollupConfigPath != "" {
		args = append(args, "--rollup-config-paths", cfg.RollupConfigPath)
	}

	return args, nil
}
