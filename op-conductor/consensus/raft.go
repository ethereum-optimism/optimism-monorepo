package consensus

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/pkg/errors"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

const defaultTimeout = 5 * time.Second

var _ Consensus = (*RaftConsensus)(nil)

// RaftConsensus implements Consensus using raft protocol.
type RaftConsensus struct {
	log       log.Logger
	rollupCfg *rollup.Config

	serverID raft.ServerID
	r        *raft.Raft

	transport *raft.NetworkTransport
	// advertisedAddr is the host & port to contact this server.
	// If empty, the address of the transport should be used instead.
	advertisedAddr string

	unsafeTracker *unsafeHeadTracker
}

type RaftConsensusConfig struct {
	ServerID string

	// AdvertisedAddr is the address to advertise,
	// i.e. the address external raft peers use to contact us.
	// If left empty, it defaults to the resulting
	// local address that we bind the underlying transport to.
	AdvertisedAddr raft.ServerAddress

	// ListenPort is the port to bind the server to.
	// This may be 0, an available port will then be selected by the system.
	ListenPort int
	// ListenAddr is the address to bind the server to.
	// E.g. use 0.0.0.0 to bind to an external-facing network.
	ListenAddr string

	StorageDir        string
	Bootstrap         bool
	RollupCfg         *rollup.Config
	SnapshotInterval  time.Duration
	SnapshotThreshold uint64
	TrailingLogs      uint64
}

// checkTCPPortOpen attempts to connect to the specified address and returns an error if the connection fails.
func checkTCPPortOpen(address string) error {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

// NewRaftConsensus creates a new RaftConsensus instance.
func NewRaftConsensus(log log.Logger, cfg *RaftConsensusConfig) (*RaftConsensus, error) {
	rc := raft.DefaultConfig()
	rc.SnapshotInterval = cfg.SnapshotInterval
	rc.TrailingLogs = cfg.TrailingLogs
	rc.SnapshotThreshold = cfg.SnapshotThreshold
	rc.LocalID = raft.ServerID(cfg.ServerID)

	baseDir := filepath.Join(cfg.StorageDir, cfg.ServerID)
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			return nil, fmt.Errorf("error creating storage dir: %w", err)
		}
	}

	var err error
	logStorePath := filepath.Join(baseDir, "raft-log.db")
	logStore, err := boltdb.NewBoltStore(logStorePath)
	if err != nil {
		return nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %w`, logStorePath, err)
	}

	stableStorePath := filepath.Join(baseDir, "raft-stable.db")
	stableStore, err := boltdb.NewBoltStore(stableStorePath)
	if err != nil {
		return nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %w`, stableStorePath, err)
	}

	snapshotStore, err := raft.NewFileSnapshotStoreWithLogger(baseDir, 1, rc.Logger)
	if err != nil {
		return nil, fmt.Errorf(`raft.NewFileSnapshotStore(%q): %w`, baseDir, err)
	}

	var advertiseAddr net.Addr
	if cfg.AdvertisedAddr == "" {
		log.Warn("No advertised address specified. Advertising local address.")
	} else {
		x, err := net.ResolveTCPAddr("tcp", string(cfg.AdvertisedAddr))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve advertised TCP address %q: %w", string(cfg.AdvertisedAddr), err)
		}
		advertiseAddr = x
		log.Info("Resolved advertising address", "adAddr", cfg.AdvertisedAddr,
			"adIP", x.IP, "adPort", x.Port, "adZone", x.Zone)
	}

	bindAddr := fmt.Sprintf("%s:%d", cfg.ListenAddr, cfg.ListenPort)
	log.Info("Binding raft server to network transport", "listenAddr", bindAddr)

	maxConnPool := 10
	timeout := 5 * time.Second

	// When advertiseAddr == nil, the transport will use the local address that it is bound to.
	transport, err := raft.NewTCPTransportWithLogger(bindAddr, advertiseAddr, maxConnPool, timeout, rc.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create raft tcp transport")
	}
	log.Info("Raft server network transport is up", "addr", transport.LocalAddr())

	fsm := NewUnsafeHeadTracker(log)

	r, err := raft.NewRaft(rc, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		log.Error("failed to create raft", "err", err)
		return nil, errors.Wrap(err, "failed to create raft")
	}

	// If bootstrap = true, start raft in bootstrap mode, this will allow the current node to elect itself as leader when there's no other participants
	// and allow other nodes to join the cluster.
	if cfg.Bootstrap {
		var advertisedAddr raft.ServerAddress
		if cfg.AdvertisedAddr == "" {
			advertisedAddr = transport.LocalAddr()
		} else {
			advertisedAddr = cfg.AdvertisedAddr
		}
		log.Info("Bootstrapping raft consensus cluster with self", "addr", advertisedAddr)

		raftCfg := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:       rc.LocalID,
					Address:  advertisedAddr,
					Suffrage: raft.Voter,
				},
			},
		}

		f := r.BootstrapCluster(raftCfg)
		if err := f.Error(); err != nil {
			return nil, errors.Wrap(err, "failed to bootstrap raft cluster")
		}
	}

	return &RaftConsensus{
		log:           log,
		r:             r,
		serverID:      raft.ServerID(cfg.ServerID),
		unsafeTracker: fsm,
		rollupCfg:     cfg.RollupCfg,
		transport:     transport,
	}, nil
}

// Addr returns the address to contact this raft consensus server.
// If no explicit address to advertise was configured,
// the local network address that the raft-consensus server is listening on will be used.
func (rc *RaftConsensus) Addr() string {
	if rc.advertisedAddr != "" {
		return rc.advertisedAddr
	}
	return string(rc.transport.LocalAddr())
}

// AddNonVoter implements Consensus, it tries to add a non-voting member into the cluster.
func (rc *RaftConsensus) AddNonVoter(id string, addr string, version uint64) error {
	if err := checkTCPPortOpen(addr); err != nil {
		rc.log.Error("connection test to member addr failed", "id", id, "addr", addr, "err", err)
		return err
	}
	if err := rc.r.AddNonvoter(raft.ServerID(id), raft.ServerAddress(addr), version, defaultTimeout).Error(); err != nil {
		rc.log.Error("failed to add non-voter", "id", id, "addr", addr, "version", version, "err", err)
		return err
	}
	return nil
}

// AddVoter implements Consensus, it tries to add a voting member into the cluster.
func (rc *RaftConsensus) AddVoter(id string, addr string, version uint64) error {
	if err := checkTCPPortOpen(addr); err != nil {
		rc.log.Error("connection test to member addr failed", "id", id, "addr", addr, "err", err)
		return err
	}
	if err := rc.r.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), version, defaultTimeout).Error(); err != nil {
		rc.log.Error("failed to add voter", "id", id, "addr", addr, "version", version, "err", err)
		return err
	}
	return nil
}

// DemoteVoter implements Consensus, it tries to demote a voting member into a non-voting member in the cluster.
func (rc *RaftConsensus) DemoteVoter(id string, version uint64) error {
	if err := rc.r.DemoteVoter(raft.ServerID(id), version, defaultTimeout).Error(); err != nil {
		rc.log.Error("failed to demote voter", "id", id, "version", version, "err", err)
		return err
	}
	return nil
}

// RemoveServer implements Consensus, it tries to remove a member (both voter or non-voter) from the cluster, if leader is being removed, it will cause a new leader election.
func (rc *RaftConsensus) RemoveServer(id string, version uint64) error {
	if err := rc.r.RemoveServer(raft.ServerID(id), version, defaultTimeout).Error(); err != nil {
		rc.log.Error("failed to remove voter", "id", id, "version", version, "err", err)
		return err
	}
	return nil
}

// Leader implements Consensus, it returns true if it is the leader of the cluster.
func (rc *RaftConsensus) Leader() bool {
	return rc.r.State() == raft.Leader
}

// LeaderWithID implements Consensus, it returns the leader's server ID and address.
func (rc *RaftConsensus) LeaderWithID() *ServerInfo {
	addr, id := rc.r.LeaderWithID()
	return &ServerInfo{
		ID:       string(id),
		Addr:     string(addr),
		Suffrage: Voter, // leader will always be Voter
	}
}

// LeaderCh implements Consensus, it returns a channel that will be notified when leadership status changes (true = leader, false = follower).
func (rc *RaftConsensus) LeaderCh() <-chan bool {
	return rc.r.LeaderCh()
}

// ServerID implements Consensus, it returns the server ID of the current server.
func (rc *RaftConsensus) ServerID() string {
	return string(rc.serverID)
}

// TransferLeader implements Consensus, it triggers leadership transfer to another member in the cluster.
func (rc *RaftConsensus) TransferLeader() error {
	if err := rc.r.LeadershipTransfer().Error(); err != nil {
		// Expected error if not leader
		if errors.Is(err, raft.ErrNotLeader) {
			return nil
		}

		rc.log.Error("failed to transfer leadership", "err", err)
		return err
	}
	return nil
}

// TransferLeaderTo implements Consensus, it triggers leadership transfer to a specific member in the cluster.
func (rc *RaftConsensus) TransferLeaderTo(id string, addr string) error {
	if err := rc.r.LeadershipTransferToServer(raft.ServerID(id), raft.ServerAddress(addr)).Error(); err != nil {
		rc.log.Error("failed to transfer leadership to server", "id", id, "addr", addr, "err", err)
		return err
	}
	return nil
}

// Shutdown implements Consensus, it shuts down the consensus protocol client.
func (rc *RaftConsensus) Shutdown() error {
	if err := rc.r.Shutdown().Error(); err != nil {
		rc.log.Error("failed to shutdown raft", "err", err)
		return err
	}
	return nil
}

// CommitUnsafePayload implements Consensus, it commits latest unsafe payload to the cluster FSM in a strongly consistent fashion.
func (rc *RaftConsensus) CommitUnsafePayload(payload *eth.ExecutionPayloadEnvelope) error {
	rc.log.Debug("committing unsafe payload", "number", uint64(payload.ExecutionPayload.BlockNumber), "hash", payload.ExecutionPayload.BlockHash.Hex())

	var buf bytes.Buffer
	if _, err := payload.MarshalSSZ(&buf); err != nil {
		return errors.Wrap(err, "failed to marshal payload envelope")
	}

	f := rc.r.Apply(buf.Bytes(), defaultTimeout)
	if err := f.Error(); err != nil {
		return errors.Wrap(err, "failed to apply payload envelope")
	}
	rc.log.Debug("unsafe payload committed", "number", uint64(payload.ExecutionPayload.BlockNumber), "hash", payload.ExecutionPayload.BlockHash.Hex())

	return nil
}

// LatestUnsafePayload implements Consensus, it returns the latest unsafe payload from FSM in a strongly consistent fashion.
func (rc *RaftConsensus) LatestUnsafePayload() (*eth.ExecutionPayloadEnvelope, error) {
	if err := rc.r.Barrier(defaultTimeout).Error(); err != nil {
		return nil, errors.Wrap(err, "failed to apply barrier")
	}

	return rc.unsafeTracker.UnsafeHead(), nil
}

// ClusterMembership implements Consensus, it returns the current cluster membership configuration.
func (rc *RaftConsensus) ClusterMembership() (*ClusterMembership, error) {
	var future raft.ConfigurationFuture
	if future = rc.r.GetConfiguration(); future.Error() != nil {
		return nil, future.Error()
	}

	var servers []ServerInfo
	for _, srv := range future.Configuration().Servers {
		servers = append(servers, ServerInfo{
			ID:       string(srv.ID),
			Addr:     string(srv.Address),
			Suffrage: ServerSuffrage(srv.Suffrage),
		})
	}
	return &ClusterMembership{
		Servers: servers,
		Version: future.Index(),
	}, nil
}
