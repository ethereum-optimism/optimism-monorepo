package node

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ethereum/go-ethereum"
	gethevent "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/node/safedb"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/conductor"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sequencing"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

var ErrAlreadyClosed = errors.New("node is already closed")

type closableSafeDB interface {
	rollup.SafeHeadListener
	SafeDBReader
	io.Closer
}

type OpNode struct {
	// Retain the config to test for active features rather than test for runtime state.
	cfg        *Config
	log        log.Logger
	appVersion string
	metrics    *metrics.Metrics

	l1HeadsSub     ethereum.Subscription // Subscription to get L1 heads (automatically re-subscribes on error)
	l1SafeSub      ethereum.Subscription // Subscription to get L1 safe blocks, a.k.a. justified data (polling)
	l1FinalizedSub ethereum.Subscription // Subscription to get L1 safe blocks, a.k.a. justified data (polling)

	eventSys   event.System
	eventDrain event.Drainer

	l1Source  *sources.L1Client     // L1 Client to fetch data from
	l2Driver  *driver.Driver        // L2 Engine to Sync
	l2Source  *sources.EngineClient // L2 Execution Engine RPC bindings
	server    *rpcServer            // RPC server hosting the rollup-node API
	p2pNode   *p2p.NodeP2P          // P2P node functionality
	p2pSigner p2p.Signer            // p2p gossip application messages will be signed with this signer
	tracer    Tracer                // tracer to get events for testing/debugging
	runCfg    *RuntimeConfig        // runtime configurables

	safeDB closableSafeDB

	rollupHalt string // when to halt the rollup, disabled if empty

	pprofService *oppprof.Service
	metricsSrv   *httputil.HTTPServer

	beacon *sources.L1BeaconClient

	supervisor *sources.SupervisorClient

	// some resources cannot be stopped directly, like the p2p gossipsub router (not our design),
	// and depend on this ctx to be closed.
	resourcesCtx   context.Context
	resourcesClose context.CancelFunc

	// Indicates when it's safe to close data sources used by the runtimeConfig bg loader
	runtimeConfigReloaderDone chan struct{}

	closed atomic.Bool

	// cancels execution prematurely, e.g. to halt. This may be nil.
	cancel context.CancelCauseFunc
	halted atomic.Bool
}

// The OpNode handles incoming gossip
var _ p2p.GossipIn = (*OpNode)(nil)

// New creates a new OpNode instance.
// The provided ctx argument is for the span of initialization only;
// the node will immediately Stop(ctx) before finishing initialization if the context is canceled during initialization.
func New(ctx context.Context, cfg *Config, log log.Logger, appVersion string, m *metrics.Metrics) (*OpNode, error) {
	if err := cfg.Check(); err != nil {
		return nil, err
	}

	n := &OpNode{
		cfg:        cfg,
		log:        log,
		appVersion: appVersion,
		metrics:    m,
		rollupHalt: cfg.RollupHalt,
		cancel:     cfg.Cancel,
	}
	// not a context leak, gossipsub is closed with a context.
	n.resourcesCtx, n.resourcesClose = context.WithCancel(context.Background())

	err := n.init(ctx, cfg)
	if err != nil {
		log.Error("Error initializing the rollup node", "err", err)
		// ensure we always close the node resources if we fail to initialize the node.
		if closeErr := n.Stop(ctx); closeErr != nil {
			return nil, multierror.Append(err, closeErr)
		}
		return nil, err
	}
	return n, nil
}

func (n *OpNode) init(ctx context.Context, cfg *Config) error {
	n.log.Info("Initializing rollup node", "version", n.appVersion)
	if err := n.initTracer(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init the trace: %w", err)
	}
	n.initEventSystem()
	if err := n.initL1(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init L1: %w", err)
	}
	if err := n.initL1BeaconAPI(ctx, cfg); err != nil {
		return err
	}
	if err := n.initL2(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init L2: %w", err)
	}
	if err := n.initRuntimeConfig(ctx, cfg); err != nil { // depends on L2, to signal initial runtime values to
		return fmt.Errorf("failed to init the runtime config: %w", err)
	}
	if err := n.initP2PSigner(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init the P2P signer: %w", err)
	}
	if err := n.initP2P(cfg); err != nil {
		return fmt.Errorf("failed to init the P2P stack: %w", err)
	}
	// Only expose the server at the end, ensuring all RPC backend components are initialized.
	if err := n.initRPCServer(cfg); err != nil {
		return fmt.Errorf("failed to init the RPC server: %w", err)
	}
	if err := n.initMetricsServer(cfg); err != nil {
		return fmt.Errorf("failed to init the metrics server: %w", err)
	}
	n.metrics.RecordInfo(n.appVersion)
	n.metrics.RecordUp()
	if err := n.initPProf(cfg); err != nil {
		return fmt.Errorf("failed to init profiling: %w", err)
	}
	return nil
}

func (n *OpNode) initEventSystem() {
	// This executor will be configurable in the future, for parallel event processing
	executor := event.NewGlobalSynchronous(n.resourcesCtx)
	sys := event.NewSystem(n.log, executor)
	sys.AddTracer(event.NewMetricsTracer(n.metrics))
	sys.Register("node", event.DeriverFunc(n.onEvent), event.DefaultRegisterOpts())
	n.eventSys = sys
	n.eventDrain = executor
}

func (n *OpNode) initTracer(ctx context.Context, cfg *Config) error {
	if cfg.Tracer != nil {
		n.tracer = cfg.Tracer
	} else {
		n.tracer = new(noOpTracer)
	}
	return nil
}

func (n *OpNode) initL1(ctx context.Context, cfg *Config) error {
	l1Node, rpcCfg, err := cfg.L1.Setup(ctx, n.log, &cfg.Rollup)
	if err != nil {
		return fmt.Errorf("failed to get L1 RPC client: %w", err)
	}

	n.l1Source, err = sources.NewL1Client(
		client.NewInstrumentedRPC(l1Node, &n.metrics.RPCMetrics.RPCClientMetrics), n.log, n.metrics.L1SourceCache, rpcCfg)
	if err != nil {
		return fmt.Errorf("failed to create L1 source: %w", err)
	}

	if err := cfg.Rollup.ValidateL1Config(ctx, n.l1Source); err != nil {
		return fmt.Errorf("failed to validate the L1 config: %w", err)
	}

	// Keep subscribed to the L1 heads, which keeps the L1 maintainer pointing to the best headers to sync
	n.l1HeadsSub = gethevent.ResubscribeErr(time.Second*10, func(ctx context.Context, err error) (gethevent.Subscription, error) {
		if err != nil {
			n.log.Warn("resubscribing after failed L1 subscription", "err", err)
		}
		return eth.WatchHeadChanges(ctx, n.l1Source, n.OnNewL1Head)
	})
	go func() {
		err, ok := <-n.l1HeadsSub.Err()
		if !ok {
			return
		}
		n.log.Error("l1 heads subscription error", "err", err)
	}()

	// Poll for the safe L1 block and finalized block,
	// which only change once per epoch at most and may be delayed.
	n.l1SafeSub = eth.PollBlockChanges(n.log, n.l1Source, n.OnNewL1Safe, eth.Safe,
		cfg.L1EpochPollInterval, time.Second*10)
	n.l1FinalizedSub = eth.PollBlockChanges(n.log, n.l1Source, n.OnNewL1Finalized, eth.Finalized,
		cfg.L1EpochPollInterval, time.Second*10)
	return nil
}

func (n *OpNode) initRuntimeConfig(ctx context.Context, cfg *Config) error {
	// attempt to load runtime config, repeat N times
	n.runCfg = NewRuntimeConfig(n.log, n.l1Source, &cfg.Rollup)

	confDepth := cfg.Driver.VerifierConfDepth
	reload := func(ctx context.Context) (eth.L1BlockRef, error) {
		fetchCtx, fetchCancel := context.WithTimeout(ctx, time.Second*10)
		l1Head, err := n.l1Source.L1BlockRefByLabel(fetchCtx, eth.Unsafe)
		fetchCancel()
		if err != nil {
			n.log.Error("failed to fetch L1 head for runtime config initialization", "err", err)
			return eth.L1BlockRef{}, err
		}

		// Apply confirmation-distance
		blNum := l1Head.Number
		if blNum >= confDepth {
			blNum -= confDepth
		}
		fetchCtx, fetchCancel = context.WithTimeout(ctx, time.Second*10)
		confirmed, err := n.l1Source.L1BlockRefByNumber(fetchCtx, blNum)
		fetchCancel()
		if err != nil {
			n.log.Error("failed to fetch confirmed L1 block for runtime config loading", "err", err, "number", blNum)
			return eth.L1BlockRef{}, err
		}

		fetchCtx, fetchCancel = context.WithTimeout(ctx, time.Second*10)
		err = n.runCfg.Load(fetchCtx, confirmed)
		fetchCancel()
		if err != nil {
			n.log.Error("failed to fetch runtime config data", "err", err)
			return l1Head, err
		}

		err = n.handleProtocolVersionsUpdate(ctx)
		return l1Head, err
	}

	// initialize the runtime config before unblocking
	if err := retry.Do0(ctx, 5, retry.Fixed(time.Second*10), func() error {
		_, err := reload(ctx)
		if errors.Is(err, errNodeHalt) { // don't retry on halt error
			err = nil
		}
		return err
	}); err != nil {
		return fmt.Errorf("failed to load runtime configuration repeatedly, last error: %w", err)
	}

	// start a background loop, to keep reloading it at the configured reload interval
	reloader := func(ctx context.Context, reloadInterval time.Duration) {
		if reloadInterval <= 0 {
			n.log.Debug("not running runtime-config reloading background loop")
			return
		}
		ticker := time.NewTicker(reloadInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// If the reload fails, we will try again the next interval.
				// Missing a runtime-config update is not critical, and we do not want to overwhelm the L1 RPC.
				l1Head, err := reload(ctx)
				if err != nil {
					if errors.Is(err, errNodeHalt) {
						n.halted.Store(true)
						if n.cancel != nil { // node cancellation is always available when started as CLI app
							n.cancel(errNodeHalt)
							return
						} else {
							n.log.Debug("opted to halt, but cannot halt node", "l1_head", l1Head)
						}
					} else {
						n.log.Warn("failed to reload runtime config", "err", err)
					}
				} else {
					n.log.Debug("reloaded runtime config", "l1_head", l1Head)
				}
			case <-ctx.Done():
				return
			}
		}
	}

	n.runtimeConfigReloaderDone = make(chan struct{})
	// Manages the lifetime of reloader. In order to safely Close the OpNode
	go func(ctx context.Context, reloadInterval time.Duration) {
		reloader(ctx, reloadInterval)
		close(n.runtimeConfigReloaderDone)
	}(n.resourcesCtx, cfg.RuntimeConfigReloadInterval) // this keeps running after initialization
	return nil
}

func (n *OpNode) initL1BeaconAPI(ctx context.Context, cfg *Config) error {
	// If Ecotone upgrade is not scheduled yet, then there is no need for a Beacon API.
	if cfg.Rollup.EcotoneTime == nil {
		return nil
	}
	// Once the Ecotone upgrade is scheduled, we must have initialized the Beacon API settings.
	if cfg.Beacon == nil {
		return fmt.Errorf("missing L1 Beacon Endpoint configuration: this API is mandatory for Ecotone upgrade at t=%d", *cfg.Rollup.EcotoneTime)
	}

	// We always initialize a client. We will get an error on requests if the client does not work.
	// This way the op-node can continue non-L1 functionality when the user chooses to ignore the Beacon API requirement.
	beaconClient, fallbacks, err := cfg.Beacon.Setup(ctx, n.log)
	if err != nil {
		return fmt.Errorf("failed to setup L1 Beacon API client: %w", err)
	}
	beaconCfg := sources.L1BeaconClientConfig{
		FetchAllSidecars: cfg.Beacon.ShouldFetchAllSidecars(),
	}
	n.beacon = sources.NewL1BeaconClient(beaconClient, beaconCfg, fallbacks...)

	// Retry retrieval of the Beacon API version, to be more robust on startup against Beacon API connection issues.
	beaconVersion, missingEndpoint, err := retry.Do2[string, bool](ctx, 5, retry.Exponential(), func() (string, bool, error) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		beaconVersion, err := n.beacon.GetVersion(ctx)
		if err != nil {
			if errors.Is(err, client.ErrNoEndpoint) {
				return "", true, nil // don't return an error, we do not have to retry when there is a config issue.
			}
			return "", false, err
		}
		return beaconVersion, false, nil
	})
	if missingEndpoint {
		// Allow the user to continue if they explicitly ignore the requirement of the endpoint.
		if cfg.Beacon.ShouldIgnoreBeaconCheck() {
			n.log.Warn("This endpoint is required for the Ecotone upgrade, but is missing, and configured to be ignored. " +
				"The node may be unable to retrieve EIP-4844 blobs data.")
			return nil
		} else {
			// If the client tells us the endpoint was not configured,
			// then explain why we need it, and what the user can do to ignore this.
			n.log.Error("The Ecotone upgrade requires a L1 Beacon API endpoint, to retrieve EIP-4844 blobs data. " +
				"This can be ignored with the --l1.beacon.ignore option, " +
				"but the node may be unable to sync from L1 without this endpoint.")
			return errors.New("missing L1 Beacon API endpoint")
		}
	} else if err != nil {
		if cfg.Beacon.ShouldIgnoreBeaconCheck() {
			n.log.Warn("Failed to check L1 Beacon API version, but configuration ignores results. "+
				"The node may be unable to retrieve EIP-4844 blobs data.", "err", err)
			return nil
		} else {
			return fmt.Errorf("failed to check L1 Beacon API version: %w", err)
		}
	} else {
		n.log.Info("Connected to L1 Beacon API, ready for EIP-4844 blobs retrieval.", "version", beaconVersion)
		return nil
	}
}

func (n *OpNode) initL2(ctx context.Context, cfg *Config) error {
	rpcClient, rpcCfg, err := cfg.L2.Setup(ctx, n.log, &cfg.Rollup)
	if err != nil {
		return fmt.Errorf("failed to setup L2 execution-engine RPC client: %w", err)
	}

	n.l2Source, err = sources.NewEngineClient(
		client.NewInstrumentedRPC(rpcClient, &n.metrics.RPCClientMetrics), n.log, n.metrics.L2SourceCache, rpcCfg,
	)
	if err != nil {
		return fmt.Errorf("failed to create Engine client: %w", err)
	}

	if err := cfg.Rollup.ValidateL2Config(ctx, n.l2Source, cfg.Sync.SyncMode == sync.ELSync); err != nil {
		return err
	}

	if cfg.Rollup.InteropTime != nil {
		cl, err := cfg.Supervisor.SupervisorClient(ctx, n.log)
		if err != nil {
			return fmt.Errorf("failed to setup supervisor RPC client: %w", err)
		}
		n.supervisor = cl
	}

	var sequencerConductor conductor.SequencerConductor = &conductor.NoOpConductor{}
	if cfg.ConductorEnabled {
		sequencerConductor = NewConductorClient(cfg, n.log, n.metrics)
	}

	// if altDA is not explicitly activated in the node CLI, the config + any error will be ignored.
	rpCfg, err := cfg.Rollup.GetOPAltDAConfig()
	if cfg.AltDA.Enabled && err != nil {
		return fmt.Errorf("failed to get altDA config: %w", err)
	}
	altDA := altda.NewAltDA(n.log, cfg.AltDA, rpCfg, n.metrics.AltDAMetrics)
	if cfg.SafeDBPath != "" {
		n.log.Info("Safe head database enabled", "path", cfg.SafeDBPath)
		safeDB, err := safedb.NewSafeDB(n.log, cfg.SafeDBPath)
		if err != nil {
			return fmt.Errorf("failed to create safe head database at %v: %w", cfg.SafeDBPath, err)
		}
		n.safeDB = safeDB
	} else {
		n.safeDB = safedb.Disabled
	}
	n.l2Driver = driver.NewDriver(n.eventSys, n.eventDrain, &cfg.Driver, &cfg.Rollup, n.l2Source, n.l1Source,
		n.supervisor, n.beacon, n, n, n.log, n.metrics, cfg.ConfigPersistence, n.safeDB, &cfg.Sync, sequencerConductor, altDA)
	return nil
}

func (n *OpNode) initRPCServer(cfg *Config) error {
	server, err := newRPCServer(&cfg.RPC, &cfg.Rollup, n.l2Source.L2Client, n.l2Driver, n.safeDB, n.log, n.appVersion, n.metrics)
	if err != nil {
		return err
	}
	if n.p2pEnabled() {
		server.EnableP2P(p2p.NewP2PAPIBackend(n.p2pNode, n.log, n.metrics))
	}
	if cfg.RPC.EnableAdmin {
		server.EnableAdminAPI(NewAdminAPI(n.l2Driver, n.metrics, n.log))
		n.log.Info("Admin RPC enabled")
	}
	n.log.Info("Starting JSON-RPC server")
	if err := server.Start(); err != nil {
		return fmt.Errorf("unable to start RPC server: %w", err)
	}
	n.log.Info("Started JSON-RPC server", "addr", server.Addr())
	n.server = server
	return nil
}

func (n *OpNode) initMetricsServer(cfg *Config) error {
	if !cfg.Metrics.Enabled {
		n.log.Info("metrics disabled")
		return nil
	}
	n.log.Debug("starting metrics server", "addr", cfg.Metrics.ListenAddr, "port", cfg.Metrics.ListenPort)
	metricsSrv, err := n.metrics.StartServer(cfg.Metrics.ListenAddr, cfg.Metrics.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	n.log.Info("started metrics server", "addr", metricsSrv.Addr())
	n.metricsSrv = metricsSrv
	return nil
}

func (n *OpNode) initPProf(cfg *Config) error {
	n.pprofService = oppprof.New(
		cfg.Pprof.ListenEnabled,
		cfg.Pprof.ListenAddr,
		cfg.Pprof.ListenPort,
		cfg.Pprof.ProfileType,
		cfg.Pprof.ProfileDir,
		cfg.Pprof.ProfileFilename,
	)

	if err := n.pprofService.Start(); err != nil {
		return fmt.Errorf("failed to start pprof service: %w", err)
	}

	return nil
}

func (n *OpNode) p2pEnabled() bool {
	return n.cfg.P2PEnabled()
}

func (n *OpNode) initP2P(cfg *Config) (err error) {
	if n.p2pNode != nil {
		panic("p2p node already initialized")
	}
	if n.p2pEnabled() {
		// TODO(protocol-quest#97): Use EL Sync instead of CL Alt sync for fetching missing blocks in the payload queue.
		n.p2pNode, err = p2p.NewNodeP2P(n.resourcesCtx, &cfg.Rollup, n.log, cfg.P2P, n, n.l2Source, n.runCfg, n.metrics, false)
		if err != nil {
			return
		}
		if n.p2pNode.Dv5Udp() != nil {
			go n.p2pNode.DiscoveryProcess(n.resourcesCtx, n.log, &cfg.Rollup, cfg.P2P.TargetPeers())
		}
	}
	return nil
}

func (n *OpNode) initP2PSigner(ctx context.Context, cfg *Config) (err error) {
	// the p2p signer setup is optional
	if cfg.P2PSigner == nil {
		return
	}
	// p2pSigner may still be nil, the signer setup may not create any signer, the signer is optional
	n.p2pSigner, err = cfg.P2PSigner.SetupSigner(ctx)
	return
}

func (n *OpNode) Start(ctx context.Context) error {
	n.log.Info("Starting execution engine driver")
	// start driving engine: sync blocks by deriving them from L1 and driving them into the engine
	if err := n.l2Driver.Start(); err != nil {
		n.log.Error("Could not start a rollup node", "err", err)
		return err
	}
	log.Info("Rollup node started")
	return nil
}

// onEvent handles broadcast events.
// The OpNode itself is a deriver to catch system-critical events.
// Other event-handling should be encapsulated into standalone derivers.
func (n *OpNode) onEvent(ev event.Event) bool {
	switch x := ev.(type) {
	case rollup.CriticalErrorEvent:
		n.log.Error("Critical error", "err", x.Err)
		n.cancel(fmt.Errorf("critical error: %w", x.Err))
		return true
	default:
		return false
	}
}

func (n *OpNode) OnNewL1Head(ctx context.Context, sig eth.L1BlockRef) {
	n.tracer.OnNewL1Head(ctx, sig)

	if n.l2Driver == nil {
		return
	}
	// Pass on the event to the L2 Engine
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	if err := n.l2Driver.OnL1Head(ctx, sig); err != nil {
		n.log.Warn("failed to notify engine driver of L1 head change", "err", err)
	}
}

func (n *OpNode) OnNewL1Safe(ctx context.Context, sig eth.L1BlockRef) {
	if n.l2Driver == nil {
		return
	}
	// Pass on the event to the L2 Engine
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	if err := n.l2Driver.OnL1Safe(ctx, sig); err != nil {
		n.log.Warn("failed to notify engine driver of L1 safe block change", "err", err)
	}
}

func (n *OpNode) OnNewL1Finalized(ctx context.Context, sig eth.L1BlockRef) {
	if n.l2Driver == nil {
		return
	}
	// Pass on the event to the L2 Engine
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	if err := n.l2Driver.OnL1Finalized(ctx, sig); err != nil {
		n.log.Warn("failed to notify engine driver of L1 finalized block change", "err", err)
	}
}

func (n *OpNode) PublishL2Payload(ctx context.Context, envelope *eth.ExecutionPayloadEnvelope) error {
	n.tracer.OnPublishL2Payload(ctx, envelope)

	// publish to p2p, if we are running p2p at all
	if n.p2pEnabled() {
		payload := envelope.ExecutionPayload
		if n.p2pSigner == nil {
			return fmt.Errorf("node has no p2p signer, payload %s cannot be published", payload.ID())
		}
		n.log.Info("Publishing signed execution payload on p2p", "id", payload.ID())
		return n.p2pNode.GossipOut().PublishL2Payload(ctx, envelope, n.p2pSigner)
	}
	// if p2p is not enabled then we just don't publish the payload
	return nil
}

func (n *OpNode) OnUnsafeL2Payload(ctx context.Context, from peer.ID, envelope *eth.ExecutionPayloadEnvelope) error {
	// ignore if it's from ourselves
	if n.p2pEnabled() && from == n.p2pNode.Host().ID() {
		return nil
	}

	n.tracer.OnUnsafeL2Payload(ctx, from, envelope)

	n.log.Info("Received signed execution payload from p2p", "id", envelope.ExecutionPayload.ID(), "peer", from,
		"txs", len(envelope.ExecutionPayload.Transactions))

	// Pass on the event to the L2 Engine
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	if err := n.l2Driver.OnUnsafeL2Payload(ctx, envelope); err != nil {
		n.log.Warn("failed to notify engine driver of new L2 payload", "err", err, "id", envelope.ExecutionPayload.ID())
	}

	return nil
}

func (n *OpNode) RequestL2Range(ctx context.Context, start, end eth.L2BlockRef) error {
	if n.p2pEnabled() && n.p2pNode.AltSyncEnabled() {
		if unixTimeStale(start.Time, 12*time.Hour) {
			n.log.Debug(
				"ignoring request to sync L2 range, timestamp is too old for p2p",
				"start", start,
				"end", end,
				"start_time", start.Time)
			return nil
		}
		return n.p2pNode.RequestL2Range(ctx, start, end)
	}
	n.log.Debug("ignoring request to sync L2 range, no sync method available", "start", start, "end", end)
	return nil
}

// unixTimeStale returns true if the unix timestamp is before the current time minus the supplied duration.
func unixTimeStale(timestamp uint64, duration time.Duration) bool {
	return time.Unix(int64(timestamp), 0).Before(time.Now().Add(-1 * duration))
}

func (n *OpNode) P2P() p2p.Node {
	return n.p2pNode
}

func (n *OpNode) RuntimeConfig() ReadonlyRuntimeConfig {
	return n.runCfg
}

// Stop stops the node and closes all resources.
// If the provided ctx is expired, the node will accelerate the stop where possible, but still fully close.
func (n *OpNode) Stop(ctx context.Context) error {
	if n.closed.Load() {
		return ErrAlreadyClosed
	}

	var result *multierror.Error

	if n.server != nil {
		if err := n.server.Stop(ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to close RPC server: %w", err))
		}
	}

	// Stop sequencer and report last hash. l2Driver can be nil if we're cleaning up a failed init.
	if n.l2Driver != nil {
		latestHead, err := n.l2Driver.StopSequencer(ctx)
		switch {
		case errors.Is(err, sequencing.ErrSequencerNotEnabled):
		case errors.Is(err, driver.ErrSequencerAlreadyStopped):
			n.log.Info("stopping node: sequencer already stopped", "latestHead", latestHead)
		case err == nil:
			n.log.Info("stopped sequencer", "latestHead", latestHead)
		default:
			result = multierror.Append(result, fmt.Errorf("error stopping sequencer: %w", err))
		}
	}
	if n.p2pNode != nil {
		if err := n.p2pNode.Close(); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to close p2p node: %w", err))
		}
		// Prevent further use of p2p.
		n.p2pNode = nil
	}
	if n.p2pSigner != nil {
		if err := n.p2pSigner.Close(); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to close p2p signer: %w", err))
		}
	}

	if n.resourcesClose != nil {
		n.resourcesClose()
	}

	// stop L1 heads feed
	if n.l1HeadsSub != nil {
		n.l1HeadsSub.Unsubscribe()
	}
	// stop polling for L1 safe-head changes
	if n.l1SafeSub != nil {
		n.l1SafeSub.Unsubscribe()
	}
	// stop polling for L1 finalized-head changes
	if n.l1FinalizedSub != nil {
		n.l1FinalizedSub.Unsubscribe()
	}

	// close L2 driver
	if n.l2Driver != nil {
		if err := n.l2Driver.Close(); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to close L2 engine driver cleanly: %w", err))
		}
	}

	if n.eventSys != nil {
		n.eventSys.Stop()
	}

	if n.safeDB != nil {
		if err := n.safeDB.Close(); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to close safe head db: %w", err))
		}
	}

	// Wait for the runtime config loader to be done using the data sources before closing them
	if n.runtimeConfigReloaderDone != nil {
		<-n.runtimeConfigReloaderDone
	}

	// close L2 engine RPC client
	if n.l2Source != nil {
		n.l2Source.Close()
	}

	// close the supervisor RPC client
	if n.supervisor != nil {
		n.supervisor.Close()
	}

	// close L1 data source
	if n.l1Source != nil {
		n.l1Source.Close()
	}

	if result == nil { // mark as closed if we successfully fully closed
		n.closed.Store(true)
	}

	if n.halted.Load() {
		// if we had a halt upon initialization, idle for a while, with open metrics, to prevent a rapid restart-loop
		tim := time.NewTimer(time.Minute * 5)
		n.log.Warn("halted, idling to avoid immediate shutdown repeats")
		defer tim.Stop()
		select {
		case <-tim.C:
		case <-ctx.Done():
		}
	}

	// Close metrics and pprof only after we are done idling
	if n.pprofService != nil {
		if err := n.pprofService.Stop(ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to close pprof server: %w", err))
		}
	}
	if n.metricsSrv != nil {
		if err := n.metricsSrv.Stop(ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to close metrics server: %w", err))
		}
	}

	return result.ErrorOrNil()
}

func (n *OpNode) Stopped() bool {
	return n.closed.Load()
}

func (n *OpNode) HTTPEndpoint() string {
	if n.server == nil {
		return ""
	}
	return fmt.Sprintf("http://%s", n.server.Addr().String())
}
