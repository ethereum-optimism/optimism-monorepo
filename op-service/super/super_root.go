package super

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type OutputRootSource interface {
	OutputAtBlock(ctx context.Context, blockNum uint64) (*eth.OutputResponse, error)
	RollupConfig(ctx context.Context) (*rollup.Config, error)
}

type chainInfo struct {
	chainID     *big.Int
	source      OutputRootSource
	blockTime   uint64
	genesisTime uint64
}

func (c *chainInfo) blockNumberAtTime(timestamp uint64) uint64 {
	if timestamp < c.genesisTime {
		return 0
	}
	return (timestamp - c.genesisTime) / c.blockTime
}

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
		chainID := config.L2ChainID
		chains = append(chains, &chainInfo{
			chainID:     chainID,
			source:      source,
			blockTime:   config.BlockTime,
			genesisTime: config.Genesis.L2Time,
		})
	}
	slices.SortFunc(chains, func(a, b *chainInfo) int {
		return a.chainID.Cmp(b.chainID)
	})
	return &SuperRootSource{chains: chains}, nil
}

func (s *SuperRootSource) CreateSuperRoot(ctx context.Context, timestamp uint64) (*eth.OutputV1, error) {
	chainOutputs := make([]eth.Bytes32, len(s.chains))
	for i, chain := range s.chains {
		blockNum := chain.blockNumberAtTime(timestamp)
		output, err := chain.source.OutputAtBlock(ctx, blockNum)
		if err != nil {
			return nil, fmt.Errorf("failed to load output root for chain %v at block %v: %w", chain.chainID, blockNum, err)
		}
		chainOutputs[i] = output.OutputRoot
	}
	output := eth.OutputV1{
		Timestamp: timestamp,
		Outputs:   chainOutputs,
	}
	return &output, nil
}
