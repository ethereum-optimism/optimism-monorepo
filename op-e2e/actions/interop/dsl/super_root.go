package dsl

import (
	"context"
	"fmt"
	"slices"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type OutputRootSource interface {
	OutputAtBlock(ctx context.Context, blockNum uint64) (*eth.OutputResponse, error)
	RollupConfig(ctx context.Context) (*rollup.Config, error)
}

type chainInfo struct {
	chainID eth.ChainID
	source  OutputRootSource
	config  *rollup.Config
}

// SuperRootSource is a testing helper to create a Super Root from a set of rollup clients
type SuperRootSource struct {
	chains []*chainInfo
}

func NewSuperRootSource(ctx context.Context, sources ...OutputRootSource) (*SuperRootSource, error) {
	chains := make([]*chainInfo, 0, len(sources))
	for _, source := range sources {
		config, err := source.RollupConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load rollup config: %w", err)
		}
		chainID := eth.ChainIDFromBig(config.L2ChainID)
		chains = append(chains, &chainInfo{
			chainID: chainID,
			source:  source,
			config:  config,
		})
	}
	slices.SortFunc(chains, func(a, b *chainInfo) int {
		return a.chainID.Cmp(b.chainID)
	})
	return &SuperRootSource{chains: chains}, nil
}

func (s *SuperRootSource) CreateSuperRoot(ctx context.Context, timestamp uint64) (*eth.SuperV1, error) {
	chains := make([]eth.ChainIDAndOutput, len(s.chains))
	for i, chain := range s.chains {
		blockNum, err := chain.config.TargetBlockNumber(timestamp)
		if err != nil {
			return nil, err
		}
		output, err := chain.source.OutputAtBlock(ctx, blockNum)
		if err != nil {
			return nil, fmt.Errorf("failed to load output root for chain %v at block %v: %w", chain.chainID, blockNum, err)
		}
		chains[i] = eth.ChainIDAndOutput{ChainID: chain.chainID, Output: output.OutputRoot}
	}
	output := eth.SuperV1{
		Timestamp: timestamp,
		Chains:    chains,
	}
	return &output, nil
}
