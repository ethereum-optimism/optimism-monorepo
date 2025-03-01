package validators

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum/go-ethereum/params"
)

// ForkConfig holds the chain configuration and latest block timestamp
// for checking if various forks are activated.
type ForkConfig struct {
	config    *params.ChainConfig
	timestamp uint64
}

// IsForkActivated checks if a specific fork is activated at the given timestamp
// based on the chain configuration.
func (fc *ForkConfig) IsForkActivated(forkName rollup.ForkName) bool {
	switch forkName {
	case "bedrock":
		// Bedrock is activated based on block number, not timestamp
		return true // Assuming bedrock is always active in the context of this validator
	case "regolith":
		return fc.config.IsOptimismRegolith(fc.timestamp)
	case "canyon":
		return fc.config.IsOptimismCanyon(fc.timestamp)
	case "ecotone":
		return fc.config.IsOptimismEcotone(fc.timestamp)
	case "fjord":
		return fc.config.IsOptimismFjord(fc.timestamp)
	case "granite":
		return fc.config.IsOptimismGranite(fc.timestamp)
	case "holocene":
		return fc.config.IsOptimismHolocene(fc.timestamp)
	case "isthmus":
		return fc.config.IsOptimismIsthmus(fc.timestamp)
	case "jovian":
		return fc.config.IsOptimismJovian(fc.timestamp)
	default:
		return false
	}
}

// validateForkCondition is a helper function that checks if a specific L2 chain meets a fork condition.
func forkConfigValidator(chainIdx uint64, forkName rollup.ForkName, shouldBeActive bool, forkConfigMarker interface{}) systest.PreconditionValidator {
	return func(t systest.T, sys system.System) (context.Context, error) {
		if len(sys.L2s()) <= int(chainIdx) {
			return nil, fmt.Errorf("chain index %d out of range, only %d L2 chains available", chainIdx, len(sys.L2s()))
		}

		chain := sys.L2s()[chainIdx]

		chainConfig, err := chain.ChainConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get chain config for L2 chain %d: %w", chainIdx, err)
		}

		block, err := chain.LatestBlock(t.Context())
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block for L2 chain %d: %w", chainIdx, err)
		}

		forkConfig := &ForkConfig{
			config:    chainConfig,
			timestamp: block.Time(),
		}
		isActive := forkConfig.IsForkActivated(forkName)
		if isActive != shouldBeActive {
			if shouldBeActive {
				return nil, fmt.Errorf("L2 chain %d does not have fork %s activated, which it should be for this validator to pass", chainIdx, forkName)
			} else {
				return nil, fmt.Errorf("L2 chain %d has fork %s activated, but it should not be for this validator to pass", chainIdx, forkName)
			}
		}

		return context.WithValue(t.Context(), forkConfigMarker, forkConfig), nil
	}
}

// ForkConfigGetter is a function type that retrieves a ForkConfig from a context.
type ForkConfigGetter = func(context.Context) *ForkConfig

// AcquireForkConfig returns a ForkConfigGetter and a PreconditionValidator
// that ensures a ForkConfig is available for the specified L2 chain.
// The ForkConfig can be used to check if various forks are activated.
func acquireForkConfig(chainIdx uint64, forkName rollup.ForkName, shouldBeActive bool) (ForkConfigGetter, systest.PreconditionValidator) {
	forkConfigMarker := &struct{}{}
	validator := forkConfigValidator(chainIdx, forkName, shouldBeActive, forkConfigMarker)
	return func(ctx context.Context) *ForkConfig {
		return ctx.Value(forkConfigMarker).(*ForkConfig)
	}, validator
}

// RequiresFork returns a validator that ensures a specific L2 chain has a specific fork activated.
func AcquireRequiresFork(chainIdx uint64, forkName rollup.ForkName) (ForkConfigGetter, systest.PreconditionValidator) {
	return acquireForkConfig(chainIdx, forkName, true)
}

// RequiresNotFork returns a validator that ensures a specific L2 chain does not have a specific fork activated.
func AcquireRequiresNotFork(chainIdx uint64, forkName rollup.ForkName) (ForkConfigGetter, systest.PreconditionValidator) {
	return acquireForkConfig(chainIdx, forkName, false)
}
