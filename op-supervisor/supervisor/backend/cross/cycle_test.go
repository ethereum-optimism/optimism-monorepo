package cross

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type mockCycleCheckDeps struct {
	openBlockFn func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error)
}

func (m *mockCycleCheckDeps) OpenBlock(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
	return m.openBlockFn(chainID, blockNum)
}

func TestHazardCycleChecks_OpenBlockError(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			return types.BlockSeal{}, 0, nil, errors.New("failed to open block")
		},
	}
	hazards := map[types.ChainIndex]types.BlockSeal{
		types.ChainIndex(1): {Number: 1},
	}
	err := HazardCycleChecks(deps, 100, hazards)
	require.Error(t, err, "expected error when OpenBlock fails")
	require.Contains(t, err.Error(), "failed to open block", "expected OpenBlock error message")
}

func TestHazardCycleChecks_InvalidLogIndex(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			msgs := map[uint32]*types.ExecutingMessage{
				5: {Chain: types.ChainIndex(1), LogIdx: 0, Timestamp: 100}, // Invalid index >= logCount
			}
			return types.BlockSeal{Number: blockNum}, 3, msgs, nil
		},
	}
	hazards := map[types.ChainIndex]types.BlockSeal{
		types.ChainIndex(1): {Number: 1},
	}
	err := HazardCycleChecks(deps, 100, hazards)
	require.ErrorIs(t, err, ErrInvalidLogIndex, "expected invalid log index error")
}

func TestHazardCycleChecks_NoHazards(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			return types.BlockSeal{Number: blockNum}, 0, make(map[uint32]*types.ExecutingMessage), nil
		},
	}
	hazards := make(map[types.ChainIndex]types.BlockSeal)
	err := HazardCycleChecks(deps, 100, hazards)
	require.NoError(t, err, "expected no error when there are no hazards")
}

func TestHazardCycleChecks_1CycleDetected(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			msgs := map[uint32]*types.ExecutingMessage{
				0: {Chain: types.ChainIndex(1), LogIdx: 0, Timestamp: 100}, // 0 points at itself
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

func TestHazardCycleChecks_2CycleDetected(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			msgs := map[uint32]*types.ExecutingMessage{
				0: {Chain: types.ChainIndex(1), LogIdx: 1, Timestamp: 100}, // 0 points to 1
				1: {Chain: types.ChainIndex(1), LogIdx: 0, Timestamp: 100}, // 1 points back to 0
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

func TestHazardCycleChecks_3CycleDetected(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			msgs := map[uint32]*types.ExecutingMessage{
				1: {Chain: types.ChainIndex(1), LogIdx: 2, Timestamp: 100}, // 0 points to 1
				2: {Chain: types.ChainIndex(1), LogIdx: 3, Timestamp: 100}, // 1 points to 2
				3: {Chain: types.ChainIndex(1), LogIdx: 1, Timestamp: 100}, // 2 points back to 0
			}
			return types.BlockSeal{Number: blockNum}, 4, msgs, nil
		},
	}
	hazards := map[types.ChainIndex]types.BlockSeal{
		types.ChainIndex(1): {Number: 1},
	}
	err := HazardCycleChecks(deps, 100, hazards)
	require.ErrorIs(t, err, ErrCycle, "expected cycle detection error for 3-node cycle")
}

func TestHazardCycleChecks_BlockMismatch(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			return types.BlockSeal{Number: blockNum + 1}, 0, make(map[uint32]*types.ExecutingMessage), nil
		},
	}
	hazards := map[types.ChainIndex]types.BlockSeal{
		types.ChainIndex(1): {Number: 1},
	}
	err := HazardCycleChecks(deps, 100, hazards)
	require.Error(t, err, "expected error due to block mismatch")
	require.Contains(t, err.Error(), "tried to open block", "expected block mismatch error message")
}

func TestHazardCycleChecks_CrossChain2CycleDetected(t *testing.T) {
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			// Create different responses based on chainID to simulate cross-chain messages
			switch chainID.String() {
			case "1":
				// Chain 1 has an executing message at log index 1 that points to Chain 2's log 1
				msgs := map[uint32]*types.ExecutingMessage{
					1: {Chain: types.ChainIndex(2), LogIdx: 1, Timestamp: 100},
				}
				return types.BlockSeal{Number: blockNum}, 2, msgs, nil
			case "2":
				// Chain 2 has an executing message at log index 1 that points to Chain 1's log 1
				msgs := map[uint32]*types.ExecutingMessage{
					1: {Chain: types.ChainIndex(1), LogIdx: 1, Timestamp: 100},
				}
				return types.BlockSeal{Number: blockNum}, 2, msgs, nil
			default:
				return types.BlockSeal{}, 0, nil, errors.New("unexpected chain")
			}
		},
	}

	hazards := map[types.ChainIndex]types.BlockSeal{
		types.ChainIndex(1): {Number: 1},
		types.ChainIndex(2): {Number: 1},
	}

	err := HazardCycleChecks(deps, 100, hazards)
	require.ErrorIs(t, err, ErrCycle, "expected cycle detection error for cycle through executing messages")
}
