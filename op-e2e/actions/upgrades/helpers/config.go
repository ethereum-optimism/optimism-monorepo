package helpers

import (
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// ApplyDeltaTimeOffset adjusts fork configuration to not conflict with the delta overrides
func ApplyDeltaTimeOffset(dp *e2eutils.DeployParams, deltaTimeOffset *hexutil.Uint64) {
	dp.DeployConfig.L2GenesisDeltaTimeOffset = deltaTimeOffset
	dp.DeployConfig.L2GenesisGraniteTimeOffset = nil
	// configure Ecotone to not be before Delta accidentally
	if dp.DeployConfig.L2GenesisEcotoneTimeOffset != nil {
		if deltaTimeOffset == nil {
			dp.DeployConfig.L2GenesisEcotoneTimeOffset = nil
		} else if *dp.DeployConfig.L2GenesisEcotoneTimeOffset < *deltaTimeOffset {
			dp.DeployConfig.L2GenesisEcotoneTimeOffset = deltaTimeOffset
		}
	}
	// configure Fjord to not be before Delta accidentally
	if dp.DeployConfig.L2GenesisFjordTimeOffset != nil {
		if deltaTimeOffset == nil {
			dp.DeployConfig.L2GenesisFjordTimeOffset = nil
		} else if *dp.DeployConfig.L2GenesisFjordTimeOffset < *deltaTimeOffset {
			dp.DeployConfig.L2GenesisFjordTimeOffset = deltaTimeOffset
		}
	}
}
