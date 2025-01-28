package rewinder

import (
	"context"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type mockEmitter struct {
	events []event.Event
}

func (m *mockEmitter) Emit(ev event.Event) {
	m.events = append(m.events, ev)
}

type mockChainsDB struct {
	rewindLocalUnsafeCalls []struct {
		chainID   eth.ChainID
		newHeight uint64
	}
	resetCrossUnsafeCalls []struct {
		chainID eth.ChainID
		number  uint64
	}
	localUnsafeHeads map[eth.ChainID]types.BlockSeal
	crossUnsafeHeads map[eth.ChainID]types.BlockSeal
	localSafeHeads   map[eth.ChainID]types.DerivedBlockSealPair
	crossSafeHeads   map[eth.ChainID]types.DerivedBlockSealPair
	finalizedHeads   map[eth.ChainID]types.BlockSeal
}

func newMockChainsDB() *mockChainsDB {
	return &mockChainsDB{
		localUnsafeHeads: make(map[eth.ChainID]types.BlockSeal),
		crossUnsafeHeads: make(map[eth.ChainID]types.BlockSeal),
		localSafeHeads:   make(map[eth.ChainID]types.DerivedBlockSealPair),
		crossSafeHeads:   make(map[eth.ChainID]types.DerivedBlockSealPair),
		finalizedHeads:   make(map[eth.ChainID]types.BlockSeal),
	}
}

func (m *mockChainsDB) FindSealedBlock(chainID eth.ChainID, number uint64) (types.BlockSeal, error) {
	return m.localUnsafeHeads[chainID], nil
}

func (m *mockChainsDB) RewindLocalUnsafe(chainID eth.ChainID, newHeight uint64) error {
	m.rewindLocalUnsafeCalls = append(m.rewindLocalUnsafeCalls, struct {
		chainID   eth.ChainID
		newHeight uint64
	}{chainID: chainID, newHeight: newHeight})
	return nil
}

func (m *mockChainsDB) LocalUnsafe(chainID eth.ChainID) (types.BlockSeal, error) {
	return m.localUnsafeHeads[chainID], nil
}

func (m *mockChainsDB) CrossUnsafe(chainID eth.ChainID) (types.BlockSeal, error) {
	return m.crossUnsafeHeads[chainID], nil
}

func (m *mockChainsDB) LocalSafe(chainID eth.ChainID) (types.DerivedBlockSealPair, error) {
	return m.localSafeHeads[chainID], nil
}

func (m *mockChainsDB) CrossSafe(chainID eth.ChainID) (types.DerivedBlockSealPair, error) {
	return m.crossSafeHeads[chainID], nil
}

func (m *mockChainsDB) Finalized(chainID eth.ChainID) (types.BlockSeal, error) {
	return m.finalizedHeads[chainID], nil
}

func (m *mockChainsDB) ResetCrossUnsafeIfNewerThan(chainID eth.ChainID, number uint64) error {
	m.resetCrossUnsafeCalls = append(m.resetCrossUnsafeCalls, struct {
		chainID eth.ChainID
		number  uint64
	}{chainID: chainID, number: number})
	return nil
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

// setupChainState is a helper function to set up the initial state of a chain
func setupChainState(chainID eth.ChainID, chainsDB *mockChainsDB, syncNode *mockSyncNode) {
	// Set up the local unsafe head at block 100
	chainsDB.localUnsafeHeads[chainID] = types.BlockSeal{
		Number:    100,
		Hash:      common.HexToHash("0x123"),
		Timestamp: 1235,
	}

	// Set up the finalized head at block 90
	chainsDB.finalizedHeads[chainID] = types.BlockSeal{
		Number:    90,
		Hash:      common.HexToHash("0xabc"),
		Timestamp: 1234,
	}

	// Set up blocks in the sync node
	// Add the common ancestor block at 95
	syncNode.blocks[95] = eth.BlockRef{
		Hash:   common.HexToHash("0xdef"),
		Number: 95,
	}
	// Add some blocks before and after to test the search
	syncNode.blocks[94] = eth.BlockRef{
		Hash:   common.HexToHash("0xabc"),
		Number: 94,
	}
	syncNode.blocks[96] = eth.BlockRef{
		Hash:   common.HexToHash("0x456"),
		Number: 96,
	}
}

// func TestRewinderHandleChainEvent(t *testing.T) {
// 	t.Run("handles chain rewind event", func(t *testing.T) {
// 		// Setup mocks
// 		chainsDB := newMockChainsDB()
// 		syncNode := newMockSyncNode()
// 		emitter := &mockEmitter{}

// 		// Create rewinder
// 		chainID := eth.ChainID{1}
// 		rewinder := New(log.New(), chainsDB)
// 		rewinder.AttachEmitter(emitter)
// 		rewinder.AttachSyncNode(chainID, syncNode)

// 		// Setup chain state with blocks
// 		setupChainState(chainID, chainsDB, syncNode)

// 		// Create and handle the rewind event to block 95
// 		ev := superevents.RewindChainEvent{
// 			ChainID: chainID,
// 			Candidate: eth.L2BlockRef{
// 				Hash:   common.HexToHash("0xdef"),
// 				Number: 95,
// 			},
// 		}
// 		require.NoError(t, rewinder.handleEventRewindChain(ev))

// 		// Verify ChainsDB was called to rewind to block 95
// 		require.Len(t, chainsDB.rewindLocalUnsafeCalls, 1, "should have one local-unsafe rewind call")
// 		require.Equal(t, chainID, chainsDB.rewindLocalUnsafeCalls[0].chainID)
// 		require.Equal(t, uint64(95), chainsDB.rewindLocalUnsafeCalls[0].newHeight)

// 		// Verify cross-unsafe was reset to block 95
// 		require.Len(t, chainsDB.resetCrossUnsafeCalls, 1, "should have one cross-unsafe reset call")
// 		require.Equal(t, chainID, chainsDB.resetCrossUnsafeCalls[0].chainID)
// 		require.Equal(t, uint64(95), chainsDB.resetCrossUnsafeCalls[0].number)
// 	})

// 	t.Run("handles rewind for multiple chains", func(t *testing.T) {
// 		// Setup mocks
// 		chainsDB := newMockChainsDB()
// 		syncNode1 := newMockSyncNode()
// 		syncNode2 := newMockSyncNode()
// 		emitter := &mockEmitter{}

// 		// Create rewinder
// 		chain1ID := eth.ChainID{1}
// 		chain2ID := eth.ChainID{2}
// 		rewinder := New(log.New(), chainsDB)
// 		rewinder.AttachEmitter(emitter)
// 		rewinder.AttachSyncNode(chain1ID, syncNode1)
// 		rewinder.AttachSyncNode(chain2ID, syncNode2)

// 		// Setup state for both chains
// 		setupChainState(chain1ID, chainsDB, syncNode1)
// 		setupChainState(chain2ID, chainsDB, syncNode2)

// 		// Rewind chain 1
// 		require.NoError(t, rewinder.handleEventRewindChain(superevents.RewindChainEvent{
// 			ChainID: chain1ID,
// 			Candidate: eth.L2BlockRef{
// 				Hash:   common.HexToHash("0xdef"),
// 				Number: 95,
// 			},
// 		}))

// 		// Rewind chain 2
// 		require.NoError(t, rewinder.handleEventRewindChain(superevents.RewindChainEvent{
// 			ChainID: chain2ID,
// 			Candidate: eth.L2BlockRef{
// 				Hash:   common.HexToHash("0xdef"),
// 				Number: 95,
// 			},
// 		}))

// 		// Verify both chains were rewound correctly
// 		require.Len(t, chainsDB.rewindLocalUnsafeCalls, 2, "should have two local-unsafe rewind calls")
// 		require.Len(t, chainsDB.resetCrossUnsafeCalls, 2, "should have two cross-unsafe reset calls")

// 		// Check chain 1 rewind
// 		require.Equal(t, chain1ID, chainsDB.rewindLocalUnsafeCalls[0].chainID)
// 		require.Equal(t, uint64(95), chainsDB.rewindLocalUnsafeCalls[0].newHeight)

// 		// Check chain 2 rewind
// 		require.Equal(t, chain2ID, chainsDB.rewindLocalUnsafeCalls[1].chainID)
// 		require.Equal(t, uint64(95), chainsDB.rewindLocalUnsafeCalls[1].newHeight)
// 	})
// }
