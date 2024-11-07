package conductor

import (
	"context"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-conductor/consensus"
	"github.com/ethereum-optimism/optimism/op-service/retry"
)

// [Category: Initial Setup]
// In this test, we test that we can successfully setup a working cluster.
func TestSequencerFailover_SetupCluster(t *testing.T) {
	_, conductors, cleanup := setupSequencerFailoverTest(t)
	defer cleanup()

	require.Equal(t, 3, len(conductors), "Expected 3 conductors")
	for _, con := range conductors {
		require.NotNil(t, con, "Expected conductor to be non-nil")
	}
}

// [Category: conductor rpc]
// In this test, we test all rpcs exposed by conductor.
func TestSequencerFailover_ConductorRPC(t *testing.T) {
	ctx := context.Background()
	sys, conductors, cleanup := setupSequencerFailoverTest(t)
	defer cleanup()

	// SequencerHealthy, Leader, AddServerAsVoter are used in setup already.

	// Test ClusterMembership
	t.Log("Testing ClusterMembership")
	c1 := conductors[Sequencer1Name]
	c2 := conductors[Sequencer2Name]
	c3 := conductors[Sequencer3Name]
	membership, err := c1.client.ClusterMembership(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, len(membership.Servers), "Expected 3 members in cluster")
	ids := make([]string, 0)
	for _, member := range membership.Servers {
		ids = append(ids, member.ID)
		require.Equal(t, consensus.Voter, member.Suffrage, "Expected all members to be voters")
	}
	sort.Strings(ids)
	require.Equal(t, []string{Sequencer1Name, Sequencer2Name, Sequencer3Name}, ids, "Expected all sequencers to be in cluster")

	// Test Active & Pause & Resume & Stop
	t.Log("Testing Active & Pause & Resume")
	active, err := c1.client.Active(ctx)
	require.NoError(t, err)
	require.True(t, active, "Expected conductor to be active")

	err = c1.client.Pause(ctx)
	require.NoError(t, err)
	active, err = c1.client.Active(ctx)
	require.NoError(t, err)
	require.False(t, active, "Expected conductor to be paused")

	err = c1.client.Resume(ctx)
	require.NoError(t, err)
	active, err = c1.client.Active(ctx)
	require.NoError(t, err)
	require.True(t, active, "Expected conductor to be active")

	t.Log("Testing LeaderWithID")
	leader1, err := c1.client.LeaderWithID(ctx)
	require.NoError(t, err)
	leader2, err := c2.client.LeaderWithID(ctx)
	require.NoError(t, err)
	leader3, err := c3.client.LeaderWithID(ctx)
	require.NoError(t, err)
	require.Equal(t, leader1.ID, leader2.ID, "Expected leader ID to be the same")
	require.Equal(t, leader1.ID, leader3.ID, "Expected leader ID to be the same")

	t.Log("Testing TransferLeader")
	lid, leader := findLeader(t, conductors)
	err = leader.client.TransferLeader(ctx)
	require.NoError(t, err, "Expected leader to transfer leadership to another node")
	_ = waitForLeadershipChange(t, leader, lid, conductors, sys)

	// old leader now became follower, we're trying to transfer leadership directly back to it.
	t.Log("Testing TransferLeaderToServer")
	fid, follower := lid, leader
	lid, leader = findLeader(t, conductors)
	err = leader.client.TransferLeaderToServer(ctx, fid, follower.ConsensusEndpoint())
	require.NoError(t, err, "Expected leader to transfer leadership to follower")
	newID := waitForLeadershipChange(t, leader, lid, conductors, sys)
	require.Equal(t, fid, newID, "Expected leader to transfer to %s", fid)

	leader = follower

	// Test AddServerAsNonvoter, do not start a new sequencer just for this purpose, use Sequencer3's rpc to start conductor.
	// This is fine as this mainly tests conductor's ability to add itself into the raft consensus cluster as a nonvoter.
	t.Log("Testing AddServerAsNonvoter")
	nonvoter, err := retry.Do[*conductor](ctx, maxSetupRetries, retryStrategy, func() (*conductor, error) {
		return setupConductor(
			t, VerifierName, t.TempDir(),
			sys.RollupEndpoint(Sequencer3Name).RPC(),
			sys.NodeEndpoint(Sequencer3Name).RPC(),
			false,
			*sys.RollupConfig,
		)
	})
	require.NoError(t, err)
	defer func() {
		err = nonvoter.service.Stop(ctx)
		require.NoError(t, err)
	}()

	membership, err = leader.client.ClusterMembership(ctx)
	require.NoError(t, err)

	err = leader.client.AddServerAsNonvoter(ctx, VerifierName, nonvoter.ConsensusEndpoint(), membership.Version-1)
	require.ErrorContains(t, err, "configuration changed since", "Expected leader to fail to add nonvoter due to version mismatch")
	membership, err = leader.client.ClusterMembership(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, len(membership.Servers), "Expected 3 members in cluster")

	err = leader.client.AddServerAsNonvoter(ctx, VerifierName, nonvoter.ConsensusEndpoint(), 0)
	require.NoError(t, err, "Expected leader to add non-voter")
	membership, err = leader.client.ClusterMembership(ctx)
	require.NoError(t, err)
	require.Equal(t, 4, len(membership.Servers), "Expected 4 members in cluster")
	require.Equal(t, consensus.Nonvoter, membership.Servers[3].Suffrage, "Expected last member to be non-voter")

	t.Log("Testing RemoveServer, call remove on follower, expected to fail")
	lid, leader = findLeader(t, conductors)
	fid, follower = findFollower(t, conductors)
	err = follower.client.RemoveServer(ctx, lid, membership.Version)
	require.ErrorContains(t, err, "node is not the leader", "Expected follower to fail to remove leader")
	membership, err = c1.client.ClusterMembership(ctx)
	require.NoError(t, err)
	require.Equal(t, 4, len(membership.Servers), "Expected 4 members in cluster")

	t.Log("Testing RemoveServer, call remove on leader, expect non-voter to be removed")
	err = leader.client.RemoveServer(ctx, VerifierName, membership.Version)
	require.NoError(t, err, "Expected leader to remove non-voter")
	membership, err = c1.client.ClusterMembership(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, len(membership.Servers), "Expected 2 members in cluster after removal")
	require.NotContains(t, memberIDs(membership), VerifierName, "Expected follower to be removed from cluster")

	t.Log("Testing RemoveServer, call remove on leader with incorrect version, expect voter not to be removed")
	err = leader.client.RemoveServer(ctx, fid, membership.Version-1)
	require.ErrorContains(t, err, "configuration changed since", "Expected leader to fail to remove follower due to version mismatch")
	membership, err = c1.client.ClusterMembership(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, len(membership.Servers), "Expected 3 members in cluster after failed removal")
	require.Contains(t, memberIDs(membership), fid, "Expected follower to not be removed from cluster")

	t.Log("Testing RemoveServer, call remove on leader, expect voter to be removed")
	err = leader.client.RemoveServer(ctx, fid, membership.Version)
	require.NoError(t, err, "Expected leader to remove follower")
	membership, err = c1.client.ClusterMembership(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(membership.Servers), "Expected 2 members in cluster after removal")
	require.NotContains(t, memberIDs(membership), fid, "Expected follower to be removed from cluster")

	// Testing the stop api
	t.Log("Testing Stop API")
	err = c1.client.Stop(ctx)
	require.NoError(t, err)
	_, err = c1.client.Stopped(ctx)
	require.Error(t, err, "Expected no connection to the conductor since it's stopped")
}

// [Category: Sequencer Failover]
// Test that the sequencer can successfully failover to a new sequencer once the active sequencer goes down.
func TestSequencerFailover_ActiveSequencerDown(t *testing.T) {
	sys, conductors, cleanup := setupSequencerFailoverTest(t)
	defer cleanup()

	ctx := context.Background()
	leaderId, leader := findLeader(t, conductors)
	err := sys.RollupNodes[leaderId].Stop(ctx) // Stop the current leader sequencer
	require.NoError(t, err)

	// The leadership change should occur with no errors
	newID := waitForLeadershipChange(t, leader, leaderId, conductors, sys)
	require.NotEqual(t, leaderId, newID, "Expected leader to change")

	// Confirm the new leader is different from the old leader
	newLeaderId, _ := findLeader(t, conductors)
	require.NotEqual(t, leaderId, newLeaderId, "Expected leader to change")

	// Check that the sequencer is healthy
	require.True(t, healthy(t, ctx, conductors[newLeaderId]))

	// Check if the new leader is sequencing
	active, err := sys.RollupClient(newLeaderId).SequencerActive(ctx)
	require.NoError(t, err)
	require.True(t, active, "Expected new leader to be sequencing")
}

// [Category: Disaster Recovery]
// Test that sequencer can successfully be started with the overrideLeader flag set to true.
func TestSequencerFailover_DisasterRecovery_OverrideLeader(t *testing.T) {
	sys, conductors, cleanup := setupSequencerFailoverTest(t)
	defer cleanup()

	// randomly stop 2 nodes in the cluster to simulate a disaster.
	ctx := context.Background()
	err := conductors[Sequencer1Name].service.Stop(ctx)
	require.NoError(t, err)
	err = conductors[Sequencer2Name].service.Stop(ctx)
	require.NoError(t, err)

	require.False(t, conductors[Sequencer3Name].service.Leader(ctx), "Expected sequencer to not be the leader")
	active, err := sys.RollupClient(Sequencer3Name).SequencerActive(ctx)
	require.NoError(t, err)
	require.False(t, active, "Expected sequencer to be inactive")

	// Start sequencer without the overrideLeader flag set to true, should fail
	err = sys.RollupClient(Sequencer3Name).StartSequencer(ctx, common.Hash{1, 2, 3})
	require.ErrorContains(t, err, "sequencer is not the leader, aborting", "Expected sequencer to fail to start")

	// Start sequencer with the overrideLeader flag set to true, should succeed
	err = sys.RollupClient(Sequencer3Name).OverrideLeader(ctx)
	require.NoError(t, err)
	blk, err := sys.NodeClient(Sequencer3Name).BlockByNumber(ctx, nil)
	require.NoError(t, err)
	err = sys.RollupClient(Sequencer3Name).StartSequencer(ctx, blk.Hash())
	require.NoError(t, err)

	active, err = sys.RollupClient(Sequencer3Name).SequencerActive(ctx)
	require.NoError(t, err)
	require.True(t, active, "Expected sequencer to be active")

	err = conductors[Sequencer3Name].client.OverrideLeader(ctx, true)
	require.NoError(t, err)
	leader, err := conductors[Sequencer3Name].client.Leader(ctx)
	require.NoError(t, err)
	require.True(t, leader, "Expected conductor to return leader true after override")
	overridden, err := conductors[Sequencer3Name].client.LeaderOverridden(ctx)
	require.NoError(t, err)
	require.True(t, overridden, "Expected conductor to return leader overridden true after override")

	// make sure all proxied method are working correctly.
	proxy, err := rpc.DialContext(ctx, conductors[Sequencer3Name].RPCEndpoint())
	require.NoError(t, err)
	err = proxy.CallContext(ctx, &active, "admin_sequencerActive")
	require.NoError(t, err)
	require.True(t, active, "Expected sequencer to be active")
	err = proxy.CallContext(ctx, nil, "optimism_syncStatus")
	require.NoError(t, err)
	var block map[string]any
	err = proxy.CallContext(ctx, &block, "eth_getBlockByNumber", "latest", false)
	require.NoError(t, err)
	err = proxy.CallContext(ctx, nil, "optimism_outputAtBlock", block["number"])
	require.NoError(t, err)
	err = proxy.CallContext(ctx, nil, "optimism_rollupConfig")
	require.NoError(t, err)

	err = conductors[Sequencer3Name].client.OverrideLeader(ctx, false)
	require.NoError(t, err)
	overridden, err = conductors[Sequencer3Name].client.LeaderOverridden(ctx)
	require.NoError(t, err)
	require.False(t, overridden, "Expected conductor to return leader overridden false after override")

	err = proxy.CallContext(ctx, &active, "admin_sequencerActive")
	require.ErrorContains(t, err, "refusing to proxy request to non-leader sequencer", "Expected sequencer to fail to get active status")
	err = proxy.CallContext(ctx, nil, "optimism_syncStatus")
	require.ErrorContains(t, err, "refusing to proxy request to non-leader sequencer", "Expected sequencer to fail to get sync status")
	err = proxy.CallContext(ctx, nil, "eth_getBlockByNumber", "latest", false)
	require.ErrorContains(t, err, "refusing to proxy request to non-leader sequencer", "Expected sequencer to fail to get block by number")
	err = proxy.CallContext(ctx, nil, "optimism_outputAtBlock", block["number"])
	require.ErrorContains(t, err, "refusing to proxy request to non-leader sequencer", "Expected sequencer to fail to get output at block")
	err = proxy.CallContext(ctx, nil, "optimism_rollupConfig")
	require.ErrorContains(t, err, "refusing to proxy request to non-leader sequencer", "Expected sequencer to fail to get rollup config")
}
