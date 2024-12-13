package conductor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"

	bss "github.com/ethereum-optimism/optimism/op-batcher/batcher"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	con "github.com/ethereum-optimism/optimism/op-conductor/conductor"
	"github.com/ethereum-optimism/optimism/op-conductor/consensus"
	conrpc "github.com/ethereum-optimism/optimism/op-conductor/rpc"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/setuputils"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	rollupNode "github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

const (
	Sequencer1Name = "sequencer1"
	Sequencer2Name = "sequencer2"
	Sequencer3Name = "sequencer3"
	VerifierName   = "verifier"

	localhost = "127.0.0.1"

	maxSetupRetries = 5
)

var retryStrategy = &retry.FixedStrategy{Dur: 50 * time.Millisecond}

type conductor struct {
	service *con.OpConductor
	client  conrpc.API
}

func (c *conductor) ConsensusEndpoint() string {
	return c.service.ConsensusEndpoint()
}

func (c *conductor) RPCEndpoint() string {
	return c.service.HTTPEndpoint()
}

func setupSequencerFailoverTest(t *testing.T) (*e2esys.System, map[string]*conductor, func()) {
	op_e2e.InitParallel(t)
	ctx := context.Background()

	sys, conductors := setupHAInfra(t, ctx)

	// form a cluster
	c1 := conductors[Sequencer1Name]
	c2 := conductors[Sequencer2Name]
	c3 := conductors[Sequencer3Name]

	require.NoError(t, waitForLeadership(t, c1))
	require.NoError(t, c1.client.AddServerAsVoter(ctx, Sequencer2Name, c2.ConsensusEndpoint(), 0))
	require.NoError(t, c1.client.AddServerAsVoter(ctx, Sequencer3Name, c3.ConsensusEndpoint(), 0))
	require.True(t, leader(t, ctx, c1))
	require.False(t, leader(t, ctx, c2))
	require.False(t, leader(t, ctx, c3))

	// start sequencing on leader
	lid, _ := findLeader(t, conductors)
	unsafeHead, err := sys.NodeClient(lid).BlockByNumber(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), unsafeHead.NumberU64())
	require.NoError(t, sys.RollupClient(lid).StartSequencer(ctx, unsafeHead.Hash()))

	// 1 batcher that listens to all 3 sequencers, in started mode.
	setupBatcher(t, sys, conductors)

	// weirdly, batcher does not submit a batch until unsafe block 9.
	// It became normal after that and submits a batch every L1 block (2s) per configuration.
	// Since our health monitor checks on safe head progression, wait for batcher to become normal before proceeding.
	_, err = wait.ForNextSafeBlock(ctx, sys.NodeClient(Sequencer1Name))
	require.NoError(t, err)
	_, err = wait.ForNextSafeBlock(ctx, sys.NodeClient(Sequencer1Name))
	require.NoError(t, err)

	// make sure conductor reports all sequencers as healthy, this means they're syncing correctly.
	require.Eventually(t, func() bool {
		return healthy(t, ctx, c1) &&
			healthy(t, ctx, c2) &&
			healthy(t, ctx, c3)
	}, 50*time.Second, 500*time.Millisecond, "Expected sequencers to become healthy")

	// unpause all conductors
	require.NoError(t, c1.client.Resume(ctx))
	require.NoError(t, c2.client.Resume(ctx))
	require.NoError(t, c3.client.Resume(ctx))

	// final check, make sure everything is in the right place
	require.True(t, conductorResumed(t, ctx, c1))
	require.True(t, conductorResumed(t, ctx, c2))
	require.True(t, conductorResumed(t, ctx, c3))
	require.False(t, conductorStopped(t, ctx, c1))
	require.False(t, conductorStopped(t, ctx, c2))
	require.False(t, conductorStopped(t, ctx, c3))
	require.True(t, conductorActive(t, ctx, c1))
	require.True(t, conductorActive(t, ctx, c2))
	require.True(t, conductorActive(t, ctx, c3))

	require.True(t, sequencerActive(t, ctx, sys.RollupClient(Sequencer1Name)))
	require.False(t, sequencerActive(t, ctx, sys.RollupClient(Sequencer2Name)))
	require.False(t, sequencerActive(t, ctx, sys.RollupClient(Sequencer3Name)))

	require.True(t, healthy(t, ctx, c1))
	require.True(t, healthy(t, ctx, c2))
	require.True(t, healthy(t, ctx, c3))

	return sys, conductors, func() {
		sys.Close()
		for _, c := range conductors {
			_ = c.service.Stop(ctx)
		}
	}
}

func setupHAInfra(t *testing.T, ctx context.Context) (*e2esys.System, map[string]*conductor) {
	startTime := time.Now()
	defer func() {
		t.Logf("setupHAInfra took %s\n", time.Since(startTime))
	}()

	conductorsReady := map[string]chan string{
		Sequencer1Name: make(chan string, 1),
		Sequencer2Name: make(chan string, 1),
		Sequencer3Name: make(chan string, 1),
	}

	// The sequencer op-node & execution engine have to be up first, to get their endpoints running.
	// The conductor is then started after, using the endpoints of op-node and execution engine.
	// The op-node, while starting, will wait for the conductor to be up and running, to get its endpoint.
	// No endpoint is reserved/hardcoded this way, this avoids CI test flakes in the setup.
	conductorEndpointFn := func(ctx context.Context, name string) (endpoint string, err error) {
		endpointCh, ok := conductorsReady[name]
		if !ok {
			return "", errors.New("conductor %s is not known")
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("failed to set up conductor timely: %w", err)
		case endpoint := <-endpointCh:
			return endpoint, nil
		}
	}

	// 3 sequencers, 1 verifier, 1 active sequencer.
	cfg := sequencerFailoverSystemConfig(t, conductorEndpointFn)

	// sys is configured to close itself on test cleanup.
	sys, err := cfg.Start(t)
	require.NoError(t, err, "must start system")

	out := make(map[string]*conductor)

	// 3 conductors that connects to 1 sequencer each.
	// initialize all conductors in paused mode
	conductorCfgs := []struct {
		name      string
		bootstrap bool
	}{
		{Sequencer1Name, true}, // one in bootstrap mode so that we can form a cluster.
		{Sequencer2Name, false},
		{Sequencer3Name, false},
	}
	for _, cfg := range conductorCfgs {
		cfg := cfg
		nodePRC := sys.RollupNodes[cfg.name].UserRPC().RPC()
		engineRPC := sys.EthInstances[cfg.name].UserRPC().RPC()

		conduc, err := setupConductor(t, cfg.name, t.TempDir(), nodePRC, engineRPC, cfg.bootstrap, *sys.RollupConfig)
		require.NoError(t, err, "failed to set up conductor %s", cfg.name)
		out[cfg.name] = conduc
		// Signal that the conductor RPC endpoint is ready
		conductorsReady[cfg.name] <- conduc.RPCEndpoint()
	}

	return sys, out
}

func setupConductor(
	t *testing.T,
	serverID, dir, nodeRPC, engineRPC string,
	bootstrap bool,
	rollupCfg rollup.Config,
) (*conductor, error) {
	cfg := con.Config{
		ConsensusAddr:           localhost,
		ConsensusPort:           0,  // let the system select a port, avoid conflicts
		ConsensusAdvertisedAddr: "", // use the local address we bind to

		RaftServerID:          serverID,
		RaftStorageDir:        dir,
		RaftBootstrap:         bootstrap,
		RaftSnapshotInterval:  120 * time.Second,
		RaftSnapshotThreshold: 8192,
		RaftTrailingLogs:      10240,
		NodeRPC:               nodeRPC,
		ExecutionRPC:          engineRPC,
		Paused:                true,
		HealthCheck: con.HealthCheckConfig{
			Interval:     1, // per test setup, l2 block time is 1s.
			MinPeerCount: 2, // per test setup, each sequencer has 2 peers
			// CI is unstable in terms of the delay between now and the head time
			// so we set the unsafe interval to 30s to avoid flakiness.
			// This is fine because there's a progression check within health monitor to check progression.
			UnsafeInterval: 30,
			SafeInterval:   30,
		},
		RollupCfg:      rollupCfg,
		RPCEnableProxy: true,
		LogConfig: oplog.CLIConfig{
			Level: log.LevelDebug,
			Color: false,
		},
		RPC: oprpc.CLIConfig{
			ListenAddr: localhost,
			ListenPort: 0, // let the system select a port
		},
	}

	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()
	service, err := con.New(ctx, &cfg, logger, "0.0.1")
	if err != nil {
		return nil, err
	}

	err = service.Start(ctx)
	if err != nil {
		return nil, err
	}

	logger.Info("Started conductor", "nodeRPC", nodeRPC, "engineRPC", engineRPC)

	rawClient, err := rpc.DialContext(ctx, service.HTTPEndpoint())
	if err != nil {
		return nil, err
	}
	t.Cleanup(rawClient.Close)
	client := conrpc.NewAPIClient(rawClient)

	return &conductor{
		service: service,
		client:  client,
	}, nil
}

func setupBatcher(t *testing.T, sys *e2esys.System, conductors map[string]*conductor) {
	// enable active sequencer follow mode.
	// in sequencer HA, all batcher / proposer requests will be proxied by conductor so that we can make sure
	// that requests are always handled by leader.
	l2EthRpc := strings.Join([]string{
		conductors[Sequencer1Name].RPCEndpoint(),
		conductors[Sequencer2Name].RPCEndpoint(),
		conductors[Sequencer3Name].RPCEndpoint(),
	}, ",")
	rollupRpc := strings.Join([]string{
		conductors[Sequencer1Name].RPCEndpoint(),
		conductors[Sequencer2Name].RPCEndpoint(),
		conductors[Sequencer3Name].RPCEndpoint(),
	}, ",")
	batcherCLIConfig := &bss.CLIConfig{
		L1EthRpc:               sys.EthInstances["l1"].UserRPC().RPC(),
		L2EthRpc:               l2EthRpc,
		RollupRpc:              rollupRpc,
		MaxPendingTransactions: 0,
		MaxChannelDuration:     1,
		MaxL1TxSize:            120_000,
		TargetNumFrames:        1,
		ApproxComprRatio:       0.4,
		SubSafetyMargin:        4,
		PollInterval:           1 * time.Second,
		TxMgrConfig:            setuputils.NewTxMgrConfig(sys.EthInstances["l1"].UserRPC(), sys.Cfg.Secrets.Batcher),
		LogConfig: oplog.CLIConfig{
			Level:  log.LevelDebug,
			Format: oplog.FormatText,
		},
		Stopped:                      false,
		BatchType:                    derive.SpanBatchType,
		DataAvailabilityType:         batcherFlags.CalldataType,
		ActiveSequencerCheckDuration: 0,
		CompressionAlgo:              derive.Zlib,
	}

	batcher, err := bss.BatcherServiceFromCLIConfig(context.Background(), "0.0.1", batcherCLIConfig, sys.Cfg.Loggers["batcher"])
	require.NoError(t, err)
	err = batcher.Start(context.Background())
	require.NoError(t, err)
	sys.BatchSubmitter = batcher
}

func sequencerFailoverSystemConfig(t *testing.T, conductorRPCEndpoints func(ctx context.Context, name string) (string, error)) e2esys.SystemConfig {
	cfg := e2esys.EcotoneSystemConfig(t, new(hexutil.Uint64))
	delete(cfg.Nodes, "sequencer")
	cfg.Nodes[Sequencer1Name] = sequencerCfg(func(ctx context.Context) (string, error) {
		return conductorRPCEndpoints(ctx, Sequencer1Name)
	})
	cfg.Nodes[Sequencer2Name] = sequencerCfg(func(ctx context.Context) (string, error) {
		return conductorRPCEndpoints(ctx, Sequencer2Name)
	})
	cfg.Nodes[Sequencer3Name] = sequencerCfg(func(ctx context.Context) (string, error) {
		return conductorRPCEndpoints(ctx, Sequencer3Name)
	})

	delete(cfg.Loggers, "sequencer")
	cfg.Loggers[Sequencer1Name] = testlog.Logger(t, log.LevelInfo).New("role", Sequencer1Name)
	cfg.Loggers[Sequencer2Name] = testlog.Logger(t, log.LevelInfo).New("role", Sequencer2Name)
	cfg.Loggers[Sequencer3Name] = testlog.Logger(t, log.LevelInfo).New("role", Sequencer3Name)

	cfg.P2PTopology = map[string][]string{
		Sequencer1Name: {Sequencer2Name, Sequencer3Name},
		Sequencer2Name: {Sequencer3Name, VerifierName},
		Sequencer3Name: {VerifierName, Sequencer1Name},
		VerifierName:   {Sequencer1Name, Sequencer2Name},
	}

	return cfg
}

func sequencerCfg(conductorRPCEndpoint rollupNode.ConductorRPCFunc) *rollupNode.Config {
	return &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   true,
			SequencerStopped:   true,
		},
		// Submitter PrivKey is set in system start for rollup nodes where sequencer = true
		RPC: rollupNode.RPCConfig{
			ListenAddr:  localhost,
			ListenPort:  0,
			EnableAdmin: true,
		},
		L1EpochPollInterval:         time.Second * 2,
		RuntimeConfigReloadInterval: time.Minute * 10,
		ConfigPersistence:           &rollupNode.DisabledConfigPersistence{},
		Sync:                        sync.Config{SyncMode: sync.CLSync},
		ConductorEnabled:            true,
		ConductorRpc:                conductorRPCEndpoint,
		ConductorRpcTimeout:         5 * time.Second,
	}
}

func waitForLeadership(t *testing.T, c *conductor) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	condition := func() (bool, error) {
		isLeader, err := c.client.Leader(ctx)
		if err != nil {
			return false, err
		}
		return isLeader, nil
	}

	return wait.For(ctx, 1*time.Second, condition)
}

func waitForLeadershipChange(t *testing.T, prev *conductor, prevID string, conductors map[string]*conductor, sys *e2esys.System) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	condition := func() (bool, error) {
		isLeader, err := prev.client.Leader(ctx)
		if err != nil {
			return false, err
		}
		return !isLeader, nil
	}

	err := wait.For(ctx, 1*time.Second, condition)
	require.NoError(t, err)

	ensureOnlyOneLeader(t, sys, conductors)
	newLeader, err := prev.client.LeaderWithID(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, newLeader.ID)
	require.NotEqual(t, prevID, newLeader.ID, "Expected a new leader")
	require.NoError(t, waitForSequencerStatusChange(t, sys.RollupClient(newLeader.ID), true))

	return newLeader.ID
}

func waitForSequencerStatusChange(t *testing.T, rollupClient *sources.RollupClient, active bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	condition := func() (bool, error) {
		isActive, err := rollupClient.SequencerActive(ctx)
		if err != nil {
			return false, err
		}
		return isActive == active, nil
	}

	return wait.For(ctx, 1*time.Second, condition)
}

func leader(t *testing.T, ctx context.Context, con *conductor) bool {
	leader, err := con.client.Leader(ctx)
	require.NoError(t, err)
	return leader
}

func healthy(t *testing.T, ctx context.Context, con *conductor) bool {
	healthy, err := con.client.SequencerHealthy(ctx)
	require.NoError(t, err)
	return healthy
}

func conductorActive(t *testing.T, ctx context.Context, con *conductor) bool {
	active, err := con.client.Active(ctx)
	require.NoError(t, err)
	return active
}

func conductorResumed(t *testing.T, ctx context.Context, con *conductor) bool {
	paused, err := con.client.Paused(ctx)
	require.NoError(t, err)
	return !paused
}

func conductorStopped(t *testing.T, ctx context.Context, con *conductor) bool {
	stopped, err := con.client.Stopped(ctx)
	require.NoError(t, err)
	return stopped
}

func sequencerActive(t *testing.T, ctx context.Context, rollupClient *sources.RollupClient) bool {
	active, err := rollupClient.SequencerActive(ctx)
	require.NoError(t, err)
	return active
}

func findLeader(t *testing.T, conductors map[string]*conductor) (string, *conductor) {
	for id, con := range conductors {
		if leader(t, context.Background(), con) {
			return id, con
		}
	}
	return "", nil
}

func findFollower(t *testing.T, conductors map[string]*conductor) (string, *conductor) {
	for id, con := range conductors {
		if !leader(t, context.Background(), con) {
			return id, con
		}
	}
	return "", nil
}

func ensureOnlyOneLeader(t *testing.T, sys *e2esys.System, conductors map[string]*conductor) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	condition := func() (bool, error) {
		leaders := 0
		for name, con := range conductors {
			leader, err := con.client.Leader(ctx)
			if err != nil {
				continue
			}
			active, err := sys.RollupClient(name).SequencerActive(ctx)
			if err != nil {
				continue
			}

			if leader && active {
				leaders++
			}
		}
		return leaders == 1, nil
	}
	require.NoError(t, wait.For(ctx, 1*time.Second, condition))
}

func memberIDs(membership *consensus.ClusterMembership) []string {
	ids := make([]string, 0, len(membership.Servers))
	for _, member := range membership.Servers {
		ids = append(ids, member.ID)
	}
	return ids
}
