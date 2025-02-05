package rewinder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// TestRewindL1 tests handling of L1 reorgs by checking that:
// 1. Only safe data is rewound
// 2. Unsafe data remains intact
// 3. The rewind point is determined by finding the common L1 ancestor
func TestRewindL1(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2A, block2B := createTestBlocks()

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2A, block2B)

	// Setup L1 blocks - initially we have block1A and block2A
	l1Block0 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   899,
	}
	l1Block1A := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       900,
		ParentHash: l1Block0.Hash,
	}
	l1Block2A := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1A.Hash,
	}

	// Setup the L1 node with initial chain
	chain.l1Node.blocks[l1Block0.Number] = l1Block0
	chain.l1Node.blocks[l1Block1A.Number] = l1Block1A
	chain.l1Node.blocks[l1Block2A.Number] = l1Block2A

	// Seal genesis and block1
	s.sealBlocks(chainID, genesis, block1)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Make genesis block derived from l1Block0 and make it safe
	s.makeBlockSafe(chainID, genesis, l1Block0, true)

	// Make block1 local-safe and cross-safe using l1Block1A
	s.makeBlockSafe(chainID, block1, l1Block1A, true)

	// Add block2A and make it local-safe and cross-safe using l1Block2A
	s.sealBlocks(chainID, block2A)
	s.makeBlockSafe(chainID, block2A, l1Block2A, true)

	// Verify block2A is the latest sealed block and is cross-safe
	s.verifyHeads(chainID, block2A.ID(), "should have set block2A as latest sealed block")

	// Now simulate L1 reorg by replacing l1Block2A with l1Block2B
	l1Block2B := eth.BlockRef{
		Hash:       common.HexToHash("0xbbb2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1A.Hash,
	}
	chain.l1Node.blocks[l1Block2B.Number] = l1Block2B

	// Trigger L1 reorg
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1Block2B.ID(),
	})

	// Verify we rewound to block1 since it's derived from l1Block1A which is still canonical
	s.verifyHeads(chainID, block1.ID(), "should have rewound to block1")
}

// TestRewindL2 tests handling of L2 reorgs via LocalDerivedEvent by checking that:
// 1. Only unsafe data is rewound
// 2. Safe data remains intact
// 3. The rewind point is determined by the parent of the mismatched block
func TestRewindL2(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2A, block2B := createTestBlocks()

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2A, block2B)

	// Setup L1 blocks
	l1Genesis := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   899,
	}
	l1Block1 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       900,
		ParentHash: l1Genesis.Hash,
	}
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	chain.l1Node.blocks[l1Genesis.Number] = l1Genesis
	chain.l1Node.blocks[l1Block1.Number] = l1Block1
	chain.l1Node.blocks[l1Block2.Number] = l1Block2

	// Seal genesis and block1
	s.sealBlocks(chainID, genesis, block1)

	// Make genesis safe and derived from L1 genesis
	s.makeBlockSafe(chainID, genesis, l1Genesis, true)

	// Make block1 local-safe and cross-safe
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	// Add block2A to unsafe chain
	s.sealBlocks(chainID, block2A)

	// Verify block2A is the latest sealed block but not safe
	s.verifyLogsHead(chainID, block2A.ID(), "should have set block2A as latest sealed block")
	s.verifyLocalSafe(chainID, block1.ID(), "block1 should still be local-safe")
	s.verifyCrossSafe(chainID, block1.ID(), "block1 should be cross-safe")

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Simulate receiving a LocalDerivedDoneEvent for block2B
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			Source: types.BlockSeal{
				Hash:   l1Block1.Hash,
				Number: l1Block1.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block2B.Hash,
				Number: block2B.Number,
			},
		},
	})

	// Verify we rewound to block1 since block2B doesn't match our unsafe block2A
	s.verifyLogsHead(chainID, block1.ID(), "should have rewound to block1")
	s.verifyLocalSafe(chainID, block1.ID(), "block1 should still be local-safe")
	s.verifyCrossSafe(chainID, block1.ID(), "block1 should still be cross-safe")

	// Add block2B
	s.sealBlocks(chainID, block2B)

	// Verify we're now on the new chain
	s.verifyLogsHead(chainID, block2B.ID(), "should be on block2B")
}

// TestNoRewindNeeded tests that no rewind occurs when:
// 1. L1 blocks match during L1 reorg check
// 2. L2 blocks match during LocalDerived check
func TestNoRewindNeeded(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2A, _ := createTestBlocks()

	// Setup sync node with blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2A)

	// Setup L1 blocks
	l1Block1 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       1001,
		ParentHash: common.HexToHash("0xaaa0"),
	}
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       1002,
		ParentHash: l1Block1.Hash,
	}
	chain.l1Node.blocks[l1Block1.Number] = l1Block1
	chain.l1Node.blocks[l1Block2.Number] = l1Block2

	// Seal genesis and block1
	s.sealBlocks(chainID, genesis, block1)

	// Make genesis safe and derived from L1 genesis
	s.makeBlockSafe(chainID, genesis, eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   1000,
	}, true)

	// Set genesis L1 block as finalized
	s.chainsDB.OnEvent(superevents.FinalizedL1RequestEvent{
		FinalizedL1: eth.BlockRef{
			Hash:   common.HexToHash("0xaaa0"),
			Number: 0,
			Time:   1000,
		},
	})

	// Make block1 local-safe and cross-safe
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	// Add block2A and make it local-safe and cross-safe
	s.sealBlocks(chainID, block2A)
	s.makeBlockSafe(chainID, block2A, l1Block2, true)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Trigger L1 reorg check with same L1 block - should not rewind
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1Block2.ID(),
	})

	// Verify no rewind occurred
	s.verifyLogsHead(chainID, block2A.ID(), "should still be on block2A")
	s.verifyCrossSafe(chainID, block2A.ID(), "block2A should still be cross-safe")

	// Trigger LocalDerived check with same L2 block - should not rewind
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			Source: types.BlockSeal{
				Hash:   l1Block2.Hash,
				Number: l1Block2.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block2A.Hash,
				Number: block2A.Number,
			},
		},
	})

	// Verify no rewind occurred
	s.verifyLogsHead(chainID, block2A.ID(), "should still be on block2A")
	s.verifyCrossSafe(chainID, block2A.ID(), "block2A should still be cross-safe")
}

// TestRewindLongChain syncs a long chain and rewinds many blocks.
func TestRewindLongChain(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create a chain with blocks 0-100
	var blocks []eth.L2BlockRef
	var l1Blocks []eth.BlockRef

	// Create L1 blocks first (one per 10 L2 blocks)
	for i := uint64(0); i <= 10; i++ {
		l1Block := eth.BlockRef{
			Hash:   common.HexToHash(fmt.Sprintf("0xaaa%d", i)),
			Number: i,
			Time:   900 + i*12,
		}
		if i > 0 {
			l1Block.ParentHash = l1Blocks[i-1].Hash
		}
		l1Blocks = append(l1Blocks, l1Block)
		chain.l1Node.blocks[i] = l1Block
	}

	// Create L2 blocks 0-100
	for i := uint64(0); i <= 100; i++ {
		l1Index := i / 10
		block := eth.L2BlockRef{
			Hash:           common.HexToHash(fmt.Sprintf("0x%d", i)),
			Number:         i,
			Time:           1000 + i,
			L1Origin:       l1Blocks[l1Index].ID(),
			SequenceNumber: i % 10,
		}
		if i > 0 {
			block.ParentHash = blocks[i-1].Hash
		}
		blocks = append(blocks, block)
	}

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(blocks...)

	// Seal all blocks
	for _, block := range blocks {
		s.sealBlocks(chainID, block)
	}

	// Make genesis safe and derived from L1 genesis
	s.makeBlockSafe(chainID, blocks[0], l1Blocks[0], true)

	// Set genesis L1 block as finalized
	s.chainsDB.OnEvent(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Blocks[0],
	})

	// Make blocks up to 95 safe
	for i := uint64(1); i <= 95; i++ {
		l1Index := i / 10
		s.makeBlockSafe(chainID, blocks[i], l1Blocks[l1Index], true)
	}

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Create a divergent block96B
	block96B := eth.L2BlockRef{
		Hash:           common.HexToHash("0xdead96"),
		Number:         96,
		ParentHash:     blocks[95].Hash,
		Time:           1000 + 96,
		L1Origin:       blocks[96].L1Origin,
		SequenceNumber: 96 % 10,
	}

	// Trigger LocalDerived event with block96B
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			Source: types.BlockSeal{
				Hash:   l1Blocks[96/10].Hash,
				Number: l1Blocks[96/10].Number,
			},
			Derived: types.BlockSeal{
				Hash:   block96B.Hash,
				Number: block96B.Number,
			},
		},
	})

	// Verify we rewound to block 95
	s.verifyLogsHead(chainID, blocks[95].ID(), "should have rewound to block 95")
}

// TestRewindMultiChain syncs two chains and rewinds both
func TestRewindMultiChain(t *testing.T) {
	chain1ID := eth.ChainID{1}
	chain2ID := eth.ChainID{2}
	s := setupTestChains(t, chain1ID, chain2ID)
	defer s.Close()

	// Create common blocks for both chains
	genesis, block1, block2A, block2B := createTestBlocks()

	// Setup L1 block
	l1Genesis := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   899,
	}
	l1Block1 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       900,
		ParentHash: l1Genesis.Hash,
	}

	// Setup both chains
	for chainID, chain := range s.chains {
		// Setup nodes
		chain.setupSyncNodeBlocks(genesis, block1, block2A, block2B)
		chain.l1Node.blocks[l1Genesis.Number] = l1Genesis
		chain.l1Node.blocks[l1Block1.Number] = l1Block1

		// Setup initial chain
		s.sealBlocks(chainID, genesis, block1, block2A)

		// Make genesis safe and derived from L1 genesis
		s.makeBlockSafe(chainID, genesis, l1Genesis, true)

		// Make block1 local-safe and cross-safe
		s.makeBlockSafe(chainID, block1, l1Block1, true)
	}

	// Set genesis as finalized for all chains
	s.chainsDB.OnEvent(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Genesis,
	})

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, s.chains[chain1ID].l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Trigger LocalDerived events for both chains
	for chainID := range s.chains {
		i.OnEvent(superevents.LocalSafeUpdateEvent{
			ChainID: chainID,
			NewLocalSafe: types.DerivedBlockSealPair{
				Source: types.BlockSeal{
					Hash:   l1Block1.Hash,
					Number: l1Block1.Number,
				},
				Derived: types.BlockSeal{
					Hash:   block2B.Hash,
					Number: block2B.Number,
				},
			},
		})
	}

	// Verify both chains rewound to block1 and maintained proper state
	for chainID := range s.chains {
		s.verifyLogsHead(chainID, block1.ID(), fmt.Sprintf("chain %v should have rewound to block1", chainID))
		s.verifyCrossSafe(chainID, block1.ID(), fmt.Sprintf("chain %v block1 should be cross-safe", chainID))
	}
}

// TestRewindL2WalkBack tests that during an L2 reorg, we correctly walk back
// parent-by-parent until finding a common ancestor when the first rewind attempt fails.
func TestRewindL2WalkBack(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()
	chainID := eth.ChainID{1}
	chain := s.chains[chainID]
	// Create a chain of blocks: genesis -> block1 -> block2 -> block3 -> block4A
	genesis := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1110"),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           1000,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa0"), Number: 0},
		SequenceNumber: 0,
	}
	block1 := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1111"),
		Number:         1,
		ParentHash:     genesis.Hash,
		Time:           1001,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa1"), Number: 1},
		SequenceNumber: 1,
	}
	block2 := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1112"),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa2"), Number: 2},
		SequenceNumber: 2,
	}
	block3 := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1113"),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa3"), Number: 3},
		SequenceNumber: 3,
	}
	block4A := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1114a"),
		Number:         4,
		ParentHash:     block3.Hash,
		Time:           1004,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa4"), Number: 4},
		SequenceNumber: 4,
	}
	// Create a divergent block4B that will trigger the reorg
	block4B := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1114b"),
		Number:         4,
		ParentHash:     block3.Hash,
		Time:           1004,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa4"), Number: 4},
		SequenceNumber: 4,
	}
	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2, block3, block4A, block4B)
	// Setup L1 blocks
	l1Genesis := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   900,
	}
	l1Block1 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       901,
		ParentHash: l1Genesis.Hash,
	}
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       902,
		ParentHash: l1Block1.Hash,
	}
	l1Block3 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa3"),
		Number:     3,
		Time:       903,
		ParentHash: l1Block2.Hash,
	}
	l1Block4 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa4"),
		Number:     4,
		Time:       904,
		ParentHash: l1Block3.Hash,
	}
	// Add L1 blocks to node
	chain.l1Node.blocks[l1Genesis.Number] = l1Genesis
	chain.l1Node.blocks[l1Block1.Number] = l1Block1
	chain.l1Node.blocks[l1Block2.Number] = l1Block2
	chain.l1Node.blocks[l1Block3.Number] = l1Block3
	chain.l1Node.blocks[l1Block4.Number] = l1Block4

	// Seal all blocks in the original chain
	s.sealBlocks(chainID, genesis, block1, block2, block3, block4A)

	// Make genesis safe and derived from L1 genesis
	s.makeBlockSafe(chainID, genesis, l1Genesis, true)

	// Set genesis L1 block as finalized
	s.chainsDB.OnEvent(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Genesis,
	})

	// Make blocks up to block3 safe
	s.makeBlockSafe(chainID, block1, l1Block1, true)
	s.makeBlockSafe(chainID, block2, l1Block2, true)
	s.makeBlockSafe(chainID, block3, l1Block3, true)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})
	// Trigger LocalDerived event with block4B
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			Source: types.BlockSeal{
				Hash:   block4B.L1Origin.Hash,
				Number: block4B.L1Origin.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block4B.Hash,
				Number: block4B.Number,
			},
		},
	})
	// Verify we rewound to block3 since it's the common ancestor
	s.verifyLogsHead(chainID, block3.ID(), "should have rewound to block3 (common ancestor)")
}

// TestRewindL1PastCrossSafe tests that when an L1 reorg occurs at a height higher than
// the CrossSafe head, only LocalSafe is rewound and CrossSafe remains untouched.
func TestRewindL1PastCrossSafe(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create blocks: genesis -> block1 -> block2 -> block3A/3B
	genesis := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1110"),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           1000,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa0"), Number: 0},
		SequenceNumber: 0,
	}
	block1 := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1111"),
		Number:         1,
		ParentHash:     genesis.Hash,
		Time:           1001,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa1"), Number: 1},
		SequenceNumber: 1,
	}
	block2 := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1112"),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa2"), Number: 2},
		SequenceNumber: 2,
	}
	block3A := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1113a"),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa3"), Number: 3},
		SequenceNumber: 3,
	}
	block3B := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1113b"),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xbbb3"), Number: 3},
		SequenceNumber: 3,
	}

	// Setup sync node with all blocks
	chain.setupSyncNodeBlocks(genesis, block1, block2, block3A, block3B)

	// Setup L1 blocks - initially we have the A chain
	l1Genesis := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   899,
	}
	l1Block1 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       900,
		ParentHash: l1Genesis.Hash,
	}
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	l1Block3A := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa3"),
		Number:     3,
		Time:       902,
		ParentHash: l1Block2.Hash,
	}

	// Setup the L1 node with initial chain
	chain.l1Node.blocks[l1Genesis.Number] = l1Genesis
	chain.l1Node.blocks[l1Block1.Number] = l1Block1
	chain.l1Node.blocks[l1Block2.Number] = l1Block2
	chain.l1Node.blocks[l1Block3A.Number] = l1Block3A

	// Seal all blocks
	s.sealBlocks(chainID, genesis, block1, block2, block3A)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Make genesis block derived from l1Genesis and make it safe
	s.makeBlockSafe(chainID, genesis, l1Genesis, true)

	// Set l1Genesis as finalized
	s.chainsDB.OnEvent(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Genesis,
	})

	// Make block1 local-safe and cross-safe
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	// Make block2 local-safe and cross-safe
	s.makeBlockSafe(chainID, block2, l1Block2, true)

	// Make block3A only local-safe (not cross-safe)
	s.makeBlockSafe(chainID, block3A, l1Block3A, false)

	// Verify initial state
	s.verifyLogsHead(chainID, block3A.ID(), "should have set block3A as latest sealed block")
	s.verifyCrossSafe(chainID, block2.ID(), "block2 should be cross-safe")

	// Now simulate L1 reorg by replacing l1Block3A with l1Block3B
	l1Block3B := eth.BlockRef{
		Hash:       common.HexToHash("0xbbb3"),
		Number:     3,
		Time:       902,
		ParentHash: l1Block2.Hash,
	}
	chain.l1Node.blocks[l1Block3B.Number] = l1Block3B

	// Trigger L1 reorg
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1Block3B.ID(),
	})

	// Verify we rewound LocalSafe to block2 since it's derived from l1Block2 which is still canonical
	s.verifyHeads(chainID, block2.ID(), "should have rewound to block2")
}

type testSetup struct {
	t        *testing.T
	logger   log.Logger
	dataDir  string
	chainsDB *db.ChainsDB
	chains   map[eth.ChainID]*testChainSetup
}

type testChainSetup struct {
	chainID  eth.ChainID
	logDB    *logs.DB
	localDB  *fromda.DB
	crossDB  *fromda.DB
	syncNode *mockSyncNode
	l1Node   *mockL1Node
}

// setupTestChains creates multiple test chains with their own DBs and sync nodes
func setupTestChains(t *testing.T, chainIDs ...eth.ChainID) *testSetup {
	logger := testlog.Logger(t, log.LvlInfo)
	dataDir := t.TempDir()

	// Create dependency set for all chains
	deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
	for i, chainID := range chainIDs {
		deps[chainID] = &depset.StaticConfigDependency{
			ChainIndex:     types.ChainIndex(i + 1),
			ActivationTime: 42,
			HistoryMinTime: 100,
		}
	}
	depSet, err := depset.NewStaticConfigDependencySet(deps)
	require.NoError(t, err)

	// Create ChainsDB with mock emitter
	chainsDB := db.NewChainsDB(logger, depSet, metrics.NoopMetrics)
	chainsDB.AttachEmitter(&mockEmitter{})

	setup := &testSetup{
		t:        t,
		logger:   logger,
		dataDir:  dataDir,
		chainsDB: chainsDB,
		chains:   make(map[eth.ChainID]*testChainSetup),
	}

	// Setup each chain
	for _, chainID := range chainIDs {
		// Create the chain directory
		chainDir := filepath.Join(dataDir, fmt.Sprintf("00%d", chainID[0]), "1")
		err = os.MkdirAll(chainDir, 0o755)
		require.NoError(t, err)

		// Create and open the log DB
		logDB, err := logs.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "log.db"), true)
		require.NoError(t, err)
		chainsDB.AddLogDB(chainID, logDB)

		// Create and open the local derived-from DB
		localDB, err := fromda.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "local_safe.db"))
		require.NoError(t, err)
		chainsDB.AddLocalDerivationDB(chainID, localDB)

		// Create and open the cross derived-from DB
		crossDB, err := fromda.NewFromFile(logger, &stubMetrics{}, filepath.Join(chainDir, "cross_safe.db"))
		require.NoError(t, err)
		chainsDB.AddCrossDerivationDB(chainID, crossDB)

		// Add cross-unsafe tracker
		chainsDB.AddCrossUnsafeTracker(chainID)

		setup.chains[chainID] = &testChainSetup{
			chainID:  chainID,
			logDB:    logDB,
			localDB:  localDB,
			crossDB:  crossDB,
			syncNode: newMockSyncNode(),
			l1Node:   newMockL1Node(),
		}
	}

	return setup
}

func (s *testSetup) Close() {
	s.chainsDB.Close()
	for _, chain := range s.chains {
		chain.Close()
	}
}

func (s *testChainSetup) Close() {
	s.logDB.Close()
	s.localDB.Close()
	s.crossDB.Close()
}

// setupSyncNodeBlocks adds the given blocks to the sync node's block map
func (s *testChainSetup) setupSyncNodeBlocks(blocks ...eth.L2BlockRef) {
	for _, block := range blocks {
		s.syncNode.blocks[block.Number] = eth.BlockRef{
			Hash:       block.Hash,
			Number:     block.Number,
			Time:       block.Time,
			ParentHash: block.ParentHash,
		}
	}
}

func (s *testSetup) makeBlockSafe(chainID eth.ChainID, block eth.L2BlockRef, l1Block eth.BlockRef, makeCrossSafe bool) {
	// Add the L1 derivation
	s.chainsDB.UpdateLocalSafe(chainID, l1Block, eth.BlockRef{
		Hash:       block.Hash,
		Number:     block.Number,
		Time:       block.Time,
		ParentHash: block.ParentHash,
	})

	if makeCrossSafe {
		require.NoError(s.t, s.chainsDB.UpdateCrossUnsafe(chainID, types.BlockSeal{
			Hash:      block.Hash,
			Number:    block.Number,
			Timestamp: block.Time,
		}))
		require.NoError(s.t, s.chainsDB.UpdateCrossSafe(chainID, l1Block, eth.BlockRef{
			Hash:       block.Hash,
			Number:     block.Number,
			Time:       block.Time,
			ParentHash: block.ParentHash,
		}))
	}
}

func (s *testSetup) verifyHeads(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	s.verifyLocalSafe(chainID, expectedHead, msg)
	s.verifyCrossSafe(chainID, expectedHead, msg)
}

func (s *testSetup) verifyLocalSafe(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	localSafe, err := s.chainsDB.LocalSafe(chainID)
	require.NoError(s.t, err)
	require.Equal(s.t, expectedHead.Hash, localSafe.Derived.Hash, msg)
}

func (s *testSetup) verifyCrossSafe(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	crossSafe, err := s.chainsDB.CrossSafe(chainID)
	require.NoError(s.t, err)
	require.Equal(s.t, expectedHead.Hash, crossSafe.Derived.Hash, msg)
}

func (s *testSetup) verifyLogsHead(chainID eth.ChainID, expectedHead eth.BlockID, msg string) {
	head, ok := s.chains[chainID].logDB.LatestSealedBlock()
	require.True(s.t, ok)
	require.Equal(s.t, expectedHead, head, msg)
}

func (s *testSetup) sealBlocks(chainID eth.ChainID, blocks ...eth.L2BlockRef) {
	for _, block := range blocks {
		require.NoError(s.t, s.chains[chainID].logDB.SealBlock(block.ParentHash, block.ID(), block.Time))
	}
}

func setupTestChain(t *testing.T) *testSetup {
	chainID := eth.ChainID{1}
	return setupTestChains(t, chainID)
}

func createTestBlocks() (genesis, block1, block2A, block2B eth.L2BlockRef) {
	l1Genesis := eth.BlockID{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
	}
	l1Block1 := eth.BlockID{
		Hash:   common.HexToHash("0xaaa1"),
		Number: 1,
	}
	l1Block2A := eth.BlockID{
		Hash:   common.HexToHash("0xaaa2"),
		Number: 2,
	}
	l1Block2B := eth.BlockID{
		Hash:   common.HexToHash("0xbbb2"),
		Number: 2,
	}

	genesis = eth.L2BlockRef{
		Hash:           common.HexToHash("0x1110"),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           1000,
		L1Origin:       l1Genesis,
		SequenceNumber: 0,
	}

	block1 = eth.L2BlockRef{
		Hash:           common.HexToHash("0x1111"),
		Number:         1,
		ParentHash:     genesis.Hash,
		Time:           1001,
		L1Origin:       l1Block1,
		SequenceNumber: 1,
	}

	block2A = eth.L2BlockRef{
		Hash:           common.HexToHash("0x222a"),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       l1Block2A,
		SequenceNumber: 2,
	}

	block2B = eth.L2BlockRef{
		Hash:           common.HexToHash("0x222b"),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       l1Block2B,
		SequenceNumber: 2,
	}

	return
}

type mockEmitter struct {
	events []event.Event
}

func (m *mockEmitter) Emit(ev event.Event) {
	m.events = append(m.events, ev)
}

type mockSyncNode struct {
	blocks map[uint64]eth.BlockRef
}

func newMockSyncNode() *mockSyncNode {
	return &mockSyncNode{
		blocks: make(map[uint64]eth.BlockRef),
	}
}

func (m *mockSyncNode) BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error) {
	return m.blocks[number], nil
}

type stubMetrics struct {
	entryCount           int64
	entriesReadForSearch int64
	derivedEntryCount    int64
}

func (s *stubMetrics) RecordDBEntryCount(kind string, count int64) {
	s.entryCount = count
}

func (s *stubMetrics) RecordDBSearchEntriesRead(count int64) {
	s.entriesReadForSearch = count
}

func (s *stubMetrics) RecordDBDerivedEntryCount(count int64) {
	s.derivedEntryCount = count
}

var _ logs.Metrics = (*stubMetrics)(nil)

type mockL1Node struct {
	blocks map[uint64]eth.BlockRef
}

func newMockL1Node() *mockL1Node {
	return &mockL1Node{
		blocks: make(map[uint64]eth.BlockRef),
	}
}

func (m *mockL1Node) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	block, ok := m.blocks[number]
	if !ok {
		return eth.L1BlockRef{}, fmt.Errorf("block not found: %d", number)
	}
	return eth.L1BlockRef{
		Hash:       block.Hash,
		Number:     block.Number,
		Time:       block.Time,
		ParentHash: block.ParentHash,
	}, nil
}
