package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-conductor/consensus"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var RPCNamespace = "conductor"

// APIClient provides a client for calling API methods.
type APIClient struct {
	c *rpc.Client
}

var _ API = (*APIClient)(nil)

// NewAPIClient creates a new APIClient instance.
func NewAPIClient(c *rpc.Client) *APIClient {
	return &APIClient{c: c}
}

func prefixRPC(method string) string {
	return RPCNamespace + "_" + method
}

// OverrideLeader implements API.
func (c *APIClient) OverrideLeader(ctx context.Context, override bool) error {
	return c.c.CallContext(ctx, nil, prefixRPC("overrideLeader"), override)
}

// LeaderOverridden implements API.
func (c *APIClient) LeaderOverridden(ctx context.Context) (bool, error) {
	var overridden bool
	err := c.c.CallContext(ctx, &overridden, prefixRPC("leaderOverridden"))
	return overridden, err
}

// Paused implements API.
func (c *APIClient) Paused(ctx context.Context) (bool, error) {
	var paused bool
	err := c.c.CallContext(ctx, &paused, prefixRPC("paused"))
	return paused, err
}

// Stopped implements API.
func (c *APIClient) Stopped(ctx context.Context) (bool, error) {
	var stopped bool
	err := c.c.CallContext(ctx, &stopped, prefixRPC("stopped"))
	return stopped, err
}

// Active implements API.
func (c *APIClient) Active(ctx context.Context) (bool, error) {
	var active bool
	err := c.c.CallContext(ctx, &active, prefixRPC("active"))
	return active, err
}

// AddServerAsNonvoter implements API.
func (c *APIClient) AddServerAsNonvoter(ctx context.Context, id string, addr string, version uint64) error {
	return c.c.CallContext(ctx, nil, prefixRPC("addServerAsNonvoter"), id, addr, version)
}

// AddServerAsVoter implements API.
func (c *APIClient) AddServerAsVoter(ctx context.Context, id string, addr string, version uint64) error {
	return c.c.CallContext(ctx, nil, prefixRPC("addServerAsVoter"), id, addr, version)
}

// RemoveServer implements API.
func (c *APIClient) RemoveServer(ctx context.Context, id string, version uint64) error {
	return c.c.CallContext(ctx, nil, prefixRPC("removeServer"), id, version)
}

// Close closes the underlying RPC client.
func (c *APIClient) Close() {
	c.c.Close()
}

// CommitUnsafePayload implements API.
func (c *APIClient) CommitUnsafePayload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error {
	return c.c.CallContext(ctx, nil, prefixRPC("commitUnsafePayload"), payload)
}

// Leader implements API.
func (c *APIClient) Leader(ctx context.Context) (bool, error) {
	var leader bool
	err := c.c.CallContext(ctx, &leader, prefixRPC("leader"))
	return leader, err
}

// LeaderWithID implements API.
func (c *APIClient) LeaderWithID(ctx context.Context) (*consensus.ServerInfo, error) {
	var info *consensus.ServerInfo
	err := c.c.CallContext(ctx, &info, prefixRPC("leaderWithID"))
	return info, err
}

// Pause implements API.
func (c *APIClient) Pause(ctx context.Context) error {
	return c.c.CallContext(ctx, nil, prefixRPC("pause"))
}

// Resume implements API.
func (c *APIClient) Resume(ctx context.Context) error {
	return c.c.CallContext(ctx, nil, prefixRPC("resume"))
}

// Stop implements API.
func (c *APIClient) Stop(ctx context.Context) error {
	return c.c.CallContext(ctx, nil, prefixRPC("stop"))
}

// TransferLeader implements API.
func (c *APIClient) TransferLeader(ctx context.Context) error {
	return c.c.CallContext(ctx, nil, prefixRPC("transferLeader"))
}

// TransferLeaderToServer implements API.
func (c *APIClient) TransferLeaderToServer(ctx context.Context, id string, addr string) error {
	return c.c.CallContext(ctx, nil, prefixRPC("transferLeaderToServer"), id, addr)
}

// SequencerHealthy implements API.
func (c *APIClient) SequencerHealthy(ctx context.Context) (bool, error) {
	var healthy bool
	err := c.c.CallContext(ctx, &healthy, prefixRPC("sequencerHealthy"))
	return healthy, err
}

// ClusterMembership implements API.
func (c *APIClient) ClusterMembership(ctx context.Context) (*consensus.ClusterMembership, error) {
	var clusterMembership consensus.ClusterMembership
	err := c.c.CallContext(ctx, &clusterMembership, prefixRPC("clusterMembership"))
	return &clusterMembership, err
}
