package rewinder

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/stretchr/testify/require"
)

// TestRewindNotNeeded tests that no rewind occurs when the state doesn't need to be rewound.
func TestRewindNotNeeded(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	l1Block2 := builder.createL1Block(l1Block1)

	genesis := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	block1 := builder.createL2Block(genesis, l1Block1.ID(), 0)
	block2 := builder.createL2Block(block1, l1Block2.ID(), 0)

	s.sealBlocks(genesis, block1, block2)
	s.makeBlockSafe(genesis, l1Block0, true)
	s.makeBlockSafe(block1, l1Block1, true)
	s.makeBlockSafe(block2, l1Block2, true)

	// Trigger potential L1 reorg but to the current block
	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: l1Block2.ID(),
	})
	s.assertAllHeads(block2.ID(), "all heads should still be on block2")

	// Trigger potential L2 reorg but to the current block
	s.emitter.Emit(superevents.LocalSafeUpdateEvent{
		ChainID: s.chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			Source: types.BlockSeal{
				Hash:   l1Block2.Hash,
				Number: l1Block2.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block2.Hash,
				Number: block2.Number,
			},
		},
	})
	s.assertAllHeads(block2.ID(), "all heads should still be on block2")
}

// TestRewindL1 tests basic handling of L1 rewinds.
func TestRewindL1(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	l1Block2A := builder.createL1Block(l1Block1)

	genesis := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	block1 := builder.createL2Block(genesis, l1Block1.ID(), 0)
	block2 := builder.createL2Block(block1, l1Block2A.ID(), 0)

	s.sealBlocks(genesis, block1, block2)
	s.makeBlockSafe(genesis, l1Block0, true)
	s.makeBlockSafe(block1, l1Block1, true)
	s.makeBlockSafe(block2, l1Block2A, true)
	s.assertAllHeads(block2.ID(), "all heads should have block2 as latest block")

	// Create alternate L1 block2B that will replace l1Block2
	l1Block2B := builder.createL1Block(l1Block1)
	err := s.l1Node.reorg(l1Block2B)
	require.NoError(t, err)
	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: l1Block2B.ID(),
	})
	s.processEvents()

	s.assertHeads(l1Block2A.ID(), block1.ID(), block1.ID(), "should have rewound safe heads to block1")
}

// TestRewindL1PastCrossSafe tests rewind behavior when L1 rewinds occur beyond the cross-safe head.
func TestRewindL1PastCrossSafe(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	// Create chain with builder
	l1Genesis := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Genesis)
	l1Block2 := builder.createL1Block(l1Block1)
	l1Block3A := builder.createL1Block(l1Block2)

	// Create L2 blocks
	genesis := builder.createL2Block(eth.L2BlockRef{}, l1Genesis.ID(), 0)
	block1 := builder.createL2Block(genesis, l1Block1.ID(), 0)
	block2 := builder.createL2Block(block1, l1Block2.ID(), 0)
	block3A := builder.createL2Block(block2, l1Block3A.ID(), 0)

	// Seal blocks and make them safe
	s.sealBlocks(genesis, block1, block2, block3A)
	s.makeBlockSafe(genesis, l1Genesis, true)
	s.makeBlockSafe(block1, l1Block1, true)
	s.makeBlockSafe(block2, l1Block2, true)
	s.makeBlockSafe(block3A, l1Block3A, true)
	s.emitter.Emit(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Block2,
	})
	s.assertAllHeads(block3A.ID(), "all heads should be on block3A")

	// Replace block3A with block3B
	l1Block3B := builder.createL1Block(l1Block2)
	err := s.l1Node.reorg(l1Block3B)
	require.NoError(t, err)
	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: l1Block3B.ID(),
	})
	s.processEvents()

	// Verify we rewound to block2
	s.assertHeads(l1Block3A.ID(), block2.ID(), block2.ID(), "should have rewound to finalized block2")

}

// TestRewindL1PastFinality tests that rewinds respect finality boundaries.
func TestRewindL1PastFinality(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	l1Block2 := builder.createL1Block(l1Block1)
	l1Block3A := builder.createL1Block(l1Block2)

	genesis := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	block1 := builder.createL2Block(genesis, l1Block1.ID(), 0)
	block2 := builder.createL2Block(block1, l1Block2.ID(), 0)
	block3A := builder.createL2Block(block2, l1Block3A.ID(), 0)

	s.sealBlocks(genesis, block1, block2, block3A)
	s.makeBlockSafe(genesis, l1Block0, true)
	s.makeBlockSafe(block1, l1Block1, true)
	s.makeBlockSafe(block2, l1Block2, true)
	s.makeBlockSafe(block3A, l1Block3A, true)
	s.emitter.Emit(superevents.FinalizedL1RequestEvent{
		FinalizedL1: l1Block2,
	})
	s.assertAllHeads(block3A.ID(), "all heads should be on block3A")

	// Replace block3A with block3B and trigger L1 reorg
	l1Block3B := builder.createL1Block(l1Block2)
	err := s.l1Node.reorg(l1Block3B)
	require.NoError(t, err)
	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: l1Block3B.ID(),
	})
	s.processEvents()

	// Verify we rewound safe heads to block2 since it's finalized
	s.assertHeads(l1Block3A.ID(), block2.ID(), block2.ID(), "should have rewound safe heads to finalized block2")
}

// TestRewindL1NoL2Impact tests L1 rewinds that don't affect L2 blocks.
func TestRewindL1NoL2Impact(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	_ = builder.createL1Block(l1Block1) // Create but don't need reference

	genesis := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	var l2Blocks []eth.L2BlockRef
	block := genesis
	for i := range 2 {
		block = builder.createL2Block(block, l1Block0.ID(), uint64(i))
		l2Blocks = append(l2Blocks, block)
		s.sealBlocks(block)
		s.makeBlockSafe(block, l1Block0, true)
	}
	head := l2Blocks[len(l2Blocks)-1].ID()
	s.assertAllHeads(head, "all heads should be on the same block")

	// Trigger L1 reorg that doesn't affect L2
	newL1Block := builder.createL1Block(l1Block1)
	err := s.l1Node.reorg(newL1Block)
	require.NoError(t, err)
	s.assertAllHeads(head, "all heads should be on the same block")
}

// TestRewindL1SingleBlockL2Impact tests L1 rewinds that affect a single L2 block.
func TestRewindL1SingleBlockL2Impact(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	l1Block2A := builder.createL1Block(l1Block1)

	l2Block0 := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	l2Block1 := builder.createL2Block(l2Block0, l1Block0.ID(), 1)
	l2Block2 := builder.createL2Block(l2Block1, l1Block1.ID(), 0)
	l2Block3 := builder.createL2Block(l2Block2, l1Block1.ID(), 1)
	l2Block4 := builder.createL2Block(l2Block3, l1Block2A.ID(), 0)

	s.sealBlocks(l2Block0, l2Block1, l2Block2, l2Block3, l2Block4)
	s.makeBlockSafe(l2Block0, l1Block0, true)
	s.makeBlockSafe(l2Block1, l1Block0, true)
	s.makeBlockSafe(l2Block2, l1Block1, true)
	s.makeBlockSafe(l2Block3, l1Block1, true)
	s.makeBlockSafe(l2Block4, l1Block2A, true)
	s.assertAllHeads(l2Block4.ID(), "all heads should be on l2Block4")

	// Create a new L1 block that will replace l1Block2A and trigger L1 reorg
	l1Block2B := builder.createL1Block(l1Block1)
	err := s.l1Node.reorg(l1Block2B)
	require.NoError(t, err)
	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: l1Block2B.ID(),
	})
	s.processEvents()

	// Verify we rewound to l2Block3 since it's derived from l1Block1 which is still canonical
	s.assertHeads(l2Block4.ID(), l2Block3.ID(), l2Block3.ID(), "should have l2Block3 as latest sealed block")
}

// TestRewindL1GenesisOnlyL2 tests rewinder behavior with only a genesis block.
func TestRewindL1GenesisOnlyL2(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Genesis := builder.createL1Block(eth.BlockRef{})
	l2Genesis := builder.createL2Block(eth.L2BlockRef{}, l1Genesis.ID(), 0)
	s.sealBlocks(l2Genesis)
	s.makeBlockSafe(l2Genesis, l1Genesis, true)
	s.assertHeads(l2Genesis.ID(), l2Genesis.ID(), l2Genesis.ID(), "should have genesis as head")

	// Try L1 reorg at genesis - should be no-op since genesis is finalized
	l1GenesisB := builder.createL1Block(eth.BlockRef{})
	err := s.l1Node.reorg(l1GenesisB)
	require.NoError(t, err)

	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: l1GenesisB.ID(),
	})

	// Verify still at genesis
	s.assertHeads(l2Genesis.ID(), l2Genesis.ID(), l2Genesis.ID(), "should still have genesis as head after L1 reorg attempt")

	// Try LocalDerived event with same genesis block - should be no-op
	s.emitter.Emit(superevents.LocalSafeUpdateEvent{
		ChainID: s.chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			Source: types.BlockSeal{
				Hash:   l1GenesisB.Hash,
				Number: l1GenesisB.Number,
			},
			Derived: types.BlockSeal{
				Hash:   l2Genesis.Hash,
				Number: l2Genesis.Number,
			},
		},
	})

	// Verify still at genesis
	s.assertHeads(l2Genesis.ID(), l2Genesis.ID(), l2Genesis.ID(), "should still have genesis as head after LocalDerived event")
}

// TODO: No L1 reorg here
// TestRewindL2 tests basic handling of L2 rewinds via LocalDerivedEvent.
func TestRewindL2LocalDerivedEvent(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	l1Block2 := builder.createL1Block(l1Block1)

	genesis := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	block1 := builder.createL2Block(genesis, l1Block1.ID(), 0)
	block2A := builder.createL2Block(block1, l1Block2.ID(), 0)

	s.sealBlocks(genesis, block1)
	s.makeBlockSafe(genesis, l1Block0, true)
	s.makeBlockSafe(block1, l1Block1, true)
	s.sealBlocks(block2A)
	s.assertHeads(block2A.ID(), block1.ID(), block1.ID(), "should have block2A as latest sealed block, block1 as safe")

	// Create alternate block2B that will replace block2A
	block2B := builder.createL2Block(block1, l1Block2.ID(), 0)
	s.emitter.Emit(superevents.LocalSafeUpdateEvent{
		ChainID: s.chainID,
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
	s.processEvents()

	// After rewind, all heads should be at block1
	s.assertAllHeads(block1.ID(), "all heads should have rewound to block1")

	// Add block2B and make it safe
	s.sealBlocks(block2B)
	s.makeBlockSafe(block2B, l1Block1, true)

	// After sealing and making block2B safe, all heads should be at block2B
	s.assertAllHeads(block2B.ID(), "all heads should be on block2B")
}

// TestL1RewindDeepImpact tests deep L1 rewinds affecting multiple L2 blocks.
func TestL1RewindDeepImpact(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	numBlocks := 120
	var l1Blocks []eth.BlockRef
	var l2Blocks []eth.L2BlockRef
	builder := newChainBuilder(s.l1Node)

	// Generate numBlocks L1 blocks and corresponding L2 blocks
	var l1Block eth.BlockRef
	var l2Block eth.L2BlockRef
	for i := range numBlocks {
		// Create L1 block
		if i == 0 {
			l1Block = builder.createL1Block(eth.BlockRef{})
		} else {
			l1Block = builder.createL1Block(l1Blocks[i-1])
		}
		l1Blocks = append(l1Blocks, l1Block)

		// Create corresponding L2 block derived from this L1 block
		if i == 0 {
			l2Block = builder.createL2Block(eth.L2BlockRef{}, l1Block.ID(), 0)
		} else {
			l2Block = builder.createL2Block(l2Blocks[i-1], l1Block.ID(), 0)
		}
		l2Blocks = append(l2Blocks, l2Block)
	}
	s.sealBlocks(l2Blocks...)
	for i := range numBlocks {
		s.makeBlockSafe(l2Blocks[i], l1Blocks[i], true)
	}

	// Verify initial safeHead is the last L2 block (derived from L1 block at height numBlocks-1)
	safeHead := l2Blocks[numBlocks-1].ID()
	s.assertAllHeads(safeHead, "all heads should be on the last block")

	// Simulate a deep L1 reorg by replacing L1 block at height 20 with a new block
	// This will invalidate blocks derived from L1 block at height >= 20 (i.e., 100 blocks rewound)
	newL1Block20B := builder.createL1Block(l1Blocks[19]) // Create conflicting block after block 19
	err := s.l1Node.reorg(newL1Block20B)
	require.NoError(t, err)
	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: newL1Block20B.ID(),
	})
	s.processEvents()

	// Expected new head is the L2 block derived from the last canonical L1 block, which is at height 19
	unsafeHead := l2Blocks[119].ID()
	safeHead = l2Blocks[19].ID()
	s.assertHeads(unsafeHead, safeHead, safeHead, "should have reverted safe heads")
}

// TestRewindL2LocalDerivationUnsafeMismatch tests handling of mismatched unsafe blocks.
func TestRewindL2LocalDerivationUnsafeMismatch(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	l1Block2 := builder.createL1Block(l1Block1)
	l1Block3 := builder.createL1Block(l1Block2)

	genesis := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	block0 := builder.createL2Block(genesis, l1Block0.ID(), 0)
	block1 := builder.createL2Block(block0, l1Block1.ID(), 0)
	block2 := builder.createL2Block(block1, l1Block2.ID(), 0)
	block3A := builder.createL2Block(block2, l1Block3.ID(), 0)

	block3B := builder.createL2Block(block2, l1Block3.ID(), 0)

	// Seal all blocks and make them safe, but block3A stays unsafe
	s.sealBlocks(block0, block1, block2, block3A)
	s.makeBlockSafe(block0, l1Block0, true)
	s.makeBlockSafe(block1, l1Block1, true)
	s.makeBlockSafe(block2, l1Block2, true)
	s.assertHeads(block3A.ID(), block2.ID(), block2.ID(), "should have block3A as latest sealed block, block2 as safe")

	// Cause L1 reorg by replacing l1Block3 with l1Block3B
	l1Block3B := builder.createL1Block(l1Block2)
	err := s.l1Node.reorg(l1Block3B)
	require.NoError(t, err)
	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: l1Block3B.ID(),
	})
	s.processEvents()

	// Now signal that block3B is the correct derivation via emitter
	s.emitter.Emit(superevents.LocalDerivedEvent{
		ChainID: s.chainID,
		Derived: types.DerivedBlockRefPair{
			Source: l1Block3B,
			Derived: eth.BlockRef{
				Hash:       block3B.Hash,
				Number:     block3B.Number,
				Time:       block3B.Time,
				ParentHash: block3B.ParentHash,
			},
		},
	})
	s.processEvents()

	// Verify we rewound to block2 (last finalized block) since block3A was mismatched
	s.assertHeads(block2.ID(), block3B.ID(), block2.ID(), "should have rewound to block2")

	// Now we can add block3B
	s.sealBlocks(block3B)
	s.assertHeads(block3B.ID(), block3B.ID(), block2.ID(), "should now have block3B sealed")

	s.makeBlockSafe(block3B, l1Block3B, true)
	s.assertAllHeads(block3B.ID(), "should have block3B for all heads")
}

// TestRewindL2LocalDerivationSafeMismatch tests handling of mismatched safe blocks.
func TestRewindL2LocalDerivationSafeMismatch(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	l1Block2 := builder.createL1Block(l1Block1)
	l1Block3 := builder.createL1Block(l1Block2)

	block0 := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	block1 := builder.createL2Block(block0, l1Block1.ID(), 0)
	block2 := builder.createL2Block(block1, l1Block2.ID(), 0)
	block3A := builder.createL2Block(block2, l1Block3.ID(), 0)
	block3B := builder.createL2Block(block2, l1Block3.ID(), 0)

	s.sealBlocks(block0, block1, block2, block3A)
	s.makeBlockSafe(block0, l1Block0, true)
	s.makeBlockSafe(block1, l1Block1, true)
	s.makeBlockSafe(block2, l1Block2, true)
	s.makeBlockSafe(block3A, l1Block3, true)
	s.assertAllHeads(block3A.ID(), "should have block3A for all heads")

	// Cause L1 reorg by replacing l1Block3 with l1Block3B
	l1Block3B := builder.createL1Block(l1Block2)
	err := s.l1Node.reorg(l1Block3B)
	require.NoError(t, err)
	s.emitter.Emit(superevents.RewindL1Event{
		IncomingBlock: l1Block3B.ID(),
	})
	s.processEvents()

	// Signal that block3B is the correct derivation via emitter
	s.emitter.Emit(superevents.LocalDerivedEvent{
		ChainID: s.chainID,
		Derived: types.DerivedBlockRefPair{
			Source: l1Block3B,
			Derived: eth.BlockRef{
				Hash:       block3B.Hash,
				Number:     block3B.Number,
				Time:       block3B.Time,
				ParentHash: block3B.ParentHash,
			},
		},
	})
	s.processEvents()
	s.assertHeads(block2.ID(), block3B.ID(), block2.ID(), "should have rewound to block2")

	// Now we can add block3B but block3B isn't cross-safe yet
	s.sealBlocks(block3B)
	s.assertHeads(block3B.ID(), block3B.ID(), block2.ID(), "should now have block3B sealed")

	// Make block3B safe
	s.makeBlockSafe(block3B, l1Block3B, true)
	s.assertAllHeads(block3B.ID(), "should have block3B for all heads")
}

// TestRewindL2ChainWithUnsafeBlocks tests chain behavior with unsafe blocks.
func TestRewindL2ChainWithUnsafeBlocks(t *testing.T) {
	s := newTestChain(t)
	defer s.close()

	builder := newChainBuilder(s.l1Node)

	l1Block0 := builder.createL1Block(eth.BlockRef{})
	l1Block1 := builder.createL1Block(l1Block0)
	l1Block2 := builder.createL1Block(l1Block1)
	l1Block3 := builder.createL1Block(l1Block2)
	l1Block4 := builder.createL1Block(l1Block3)

	genesis := builder.createL2Block(eth.L2BlockRef{}, l1Block0.ID(), 0)
	block0 := builder.createL2Block(genesis, l1Block0.ID(), 0)
	block1 := builder.createL2Block(block0, l1Block1.ID(), 0)
	block2 := builder.createL2Block(block1, l1Block2.ID(), 0)
	block3 := builder.createL2Block(block2, l1Block3.ID(), 0)

	block4A := builder.createL2Block(block3, l1Block4.ID(), 0)
	block4B := builder.createL2Block(block3, l1Block4.ID(), 0)

	s.sealBlocks(block0, block1, block2, block3)
	s.makeBlockSafe(block0, l1Block0, true)
	s.makeBlockSafe(block1, l1Block1, true)
	s.makeBlockSafe(block2, l1Block2, true)
	s.makeBlockSafe(block3, l1Block3, true)
	s.sealBlocks(block4A)
	s.assertHeads(block4A.ID(), block3.ID(), block3.ID(), "should have block4A as latest sealed block, block3 as safe")

	// Make block4B safe, which should conflict with our unsafe block4A
	s.emitter.Emit(superevents.LocalSafeUpdateEvent{
		ChainID: s.chainID,
		NewLocalSafe: types.DerivedBlockSealPair{
			Source: types.BlockSeal{
				Hash:   l1Block4.Hash,
				Number: l1Block4.Number,
			},
			Derived: types.BlockSeal{
				Hash:   block4B.Hash,
				Number: block4B.Number,
			},
		},
	})
	s.processEvents()
	s.assertAllHeads(block3.ID(), "should have rewound all heads to block3")

	// Add block4B to the chain
	s.sealBlocks(block4B)
	s.makeBlockSafe(block4B, l1Block4, true)
	s.assertAllHeads(block4B.ID(), "should now have block4B for all heads")
}
