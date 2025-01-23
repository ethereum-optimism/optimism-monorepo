package invalidator

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockSyncSource struct {
	blockRefByNumber func(ctx context.Context, num uint64) (eth.BlockRef, error)
}

func (m *mockSyncSource) BlockRefByNumber(ctx context.Context, num uint64) (eth.BlockRef, error) {
	if m.blockRefByNumber == nil {
		return eth.BlockRef{}, fmt.Errorf("blockRefByNumber not set")
	}
	return m.blockRefByNumber(ctx, num)
}

// type mockMetrics struct {
// 	reorgEvents        map[string]int
// 	reorgFailures      map[string]int
// 	reorgBlocksRewound map[string]uint64
// 	handlingTimes      map[string]time.Duration
// }

// func newMockMetrics() *mockMetrics {
// 	return &mockMetrics{
// 		reorgEvents:        make(map[string]int),
// 		reorgFailures:      make(map[string]int),
// 		reorgBlocksRewound: make(map[string]uint64),
// 		handlingTimes:      make(map[string]time.Duration),
// 	}
// }

// func (m *mockMetrics) RecordReorgEvent(chainID eth.ChainID, reorgType string) {
// 	key := chainID.String() + ":" + reorgType
// 	fmt.Println("RecordReorgEvent", key)
// 	m.reorgEvents[key]++
// }

// func (m *mockMetrics) RecordReorgHandlingTime(chainID eth.ChainID, reorgType string, duration time.Duration) {
// 	key := chainID.String() + ":" + reorgType
// 	m.handlingTimes[key] = duration
// }

// func (m *mockMetrics) RecordReorgFailure(chainID eth.ChainID, reorgType string, err error) {
// 	key := chainID.String() + ":" + reorgType
// 	m.reorgFailures[key]++
// }

// func (m *mockMetrics) RecordReorgBlocksRewound(chainID eth.ChainID, reorgType string, blocks uint64) {
// 	key := chainID.String() + ":" + reorgType
// 	m.reorgBlocksRewound[key] = blocks
// }

type mockDatabaseRewinder struct {
	rewindCalls []struct {
		chain     eth.ChainID
		headBlock eth.BlockID
	}
	rewindErr error
}

func (m *mockDatabaseRewinder) Rewind(chain eth.ChainID, headBlock eth.BlockID) error {
	m.rewindCalls = append(m.rewindCalls, struct {
		chain     eth.ChainID
		headBlock eth.BlockID
	}{chain: chain, headBlock: headBlock})
	return m.rewindErr
}

func (m *mockDatabaseRewinder) LatestBlockNum(chain eth.ChainID) (num uint64, ok bool) {
	return 0, true
}

type mockChainResetter struct {
	resetCalls []eth.L2BlockRef
	resetErr   error
}

func (m *mockChainResetter) ResetToBlock(_ context.Context, ref eth.L2BlockRef) error {
	m.resetCalls = append(m.resetCalls, ref)
	return m.resetErr
}

type mockEmitter struct {
	localUnsafeEvents []superevents.InvalidateLocalUnsafeEvent
	crossUnsafeEvents []superevents.InvalidateCrossUnsafeEvent
}

func (m *mockEmitter) Emit(ev event.Event) {
	switch x := ev.(type) {
	case superevents.InvalidateLocalUnsafeEvent:
		m.localUnsafeEvents = append(m.localUnsafeEvents, x)
	case superevents.InvalidateCrossUnsafeEvent:
		m.crossUnsafeEvents = append(m.crossUnsafeEvents, x)
	}
}

// func TestInvalidator_HandleInvalidation(t *testing.T) {
// 	t.Run("handles local unsafe reorg", func(t *testing.T) {
// 		rewinder := &mockDatabaseRewinder{}
// 		resetter := &mockChainResetter{}
// 		emitter := &mockEmitter{}
// 		metrics := newMockMetrics()
// 		// syncSource := &mockSyncSource{}

// 		invalidator := New(log.New(), rewinder)
// 		invalidator.AttachEmitter(emitter)
// 		invalidator.RegisterChain(eth.ChainID{}, resetter)

// 		badRef := eth.BlockRef{
// 			Hash:       common.Hash{0x01},
// 			ParentHash: common.Hash{0x02},
// 			Number:     100,
// 		}
// 		ev := superevents.InvalidationEvent{
// 			ChainID: eth.ChainID{},
// 			Type:    superevents.InvalidationTypeLocalUnsafe,
// 			BadRef:  badRef,
// 		}

// 		invalidator.handleInvalidation(ev)

// 		require.Len(t, rewinder.rewindCalls, 1)
// 		require.Equal(t, eth.ChainID{}, rewinder.rewindCalls[0].chain)
// 		require.Equal(t, badRef.ID(), rewinder.rewindCalls[0].headBlock)

// 		require.Len(t, resetter.resetCalls, 1)
// 		expectedGoodRef := eth.L2BlockRef{
// 			Hash:           badRef.ParentHash,
// 			Number:         badRef.Number - 1,
// 			ParentHash:     common.Hash{},
// 			Time:           0,
// 			L1Origin:       eth.BlockID{},
// 			SequenceNumber: 0,
// 		}
// 		require.Equal(t, expectedGoodRef, resetter.resetCalls[0])

// 		// Verify metrics
// 		key := eth.ChainID{}.String() + ":unsafe"
// 		require.Equal(t, 1, metrics.reorgEvents[key])
// 		require.NotZero(t, metrics.handlingTimes[key])
// 		require.Zero(t, metrics.reorgFailures[key])
// 	})

// 	t.Run("handles safe reorg", func(t *testing.T) {
// 		rewinder := &mockDatabaseRewinder{}
// 		resetter1 := &mockChainResetter{}
// 		resetter2 := &mockChainResetter{}
// 		emitter := &mockEmitter{}
// 		metrics := newMockMetrics()

// 		invalidator := New(log.New(), rewinder)
// 		invalidator.AttachEmitter(emitter)

// 		chain1 := eth.ChainID{1}
// 		chain2 := eth.ChainID{2}
// 		invalidator.RegisterChain(chain1, resetter1)
// 		invalidator.RegisterChain(chain2, resetter2)

// 		badRef := eth.BlockRef{
// 			Hash:       common.Hash{0x01},
// 			ParentHash: common.Hash{0x02},
// 			Number:     100,
// 		}
// 		ev := superevents.InvalidationEvent{
// 			ChainID: chain1,
// 			Type:    superevents.InvalidationTypeCrossSafe,
// 			BadRef:  badRef,
// 		}

// 		invalidator.handleInvalidation(ev)

// 		require.Len(t, rewinder.rewindCalls, 1)
// 		require.Equal(t, chain1, rewinder.rewindCalls[0].chain)
// 		require.Equal(t, badRef.ID(), rewinder.rewindCalls[0].headBlock)

// 		expectedGoodRef := eth.L2BlockRef{
// 			Hash:           badRef.ParentHash,
// 			Number:         badRef.Number - 1,
// 			ParentHash:     common.Hash{},
// 			Time:           0,
// 			L1Origin:       eth.BlockID{},
// 			SequenceNumber: 0,
// 		}

// 		// Both chains should be reset
// 		require.Len(t, resetter1.resetCalls, 1)
// 		require.Equal(t, expectedGoodRef, resetter1.resetCalls[0])
// 		require.Len(t, resetter2.resetCalls, 1)
// 		require.Equal(t, expectedGoodRef, resetter2.resetCalls[0])

// 		// Verify metrics
// 		key := chain1.String() + ":safe"
// 		require.Equal(t, 1, metrics.reorgEvents[key])
// 		require.NotZero(t, metrics.handlingTimes[key])
// 		require.Zero(t, metrics.reorgFailures[key])
// 	})

// 	t.Run("handles rewind error", func(t *testing.T) {
// 		rewinder := &mockDatabaseRewinder{
// 			rewindErr: errors.New("rewind failed"),
// 		}
// 		resetter := &mockChainResetter{}
// 		emitter := &mockEmitter{}
// 		metrics := newMockMetrics()

// 		invalidator := New(log.New(), rewinder)
// 		invalidator.AttachEmitter(emitter)
// 		invalidator.RegisterChain(eth.ChainID{}, resetter)

// 		badRef := eth.BlockRef{
// 			Hash:       common.Hash{0x01},
// 			ParentHash: common.Hash{0x02},
// 			Number:     100,
// 		}
// 		ev := superevents.InvalidationEvent{
// 			ChainID: eth.ChainID{},
// 			Type:    superevents.InvalidationTypeLocalUnsafe,
// 			BadRef:  badRef,
// 		}

// 		invalidator.handleInvalidation(ev)

// 		require.Len(t, rewinder.rewindCalls, 1)
// 		require.Len(t, resetter.resetCalls, 0) // Should not reset if rewind fails

// 		// Verify metrics
// 		key := eth.ChainID{}.String() + ":unsafe"
// 		require.Equal(t, 1, metrics.reorgEvents[key])
// 		require.Equal(t, 1, metrics.reorgFailures[key])
// 	})
// }

// func TestInvalidator_HandleEdgeCases(t *testing.T) {
// 	t.Run("handles missing chain resetter", func(t *testing.T) {
// 		rewinder := &mockDatabaseRewinder{}
// 		emitter := &mockEmitter{}
// 		metrics := newMockMetrics()

// 		invalidator := New(log.New(), rewinder)
// 		invalidator.AttachEmitter(emitter)

// 		badRef := eth.BlockRef{
// 			Hash:       common.Hash{0x01},
// 			ParentHash: common.Hash{0x02},
// 			Number:     100,
// 		}
// 		ev := superevents.InvalidationEvent{
// 			ChainID: eth.ChainID{1}, // Chain not registered
// 			Type:    superevents.InvalidationTypeLocalUnsafe,
// 			BadRef:  badRef,
// 		}

// 		invalidator.handleInvalidation(ev)

// 		require.Len(t, rewinder.rewindCalls, 1) // Should still rewind DB
// 		require.Equal(t, eth.ChainID{1}, rewinder.rewindCalls[0].chain)
// 		require.Equal(t, badRef.ID(), rewinder.rewindCalls[0].headBlock)

// 		// Verify metrics
// 		key := eth.ChainID{1}.String() + ":unsafe"
// 		require.Equal(t, 1, metrics.reorgEvents[key])
// 		require.NotZero(t, metrics.handlingTimes[key])
// 		require.Zero(t, metrics.reorgFailures[key])
// 	})

// 	t.Run("handles multiple chains with different reorg types", func(t *testing.T) {
// 		rewinder := &mockDatabaseRewinder{}
// 		resetter1 := &mockChainResetter{}
// 		resetter2 := &mockChainResetter{}
// 		emitter := &mockEmitter{}
// 		metrics := newMockMetrics()

// 		invalidator := New(log.New(), rewinder)
// 		invalidator.AttachEmitter(emitter)

// 		chain1 := eth.ChainID{1}
// 		chain2 := eth.ChainID{2}
// 		invalidator.RegisterChain(chain1, resetter1)
// 		invalidator.RegisterChain(chain2, resetter2)

// 		// Test unsafe reorg on chain1
// 		badRef1 := eth.BlockRef{
// 			Hash:       common.Hash{0x01},
// 			ParentHash: common.Hash{0x02},
// 			Number:     100,
// 		}
// 		ev1 := superevents.InvalidationEvent{
// 			ChainID: chain1,
// 			Type:    superevents.InvalidationTypeLocalUnsafe,
// 			BadRef:  badRef1,
// 		}
// 		invalidator.handleInvalidation(ev1)

// 		// Test cross unsafe reorg on chain2
// 		badRef2 := eth.BlockRef{
// 			Hash:       common.Hash{0x03},
// 			ParentHash: common.Hash{0x04},
// 			Number:     200,
// 		}
// 		ev2 := superevents.InvalidationEvent{
// 			ChainID: chain2,
// 			Type:    superevents.InvalidationTypeCrossUnsafe,
// 			BadRef:  badRef2,
// 		}
// 		invalidator.handleInvalidation(ev2)

// 		// Verify chain1 rewind and reset
// 		require.Len(t, rewinder.rewindCalls, 2)
// 		require.Equal(t, chain1, rewinder.rewindCalls[0].chain)
// 		require.Equal(t, badRef1.ID(), rewinder.rewindCalls[0].headBlock)
// 		require.Len(t, resetter1.resetCalls, 1)

// 		// Verify chain2 rewind and reset
// 		require.Equal(t, chain2, rewinder.rewindCalls[1].chain)
// 		require.Equal(t, badRef2.ID(), rewinder.rewindCalls[1].headBlock)
// 		require.Len(t, resetter2.resetCalls, 1)

// 		// Verify metrics
// 		key1 := chain1.String() + ":unsafe"
// 		require.Equal(t, 1, metrics.reorgEvents[key1])
// 		require.NotZero(t, metrics.handlingTimes[key1])
// 		require.Zero(t, metrics.reorgFailures[key1])

// 		key2 := chain2.String() + ":cross_unsafe"
// 		require.Equal(t, 1, metrics.reorgEvents[key2])
// 		require.NotZero(t, metrics.handlingTimes[key2])
// 		require.Zero(t, metrics.reorgFailures[key2])
// 	})

// 	t.Run("handles concurrent safe reorg", func(t *testing.T) {
// 		rewinder := &mockDatabaseRewinder{}
// 		resetter1 := &mockChainResetter{}
// 		resetter2 := &mockChainResetter{}
// 		emitter := &mockEmitter{}
// 		metrics := newMockMetrics()

// 		invalidator := New(log.New(), rewinder)
// 		invalidator.AttachEmitter(emitter)

// 		chain1 := eth.ChainID{1}
// 		chain2 := eth.ChainID{2}
// 		invalidator.RegisterChain(chain1, resetter1)
// 		invalidator.RegisterChain(chain2, resetter2)

// 		// Trigger safe reorg
// 		badRef := eth.BlockRef{
// 			Hash:       common.Hash{0x01},
// 			ParentHash: common.Hash{0x02},
// 			Number:     100,
// 		}
// 		ev := superevents.InvalidationEvent{
// 			ChainID: chain1,
// 			Type:    superevents.InvalidationTypeCrossSafe,
// 			BadRef:  badRef,
// 		}
// 		invalidator.handleInvalidation(ev)

// 		// Verify both chains were reset
// 		require.Len(t, rewinder.rewindCalls, 1)
// 		require.Equal(t, chain1, rewinder.rewindCalls[0].chain)
// 		require.Equal(t, badRef.ID(), rewinder.rewindCalls[0].headBlock)

// 		require.Len(t, resetter1.resetCalls, 1)
// 		require.Len(t, resetter2.resetCalls, 1)

// 		// Both chains should be reset to the same block
// 		expectedGoodRef := eth.L2BlockRef{
// 			Hash:           badRef.ParentHash,
// 			Number:         badRef.Number - 1,
// 			ParentHash:     common.Hash{},
// 			Time:           0,
// 			L1Origin:       eth.BlockID{},
// 			SequenceNumber: 0,
// 		}
// 		require.Equal(t, expectedGoodRef, resetter1.resetCalls[0])
// 		require.Equal(t, expectedGoodRef, resetter2.resetCalls[0])

// 		// Verify metrics
// 		key := chain1.String() + ":safe"
// 		require.Equal(t, 1, metrics.reorgEvents[key])
// 		require.NotZero(t, metrics.handlingTimes[key])
// 		require.Zero(t, metrics.reorgFailures[key])
// 	})
// }

type mockChainsDB struct {
	invalidateLocalUnsafeCalls []struct {
		chainID   eth.ChainID
		candidate eth.L2BlockRef
	}
	invalidateCrossUnsafeCalls []struct {
		chainID   eth.ChainID
		candidate eth.L2BlockRef
	}
}

// Verify mockChainsDB implements db
var _ db = (*mockChainsDB)(nil)

func (m *mockChainsDB) InvalidateLocalUnsafe(chainID eth.ChainID, candidate eth.L2BlockRef) error {
	m.invalidateLocalUnsafeCalls = append(m.invalidateLocalUnsafeCalls, struct {
		chainID   eth.ChainID
		candidate eth.L2BlockRef
	}{chainID: chainID, candidate: candidate})
	return nil
}

func (m *mockChainsDB) InvalidateCrossUnsafe(chainID eth.ChainID, candidate eth.L2BlockRef) error {
	m.invalidateCrossUnsafeCalls = append(m.invalidateCrossUnsafeCalls, struct {
		chainID   eth.ChainID
		candidate eth.L2BlockRef
	}{chainID: chainID, candidate: candidate})
	return nil
}

func TestInvalidator_HandleLocalUnsafeInvalidation(t *testing.T) {
	t.Run("handles local-unsafe invalidation", func(t *testing.T) {
		// Setup mocks
		chainsDB := &mockChainsDB{}

		// Create invalidator
		chainID := eth.ChainID{1}
		invalidator := New(log.New(), chainsDB)

		// Create the block to invalidate
		badBlock := eth.L2BlockRef{
			Hash:       common.HexToHash("0x123"),
			ParentHash: common.HexToHash("0xaaa"),
			Number:     100,
			Time:       1235,
			L1Origin: eth.BlockID{
				Hash:   common.HexToHash("0xbbb"),
				Number: 50,
			},
			SequenceNumber: 10,
		}

		// Create and handle the invalidation event
		ev := superevents.InvalidateLocalUnsafeEvent{
			ChainID:   chainID,
			Candidate: badBlock,
		}
		invalidator.OnEvent(ev)

		// Verify ChainsDB was called to invalidate the block
		require.Len(t, chainsDB.invalidateLocalUnsafeCalls, 1, "should have one local-unsafe invalidation call")
		require.Equal(t, chainID, chainsDB.invalidateLocalUnsafeCalls[0].chainID)
		require.Equal(t, badBlock, chainsDB.invalidateLocalUnsafeCalls[0].candidate)
		require.Empty(t, chainsDB.invalidateCrossUnsafeCalls, "should not have any cross-unsafe invalidation calls")
	})

	t.Run("handles cross-unsafe invalidation", func(t *testing.T) {
		// Setup mocks
		chainsDB := &mockChainsDB{}

		// Create invalidator
		chainID := eth.ChainID{1}
		invalidator := New(log.New(), chainsDB)

		// Create the block to invalidate
		badBlock := eth.L2BlockRef{
			Hash:       common.HexToHash("0x123"),
			ParentHash: common.HexToHash("0xaaa"),
			Number:     100,
			Time:       1235,
			L1Origin: eth.BlockID{
				Hash:   common.HexToHash("0xbbb"),
				Number: 50,
			},
			SequenceNumber: 10,
		}

		// Create and handle the invalidation event
		ev := superevents.InvalidateCrossUnsafeEvent{
			ChainID:   chainID,
			Candidate: badBlock,
		}
		invalidator.handleCrossUnsafeInvalidation(ev)

		// Verify ChainsDB was called to invalidate the block
		require.Len(t, chainsDB.invalidateCrossUnsafeCalls, 1, "should have one cross-unsafe invalidation call")
		require.Equal(t, chainID, chainsDB.invalidateCrossUnsafeCalls[0].chainID)
		require.Equal(t, badBlock, chainsDB.invalidateCrossUnsafeCalls[0].candidate)
		require.Empty(t, chainsDB.invalidateLocalUnsafeCalls, "should not have any local-unsafe invalidation calls")
	})
}
