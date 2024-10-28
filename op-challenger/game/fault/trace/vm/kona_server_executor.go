package vm

import (
	"errors"
	"strconv"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
)

type KonaExecutor struct {
	nativeMode    bool
	clientBinPath string
}

var _ OracleServerExecutor = (*KonaExecutor)(nil)

func NewKonaExecutor() *KonaExecutor {
	return &KonaExecutor{nativeMode: false}
}

func NewNativeKonaExecutor(clientBinPath string) *KonaExecutor {
	return &KonaExecutor{nativeMode: true, clientBinPath: clientBinPath}
}

func (s *KonaExecutor) OracleCommand(cfg Config, dataDir string, inputs utils.LocalGameInputs) ([]string, error) {
	args := []string{
		cfg.Server,
		"--l1-node-address", cfg.L1,
		"--l1-beacon-address", cfg.L1Beacon,
		"--l2-node-address", cfg.L2,
		"--l1-head", inputs.L1Head.Hex(),
		"--l2-head", inputs.L2Head.Hex(),
		"--l2-output-root", inputs.L2OutputRoot.Hex(),
		"--l2-claim", inputs.L2Claim.Hex(),
		"--l2-block-number", inputs.L2BlockNumber.Text(10),
	}

	if s.nativeMode {
		args = append(args, "--exec", s.clientBinPath)
	} else {
		args = append(args, "--server")
		args = append(args, "--data-dir", dataDir)
	}

	if cfg.RollupConfigPath != "" {
		args = append(args, "--rollup-config-path", cfg.RollupConfigPath)
	} else {
		if cfg.Network == "" {
			return nil, errors.New("network is not defined")
		}

		chainCfg := chaincfg.ChainByName(cfg.Network)
		args = append(args, "--l2-chain-id", strconv.FormatUint(chainCfg.ChainID, 10))
	}

	return args, nil
}
