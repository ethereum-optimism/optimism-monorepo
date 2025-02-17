package status

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestInitialSyncStatus(t *testing.T) {
	chains := []eth.ChainID{eth.ChainIDFromUInt64(1), eth.ChainIDFromUInt64(2)}
	tracker := NewStatusTracker(chains)
	status, err := tracker.SyncStatus()
	require.NoError(t, err)
	require.Zero(t, status.MinSyncedL1)
	require.Len(t, status.Chains, 2)
}

func TestUpdateMinSyncedL1(t *testing.T) {
	chain1 := eth.ChainIDFromUInt64(1)
	chain2 := eth.ChainIDFromUInt64(2)
	chains := []eth.ChainID{chain1, chain2}
	tracker := NewStatusTracker(chains)
	minL1 := eth.BlockRef{Number: 204, Hash: common.Hash{0xaa}}
	tracker.OnEvent(superevents.LocalDerivedOriginUpdateEvent{
		ChainID: chain1,
		Origin:  minL1,
	})
	tracker.OnEvent(superevents.LocalDerivedOriginUpdateEvent{
		ChainID: chain2,
		Origin:  eth.BlockRef{Number: 228, Hash: common.Hash{0xbb}},
	})
	status, err := tracker.SyncStatus()
	require.NoError(t, err)
	require.EqualValues(t, minL1, status.MinSyncedL1)
}

func TestUpdateLocalUnsafe(t *testing.T) {
	chain1 := eth.ChainIDFromUInt64(1)
	chain2 := eth.ChainIDFromUInt64(2)
	chains := []eth.ChainID{chain1, chain2}
	tracker := NewStatusTracker(chains)
	chain1Unsafe := eth.BlockRef{Number: 204, Hash: common.Hash{0xaa}}
	chain2Unsafe := eth.BlockRef{Number: 228, Hash: common.Hash{0xbb}}
	tracker.OnEvent(superevents.LocalUnsafeUpdateEvent{
		ChainID:        chain1,
		NewLocalUnsafe: chain1Unsafe,
	})
	tracker.OnEvent(superevents.LocalUnsafeUpdateEvent{
		ChainID:        chain2,
		NewLocalUnsafe: chain2Unsafe,
	})
	status, err := tracker.SyncStatus()
	require.NoError(t, err)
	require.Equal(t, chain1Unsafe, status.Chains[chain1].LocalUnsafe)
	require.Equal(t, chain2Unsafe, status.Chains[chain2].LocalUnsafe)
}

func TestUpdateCrossSafe(t *testing.T) {
	chain1 := eth.ChainIDFromUInt64(1)
	chain2 := eth.ChainIDFromUInt64(2)
	chains := []eth.ChainID{chain1, chain2}
	tracker := NewStatusTracker(chains)
	chain1Safe := types.DerivedBlockSealPair{
		Derived: types.BlockSeal{
			Number:    204,
			Hash:      common.Hash{0xaa},
			Timestamp: 204000,
		},
	}
	chain2Safe := types.DerivedBlockSealPair{
		Derived: types.BlockSeal{
			Number:    20,
			Hash:      common.Hash{0xaa},
			Timestamp: 228000,
		},
	}
	tracker.OnEvent(superevents.CrossSafeUpdateEvent{
		ChainID:      chain1,
		NewCrossSafe: chain1Safe,
	})
	tracker.OnEvent(superevents.CrossSafeUpdateEvent{
		ChainID:      chain2,
		NewCrossSafe: chain2Safe,
	})
	status, err := tracker.SyncStatus()
	require.NoError(t, err)
	require.Equal(t, chain1Safe.Derived.Timestamp, status.SafeTimestamp)
	require.Equal(t, chain1Safe.Derived.ID(), status.Chains[chain1].Safe)
	require.Equal(t, chain2Safe.Derived.ID(), status.Chains[chain2].Safe)
}

func TestUpdateFinalized(t *testing.T) {
	chain1 := eth.ChainIDFromUInt64(1)
	chain2 := eth.ChainIDFromUInt64(2)
	chains := []eth.ChainID{chain1, chain2}
	tracker := NewStatusTracker(chains)
	chain1Finalized := types.BlockSeal{
		Number:    204,
		Hash:      common.Hash{0xaa},
		Timestamp: 204000,
	}
	chain2Finalized := types.BlockSeal{
		Number:    20,
		Hash:      common.Hash{0xaa},
		Timestamp: 228000,
	}
	tracker.OnEvent(superevents.FinalizedL2UpdateEvent{
		ChainID:     chain1,
		FinalizedL2: chain1Finalized,
	})
	tracker.OnEvent(superevents.FinalizedL2UpdateEvent{
		ChainID:     chain2,
		FinalizedL2: chain2Finalized,
	})
	status, err := tracker.SyncStatus()
	require.NoError(t, err)
	require.Equal(t, chain1Finalized.Timestamp, status.FinalizedTimestamp)
	require.Equal(t, chain1Finalized.ID(), status.Chains[chain1].Finalized)
	require.Equal(t, chain2Finalized.ID(), status.Chains[chain2].Finalized)
}
