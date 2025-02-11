package dsl

import (
	"context"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"
)

type Outputs struct {
	t               helpers.Testing
	superRootSource *SuperRootSource
}

func (d *Outputs) SuperRoot(timestamp uint64) eth.Super {
	ctx, cancel := context.WithTimeout(d.t.Ctx(), 30*time.Second)
	defer cancel()
	root, err := d.superRootSource.CreateSuperRoot(ctx, timestamp)
	require.NoError(d.t, err)
	return root
}

func (d *Outputs) OutputRootAtTimestamp(chain *Chain, timestamp uint64) *eth.OutputResponse {
	ctx, cancel := context.WithTimeout(d.t.Ctx(), 30*time.Second)
	defer cancel()
	blockNum, err := chain.RollupCfg.TargetBlockNumber(timestamp)
	require.NoError(d.t, err)
	output, err := chain.Sequencer.RollupClient().OutputAtBlock(ctx, blockNum)
	require.NoError(d.t, err)
	return output
}

func (d *Outputs) OptimisticBlockAtTimestamp(chain *Chain, timestamp uint64) types.OptimisticBlock {
	root := d.OutputRootAtTimestamp(chain, timestamp)
	return types.OptimisticBlock{BlockHash: root.BlockRef.Hash, OutputRoot: root.OutputRoot}
}

func (d *Outputs) TransitionState(timestamp uint64, step uint64, pendingProgress ...types.OptimisticBlock) *types.TransitionState {
	return &types.TransitionState{
		SuperRoot:       d.SuperRoot(timestamp).Marshal(),
		PendingProgress: pendingProgress,
		Step:            step,
	}
}
