package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-conductor/consensus"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type conductor interface {
	OverrideLeader(override bool)
	LeaderOverridden() bool
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
	Stop(ctx context.Context) error
	Paused() bool
	Stopped() bool
	SequencerHealthy(ctx context.Context) bool

	Leader(ctx context.Context) bool
	LeaderWithID(ctx context.Context) *consensus.ServerInfo
	AddServerAsVoter(ctx context.Context, id string, addr string, version uint64) error
	AddServerAsNonvoter(ctx context.Context, id string, addr string, version uint64) error
	RemoveServer(ctx context.Context, id string, version uint64) error
	TransferLeader(ctx context.Context) error
	TransferLeaderToServer(ctx context.Context, id string, addr string) error
	CommitUnsafePayload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error
	ClusterMembership(ctx context.Context) (*consensus.ClusterMembership, error)
}

// APIBackend is the backend implementation of the API.
// TODO: (https://github.com/ethereum-optimism/protocol-quest/issues/45) Add metrics tracer here.
type APIBackend struct {
	log log.Logger
	con conductor
}

// NewAPIBackend creates a new APIBackend instance.
func NewAPIBackend(log log.Logger, con conductor) *APIBackend {
	return &APIBackend{
		log: log,
		con: con,
	}
}

var _ API = (*APIBackend)(nil)

// OverrideLeader implements API.
func (api *APIBackend) OverrideLeader(_ context.Context, override bool) error {
	api.con.OverrideLeader(override)
	return nil
}

// LeaderOverridden implements API.
func (api *APIBackend) LeaderOverridden(_ context.Context) (bool, error) {
	return api.con.LeaderOverridden(), nil
}

// Paused implements API.
func (api *APIBackend) Paused(ctx context.Context) (bool, error) {
	return api.con.Paused(), nil
}

// Stopped implements API.
func (api *APIBackend) Stopped(ctx context.Context) (bool, error) {
	return api.con.Stopped(), nil
}

// Active implements API.
func (api *APIBackend) Active(_ context.Context) (bool, error) {
	return !api.con.Stopped() && !api.con.Paused(), nil
}

// AddServerAsNonvoter implements API.
func (api *APIBackend) AddServerAsNonvoter(ctx context.Context, id string, addr string, version uint64) error {
	return api.con.AddServerAsNonvoter(ctx, id, addr, version)
}

// AddServerAsVoter implements API.
func (api *APIBackend) AddServerAsVoter(ctx context.Context, id string, addr string, version uint64) error {
	return api.con.AddServerAsVoter(ctx, id, addr, version)
}

// RemoveServer implements API.
func (api *APIBackend) RemoveServer(ctx context.Context, id string, version uint64) error {
	return api.con.RemoveServer(ctx, id, version)
}

// CommitUnsafePayload implements API.
func (api *APIBackend) CommitUnsafePayload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error {
	return api.con.CommitUnsafePayload(ctx, payload)
}

// Leader implements API, returns true if current conductor is leader of the cluster.
func (api *APIBackend) Leader(ctx context.Context) (bool, error) {
	return api.con.Leader(ctx), nil
}

// LeaderWithID implements API, returns the leader's server ID and address (not necessarily the current conductor).
func (api *APIBackend) LeaderWithID(ctx context.Context) (*consensus.ServerInfo, error) {
	return api.con.LeaderWithID(ctx), nil
}

// Pause implements API.
func (api *APIBackend) Pause(ctx context.Context) error {
	return api.con.Pause(ctx)
}

// Resume implements API.
func (api *APIBackend) Resume(ctx context.Context) error {
	return api.con.Resume(ctx)
}

// Stop implements API.
func (api *APIBackend) Stop(ctx context.Context) error {
	return api.con.Stop(ctx)
}

// TransferLeader implements API. With Raft implementation, a successful call does not mean that leadership transfer is complete
// It just means that leadership transfer is in progress (current leader has initiated a new leader election round and stepped down as leader)
func (api *APIBackend) TransferLeader(ctx context.Context) error {
	return api.con.TransferLeader(ctx)
}

// TransferLeaderToServer implements API. With Raft implementation, a successful call does not mean that leadership transfer is complete
// It just means that leadership transfer is in progress (current leader has initiated a new leader election round and stepped down as leader)
func (api *APIBackend) TransferLeaderToServer(ctx context.Context, id string, addr string) error {
	return api.con.TransferLeaderToServer(ctx, id, addr)
}

// SequencerHealthy implements API.
func (api *APIBackend) SequencerHealthy(ctx context.Context) (bool, error) {
	return api.con.SequencerHealthy(ctx), nil
}

// ClusterMembership implements API.
func (api *APIBackend) ClusterMembership(ctx context.Context) (*consensus.ClusterMembership, error) {
	return api.con.ClusterMembership(ctx)
}
