package cross

import (
	"errors"
	"fmt"
	"strconv"
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

type chainBlockDef struct {
	logCount uint32
	messages map[uint32]*types.ExecutingMessage
	error    error
}

type testCase struct {
	name        string
	chainBlocks map[string]chainBlockDef
	expectErr   error
	hazards     map[types.ChainIndex]types.BlockSeal
	openBlockFn func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error)
	msg         string
}

func chainIndex(s string) types.ChainIndex {
	id, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("invalid chain index in test: %v", err))
	}
	return types.ChainIndex(id)
}

func execMsg(chain string, logIdx uint32) *types.ExecutingMessage {
	return &types.ExecutingMessage{
		Chain:     chainIndex(chain),
		LogIdx:    logIdx,
		Timestamp: 100,
	}
}

var emptyChainBlocks = map[string]chainBlockDef{
	"1": {
		logCount: 0,
		messages: map[uint32]*types.ExecutingMessage{},
	},
}

func TestHazardCycleChecksFailures(t *testing.T) {
	tests := []testCase{
		{
			name:        "no hazards",
			chainBlocks: emptyChainBlocks,
			hazards:     make(map[types.ChainIndex]types.BlockSeal),
			expectErr:   nil,
			msg:         "expected no error when there are no hazards",
		},
		{
			name:        "failed to open block error",
			chainBlocks: emptyChainBlocks,
			openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
				return types.BlockSeal{}, 0, nil, ErrFailedToOpenBlock
			},
			expectErr: errors.New("failed to open block"),
			msg:       "expected error when OpenBlock fails",
		},
		{
			name:        "block mismatch error",
			chainBlocks: emptyChainBlocks,
			// openBlockFn returns a block number that doesn't match the expected block number.
			openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
				return types.BlockSeal{Number: blockNum + 1}, 0, make(map[uint32]*types.ExecutingMessage), nil
			},
			expectErr: errors.New("tried to open block"),
			msg:       "expected error due to block mismatch",
		},
		{
			name: "invalid log index error",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 3,
					messages: map[uint32]*types.ExecutingMessage{
						5: execMsg("1", 0), // Invalid index >= logCount.
					},
				},
			},
			expectErr: ErrInvalidLogIndex,
			msg:       "expected invalid log index error",
		},
		{
			name: "self reference detected error",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 1,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("1", 0), // Points at itself.
					},
				},
			},
			expectErr: ErrSelfReferencing,
			msg:       "expected self reference detection error",
		},
		{
			name: "unknown chain",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("2", 0), // References chain 2 which isn't in hazards.
					},
				},
			},
			hazards: map[types.ChainIndex]types.BlockSeal{
				1: {Number: 1}, // Only include chain 1.
			},
			expectErr: ErrUnknownChain,
			msg:       "expected unknown chain error",
		},
	}
	runHazardTestCaseGroup(t, "Failure", tests)
}

func TestHazardCycleChecksNoCycle(t *testing.T) {
	tests := []testCase{
		{
			name:        "no logs",
			chainBlocks: emptyChainBlocks,
			expectErr:   nil,
			msg:         "expected no cycle found for block with no logs",
		},
		{
			name: "one basic log",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 1,
					messages: map[uint32]*types.ExecutingMessage{},
				},
			},
			msg: "expected no cycle found for single basic log",
		},
		{
			name: "one exec log",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 0),
					},
				},
			},
			msg: "expected no cycle found for single exec log",
		},
		{
			name: "two basic logs",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{},
				},
			},
			msg: "expected no cycle found for two basic logs",
		},
		{
			name: "two exec logs to same target",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 3,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 0),
						2: execMsg("1", 0),
					},
				},
			},
			msg: "expected no cycle found for two exec logs pointing at the same log",
		},
		{
			name: "two exec logs to different targets",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 3,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 0),
						2: execMsg("1", 1),
					},
				},
			},
			msg: "expected no cycle found for two exec logs pointing at the different logs",
		},
		{
			name: "one basic log one exec log",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 0),
					},
				},
			},
			msg: "expected no cycle found for one basic and one exec log",
		},
		{
			name: "first log is exec",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 1,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("2", 0),
					},
				},
				"2": {
					logCount: 1,
					messages: nil,
				},
			},
			msg: "expected no cycle found first log is exec",
		},
	}
	runHazardTestCaseGroup(t, "NoCycle", tests)
}

func TestHazardCycleChecksCycle(t *testing.T) {
	tests := []testCase{
		{
			name: "2-cycle in single chain with first log",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 3,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("1", 2),
						2: execMsg("1", 0),
					},
				},
			},
			expectErr: ErrCycle,
			msg:       "expected cycle detection error",
		},
		{
			name: "2-cycle in single chain with first log, adjacent",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("1", 1),
						1: execMsg("1", 0),
					},
				},
			},
			expectErr: ErrCycle,
			msg:       "expected cycle detection error",
		},
		{
			name: "2-cycle in single chain, not first, adjacent",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 3,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 2),
						2: execMsg("1", 1),
					},
				},
			},
			expectErr: ErrCycle,
			msg:       "expected cycle detection error",
		},
		{
			name: "2-cycle in single chain, not first, not adjacent",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 4,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 3),
						3: execMsg("1", 1),
					},
				},
			},
			expectErr: ErrCycle,
			msg:       "expected cycle detection error",
		},
		{
			name: "2-cycle across chains",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("2", 1),
					},
				},
				"2": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 1),
					},
				},
			},
			expectErr: ErrCycle,
			msg:       "expected cycle detection error for cycle through executing messages",
		},
		{
			name: "3-cycle in single chain",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 4,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 2), // Points to log 2
						2: execMsg("1", 3), // Points to log 3
						3: execMsg("1", 1), // Points back to log 1
					},
				},
			},
			expectErr: ErrCycle,
			msg:       "expected cycle detection error for 3-node cycle",
		},
	}
	runHazardTestCaseGroup(t, "Cycle", tests)
}

func runHazardTestCaseGroup(t *testing.T, group string, tests []testCase) {
	for _, tc := range tests {
		t.Run(group+"/"+tc.name, func(t *testing.T) {
			runHazardTestCase(t, tc)
		})
	}
}

func runHazardTestCase(t *testing.T, tc testCase) {
	// Create mocked dependencies
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (types.BlockSeal, uint32, map[uint32]*types.ExecutingMessage, error) {
			// Use override if provided
			if tc.openBlockFn != nil {
				return tc.openBlockFn(chainID, blockNum)
			}

			// Default behavior
			chainStr := chainID.String()
			def, ok := tc.chainBlocks[chainStr]
			if !ok {
				return types.BlockSeal{}, 0, nil, errors.New("unexpected chain")
			}
			if def.error != nil {
				return types.BlockSeal{}, 0, nil, def.error
			}
			return types.BlockSeal{Number: blockNum}, def.logCount, def.messages, nil
		},
	}

	// Generate hazards map automatically if not explicitly provided
	var hazards map[types.ChainIndex]types.BlockSeal
	if tc.hazards != nil {
		hazards = tc.hazards
	} else {
		hazards = make(map[types.ChainIndex]types.BlockSeal)
		for chainStr := range tc.chainBlocks {
			hazards[chainIndex(chainStr)] = types.BlockSeal{Number: 1}
		}
	}

	err := HazardCycleChecks(deps, 100, hazards)

	// No error expected
	if tc.expectErr == nil {
		require.NoError(t, err, tc.msg)
		return
	}

	// Error expected, make sure it's the right one
	require.Error(t, err, tc.msg)
	if errors.Is(err, tc.expectErr) {
		require.ErrorIs(t, err, tc.expectErr, tc.msg)
	} else {
		require.Contains(t, err.Error(), tc.expectErr.Error(), tc.msg)
	}
}
