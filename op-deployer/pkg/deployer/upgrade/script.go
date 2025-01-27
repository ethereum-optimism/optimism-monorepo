package upgrade

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type UpgradeScript struct {
	Run func(common.Address, []*OpChainConfig) error
}

func RunUpgradeScript(
	host *script.Host,
	opcmAddr common.Address,
	configs []*OpChainConfig,
	scriptFile string,
	contractName string,
) error {
	upgradeScript, cleanupUpgrade, err := script.WithScript[UpgradeScript](host, scriptFile, contractName)
	if err != nil {
		return fmt.Errorf("failed to load %s script: %w", scriptFile, err)
	}
	defer cleanupUpgrade()

	if err := upgradeScript.Run(opcmAddr, configs); err != nil {
		return fmt.Errorf("failed to run %s script: %w", scriptFile, err)
	}

	return nil
}
