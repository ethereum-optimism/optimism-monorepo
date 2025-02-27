package cross

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestHazardSet_Build(t *testing.T) {
	vectors := []testVector{
		{
			name: "Empty Message List",
			blocks: []blockDef{
				makeBlock(0, 100, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{},
		},
		{
			name: "Single Dependency",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1)),
				makeBlock(1, 100, 1), // Referenced block from chain 1
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 100, 1),
			},
		},
		{
			name: "Multiple Messages Same Block",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1), makeMessage(1, 100, 1, 2)),
				makeBlock(1, 100, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 100, 1),
			},
		},
		{
			name: "Multiple Messages Different Blocks Different Chains",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1), makeMessage(2, 100, 1, 1)),
				makeBlock(1, 100, 1),
				makeBlock(2, 100, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 100, 1),
				2: makeBlockSeal(1, 100, 2),
			},
		},
		{
			name: "Multiple Messages Different Blocks Same Chain",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1), makeMessage(1, 100, 2, 1)),
				makeBlock(1, 100, 1),
				makeBlock(1, 100, 2),
			},
			expectErr: fmt.Errorf("already depend on"),
		},
		{
			name: "Hazards Across Multiple Chains",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1), makeMessage(2, 100, 1, 1)),
				makeBlock(1, 100, 1),
				makeBlock(2, 100, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 100, 1),
				2: makeBlockSeal(1, 100, 2),
			},
		},
		{
			name: "Recursive Hazards",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1)),
				makeBlock(1, 100, 1, makeMessage(2, 100, 1, 1)),
				makeBlock(2, 100, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 100, 1),
				2: makeBlockSeal(1, 100, 2),
			},
		},
		{
			name: "Recursive Hazards - Missing Intermediate Block",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1)),
				// Block 1 in Chain 1 is missing
				makeBlock(2, 100, 1),
			},
			expectErr: types.ErrFuture,
		},
		{
			name: "Invalid Timestamp - Future Message",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 200, 1, 1)), // Message timestamp > block timestamp
				makeBlock(1, 100, 1),
			},
			expectErr: fmt.Errorf("breaks timestamp invariant"),
		},
		{
			name: "Invalid Timestamp - Zero",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 0, 1, 1)),
				makeBlock(1, 100, 1),
			},
			expectErr: types.ErrFuture,
		},
		{
			name: "Missing Block - Message References Non-existent Block",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 999, 1)), // Block 999 doesn't exist
			},
			expectErr: types.ErrFuture,
		},
		{
			name: "Missing Block - Chain Break",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1)),
				makeBlock(1, 100, 1, makeMessage(2, 100, 1, 1)), // Message references block in chain 2 that doesn't exist
			},
			expectErr: types.ErrFuture,
		},
		{
			name: "Invalid Block Number - Zero",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 0, 1)), // Invalid block number
			},
			expectErr: types.ErrFuture,
		},
		{
			name: "Recursive Hazards - Diamond Pattern",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1), makeMessage(2, 100, 1, 1)),
				makeBlock(1, 100, 1, makeMessage(3, 100, 1, 1)),
				makeBlock(2, 100, 1, makeMessage(3, 100, 1, 1)),
				makeBlock(3, 100, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 100, 1),
				2: makeBlockSeal(1, 100, 2),
				3: makeBlockSeal(1, 100, 3),
			},
		},
		{
			name: "Multiple Independent Chains",
			blocks: []blockDef{
				// Chain 0 -> (Chain 1 & Chain 2)
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1), makeMessage(2, 100, 1, 1)),

				// Chain 1 -> Chain 3
				makeBlock(1, 100, 1, makeMessage(3, 100, 1, 1)),

				// Chain 2 -> Chain 4
				makeBlock(2, 100, 1, makeMessage(4, 100, 1, 1)),

				// No dependencies
				makeBlock(3, 100, 1),
				makeBlock(4, 100, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 100, 1),
				2: makeBlockSeal(1, 100, 2),
				3: makeBlockSeal(1, 100, 3),
				4: makeBlockSeal(1, 100, 4),
			},
		},
		{
			name: "Already Processed Block",
			blocks: []blockDef{
				makeBlock(0, 100, 1, makeMessage(1, 100, 1, 1), makeMessage(2, 100, 1, 1)),
				// Both chain 1 and 2 reference chain 3
				makeBlock(1, 100, 1, makeMessage(3, 100, 1, 1)),
				makeBlock(2, 100, 1, makeMessage(3, 100, 1, 1)),
				makeBlock(3, 100, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 100, 1),
				2: makeBlockSeal(1, 100, 2),
				3: makeBlockSeal(1, 100, 3),
			},
		},
	}

	for _, tc := range vectors {
		t.Run(tc.name, func(t *testing.T) {
			logger := newTestLogger(t)

			deps := newMockHazardDeps(t, tc)
			if tc.verifyBlockFn != nil {
				deps.verifyBlockFn = tc.verifyBlockFn
			}

			// Create HazardSet with first block
			if len(tc.blocks) == 0 {
				t.Fatal("test case must have at least one block")
			}
			firstBlock := tc.blocks[0]
			seal := types.BlockSeal{
				Number:    firstBlock.number,
				Timestamp: firstBlock.timestamp,
				Hash:      firstBlock.hash,
			}
			chainID := eth.ChainIDFromUInt64(uint64(firstBlock.chain))

			hs, err := NewHazardSet(deps, logger, chainID, seal)
			if tc.expectErr != nil {
				t.Log("error creating hazard set", "block", firstBlock, "error", err)
				require.Error(t, err, "expected error %s, got %v", tc.expectErr, err)
				require.Contains(t, err.Error(), tc.expectErr.Error(), "expected error %s, got %v", tc.expectErr, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, hs.Entries())
		})
	}
}

func TestHazardSet_CrossValidBlocks(t *testing.T) {
	require := require.New(t)
	logger := newTestLogger(t)
	depSet := &hazardMockDependencySet{}

	// Helper function to create block hash
	makeBlockHash := func(chainID eth.ChainID, num uint64) common.Hash {
		var hash common.Hash
		// Put the number in the first 8 bytes (consistent format)
		binary.BigEndian.PutUint64(hash[:8], num)
		// Use the chain ID to distinguish chains
		hash[15] = byte(chainID[0]) // Use first byte of chainID
		return hash
	}

	// Define a struct for test blocks
	type testBlockDef struct {
		chain     eth.ChainID
		num       uint64
		timestamp uint64
		msgs      []struct {
			Chain     types.ChainIndex
			BlockNum  uint64
			LogIndex  uint32
			Timestamp uint64
		}
	}

	// Helper function to create a block map
	makeBlockMap := func(blocks []testBlockDef) map[blockKey]blockDef {
		blockMap := make(map[blockKey]blockDef)
		for _, block := range blocks {
			// Extract chain index from chain ID
			chainIndex, err := depSet.ChainIndexFromID(block.chain)
			if err != nil {
				// Use a fallback if error
				chainIndex = types.ChainIndex(block.chain[0])
			}

			key := blockKey{
				chain:  chainIndex,
				number: block.num,
			}

			// Convert messages to the expected format
			messages := make([]*types.ExecutingMessage, 0, len(block.msgs))
			for _, msg := range block.msgs {
				messages = append(messages, &types.ExecutingMessage{
					Chain:     msg.Chain,
					BlockNum:  msg.BlockNum,
					LogIdx:    msg.LogIndex,
					Timestamp: msg.Timestamp,
				})
			}

			// Use the provided timestamp or default to 100
			timestamp := block.timestamp
			if timestamp == 0 {
				timestamp = 100
			}

			blockMap[key] = blockDef{
				chain:     chainIndex,
				number:    block.num,
				timestamp: timestamp,
				hash:      makeBlockHash(block.chain, block.num),
				messages:  messages,
			}
		}
		return blockMap
	}

	// Define test vectors and the expected results for each chain
	vectors := []struct {
		name            string
		blocks          []testBlockDef
		verifyBlockFn   func(chainID eth.ChainID, block eth.BlockID) error
		expectedHazards map[types.ChainIndex]types.BlockSeal
		expectedErr     error
	}{
		{
			name: "Valid: Block Is Cross-Valid",
			blocks: []testBlockDef{
				{
					chain:     eth.ChainIDFromUInt64(0),
					num:       10,
					timestamp: 100,
					msgs: []struct {
						Chain     types.ChainIndex
						BlockNum  uint64
						LogIndex  uint32
						Timestamp uint64
					}{
						{
							Chain:     1,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 100,
						},
					},
				},
				{
					chain:     eth.ChainIDFromUInt64(1),
					num:       5,
					timestamp: 100,
				},
			},
			expectedHazards: map[types.ChainIndex]types.BlockSeal{
				1: {
					Hash:      makeBlockHash(eth.ChainIDFromUInt64(1), 5),
					Number:    5,
					Timestamp: 100,
				},
			},
			verifyBlockFn: func(chainID eth.ChainID, block eth.BlockID) error {
				return fmt.Errorf("block %s is not cross-valid", block)
			},
		},
		{
			name: "Invalid: Block Is Not Cross-Valid",
			blocks: []testBlockDef{
				{
					chain:     eth.ChainIDFromUInt64(0),
					num:       10,
					timestamp: 100,
					msgs: []struct {
						Chain     types.ChainIndex
						BlockNum  uint64
						LogIndex  uint32
						Timestamp uint64
					}{
						{
							Chain:     1,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 100,
						},
					},
				},
				{
					chain:     eth.ChainIDFromUInt64(1),
					num:       5,
					timestamp: 100,
				},
			},
			expectedHazards: map[types.ChainIndex]types.BlockSeal{
				1: {
					Hash:      makeBlockHash(eth.ChainIDFromUInt64(1), 5),
					Number:    5,
					Timestamp: 100,
				},
			},
			verifyBlockFn: func(chainID eth.ChainID, block eth.BlockID) error {
				// No blocks are considered cross-valid
				return fmt.Errorf("block %s is not cross-valid", block)
			},
		},
		{
			name: "Mix: Some Blocks Are Cross-Valid",
			blocks: []testBlockDef{
				{
					chain:     eth.ChainIDFromUInt64(0),
					num:       10,
					timestamp: 100,
					msgs: []struct {
						Chain     types.ChainIndex
						BlockNum  uint64
						LogIndex  uint32
						Timestamp uint64
					}{
						{
							Chain:     1,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 100,
						},
						{
							Chain:     2,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 100,
						},
					},
				},
				{
					chain:     eth.ChainIDFromUInt64(1),
					num:       5,
					timestamp: 100,
				},
				{
					chain:     eth.ChainIDFromUInt64(2),
					num:       5,
					timestamp: 100,
				},
			},
			expectedHazards: map[types.ChainIndex]types.BlockSeal{
				1: {
					Hash:      makeBlockHash(eth.ChainIDFromUInt64(1), 5),
					Number:    5,
					Timestamp: 100,
				},
				2: {
					Hash:      makeBlockHash(eth.ChainIDFromUInt64(2), 5),
					Number:    5,
					Timestamp: 100,
				},
			},
			verifyBlockFn: func(chainID eth.ChainID, block eth.BlockID) error {
				// Cross-validation check only applies to dependent blocks when hazards are being built
				return fmt.Errorf("block %s is not cross-valid", block)
			},
		},
		{
			name: "Error: Verification Error",
			blocks: []testBlockDef{
				{
					chain:     eth.ChainIDFromUInt64(0),
					num:       10,
					timestamp: 200,
					msgs: []struct {
						Chain     types.ChainIndex
						BlockNum  uint64
						LogIndex  uint32
						Timestamp uint64
					}{
						{
							Chain:     1,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 100,
						},
					},
				},
				{
					chain:     eth.ChainIDFromUInt64(1),
					num:       5,
					timestamp: 100,
				},
			},
			expectedErr: fmt.Errorf("failed to build hazard set: msg ExecMsg(chainIndex: 1, block: 5, log: 1, time: 100, logHash: 0x0000000000000000000000000000000000000000000000000000000000000000) included in non-cross-safe block BlockSeal(hash:0x0000000000000005000000000000000100000000000000000000000000000000, number:5, time:100): block 0x0000000000000005000000000000000100000000000000000000000000000000:5 (chain 1) is not cross-valid: verification database error"),
			verifyBlockFn: func(chainID eth.ChainID, block eth.BlockID) error {
				return fmt.Errorf("verification database error")
			},
		},
		{
			name: "Recursive: Chain 1 Cross-Valid",
			blocks: []testBlockDef{
				{
					chain:     eth.ChainIDFromUInt64(0),
					num:       10,
					timestamp: 100,
					msgs: []struct {
						Chain     types.ChainIndex
						BlockNum  uint64
						LogIndex  uint32
						Timestamp uint64
					}{
						{
							Chain:     1,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 100,
						},
					},
				},
				{
					chain:     eth.ChainIDFromUInt64(1),
					num:       5,
					timestamp: 100,
					msgs: []struct {
						Chain     types.ChainIndex
						BlockNum  uint64
						LogIndex  uint32
						Timestamp uint64
					}{
						{
							Chain:     2,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 100,
						},
					},
				},
				{
					chain:     eth.ChainIDFromUInt64(2),
					num:       5,
					timestamp: 100,
				},
			},
			expectedHazards: map[types.ChainIndex]types.BlockSeal{
				1: {
					Hash:      makeBlockHash(eth.ChainIDFromUInt64(1), 5),
					Number:    5,
					Timestamp: 100,
				},
				2: {
					Hash:      makeBlockHash(eth.ChainIDFromUInt64(2), 5),
					Number:    5,
					Timestamp: 100,
				},
			},
			verifyBlockFn: func(chainID eth.ChainID, block eth.BlockID) error {
				// Cross-validation only skips later additions
				return fmt.Errorf("block %s is not cross-valid", block)
			},
		},
		{
			name: "Recursive: Chain 2 Cross-Valid",
			blocks: []testBlockDef{
				{
					chain:     eth.ChainIDFromUInt64(0),
					num:       10,
					timestamp: 100,
					msgs: []struct {
						Chain     types.ChainIndex
						BlockNum  uint64
						LogIndex  uint32
						Timestamp uint64
					}{
						{
							Chain:     1,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 50, // Use a lower timestamp to trigger IsCrossValidBlock check
						},
					},
				},
				{
					chain:     eth.ChainIDFromUInt64(1),
					num:       5,
					timestamp: 100,
					msgs: []struct {
						Chain     types.ChainIndex
						BlockNum  uint64
						LogIndex  uint32
						Timestamp uint64
					}{
						{
							Chain:     2,
							BlockNum:  5,
							LogIndex:  1,
							Timestamp: 50, // Use a lower timestamp to trigger IsCrossValidBlock check
						},
					},
				},
				{
					chain:     eth.ChainIDFromUInt64(2),
					num:       5,
					timestamp: 100,
				},
			},
			// This test expects to fail because Chain 1 is not cross-valid
			expectedErr: fmt.Errorf("is not cross-valid"),
		},
	}

	for _, tc := range vectors {
		t.Run(tc.name, func(t *testing.T) {
			// Create dependency mock
			mockDeps := &mockHazardDeps{
				logger:        logger,
				deps:          depSet,
				blockMap:      makeBlockMap(tc.blocks),
				verifyBlockFn: tc.verifyBlockFn,
			}

			// Create the HazardSet for the first block (candidate block)
			firstBlock := tc.blocks[0]
			candidateSeal := types.BlockSeal{
				Hash:      makeBlockHash(firstBlock.chain, firstBlock.num),
				Number:    firstBlock.num,
				Timestamp: firstBlock.timestamp,
			}

			// Create the HazardSet - this should recursively build the entire set
			hs, err := NewHazardSet(mockDeps, logger, firstBlock.chain, candidateSeal)
			if tc.expectedErr != nil {
				require.Error(err)
				require.Contains(err.Error(), tc.expectedErr.Error())
				return
			}
			require.NoError(err)
			require.NotNil(hs)

			// Verify the hazard set entries match the expected hazards
			require.Equal(tc.expectedHazards, hs.Entries())
		})
	}
}

// mockHazardDeps implements HazardDeps for testing
type mockHazardDeps struct {
	logger        log.Logger
	containsFn    func(chain eth.ChainID, query types.ContainsQuery) (types.BlockSeal, error)
	verifyBlockFn func(chainID eth.ChainID, block eth.BlockID) error
	openBlockFn   func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
	deps          depset.DependencySet
	blockMap      map[blockKey]blockDef
}

func (m *mockHazardDeps) Contains(chain eth.ChainID, query types.ContainsQuery) (types.BlockSeal, error) {
	if m.containsFn != nil {
		return m.containsFn(chain, query)
	}
	// Look up the block in our test data
	chainIndex, err := m.ChainIndexFromID(chain)
	if err != nil {
		return types.BlockSeal{}, err
	}

	// Validate timestamp is greater than 0
	if query.Timestamp == 0 {
		return types.BlockSeal{}, fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture)
	}

	key := blockKey{
		chain:  chainIndex,
		number: query.BlockNum,
	}
	if block, ok := m.blockMap[key]; ok {
		// Check timestamp invariant
		if query.Timestamp > block.timestamp {
			return types.BlockSeal{}, fmt.Errorf("message timestamp %d breaks timestamp invariant with block timestamp %d", query.Timestamp, block.timestamp)
		}
		return types.BlockSeal{
			Number:    block.number,
			Timestamp: block.timestamp,
			Hash:      block.hash,
		}, nil
	}
	return types.BlockSeal{}, fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture)
}

func (m *mockHazardDeps) IsCrossValidBlock(chainID eth.ChainID, block eth.BlockID) error {
	if m.verifyBlockFn != nil {
		err := m.verifyBlockFn(chainID, block)
		if err != nil {
			// Format a clear error message that includes both the block and chain information
			// This ensures errors are properly identified and propagated
			chainIdx, _ := m.deps.ChainIndexFromID(chainID)
			return fmt.Errorf("block %s (chain %d) is not cross-valid: %w", block, chainIdx, err)
		}
		return nil
	}
	// By default, blocks are not cross-valid
	return fmt.Errorf("block %s is not cross-valid", block)
}

func (m *mockHazardDeps) OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	if m.openBlockFn != nil {
		return m.openBlockFn(chainID, blockNum)
	}
	// Look up the block in our test data
	chainIndex, err := m.ChainIndexFromID(chainID)
	if err != nil {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("failed to get chain index: %w", err)
	}
	key := blockKey{
		chain:  chainIndex,
		number: blockNum,
	}
	if block, ok := m.blockMap[key]; ok {
		// Convert messages slice to map
		msgMap := make(map[uint32]*types.ExecutingMessage)
		for i, msg := range block.messages {
			msgMap[uint32(i)] = msg
		}
		m.Logger().Debug("Opening block", "chainID", chainID, "blockNum", blockNum, "hash", block.hash.String()[:6]+".."+block.hash.String()[60:])
		return eth.BlockRef{
			Hash:   block.hash,
			Number: block.number,
			Time:   block.timestamp,
		}, uint32(len(block.messages)), msgMap, nil
	}
	return eth.BlockRef{}, 0, nil, types.ErrFuture
}

func (m *mockHazardDeps) DependencySet() depset.DependencySet {
	return m.deps
}

func (m *mockHazardDeps) ChainIndexFromID(id eth.ChainID) (types.ChainIndex, error) {
	return m.deps.ChainIndexFromID(id)
}

func (m *mockHazardDeps) Logger() log.Logger {
	return m.logger
}

func makeBlock(chain types.ChainIndex, timestamp, number uint64, messages ...*types.ExecutingMessage) blockDef {
	return blockDef{
		number:    number,
		timestamp: timestamp,
		chain:     chain,
		hash:      common.Hash{byte(chain), byte(number)}, // Deterministic hash based on chain and number
		messages:  messages,
	}
}

func makeMessage(chain types.ChainIndex, timestamp, blockNum uint64, logIdx uint32) *types.ExecutingMessage {
	return &types.ExecutingMessage{
		Chain:     chain,
		BlockNum:  blockNum,
		Timestamp: timestamp,
		LogIdx:    logIdx,
	}
}

func makeBlockSeal(number, timestamp uint64, chain types.ChainIndex) types.BlockSeal {
	return types.BlockSeal{
		Number:    number,
		Timestamp: timestamp,
		Hash:      common.Hash{byte(chain), byte(number)}, // Match block hash generation
	}
}

type testVector struct {
	name          string
	blocks        []blockDef
	expected      map[types.ChainIndex]types.BlockSeal
	expectErr     error
	verifyBlockFn func(chainID eth.ChainID, block eth.BlockID) error
}

type blockDef struct {
	number    uint64
	timestamp uint64
	hash      common.Hash
	chain     types.ChainIndex
	messages  []*types.ExecutingMessage
}

func newMockHazardDeps(t *testing.T, tc testVector) *mockHazardDeps {
	t.Helper()

	// Create a map of all blocks for quick lookup
	blockMap := make(map[blockKey]blockDef)
	for _, block := range tc.blocks {
		key := blockKey{
			chain:  block.chain,
			number: block.number,
		}
		blockMap[key] = block
	}

	mock := &mockHazardDeps{
		logger:   newTestLogger(t),
		deps:     &hazardMockDependencySet{},
		blockMap: blockMap,
	}

	// Set the verifyBlockFn if provided in the test vector
	if tc.verifyBlockFn != nil {
		mock.verifyBlockFn = tc.verifyBlockFn
	}

	return mock
}

// hazardMockDependencySet wraps mockDependencySet and makes it easier to test
// with 0 chainIndex values.
type hazardMockDependencySet struct {
	mockDependencySet
}

func (m hazardMockDependencySet) ChainIDFromIndex(idx types.ChainIndex) (eth.ChainID, error) {
	return eth.ChainIDFromUInt64(uint64(idx)), nil
}

func (m hazardMockDependencySet) ChainIndexFromID(id eth.ChainID) (types.ChainIndex, error) {
	return types.ChainIndex(id[0]), nil
}

type blockKey struct {
	chain  types.ChainIndex
	number uint64
}

func newTestLogger(t *testing.T) log.Logger {
	return testlog.Logger(t, log.LevelDebug)
}
