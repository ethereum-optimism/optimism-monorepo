package rpc

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-conductor/consensus"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var ErrNotLeader = errors.New("refusing to proxy request to non-leader sequencer")

// API defines the interface for the op-conductor API.
type API interface {
	// OverrideLeader is used to override or clear override for the leader status.
	// It does not impact the actual raft consensus leadership status. It is supposed to be used when the cluster is unhealthy
	// and the node is the only one up, to allow batcher to be able to connect to the node, so that it could download blocks from the manually started sequencer.
	// override: true => force current conductor to be treated as leader regardless of the actual leadership status in raft.
	// override: false => clear the override, return the actual leadership status in raft.
	OverrideLeader(ctx context.Context, override bool) error
	// LeaderOverridden returns true if the leader status is overridden.
	LeaderOverridden(ctx context.Context) (bool, error)
	// Pause pauses op-conductor.
	Pause(ctx context.Context) error
	// Resume resumes op-conductor.
	Resume(ctx context.Context) error
	// Stop stops op-conductor.
	Stop(ctx context.Context) error
	// Paused returns true if op-conductor is paused.
	Paused(ctx context.Context) (bool, error)
	// Stopped returns true if op-conductor is stopped.
	Stopped(ctx context.Context) (bool, error)
	// SequencerHealthy returns true if the sequencer is healthy.
	SequencerHealthy(ctx context.Context) (bool, error)

	// Consensus related APIs
	// Leader returns true if the server is the leader.
	Leader(ctx context.Context) (bool, error)
	// LeaderWithID returns the current leader's server info.
	LeaderWithID(ctx context.Context) (*consensus.ServerInfo, error)
	// AddServerAsVoter adds a server as a voter to the cluster.
	AddServerAsVoter(ctx context.Context, id string, addr string, version uint64) error
	// AddServerAsNonvoter adds a server as a non-voter to the cluster. non-voter will not participate in leader election.
	AddServerAsNonvoter(ctx context.Context, id string, addr string, version uint64) error
	// RemoveServer removes a server from the cluster.
	RemoveServer(ctx context.Context, id string, version uint64) error
	// TransferLeader transfers leadership to another server.
	TransferLeader(ctx context.Context) error
	// TransferLeaderToServer transfers leadership to a specific server.
	TransferLeaderToServer(ctx context.Context, id string, addr string) error
	// ClusterMembership returns the current cluster membership configuration.
	ClusterMembership(ctx context.Context) (*consensus.ClusterMembership, error)

	// APIs called by op-node
	// Active returns true if op-conductor is active (not paused or stopped).
	Active(ctx context.Context) (bool, error)
	// CommitUnsafePayload commits an unsafe payload (latest head) to the consensus layer.
	CommitUnsafePayload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error
}

// ExecutionProxyAPI defines the methods proxied to the execution 'eth_' rpc backend
// This should include all methods that are called by op-batcher or op-proposer
type ExecutionProxyAPI interface {
	GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error)
}

// ExecutionMinerProxyAPI defines the methods proxied to the execution 'miner_' rpc backend
// This should include all methods that are called by op-batcher or op-proposer
type ExecutionMinerProxyAPI interface {
	SetMaxDASize(ctx context.Context, maxTxSize hexutil.Big, maxBlockSize hexutil.Big) bool
}

// NodeProxyAPI defines the methods proxied to the node 'optimism_' rpc backend
// This should include all methods that are called by op-batcher or op-proposer
type NodeProxyAPI interface {
	OutputAtBlock(ctx context.Context, blockNumString string) (*eth.OutputResponse, error)
	SyncStatus(ctx context.Context) (*eth.SyncStatus, error)
	RollupConfig(ctx context.Context) (*rollup.Config, error)
}

// NodeAdminProxyAPI defines the methods proxied to the node 'admin_' rpc backend
// This should include all methods that are called by op-batcher or op-proposer
type NodeAdminProxyAPI interface {
	SequencerActive(ctx context.Context) (bool, error)
}
