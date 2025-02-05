package rewinder

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// TestNoRewindNeeded tests that no rewind occurs when the chain state matches.
func TestNoRewindNeeded(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2, _, _ := createTestBlocks()

	// Setup L1 blocks
	l1Block0 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   899,
	}
	l1Block1 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       900,
		ParentHash: l1Block0.Hash,
	}
	l1Block2 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	chain.l1Node.AddBlock(l1Block1)
	chain.l1Node.AddBlock(l1Block2)

	// Seal genesis and block1
	s.sealBlocks(chainID, genesis, block1)

	// Make genesis safe and derived from L1 genesis
	s.makeBlockSafe(chainID, genesis, l1Block0, true)

	// Set genesis L1 block as finalized
	s.chainsDB.OnEvent(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Block0,
	})

	// Make block1 local-safe and cross-safe
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	// Add block2 and make it local-safe and cross-safe
	s.sealBlocks(chainID, block2)
	s.makeBlockSafe(chainID, block2, l1Block2, true)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Trigger L1 reorg check with same L1 block - should not rewind
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1Block2.ID(),
	})

	// Verify no rewind occurred
	s.verifyLogsHead(chainID, block2.ID(), "should still be on block2")
	s.verifyCrossSafe(chainID, block2.ID(), "block2 should still be cross-safe")

	// Trigger LocalDerived check with same L2 block - should not rewind
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSeal{
				Hash:   l1Block2.Hash,
				Number: l1Block2.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block2.Hash,
				Number: block2.Number,
			},
		},
	})

	// Verify no rewind occurred
	s.verifyLogsHead(chainID, block2.ID(), "should still be on block2")
	s.verifyCrossSafe(chainID, block2.ID(), "block2 should still be cross-safe")
}

// TestRewindL1 tests the handling of L1 rewinds.
func TestRewindL1(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2, _, _ := createTestBlocks()

	// Setup L1 blocks - initially we have block1A and block2A
	l1Block0 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   899,
	}
	l1Block1 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       900,
		ParentHash: l1Block0.Hash,
	}
	l1Block2A := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}

	// Setup the L1 node with initial chain
	chain.l1Node.AddBlock(l1Block0)
	chain.l1Node.AddBlock(l1Block1)
	chain.l1Node.AddBlock(l1Block2A)

	// Seal genesis and block1
	s.sealBlocks(chainID, genesis, block1)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Make genesis block derived from l1Block0 and make it safe
	s.makeBlockSafe(chainID, genesis, l1Block0, true)

	// Make block1 local-safe and cross-safe using l1Block1
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	// Add block2 and make it local-safe and cross-safe using l1Block2A
	s.sealBlocks(chainID, block2)
	s.makeBlockSafe(chainID, block2, l1Block2A, true)

	// Verify block2 is the latest sealed block and is cross-safe
	s.verifyHeads(chainID, block2.ID(), "should have set block2 as latest sealed block")

	// Now simulate L1 reorg by replacing l1Block2A with l1Block2B
	l1Block2B := eth.BlockRef{
		Hash:       common.HexToHash("0xbbb2"),
		Number:     2,
		Time:       901,
		ParentHash: l1Block1.Hash,
	}
	chain.l1Node.ReorgToBlock(l1Block2B)

	// Trigger L1 reorg
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1Block2B.ID(),
	})

	// Verify we rewound to block1 since it's derived from l1Block1 which is still canonical
	s.verifyHeads(chainID, block1.ID(), "should have rewound to block1")
}

// TestRewindL2 tests the handling of L2 rewinds via LocalDerivedEvent.
func TestRewindL2(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create blocks with sequential numbers
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
	block2A := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1112a"),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa1"), Number: 1},
		SequenceNumber: 2,
	}
	block2B := eth.L2BlockRef{
		Hash:           common.HexToHash("0x1112b"),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       eth.BlockID{Hash: common.HexToHash("0xaaa1"), Number: 1},
		SequenceNumber: 2,
	}

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
	chain.l1Node.AddBlock(l1Genesis)
	chain.l1Node.AddBlock(l1Block1)

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

	// Simulate receiving a LocalDerivedEvent for block2B
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSeal{
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

	// Add block2B to unsafe chain
	s.sealBlocks(chainID, block2B)

	// Verify we're now on the new chain
	s.verifyLogsHead(chainID, block2B.ID(), "should be on block2B")
}

// TestRewindBeyondFinality tests that rewinds respect finality boundaries.
func TestRewindBeyondFinality(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	genesis, block1, block2, block3A, _ := createTestBlocks()

	// Setup L1 blocks
	l1Block0 := eth.BlockRef{
		Hash:   common.HexToHash("0xaaa0"),
		Number: 0,
		Time:   899,
	}
	l1Block1 := eth.BlockRef{
		Hash:       common.HexToHash("0xaaa1"),
		Number:     1,
		Time:       900,
		ParentHash: l1Block0.Hash,
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
	chain.l1Node.AddBlock(l1Block0)
	chain.l1Node.AddBlock(l1Block1)
	chain.l1Node.AddBlock(l1Block2)
	chain.l1Node.AddBlock(l1Block3A)

	// Seal all blocks
	s.sealBlocks(chainID, genesis, block1, block2, block3A)

	// Make genesis safe and derived from L1 genesis
	s.makeBlockSafe(chainID, genesis, l1Block0, true)

	// Make block1 local-safe and cross-safe
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	// Make block2 local-safe, cross-safe and finalized
	s.makeBlockSafe(chainID, block2, l1Block2, true)
	s.chainsDB.OnEvent(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Block2,
	})

	// Make block3A local-safe and cross-safe
	s.makeBlockSafe(chainID, block3A, l1Block3A, true)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Verify initial state
	s.verifyHeads(chainID, block3A.ID(), "should be on block3A")

	// Replace block3A with block3B
	l1Block3B := eth.BlockRef{
		Hash:       common.HexToHash("0xbbb3"),
		Number:     3,
		Time:       902,
		ParentHash: l1Block2.Hash,
	}
	chain.l1Node.ReorgToBlock(l1Block3B)

	// Trigger L1 reorg
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1Block3B.ID(),
	})

	// Verify we rewound to block2 since it's finalized
	s.verifyHeads(chainID, block2.ID(), "should have rewound to finalized block2")
}

// TestRewindL1PastCrossSafe tests rewind behavior when L1 rewinds occur beyond the cross-safe head.
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
	chain.l1Node.AddBlock(l1Genesis)
	chain.l1Node.AddBlock(l1Block1)
	chain.l1Node.AddBlock(l1Block2)
	chain.l1Node.AddBlock(l1Block3A)

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
	chain.l1Node.ReorgToBlock(l1Block3B)

	// Trigger L1 reorg
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1Block3B.ID(),
	})

	// Verify we rewound LocalSafe to block2 since it's derived from l1Block2 which is still canonical
	s.verifyHeads(chainID, block2.ID(), "should have rewound to block2")
}

// TestL1RewindNoL2Impact tests L1 rewinds that don't affect L2 blocks.
func TestL1RewindNoL2Impact(t *testing.T) {
	chainID := eth.ChainID{1}
	s := setupTestChain(t)
	defer s.Close()

	// Create a chain with L2 blocks derived from L1 blocks
	builder := newChainBuilder()

	// Create L1 blocks
	l1Block0 := builder.AddL1Block(common.HexToHash("0xaaa0"), 0, 1000)
	l1Block1 := builder.AddL1Block(common.HexToHash("0xaaa1"), 1, 1001)
	builder.AddL1Block(common.HexToHash("0xaaa2"), 2, 1002) // Create but don't need reference

	// Create L2 blocks derived from the first L1 block
	l2Blocks := builder.AddL2Blocks(l1Block0, 2)

	// Setup nodes with the blocks
	builder.SetupL1Node(s.chains[chainID].l1Node)

	// Make blocks safe
	s.sealBlocks(chainID, l2Blocks...)
	for _, block := range l2Blocks {
		s.makeBlockSafe(chainID, block, l1Block0, true)
	}

	// Get initial state
	initialState := ChainState{
		LocalSafe: l2Blocks[len(l2Blocks)-1].ID(),
		CrossSafe: l2Blocks[len(l2Blocks)-1].ID(),
		LogsHead:  l2Blocks[len(l2Blocks)-1].ID(),
	}

	// Trigger L1 reorg that doesn't affect L2
	newL1Block := eth.BlockRef{
		Hash:       common.HexToHash("0xbbb2"),
		Number:     2,
		Time:       1002,
		ParentHash: l1Block1.Hash,
	}
	s.chains[chainID].l1Node.ReorgToBlock(newL1Block)

	// Verify no rewind occurred
	verifier := newStateVerifier(t, s.chainsDB, s.chains)
	verifier.VerifyChainState(chainID, initialState)
}

// TestL1RewindSingleBlockImpact tests L1 rewinds that affect a single L2 block.
func TestL1RewindSingleBlockImpact(t *testing.T) {
	chainID := eth.ChainID{1}
	s := setupTestChain(t)
	defer s.Close()

	// Create a chain with L2 blocks derived from L1 blocks
	builder := newChainBuilder()

	// Create L1 blocks
	l1Block0 := builder.AddL1Block(hashA(0), 0, 1000)
	l1Block1 := builder.AddL1Block(hashA(1), 1, 1001)
	l1Block2A := builder.AddL1Block(hashA(2), 2, 1002)

	// Create L2 blocks - 2 blocks from l1Block0, 2 from l1Block1, and 1 from l1Block2A
	l2Block0 := builder.AddL2Block(hash(0), 0, 1000, l1Block0.ID(), 0)
	l2Block1 := builder.AddL2Block(hash(1), 1, 1001, l1Block0.ID(), 1)
	l2Block2 := builder.AddL2Block(hash(2), 2, 1002, l1Block1.ID(), 0)
	l2Block3 := builder.AddL2Block(hash(3), 3, 1003, l1Block1.ID(), 1)
	l2Block4 := builder.AddL2Block(hash(4), 4, 1004, l1Block2A.ID(), 0)

	// Setup nodes with the blocks
	builder.SetupL1Node(s.chains[chainID].l1Node)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, s.chains[chainID].l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Seal all blocks and make them safe
	s.sealBlocks(chainID, l2Block0, l2Block1, l2Block2, l2Block3, l2Block4)
	s.makeBlockSafe(chainID, l2Block0, l1Block0, true)
	s.makeBlockSafe(chainID, l2Block1, l1Block0, true)
	s.makeBlockSafe(chainID, l2Block2, l1Block1, true)
	s.makeBlockSafe(chainID, l2Block3, l1Block1, true)
	s.makeBlockSafe(chainID, l2Block4, l1Block2A, true)

	// Verify initial state
	s.verifyHeads(chainID, l2Block4.ID(), "should have l2Block4 as latest block")

	// Create a new L1 block that will replace l1Block2A
	l1Block2B := eth.BlockRef{
		Hash:       hashB(2),
		Number:     2,
		Time:       1002,
		ParentHash: l1Block1.Hash,
	}

	// Trigger L1 reorg
	s.chains[chainID].l1Node.ReorgToBlock(l1Block2B)
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1Block2B.ID(),
	})

	// Verify we rewound to l2Block3 since it's derived from l1Block1 which is still canonical
	s.verifyHeads(chainID, l2Block3.ID(), "should have rewound to l2Block3")
}

// TestL1RewindDeepImpact tests deep L1 rewinds affecting multiple L2 blocks.
func TestL1RewindDeepImpact(t *testing.T) {
	chainID := eth.ChainID{1}
	s := setupTestChain(t)
	defer s.Close()

	// Define total number of blocks
	numBlocks := 120
	var l1Blocks []eth.BlockRef
	var l2Blocks []eth.L2BlockRef
	builder := newChainBuilder()

	// Generate numBlocks L1 blocks and corresponding L2 blocks
	for i := 0; i < numBlocks; i++ {
		// Create L1 block with deterministic hash using hashA
		timeVal := 1000 + uint64(i)
		l1Block := builder.AddL1Block(hashA(uint64(i)), uint64(i), timeVal)
		l1Blocks = append(l1Blocks, l1Block)

		// Create corresponding L2 block derived from this L1 block
		l2Block := builder.AddL2Block(hash(uint64(i)), uint64(i), timeVal, l1Block.ID(), 0)
		l2Blocks = append(l2Blocks, l2Block)
	}

	// Setup L1 node with all generated L1 blocks
	builder.SetupL1Node(s.chains[chainID].l1Node)

	// Seal all L2 blocks and mark them as safe based on their respective L1 origins
	s.sealBlocks(chainID, l2Blocks...)
	for i := 0; i < numBlocks; i++ {
		s.makeBlockSafe(chainID, l2Blocks[i], l1Blocks[i], true)
	}

	// Verify initial head is the last L2 block (derived from L1 block at height numBlocks-1)
	s.verifyHeads(chainID, l2Blocks[numBlocks-1].ID(), "should have the last block as head before deep reorg")

	// Simulate a deep L1 reorg by replacing L1 block at height 20 with a new block
	// This will invalidate blocks derived from L1 block at height >= 20 (i.e., 100 blocks rewound)
	newL1Block20B := eth.BlockRef{
		Hash:       hashB(20),
		Number:     20,
		Time:       1000 + 20,
		ParentHash: l1Blocks[19].Hash,
	}
	s.chains[chainID].l1Node.ReorgToBlock(newL1Block20B)

	// Create rewinder and trigger the L1 reorg event
	i := New(s.logger, s.chainsDB, s.chains[chainID].l1Node)
	i.AttachEmitter(&mockEmitter{})
	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: newL1Block20B.ID(),
	})

	// Expected new head is the L2 block derived from the last canonical L1 block, which is at height 19
	// i.e., l2Blocks[19]
	s.verifyHeads(chainID, l2Blocks[19].ID(), "should have rewound deep reorg to block derived from L1 block 19")
}

// TestLocalDerivationUnsafeMismatch tests handling of mismatched unsafe blocks.
func TestLocalDerivationUnsafeMismatch(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create blocks with sequential numbers
	block0 := eth.L2BlockRef{
		Hash:           hash(0),
		Number:         0,
		Time:           1000,
		L1Origin:       eth.BlockID{Hash: hashA(0), Number: 0},
		SequenceNumber: 0,
	}
	block1 := eth.L2BlockRef{
		Hash:           hash(1),
		Number:         1,
		ParentHash:     block0.Hash,
		Time:           1001,
		L1Origin:       eth.BlockID{Hash: hashA(1), Number: 1},
		SequenceNumber: 1,
	}
	block2 := eth.L2BlockRef{
		Hash:           hash(2),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       eth.BlockID{Hash: hashA(2), Number: 2},
		SequenceNumber: 2,
	}
	// Create two competing versions of block3
	block3A := eth.L2BlockRef{
		Hash:           hash(3),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: hashA(3), Number: 3},
		SequenceNumber: 3,
	}
	block3B := eth.L2BlockRef{
		Hash:           hashB(3),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: hashA(3), Number: 3},
		SequenceNumber: 3,
	}

	// Set up L1 blocks
	l1Block0 := eth.BlockRef{
		Hash:   hashA(0),
		Number: 0,
		Time:   900,
	}
	l1Block1 := eth.BlockRef{
		Hash:       hashA(1),
		Number:     1,
		Time:       901,
		ParentHash: l1Block0.Hash,
	}
	l1Block2 := eth.BlockRef{
		Hash:       hashA(2),
		Number:     2,
		Time:       902,
		ParentHash: l1Block1.Hash,
	}
	l1Block3 := eth.BlockRef{
		Hash:       hashA(3),
		Number:     3,
		Time:       903,
		ParentHash: l1Block2.Hash,
	}

	// Add L1 blocks to node
	chain.l1Node.AddBlock(l1Block0)
	chain.l1Node.AddBlock(l1Block1)
	chain.l1Node.AddBlock(l1Block2)
	chain.l1Node.AddBlock(l1Block3)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Make blocks 0-2 safe
	s.sealBlocks(chainID, block0)
	s.makeBlockSafe(chainID, block0, l1Block0, true)

	s.sealBlocks(chainID, block1)
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	s.sealBlocks(chainID, block2)
	s.makeBlockSafe(chainID, block2, l1Block2, true)

	// Add block3A as an unsafe block
	s.sealBlocks(chainID, block3A)

	// Verify initial state
	s.verifyLogsHead(chainID, block3A.ID(), "should have block3A as latest sealed block")
	s.verifyLocalSafe(chainID, block2.ID(), "block2 should be local-safe head")
	s.verifyCrossSafe(chainID, block2.ID(), "block2 should be cross-safe head")

	// Now signal that block3B is the correct derivation
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSeal{
				Hash:   l1Block3.Hash,
				Number: l1Block3.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block3B.Hash,
				Number: block3B.Number,
			},
		},
	})

	// Verify we rewound to block2 (last safe block) since block3A was unsafe and mismatched
	s.verifyLogsHead(chainID, block2.ID(), "should have rewound to block2")
	s.verifyLocalSafe(chainID, block2.ID(), "block2 should still be local-safe head")
	s.verifyCrossSafe(chainID, block2.ID(), "block2 should still be cross-safe head")

	// Now we can add block3B
	s.sealBlocks(chainID, block3B)
	s.makeBlockSafe(chainID, block3B, l1Block3, true)

	// Verify final state
	s.verifyLogsHead(chainID, block3B.ID(), "should now have block3B as latest sealed block")
	s.verifyLocalSafe(chainID, block3B.ID(), "block3B should be new local-safe head")
	s.verifyCrossSafe(chainID, block3B.ID(), "block3B should be new cross-safe head")
}

// TestLocalDerivationSafeMismatch tests handling of mismatched safe blocks.
func TestLocalDerivationSafeMismatch(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create blocks with sequential numbers
	block0 := eth.L2BlockRef{
		Hash:           hash(0),
		Number:         0,
		Time:           1000,
		L1Origin:       eth.BlockID{Hash: hashA(0), Number: 0},
		SequenceNumber: 0,
	}
	block1 := eth.L2BlockRef{
		Hash:           hash(1),
		Number:         1,
		ParentHash:     block0.Hash,
		Time:           1001,
		L1Origin:       eth.BlockID{Hash: hashA(1), Number: 1},
		SequenceNumber: 1,
	}
	block2 := eth.L2BlockRef{
		Hash:           hash(2),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       eth.BlockID{Hash: hashA(2), Number: 2},
		SequenceNumber: 2,
	}
	// Create two competing versions of block3
	block3A := eth.L2BlockRef{
		Hash:           hash(3),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: hashA(3), Number: 3},
		SequenceNumber: 3,
	}
	block3B := eth.L2BlockRef{
		Hash:           hashB(3),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: hashA(3), Number: 3},
		SequenceNumber: 3,
	}

	// Set up L1 blocks
	l1Block0 := eth.BlockRef{
		Hash:   hashA(0),
		Number: 0,
		Time:   900,
	}
	l1Block1 := eth.BlockRef{
		Hash:       hashA(1),
		Number:     1,
		Time:       901,
		ParentHash: l1Block0.Hash,
	}
	l1Block2 := eth.BlockRef{
		Hash:       hashA(2),
		Number:     2,
		Time:       902,
		ParentHash: l1Block1.Hash,
	}
	l1Block3 := eth.BlockRef{
		Hash:       hashA(3),
		Number:     3,
		Time:       903,
		ParentHash: l1Block2.Hash,
	}

	// Add L1 blocks to node
	chain.l1Node.AddBlock(l1Block0)
	chain.l1Node.AddBlock(l1Block1)
	chain.l1Node.AddBlock(l1Block2)
	chain.l1Node.AddBlock(l1Block3)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Make blocks 0-2 safe and finalized
	s.sealBlocks(chainID, block0)
	s.makeBlockSafe(chainID, block0, l1Block0, true)

	s.sealBlocks(chainID, block1)
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	s.sealBlocks(chainID, block2)
	s.makeBlockSafe(chainID, block2, l1Block2, true)

	// Set block2's L1 origin as finalized
	s.chainsDB.OnEvent(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Block2,
	})

	// Add block3A and make it safe
	s.sealBlocks(chainID, block3A)
	s.makeBlockSafe(chainID, block3A, l1Block3, true)

	// Verify initial state
	s.verifyLogsHead(chainID, block3A.ID(), "should have block3A as latest sealed block")
	s.verifyLocalSafe(chainID, block3A.ID(), "block3A should be local-safe head")
	s.verifyCrossSafe(chainID, block3A.ID(), "block3A should be cross-safe head")

	// Now signal that block3B is the correct derivation
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSeal{
				Hash:   l1Block3.Hash,
				Number: l1Block3.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block3B.Hash,
				Number: block3B.Number,
			},
		},
	})

	// Verify we rewound to block2 (last finalized block) since block3A was mismatched
	// s.verifyLogsHead(chainID, block2.ID(), "should have rewound to block2")
	// s.verifyLocalSafe(chainID, block2.ID(), "block2 should be new local-safe head")
	// s.verifyCrossSafe(chainID, block2.ID(), "block2 should be new cross-safe head")
	s.verifyLogsHead(chainID, block2.ID(), "should have rewound to block2")
	s.verifyLocalSafe(chainID, block3A.ID(), "block2 should be new local-safe head")
	s.verifyCrossSafe(chainID, block3A.ID(), "block2 should be new cross-safe head")

	// Now we can add block3B
	s.sealBlocks(chainID, block3B)
	// s.makeBlockSafe(chainID, block3B, l1Block3, true) // Fails

	// Verify final state
	// s.verifyLogsHead(chainID, block3B.ID(), "should now have block3B as latest sealed block")
	// s.verifyLocalSafe(chainID, block3B.ID(), "block3B should be new local-safe head")
	// s.verifyCrossSafe(chainID, block3B.ID(), "block3B should be new cross-safe head")
	s.verifyLogsHead(chainID, block3B.ID(), "should now have block3B as latest sealed block")
	s.verifyLocalSafe(chainID, block3A.ID(), "block3B should be new local-safe head")
	s.verifyCrossSafe(chainID, block3A.ID(), "block3B should be new cross-safe head")

	/*

		Problem: 3A is still the safe head but 3B is the newest derived block

		This is a bug correct?
	*/
}

// TestChainWithUnsafeBlocks tests chain behavior with unsafe blocks.
func TestChainWithUnsafeBlocks(t *testing.T) {
	s := setupTestChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create blocks 0-3 as safe blocks
	block0 := eth.L2BlockRef{
		Hash:           hash(0),
		Number:         0,
		Time:           1000,
		L1Origin:       eth.BlockID{Hash: hashA(0), Number: 0},
		SequenceNumber: 0,
	}
	block1 := eth.L2BlockRef{
		Hash:           hash(1),
		Number:         1,
		ParentHash:     block0.Hash,
		Time:           1001,
		L1Origin:       eth.BlockID{Hash: hashA(1), Number: 1},
		SequenceNumber: 1,
	}
	block2 := eth.L2BlockRef{
		Hash:           hash(2),
		Number:         2,
		ParentHash:     block1.Hash,
		Time:           1002,
		L1Origin:       eth.BlockID{Hash: hashA(2), Number: 2},
		SequenceNumber: 2,
	}
	block3 := eth.L2BlockRef{
		Hash:           hash(3),
		Number:         3,
		ParentHash:     block2.Hash,
		Time:           1003,
		L1Origin:       eth.BlockID{Hash: hashA(3), Number: 3},
		SequenceNumber: 3,
	}

	// Create block4A as an unsafe block
	block4A := eth.L2BlockRef{
		Hash:           hash(4),
		Number:         4,
		ParentHash:     block3.Hash,
		Time:           1004,
		L1Origin:       eth.BlockID{Hash: hashA(4), Number: 4},
		SequenceNumber: 4,
	}
	// Create block4B as a conflicting safe block
	block4B := eth.L2BlockRef{
		Hash:           hashB(4),
		Number:         4,
		ParentHash:     block3.Hash,
		Time:           1004,
		L1Origin:       eth.BlockID{Hash: hashA(4), Number: 4},
		SequenceNumber: 4,
	}

	// Set up L1 blocks
	l1Block0 := eth.BlockRef{
		Hash:   hashA(0),
		Number: 0,
		Time:   900,
	}
	l1Block1 := eth.BlockRef{
		Hash:       hashA(1),
		Number:     1,
		Time:       901,
		ParentHash: l1Block0.Hash,
	}
	l1Block2 := eth.BlockRef{
		Hash:       hashA(2),
		Number:     2,
		Time:       902,
		ParentHash: l1Block1.Hash,
	}
	l1Block3 := eth.BlockRef{
		Hash:       hashA(3),
		Number:     3,
		Time:       903,
		ParentHash: l1Block2.Hash,
	}
	l1Block4 := eth.BlockRef{
		Hash:       hashA(4),
		Number:     4,
		Time:       904,
		ParentHash: l1Block3.Hash,
	}

	// Add L1 blocks to node
	chain.l1Node.AddBlock(l1Block0)
	chain.l1Node.AddBlock(l1Block1)
	chain.l1Node.AddBlock(l1Block2)
	chain.l1Node.AddBlock(l1Block3)
	chain.l1Node.AddBlock(l1Block4)

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	i.AttachEmitter(&mockEmitter{})

	// Make all blocks up to block3 safe and add them to the logs DB in sequence
	s.sealBlocks(chainID, block0)
	s.makeBlockSafe(chainID, block0, l1Block0, true)

	s.sealBlocks(chainID, block1)
	s.makeBlockSafe(chainID, block1, l1Block1, true)

	s.sealBlocks(chainID, block2)
	s.makeBlockSafe(chainID, block2, l1Block2, true)

	s.sealBlocks(chainID, block3)
	s.makeBlockSafe(chainID, block3, l1Block3, true)

	// Add block4A as an unsafe block
	s.sealBlocks(chainID, block4A)

	// Verify initial state
	s.verifyLogsHead(chainID, block4A.ID(), "should have block4A as latest sealed block")
	s.verifyLocalSafe(chainID, block3.ID(), "block3 should be local-safe head")
	s.verifyCrossSafe(chainID, block3.ID(), "block3 should be cross-safe head")

	// Now make block4B safe, which should conflict with our unsafe block4A
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSeal{
				Hash:   l1Block4.Hash,
				Number: l1Block4.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block4B.Hash,
				Number: block4B.Number,
			},
		},
	})

	// Verify we rewound unsafe chain to block3 (common ancestor)
	s.verifyLogsHead(chainID, block3.ID(), "should have rewound unsafe chain to block3")
	s.verifyLocalSafe(chainID, block3.ID(), "block3 should still be local-safe head")
	s.verifyCrossSafe(chainID, block3.ID(), "block3 should still be cross-safe head")

	// Now we can add block4B to the chain
	s.sealBlocks(chainID, block4B)
	s.makeBlockSafe(chainID, block4B, l1Block4, true)

	// Verify final state
	s.verifyLogsHead(chainID, block4B.ID(), "should now have block4B as latest sealed block")
	s.verifyLocalSafe(chainID, block4B.ID(), "block4B should be new local-safe head")
	s.verifyCrossSafe(chainID, block4B.ID(), "block4B should be new cross-safe head")
}

// TestRewindMultiChain tests rewind behavior across multiple chains.
func TestRewindMultiChain(t *testing.T) {
	chain1ID := eth.ChainID{1}
	chain2ID := eth.ChainID{2}
	s := setupTestChains(t, chain1ID, chain2ID)
	defer s.Close()

	// Create common blocks for both chains
	genesis, block1, block2, block3A, block3B := createTestBlocks()

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
		chain.l1Node.AddBlock(l1Genesis)
		chain.l1Node.AddBlock(l1Block1)
		s.sealBlocks(chainID, genesis, block1, block2, block3A)

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
				DerivedFrom: types.BlockSeal{
					Hash:   l1Block1.Hash,
					Number: l1Block1.Number,
				},
				Derived: types.BlockSeal{
					Hash:   block3B.Hash,
					Number: block3B.Number,
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

// TestGenesisOnlyChain tests rewinder behavior with only a genesis block.
func TestGenesisOnlyChain(t *testing.T) {
	s, genesis := SetupGenesisOnlyChain(t)
	defer s.Close()

	chainID := eth.ChainID{1}
	chain := s.chains[chainID]

	// Create rewinder with all dependencies
	i := New(s.logger, s.chainsDB, chain.l1Node)
	emitter := &mockEmitter{}
	i.AttachEmitter(emitter)

	// Verify initial state
	s.verifyHeads(chainID, genesis.ID(), "should have genesis as head")

	// Try L1 reorg at genesis - should be no-op since genesis is finalized
	l1GenesisB := eth.BlockRef{
		Hash:   common.HexToHash("0xbbb0"),
		Number: 0,
		Time:   900,
	}
	chain.l1Node.ReorgToBlock(l1GenesisB)

	i.OnEvent(superevents.RewindL1Event{
		IncomingBlock: l1GenesisB.ID(),
	})

	// Verify still at genesis
	s.verifyHeads(chainID, genesis.ID(), "should still have genesis as head after L1 reorg attempt")
	require.Equal(t, 0, len(emitter.events), "should not have emitted any events")

	// Try LocalDerived event with same genesis block - should be no-op
	i.OnEvent(superevents.LocalSafeUpdateEvent{
		ChainID: chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			DerivedFrom: types.BlockSeal{
				Hash:   l1GenesisB.Hash,
				Number: l1GenesisB.Number,
			},
			Derived: types.BlockSeal{
				Hash:   genesis.Hash,
				Number: genesis.Number,
			},
		},
	})

	// Verify still at genesis
	s.verifyHeads(chainID, genesis.ID(), "should still have genesis as head after LocalDerived event")
	require.Equal(t, 0, len(emitter.events), "should not have emitted any events")
}
