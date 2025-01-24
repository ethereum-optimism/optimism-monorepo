package conductor

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/raft"
	"github.com/pkg/errors"

	"github.com/ethereum-optimism/optimism/op-conductor/client"
	"github.com/ethereum-optimism/optimism/op-conductor/consensus"
	"github.com/ethereum-optimism/optimism/op-conductor/health"
	"github.com/ethereum-optimism/optimism/op-conductor/metrics"
	conductorrpc "github.com/ethereum-optimism/optimism/op-conductor/rpc"
	opp2p "github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	opclient "github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

var (
	ErrResumeTimeout      = errors.New("timeout to resume conductor")
	ErrPauseTimeout       = errors.New("timeout to pause conductor")
	ErrUnsafeHeadMismatch = errors.New("unsafe head mismatch")
	ErrNoUnsafeHead       = errors.New("no unsafe head")
)

// New creates a new OpConductor instance.
func New(ctx context.Context, cfg *Config, log log.Logger, version string) (*OpConductor, error) {
	return NewOpConductor(ctx, cfg, log, metrics.NewMetrics(), version, nil, nil, nil)
}

// NewOpConductor creates a new OpConductor instance.
func NewOpConductor(
	ctx context.Context,
	cfg *Config,
	log log.Logger,
	m metrics.Metricer,
	version string,
	ctrl client.SequencerControl,
	cons consensus.Consensus,
	hmon health.HealthMonitor,
) (*OpConductor, error) {
	if err := cfg.Check(); err != nil {
		return nil, errors.Wrap(err, "invalid config")
	}

	oc := &OpConductor{
		log:          log,
		version:      version,
		cfg:          cfg,
		metrics:      m,
		pauseCh:      make(chan struct{}),
		pauseDoneCh:  make(chan struct{}),
		resumeCh:     make(chan struct{}),
		resumeDoneCh: make(chan struct{}),
		actionCh:     make(chan struct{}, 1),
		ctrl:         ctrl,
		cons:         cons,
		hmon:         hmon,
		retryBackoff: func() time.Duration { return time.Duration(rand.Intn(2000)) * time.Millisecond },
	}
	oc.loopActionFn = oc.loopAction

	// explicitly set all atomic.Bool values
	oc.leader.Store(false)         // upon start, it should not be the leader unless specified otherwise by raft bootstrap, in that case, it'll receive a leadership update from consensus.
	oc.leaderOverride.Store(false) // default to no override.
	oc.healthy.Store(true)         // default to healthy unless reported otherwise by health monitor.
	oc.seqActive.Store(false)      // explicitly set to false by default, the real value will be reported after sequencer control initialization.
	oc.paused.Store(cfg.Paused)
	oc.stopped.Store(false)

	// do not rely on the default context, use a dedicated context for shutdown.
	oc.shutdownCtx, oc.shutdownCancel = context.WithCancel(context.Background())

	err := oc.init(ctx)
	if err != nil {
		log.Error("failed to initialize OpConductor", "err", err)
		// ensure we always close the resources if we fail to initialize the conductor.
		closeErr := oc.Stop(ctx)
		if closeErr != nil {
			err = multierror.Append(err, closeErr)
		}
		return nil, err
	}

	return oc, nil
}

func (c *OpConductor) init(ctx context.Context) error {
	c.log.Info("initializing OpConductor", "version", c.version)
	if err := c.initSequencerControl(ctx); err != nil {
		return errors.Wrap(err, "failed to initialize sequencer control")
	}
	if err := c.initConsensus(ctx); err != nil {
		return errors.Wrap(err, "failed to initialize consensus")
	}
	if err := c.initHealthMonitor(ctx); err != nil {
		return errors.Wrap(err, "failed to initialize health monitor")
	}
	if err := c.initRPCServer(ctx); err != nil {
		return errors.Wrap(err, "failed to initialize rpc server")
	}
	return nil
}

func (c *OpConductor) initSequencerControl(ctx context.Context) error {
	if c.ctrl != nil {
		return nil
	}

	ec, err := opclient.NewRPC(ctx, c.log, c.cfg.ExecutionRPC)
	if err != nil {
		return errors.Wrap(err, "failed to create geth rpc client")
	}
	execCfg := sources.L2ClientDefaultConfig(&c.cfg.RollupCfg, true)
	// TODO: Add metrics tracer here. tracked by https://github.com/ethereum-optimism/protocol-quest/issues/45
	exec, err := sources.NewEthClient(ec, c.log, nil, &execCfg.EthClientConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create geth client")
	}

	nc, err := opclient.NewRPC(ctx, c.log, c.cfg.NodeRPC)
	if err != nil {
		return errors.Wrap(err, "failed to create node rpc client")
	}
	node := sources.NewRollupClient(nc)
	c.ctrl = client.NewSequencerControl(exec, node)

	enabled, err := retry.Do(ctx, 60, retry.Fixed(5*time.Second), func() (bool, error) {
		enabled, err := c.ctrl.ConductorEnabled(ctx)
		if rpcErr, ok := err.(rpc.Error); ok {
			errCode := rpcErr.ErrorCode()
			errText := strings.ToLower(err.Error())
			if errCode == -32601 || strings.Contains(errText, "method not found") { // method not found error
				c.log.Warn("Warning: conductorEnabled method not found, please upgrade your op-node to the latest version, continuing...")
				return true, nil
			}
		}
		return enabled, err
	})
	if err != nil {
		return errors.Wrap(err, "failed to connect to sequencer")
	}
	if !enabled {
		return errors.New("conductor is not enabled on sequencer, exiting...")
	}

	return c.updateSequencerActiveStatus()
}

func (c *OpConductor) initConsensus(ctx context.Context) error {
	if c.cons != nil {
		return nil
	}

	raftConsensusConfig := &consensus.RaftConsensusConfig{
		ServerID: c.cfg.RaftServerID,
		// AdvertisedAddr may be empty: the server will then default to what it binds to.
		AdvertisedAddr:    raft.ServerAddress(c.cfg.ConsensusAdvertisedAddr),
		ListenAddr:        c.cfg.ConsensusAddr,
		ListenPort:        c.cfg.ConsensusPort,
		StorageDir:        c.cfg.RaftStorageDir,
		Bootstrap:         c.cfg.RaftBootstrap,
		RollupCfg:         &c.cfg.RollupCfg,
		SnapshotInterval:  c.cfg.RaftSnapshotInterval,
		SnapshotThreshold: c.cfg.RaftSnapshotThreshold,
		TrailingLogs:      c.cfg.RaftTrailingLogs,
	}
	cons, err := consensus.NewRaftConsensus(c.log, raftConsensusConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create raft consensus")
	}
	c.cons = cons
	c.leaderUpdateCh = c.cons.LeaderCh()
	return nil
}

func (c *OpConductor) initHealthMonitor(ctx context.Context) error {
	if c.hmon != nil {
		return nil
	}

	nc, err := opclient.NewRPC(ctx, c.log, c.cfg.NodeRPC)
	if err != nil {
		return errors.Wrap(err, "failed to create node rpc client")
	}
	node := sources.NewRollupClient(nc)

	pc, err := rpc.DialContext(ctx, c.cfg.NodeRPC)
	if err != nil {
		return errors.Wrap(err, "failed to create p2p rpc client")
	}
	p2p := opp2p.NewClient(pc)

	c.hmon = health.NewSequencerHealthMonitor(
		c.log,
		c.metrics,
		c.cfg.HealthCheck.Interval,
		c.cfg.HealthCheck.UnsafeInterval,
		c.cfg.HealthCheck.SafeInterval,
		c.cfg.HealthCheck.MinPeerCount,
		c.cfg.HealthCheck.SafeEnabled,
		&c.cfg.RollupCfg,
		node,
		p2p,
	)
	c.healthUpdateCh = c.hmon.Subscribe()

	return nil
}

func (oc *OpConductor) initRPCServer(ctx context.Context) error {
	server := oprpc.NewServer(
		oc.cfg.RPC.ListenAddr,
		oc.cfg.RPC.ListenPort,
		oc.version,
		oprpc.WithLogger(oc.log),
	)
	api := conductorrpc.NewAPIBackend(oc.log, oc)
	server.AddAPI(rpc.API{
		Namespace: conductorrpc.RPCNamespace,
		Version:   oc.version,
		Service:   api,
	})

	if oc.cfg.RPCEnableProxy {
		execClient, err := dial.DialEthClientWithTimeout(ctx, 1*time.Minute, oc.log, oc.cfg.ExecutionRPC)
		if err != nil {
			return errors.Wrap(err, "failed to create execution rpc client")
		}
		executionProxy := conductorrpc.NewExecutionProxyBackend(oc.log, oc, execClient)
		server.AddAPI(rpc.API{
			Namespace: conductorrpc.ExecutionRPCNamespace,
			Service:   executionProxy,
		})
		execMinerProxy := conductorrpc.NewExecutionMinerProxyBackend(oc.log, oc, execClient)
		server.AddAPI(rpc.API{
			Namespace: conductorrpc.ExecutionMinerRPCNamespace,
			Service:   execMinerProxy,
		})

		nodeClient, err := dial.DialRollupClientWithTimeout(ctx, 1*time.Minute, oc.log, oc.cfg.NodeRPC)
		if err != nil {
			return errors.Wrap(err, "failed to create node rpc client")
		}
		nodeProxy := conductorrpc.NewNodeProxyBackend(oc.log, oc, nodeClient)
		server.AddAPI(rpc.API{
			Namespace: conductorrpc.NodeRPCNamespace,
			Service:   nodeProxy,
		})

		nodeAdminProxy := conductorrpc.NewNodeAdminProxyBackend(oc.log, oc, nodeClient)
		server.AddAPI(rpc.API{
			Namespace: conductorrpc.NodeAdminRPCNamespace,
			Service:   nodeAdminProxy,
		})
	}

	oc.rpcServer = server
	return nil
}

// OpConductor represents a full conductor instance and its resources, it does:
//  1. performs health checks on sequencer
//  2. participate in consensus protocol for leader election
//  3. and control sequencer state based on leader, sequencer health and sequencer active status.
//
// OpConductor has three states:
//  1. running: it is running normally, which executes control loop and participates in leader election.
//  2. paused: control loop (sequencer start/stop) is paused, but it still participates in leader election, and receives health updates.
//  3. stopped: it is stopped, which means it is not participating in leader election and control loop. OpConductor cannot be started again from stopped mode.
type OpConductor struct {
	log     log.Logger
	version string
	cfg     *Config
	metrics metrics.Metricer

	ctrl client.SequencerControl
	cons consensus.Consensus
	hmon health.HealthMonitor

	leader         atomic.Bool
	leaderOverride atomic.Bool
	seqActive      atomic.Bool
	healthy        atomic.Bool
	hcerr          error // error from health check
	prevState      *state

	healthUpdateCh <-chan error
	leaderUpdateCh <-chan bool
	loopActionFn   func() // loopActionFn defines the logic to be executed inside control loop.

	wg             sync.WaitGroup
	pauseCh        chan struct{}
	pauseDoneCh    chan struct{}
	resumeCh       chan struct{}
	resumeDoneCh   chan struct{}
	actionCh       chan struct{}
	paused         atomic.Bool
	stopped        atomic.Bool
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	rpcServer     *oprpc.Server
	metricsServer *httputil.HTTPServer

	retryBackoff func() time.Duration
}

type state struct {
	leader, healthy, active bool
}

// NewState creates a new state instance.
func NewState(leader, healthy, active bool) *state {
	return &state{
		leader:  leader,
		healthy: healthy,
		active:  active,
	}
}

func (s *state) Equal(other *state) bool {
	return s.leader == other.leader && s.healthy == other.healthy && s.active == other.active
}

func (s *state) String() string {
	return fmt.Sprintf("leader: %t, healthy: %t, active: %t", s.leader, s.healthy, s.active)
}

var _ cliapp.Lifecycle = (*OpConductor)(nil)

// Start implements cliapp.Lifecycle.
func (oc *OpConductor) Start(ctx context.Context) error {
	oc.log.Info("starting OpConductor")

	if err := oc.hmon.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start health monitor")
	}

	oc.log.Info("starting JSON-RPC server")
	if err := oc.rpcServer.Start(); err != nil {
		return errors.Wrap(err, "failed to start JSON-RPC server")
	}

	if oc.cfg.MetricsConfig.Enabled {
		oc.log.Info("starting metrics server")
		m, ok := oc.metrics.(opmetrics.RegistryMetricer)
		if !ok {
			return fmt.Errorf("metrics were enabled, but metricer %T does not expose registry for metrics-server", oc.metrics)
		}
		metricsServer, err := opmetrics.StartServer(m.Registry(), oc.cfg.MetricsConfig.ListenAddr, oc.cfg.MetricsConfig.ListenPort)
		if err != nil {
			return errors.Wrap(err, "failed to start metrics server")
		}
		oc.metricsServer = metricsServer
	}

	oc.wg.Add(1)
	go oc.loop()

	oc.metrics.RecordInfo(oc.version)
	oc.metrics.RecordUp()

	oc.log.Info("OpConductor started")
	// queue an action in case sequencer is not in the desired state.
	oc.prevState = NewState(oc.leader.Load(), oc.healthy.Load(), oc.seqActive.Load())
	oc.queueAction()

	return nil
}

// Stop implements cliapp.Lifecycle.
func (oc *OpConductor) Stop(ctx context.Context) error {
	if oc.Stopped() {
		oc.log.Info("OpConductor already stopped")
		return nil
	}

	oc.log.Info("stopping OpConductor")
	var result *multierror.Error

	// close control loop
	oc.shutdownCancel()
	oc.wg.Wait()

	if oc.rpcServer != nil {
		if err := oc.rpcServer.Stop(); err != nil {
			result = multierror.Append(result, errors.Wrap(err, "failed to stop rpc server"))
		}
	}

	// stop health check
	if oc.hmon != nil {
		if err := oc.hmon.Stop(); err != nil {
			result = multierror.Append(result, errors.Wrap(err, "failed to stop health monitor"))
		}
	}

	if oc.cons != nil {
		if err := oc.cons.Shutdown(); err != nil {
			result = multierror.Append(result, errors.Wrap(err, "failed to shutdown consensus"))
		}
	}

	if oc.metricsServer != nil {
		if err := oc.metricsServer.Shutdown(ctx); err != nil {
			result = multierror.Append(result, errors.Wrap(err, "failed to stop metrics server"))
		}
	}

	if result.ErrorOrNil() != nil {
		oc.log.Error("failed to stop OpConductor", "err", result.ErrorOrNil())
		return result.ErrorOrNil()
	}

	oc.stopped.Store(true)
	oc.log.Info("OpConductor stopped")
	return nil
}

// Stopped implements cliapp.Lifecycle.
func (oc *OpConductor) Stopped() bool {
	return oc.stopped.Load()
}

// Pause pauses the control loop of OpConductor, but still allows it to participate in leader election.
func (oc *OpConductor) Pause(ctx context.Context) error {
	select {
	case oc.pauseCh <- struct{}{}:
		<-oc.pauseDoneCh
		oc.log.Info("OpConductor has been paused")
		return nil
	case <-ctx.Done():
		return ErrPauseTimeout
	}
}

// Resume resumes the control loop of OpConductor.
func (oc *OpConductor) Resume(ctx context.Context) error {
	err := oc.updateSequencerActiveStatus()
	if err != nil {
		return errors.Wrap(err, "cannot resume because failed to get sequencer active status")
	}

	select {
	case oc.resumeCh <- struct{}{}:
		<-oc.resumeDoneCh
		oc.log.Info("OpConductor has been resumed")
		return nil
	case <-ctx.Done():
		return ErrResumeTimeout
	}
}

// Paused returns true if OpConductor is paused.
func (oc *OpConductor) Paused() bool {
	return oc.paused.Load()
}

// ConsensusEndpoint returns the raft consensus server address to connect to.
func (oc *OpConductor) ConsensusEndpoint() string {
	return oc.cons.Addr()
}

// HTTPEndpoint returns the HTTP RPC endpoint
func (oc *OpConductor) HTTPEndpoint() string {
	if oc.rpcServer == nil {
		return ""
	}
	return fmt.Sprintf("http://%s", oc.rpcServer.Endpoint())
}

func (oc *OpConductor) OverrideLeader(override bool) {
	oc.leaderOverride.Store(override)
}

func (oc *OpConductor) LeaderOverridden() bool {
	return oc.leaderOverride.Load()
}

// Leader returns true if OpConductor is the leader.
func (oc *OpConductor) Leader(ctx context.Context) bool {
	return oc.LeaderOverridden() || oc.cons.Leader()
}

// LeaderWithID returns the current leader's server ID and address.
func (oc *OpConductor) LeaderWithID(ctx context.Context) *consensus.ServerInfo {
	if oc.LeaderOverridden() {
		return &consensus.ServerInfo{
			ID:       "N/A (Leader overridden)",
			Addr:     "N/A",
			Suffrage: 0,
		}
	}

	return oc.cons.LeaderWithID()
}

// AddServerAsVoter adds a server as a voter to the cluster.
func (oc *OpConductor) AddServerAsVoter(_ context.Context, id string, addr string, version uint64) error {
	return oc.cons.AddVoter(id, addr, version)
}

// AddServerAsNonvoter adds a server as a non-voter to the cluster. non-voter will not participate in leader election.
func (oc *OpConductor) AddServerAsNonvoter(_ context.Context, id string, addr string, version uint64) error {
	return oc.cons.AddNonVoter(id, addr, version)
}

// RemoveServer removes a server from the cluster.
func (oc *OpConductor) RemoveServer(_ context.Context, id string, version uint64) error {
	return oc.cons.RemoveServer(id, version)
}

// TransferLeader transfers leadership to another server.
func (oc *OpConductor) TransferLeader(_ context.Context) error {
	return oc.cons.TransferLeader()
}

// TransferLeaderToServer transfers leadership to a specific server.
func (oc *OpConductor) TransferLeaderToServer(_ context.Context, id string, addr string) error {
	return oc.cons.TransferLeaderTo(id, addr)
}

// CommitUnsafePayload commits an unsafe payload (latest head) to the cluster FSM ensuring strong consistency by leveraging Raft consensus mechanisms.
func (oc *OpConductor) CommitUnsafePayload(_ context.Context, payload *eth.ExecutionPayloadEnvelope) error {
	return oc.cons.CommitUnsafePayload(payload)
}

// SequencerHealthy returns true if sequencer is healthy.
func (oc *OpConductor) SequencerHealthy(_ context.Context) bool {
	return oc.healthy.Load()
}

// ClusterMembership returns current cluster's membership information.
func (oc *OpConductor) ClusterMembership(_ context.Context) (*consensus.ClusterMembership, error) {
	return oc.cons.ClusterMembership()
}

// LatestUnsafePayload returns the latest unsafe payload envelope from FSM in a strongly consistent fashion.
func (oc *OpConductor) LatestUnsafePayload(_ context.Context) (*eth.ExecutionPayloadEnvelope, error) {
	return oc.cons.LatestUnsafePayload()
}

func (oc *OpConductor) loop() {
	defer oc.wg.Done()

	for {
		startTime := time.Now()
		select {
		case <-oc.shutdownCtx.Done():
			return
		default:
			oc.loopActionFn()
		}
		oc.metrics.RecordLoopExecutionTime(time.Since(startTime).Seconds())
	}
}

func (oc *OpConductor) loopAction() {
	select {
	case healthy := <-oc.healthUpdateCh:
		oc.handleHealthUpdate(healthy)
	case leader := <-oc.leaderUpdateCh:
		oc.handleLeaderUpdate(leader)
	case <-oc.pauseCh:
		oc.paused.Store(true)
		oc.pauseDoneCh <- struct{}{}
	case <-oc.resumeCh:
		oc.paused.Store(false)
		oc.resumeDoneCh <- struct{}{}
		// queue an action to make sure sequencer is in the desired state after resume.
		oc.queueAction()
	case <-oc.shutdownCtx.Done():
		return
	case <-oc.actionCh:
		oc.action()
	}
}

func (oc *OpConductor) queueAction() {
	select {
	case oc.actionCh <- struct{}{}:
	default:
		// do nothing if there's an action queued already, this is fine because whenever an action is executed,
		// it is guaranteed to have the latest status and bring the sequencer to the desired state.
	}
}

// handleLeaderUpdate handles leadership update from consensus.
func (oc *OpConductor) handleLeaderUpdate(leader bool) {
	oc.log.Info("Leadership status changed", "server", oc.cons.ServerID(), "leader", leader)

	oc.leader.Store(leader)
	oc.queueAction()
}

// handleHealthUpdate handles health update from health monitor.
func (oc *OpConductor) handleHealthUpdate(hcerr error) {
	oc.log.Debug("received health update", "server", oc.cons.ServerID(), "error", hcerr)
	healthy := hcerr == nil
	if !healthy {
		oc.log.Error("Sequencer is unhealthy", "server", oc.cons.ServerID(), "err", hcerr)
		// always queue an action if it's unhealthy, it could be an no-op in the handler.
		oc.queueAction()
	}

	if old := oc.healthy.Swap(healthy); old != healthy {
		oc.log.Info("Health state changed", "old", old, "new", healthy)
		// queue an action if health status changed.
		oc.queueAction()
	}

	oc.hcerr = hcerr
}

// action tries to bring the sequencer to the desired state, a retry will be queued if any action failed.
func (oc *OpConductor) action() {
	if oc.Paused() {
		return
	}

	var err error
	status := NewState(oc.leader.Load(), oc.healthy.Load(), oc.seqActive.Load())
	oc.log.Debug("entering action with status", "status", status)

	// exhaust all cases below for completeness, 3 state, 8 cases.
	switch {
	case !status.leader && !status.healthy && !status.active:
		// if follower is not healthy and not sequencing, just log an error
		oc.log.Error("server (follower) is not healthy", "server", oc.cons.ServerID())
	case !status.leader && !status.healthy && status.active:
		// sequencer is not leader, not healthy, but it is sequencing, stop it
		err = oc.stopSequencer()
	case !status.leader && status.healthy && !status.active:
		// normal follower, do nothing
	case !status.leader && status.healthy && status.active:
		// stop sequencer, this happens when current server steps down as leader.
		err = oc.stopSequencer()
	case status.leader && !status.healthy && !status.active:
		// There are 2 scenarios we need to handle:
		// 1. current node is follower, active sequencer became unhealthy and started the leadership transfer process.
		//    however if leadership transfer took longer than the time for health monitor to treat the node as unhealthy,
		//    then basically the entire network is stalled and we need to start sequencing in this case.
		if !oc.prevState.leader && !oc.prevState.active && !errors.Is(oc.hcerr, health.ErrSequencerConnectionDown) {
			err = oc.startSequencer()
			if err != nil {
				oc.log.Error("failed to start sequencer, transferring leadership instead", "server", oc.cons.ServerID(), "err", err)
			} else {
				break
			}
		}

		// 2. for other cases, we should try to transfer leader to another node.
		//    for example, if follower became a leader and unhealthy at the same time (just unhealthy itself), then we should transfer leadership.
		err = oc.transferLeader()
	case status.leader && !status.healthy && status.active:
		// There are two scenarios we need to handle here:
		// 1. we're transitioned from case status.leader && !status.healthy && !status.active, see description above
		//    then we should continue to sequence blocks and try to bring ourselves back to healthy state.
		//    note: we need to also make sure that the health error is not due to ErrSequencerConnectionDown
		//    		because in this case, we should stop sequencing and transfer leadership to other nodes.
		if oc.prevState.leader && !oc.prevState.healthy && !oc.prevState.active && !errors.Is(oc.hcerr, health.ErrSequencerConnectionDown) {
			err = errors.New("waiting for sequencing to become healthy by itself")
			break
		}

		// 2. we're here because an healthy leader became unhealthy itself
		//    then we should try to stop sequencing locally and transfer leadership.
		var result *multierror.Error
		// Try to stop sequencer first, but since sequencer is not healthy, we may not be able to stop it.
		// In this case, it's fine to continue to try to transfer leadership to another server. This is safe because
		// 1. if leadership transfer succeeded, then we'll retry and enter case !status.leader && status.healthy && status.active, which will try to stop sequencer.
		// 2. even if the retry continues to fail and current server stays in active sequencing mode, it would be safe because our hook in op-node will prevent it from committing any new blocks to the network via p2p (if it's not leader any more)
		if e := oc.stopSequencer(); e != nil {
			result = multierror.Append(result, e)
		}
		// try to transfer leadership to another server despite if sequencer is stopped or not. There are 4 scenarios here:
		// 1. [sequencer stopped, leadership transfer succeeded] which is the happy case and we handed over sequencing to another server.
		// 2. [sequencer stopped, leadership transfer failed] we'll enter into case status.leader && !status.healthy && !status.active and retry transfer leadership.
		// 3. [sequencer active, leadership transfer succeeded] we'll enter into case !status.leader && status.healthy && status.active and retry stop sequencer.
		// 4. [sequencer active, leadership transfer failed] we're in the same state and will retry here again.
		if e := oc.transferLeader(); e != nil {
			result = multierror.Append(result, e)
		}
		err = result.ErrorOrNil()
	case status.leader && status.healthy && !status.active:
		// start sequencer
		err = oc.startSequencer()
	case status.leader && status.healthy && status.active:
		// normal leader, do nothing
	}

	oc.log.Debug("exiting action with status and error", "status", status, "err", err)
	if err != nil {
		select {
		case <-oc.shutdownCtx.Done():
		case <-time.After(oc.retryBackoff()):
			oc.log.Error("failed to execute step, queueing another one to retry", "err", err, "status", status)
			oc.queueAction()
		}
		return
	}

	if !status.Equal(oc.prevState) {
		oc.log.Info("state changed", "prev_state", oc.prevState, "new_state", status)
		oc.prevState = status
		oc.metrics.RecordStateChange(status.leader, status.healthy, status.active)
	}
}

// transferLeader tries to transfer leadership to another server.
func (oc *OpConductor) transferLeader() error {
	// TransferLeader here will do round robin to try to transfer leadership to the next healthy node.
	oc.log.Info("transferring leadership", "server", oc.cons.ServerID())
	err := oc.cons.TransferLeader()
	oc.metrics.RecordLeaderTransfer(err == nil)
	if err == nil {
		oc.leader.Store(false)
		return nil // success
	}

	switch {
	case errors.Is(err, raft.ErrNotLeader):
		// This node is not the leader, do nothing.
		oc.log.Warn("cannot transfer leadership since current server is not the leader")
		return nil
	default:
		oc.log.Error("failed to transfer leadership", "err", err)
		return err
	}
}

func (oc *OpConductor) stopSequencer() error {
	oc.log.Info(
		"stopping sequencer",
		"server", oc.cons.ServerID(),
		"leader", oc.leader.Load(),
		"healthy", oc.healthy.Load(),
		"active", oc.seqActive.Load())

	// Quoting (@zhwrd): StopSequencer is called after conductor loses leadership. In the event that
	// the StopSequencer call fails, it actually has little real consequences because the sequencer
	// cant produce a block and gossip / commit it to the raft log (requires leadership). Once
	// conductor comes back up it will check its leader and sequencer state and attempt to stop the
	// sequencer again. So it is "okay" to fail to stop a sequencer, the state will eventually be
	// rectified and we won't have two active sequencers that are actually producing blocks.
	//
	// To that end we allow to cancel the StopSequencer call if we're shutting down.
	latestHead, err := oc.ctrl.StopSequencer(oc.shutdownCtx)
	if err == nil {
		// None of the consensus state should have changed here so don't log it again.
		oc.log.Info("stopped sequencer", "latestHead", latestHead)
	} else {
		if strings.Contains(err.Error(), driver.ErrSequencerAlreadyStopped.Error()) {
			oc.log.Warn("sequencer already stopped", "err", err)
		} else {
			return errors.Wrap(err, "failed to stop sequencer")
		}
	}
	oc.metrics.RecordStopSequencer(err == nil)
	oc.seqActive.Store(false)
	return nil
}

func (oc *OpConductor) startSequencer() error {
	ctx := context.Background()

	// When starting sequencer, we need to make sure that the current node has the latest unsafe head from the consensus protocol
	// If not, then we wait for the unsafe head to catch up or gossip it to op-node manually from op-conductor.
	unsafeInCons, unsafeInNode, err := oc.compareUnsafeHead(ctx)
	// if there's a mismatch, try to post the unsafe head to op-node
	if errors.Is(err, ErrUnsafeHeadMismatch) && uint64(unsafeInCons.ExecutionPayload.BlockNumber)-unsafeInNode.NumberU64() == 1 {
		// tries to post the unsafe head to op-node when head is only 1 block behind (most likely due to gossip delay)
		oc.log.Debug(
			"posting unsafe head to op-node",
			"consensus_num", uint64(unsafeInCons.ExecutionPayload.BlockNumber),
			"consensus_hash", unsafeInCons.ExecutionPayload.BlockHash.Hex(),
			"node_num", unsafeInNode.NumberU64(),
			"node_hash", unsafeInNode.Hash().Hex(),
		)
		if err := oc.ctrl.PostUnsafePayload(ctx, unsafeInCons); err != nil {
			oc.log.Error("failed to post unsafe head payload envelope to op-node", "err", err)
			return err
		}
	} else if err != nil {
		return err
	}

	oc.log.Info("starting sequencer", "server", oc.cons.ServerID(), "leader", oc.leader.Load(), "healthy", oc.healthy.Load(), "active", oc.seqActive.Load())
	err = oc.ctrl.StartSequencer(ctx, unsafeInCons.ExecutionPayload.BlockHash)
	if err != nil {
		// cannot directly compare using Errors.Is because the error is returned from an JSON RPC server which lost its type.
		if !strings.Contains(err.Error(), driver.ErrSequencerAlreadyStarted.Error()) {
			return fmt.Errorf("failed to start sequencer: %w", err)
		} else {
			oc.log.Warn("sequencer already started.", "err", err)
		}
	}
	oc.metrics.RecordStartSequencer(err == nil)

	oc.seqActive.Store(true)
	return nil
}

func (oc *OpConductor) compareUnsafeHead(ctx context.Context) (*eth.ExecutionPayloadEnvelope, eth.BlockInfo, error) {
	unsafeInCons, err := oc.cons.LatestUnsafePayload()
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to retrieve unsafe head from consensus")
	}
	if unsafeInCons == nil {
		return nil, nil, ErrNoUnsafeHead
	}

	unsafeInNode, err := oc.ctrl.LatestUnsafeBlock(ctx)
	if err != nil {
		return unsafeInCons, nil, errors.Wrap(err, "failed to get latest unsafe block from EL during compareUnsafeHead phase")
	}

	oc.log.Debug("comparing unsafe head", "consensus", uint64(unsafeInCons.ExecutionPayload.BlockNumber), "node", unsafeInNode.NumberU64())
	if unsafeInCons.ExecutionPayload.BlockHash != unsafeInNode.Hash() {
		oc.log.Warn(
			"latest unsafe block in consensus is not the same as the one in op-node",
			"consensus_hash", unsafeInCons.ExecutionPayload.BlockHash,
			"consensus_num", uint64(unsafeInCons.ExecutionPayload.BlockNumber),
			"node_hash", unsafeInNode.Hash(),
			"node_num", unsafeInNode.NumberU64(),
		)

		return unsafeInCons, unsafeInNode, ErrUnsafeHeadMismatch
	}

	return unsafeInCons, unsafeInNode, nil
}

func (oc *OpConductor) updateSequencerActiveStatus() error {
	active, err := oc.ctrl.SequencerActive(oc.shutdownCtx)
	if err != nil {
		return errors.Wrap(err, "failed to get sequencer active status")
	}
	oc.log.Info("sequencer active status updated", "active", active)
	oc.seqActive.Store(active)
	return nil
}
