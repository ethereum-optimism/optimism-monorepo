package cross

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type mockCycleCheckDeps struct {
	openBlockFn func(chainID types.ChainID, blockNum uint64) (eth.BlockRef, uint32, map[uint32]*types.ExecutingMessage, error)
}

func (m *mockCycleCheckDeps) OpenBlock(chainID types.ChainID, blockNum uint64) (eth.BlockRef, uint32, map[uint32]*types.ExecutingMessage, error) {
	return m.openBlockFn(chainID, blockNum)
}

type chainBlockDef struct {
	logCount uint32
	messages map[uint32]*types.ExecutingMessage
	error    error
}

type hazardCycleChecksTestCase struct {
	name        string
	chainBlocks map[string]chainBlockDef
	expectErr   error
	msg         string

	// Optional overrides
	hazards     map[types.ChainIndex]types.BlockSeal
	openBlockFn func(chainID types.ChainID, blockNum uint64) (eth.BlockRef, uint32, map[uint32]*types.ExecutingMessage, error)
}

func runHazardCycleChecksTestCaseGroup(t *testing.T, group string, tests []hazardCycleChecksTestCase) {
	for _, tc := range tests {
		t.Run(group+"/"+tc.name, func(t *testing.T) {
			runHazardCycleChecksTestCase(t, tc)
		})
	}
}

func runHazardCycleChecksTestCase(t *testing.T, tc hazardCycleChecksTestCase) {
	// Create mocked dependencies
	deps := &mockCycleCheckDeps{
		openBlockFn: func(chainID types.ChainID, blockNum uint64) (eth.BlockRef, uint32, map[uint32]*types.ExecutingMessage, error) {
			// Use override if provided
			if tc.openBlockFn != nil {
				return tc.openBlockFn(chainID, blockNum)
			}

			// Default behavior
			chainStr := chainID.String()
			def, ok := tc.chainBlocks[chainStr]
			if !ok {
				return eth.BlockRef{}, 0, nil, errors.New("unexpected chain")
			}
			if def.error != nil {
				return eth.BlockRef{}, 0, nil, def.error
			}
			return eth.BlockRef{Number: blockNum}, def.logCount, def.messages, nil
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

	// Run the test
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

func chainIndex(s string) types.ChainIndex {
	id, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("invalid chain index in test: %v", err))
	}
	return types.ChainIndex(id)
}

func execMsg(chain string, logIdx uint32) *types.ExecutingMessage {
	return execMsgWithTimestamp(chain, logIdx, 100)
}

func execMsgWithTimestamp(chain string, logIdx uint32, timestamp uint64) *types.ExecutingMessage {
	return &types.ExecutingMessage{
		Chain:     chainIndex(chain),
		LogIdx:    logIdx,
		Timestamp: timestamp,
	}
}

var emptyChainBlocks = map[string]chainBlockDef{
	"1": {
		logCount: 0,
		messages: map[uint32]*types.ExecutingMessage{},
	},
}

func TestHazardCycleChecksFailures(t *testing.T) {
	testOpenBlockErr := errors.New("test OpenBlock error")
	tests := []hazardCycleChecksTestCase{
		{
			name:        "empty hazards",
			chainBlocks: emptyChainBlocks,
			hazards:     make(map[types.ChainIndex]types.BlockSeal),
			expectErr:   nil,
			msg:         "expected no error when there are no hazards",
		},
		{
			name:        "nil hazards",
			chainBlocks: emptyChainBlocks,
			hazards:     nil,
			expectErr:   nil,
			msg:         "expected no error when there are nil hazards",
		},
		{
			name:        "nil blocks",
			chainBlocks: nil,
			hazards:     nil,
			expectErr:   nil,
			msg:         "expected no error when there are nil blocks and hazards",
		},
		{
			name:        "failed to open block error",
			chainBlocks: emptyChainBlocks,
			openBlockFn: func(chainID types.ChainID, blockNum uint64) (eth.BlockRef, uint32, map[uint32]*types.ExecutingMessage, error) {
				return eth.BlockRef{}, 0, nil, testOpenBlockErr
			},
			expectErr: errors.New("failed to open block"),
			msg:       "expected error when OpenBlock fails",
		},
		{
			name:        "block mismatch error",
			chainBlocks: emptyChainBlocks,
			// openBlockFn returns a block number that doesn't match the expected block number.
			openBlockFn: func(chainID types.ChainID, blockNum uint64) (eth.BlockRef, uint32, map[uint32]*types.ExecutingMessage, error) {
				return eth.BlockRef{Number: blockNum + 1}, 0, make(map[uint32]*types.ExecutingMessage), nil
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
			expectErr: ErrExecMsgHasInvalidIndex,
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
			expectErr: types.ErrConflict,
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
			expectErr: ErrExecMsgUnknownChain,
			msg:       "expected unknown chain error",
		},
	}
	runHazardCycleChecksTestCaseGroup(t, "Failure", tests)
}

func TestHazardCycleChecksNoCycle(t *testing.T) {
	tests := []hazardCycleChecksTestCase{
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
		{
			name: "cycle through older timestamp",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("2", 0),
						1: execMsgWithTimestamp("2", 1, 101),
					},
				},
				"2": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("1", 1),
					},
				},
			},
			msg: "expected no cycle detection error for cycle through messages with different timestamps",
		},
		// This should be caught by earlier validations, but included for completeness.
		{
			name: "cycle through younger timestamp",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("2", 0),
						1: execMsgWithTimestamp("2", 1, 99),
					},
				},
				"2": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("1", 1),
					},
				},
			},
			msg: "expected no cycle detection error for cycle through messages with different timestamps",
		},
	}
	runHazardCycleChecksTestCaseGroup(t, "NoCycle", tests)
}

func TestHazardCycleChecksCycle(t *testing.T) {
	tests := []hazardCycleChecksTestCase{
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
						1: execMsg("2", 0),
					},
				},
				"2": {
					logCount: 2,
					messages: map[uint32]*types.ExecutingMessage{
						0: execMsg("1", 1),
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
		{
			name: "cycle through adjacency dependency",
			chainBlocks: map[string]chainBlockDef{
				"1": {
					logCount: 10,
					messages: map[uint32]*types.ExecutingMessage{
						1: execMsg("1", 5), // Points to log 5
						5: execMsg("1", 2), // Points back to log 2 which is adjacent to log 1
					},
				},
			},
			expectErr: ErrCycle,
			msg:       "expected cycle detection error for when cycle goes through adjacency dependency",
		},
		{
			name: "2-cycle across chains with 3 hazard chains",
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
				"3": {},
			},
			expectErr: ErrCycle,
			hazards: map[types.ChainIndex]types.BlockSeal{
				1: {Number: 1},
				2: {Number: 1},
				3: {Number: 1},
			},
			msg: "expected cycle detection error for cycle through executing messages",
		},
	}
	runHazardCycleChecksTestCaseGroup(t, "Cycle", tests)
}

const (
	largeGraphChains       = 10
	largeGraphLogsPerChain = 10000
)

func TestHazardCycleChecksLargeGraphNoCycle(t *testing.T) {
	// Create a large but acyclic graph
	chainBlocks := make(map[string]chainBlockDef)
	for i := 1; i <= largeGraphChains; i++ {
		msgs := make(map[uint32]*types.ExecutingMessage)
		// Create a chain of dependencies across chains
		if i > 1 {
			for j := uint32(0); j < largeGraphLogsPerChain; j++ {
				// Point to previous chain, same log index
				msgs[j] = execMsg(strconv.Itoa(i-1), j)
			}
		}
		chainBlocks[strconv.Itoa(i)] = chainBlockDef{
			logCount: largeGraphLogsPerChain,
			messages: msgs,
		}
	}

	tc := hazardCycleChecksTestCase{
		name:        "Large graph without cycles",
		chainBlocks: chainBlocks,
		expectErr:   nil,
		msg:         "expected no cycle in large acyclic graph",
	}
	runHazardCycleChecksTestCase(t, tc)
}

func TestHazardCycleChecksLargeGraphCycle(t *testing.T) {
	// Create a large graph with a cycle hidden in it
	const cycleChain = 3
	const cycleLogIndex = 5678

	chainBlocks := make(map[string]chainBlockDef)
	for i := 1; i <= largeGraphChains; i++ {
		msgs := make(map[uint32]*types.ExecutingMessage)

		// Create a chain of dependencies across chains
		if i > 1 {
			for j := uint32(0); j < largeGraphLogsPerChain; j++ {
				if i == cycleChain && j == cycleLogIndex {
					// Create a cycle by pointing back to chain 1
					msgs[j] = execMsg("1", cycleLogIndex+1)
				} else {
					// Normal case: point to previous chain, same log index
					msgs[j] = execMsg(strconv.Itoa(i-1), j)
				}
			}
		} else {
			// In chain 1, create the other side of the cycle
			msgs[cycleLogIndex+1] = execMsg(strconv.Itoa(cycleChain), cycleLogIndex)
		}

		chainBlocks[strconv.Itoa(i)] = chainBlockDef{
			logCount: largeGraphLogsPerChain,
			messages: msgs,
		}
	}

	tc := hazardCycleChecksTestCase{
		name:        "Large graph with cycle",
		chainBlocks: chainBlocks,
		expectErr:   ErrCycle,
		msg:         "expected to detect cycle in large cyclic graph",
	}
	runHazardCycleChecksTestCase(t, tc)
}
