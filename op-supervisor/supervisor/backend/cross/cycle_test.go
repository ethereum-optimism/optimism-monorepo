package cross

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type mockCycleCheckDeps struct {
	openBlockFn func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, []*types.ExecutingMessage, error)
}

func (m *mockCycleCheckDeps) OpenBlock(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, []*types.ExecutingMessage, error) {
	return m.openBlockFn(chainID, blockNum)
}

func TestHazardCycleChecks_NoHazards(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, []*types.ExecutingMessage, error) {
			return types.BlockSeal{Number: blockNum}, 0, nil, nil
		},
	}
	hazards := make(map[types.ChainIndex]types.BlockSeal)
	err := HazardCycleChecks(deps, 100, hazards)
	require.NoError(t, err, "expected no error when there are no hazards")
}

func TestHazardCycleChecks_CycleDetected(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, []*types.ExecutingMessage, error) {
			msgs := []*types.ExecutingMessage{
				{Chain: types.ChainIndex(1), LogIdx: 0, Timestamp: 100},
				{Chain: types.ChainIndex(1), LogIdx: 1, Timestamp: 100},
				{Chain: types.ChainIndex(1), LogIdx: 0, Timestamp: 100}, // Creates a cycle
			}
			return types.BlockSeal{Number: blockNum}, 3, msgs, nil
		},
	}
	hazards := map[types.ChainIndex]types.BlockSeal{
		types.ChainIndex(1): {Number: 1},
	}
	err := HazardCycleChecks(deps, 100, hazards)
	require.ErrorIs(t, err, ErrCycle, "expected cycle detection error")
}

func TestHazardCycleChecks_BlockMismatch(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, []*types.ExecutingMessage, error) {
			return types.BlockSeal{Number: blockNum + 1}, 0, nil, nil
		},
	}
	hazards := map[types.ChainIndex]types.BlockSeal{
		types.ChainIndex(1): {Number: 1},
	}
	err := HazardCycleChecks(deps, 100, hazards)
	require.Error(t, err, "expected error due to block mismatch")
	require.Contains(t, err.Error(), "tried to open block", "expected block mismatch error message")
}

func TestHazardCycleChecks_OpenBlockError(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, []*types.ExecutingMessage, error) {
			return types.BlockSeal{}, 0, nil, errors.New("failed to open block")
		},
	}
	hazards := map[types.ChainIndex]types.BlockSeal{
		types.ChainIndex(1): {Number: 1},
	}
	err := HazardCycleChecks(deps, 100, hazards)
	require.Error(t, err, "expected error when OpenBlock fails")
	require.Contains(t, err.Error(), "tried to open block", "expected OpenBlock error message")
}
