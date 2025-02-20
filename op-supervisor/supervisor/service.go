package supervisor

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/tasks"
	"github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/sync"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/frontend"
)

type Backend interface {
	frontend.Backend
}

// SupervisorService implements the full-environment bells and whistles around the Supervisor.
// This includes the setup and teardown of metrics, pprof, admin RPC, regular RPC etc.
type SupervisorService struct {
	closing atomic.Bool

	log log.Logger

	metrics metrics.Metricer

	poller *tasks.Poller

	backend Backend

	pprofService *oppprof.Service
	metricsSrv   *httputil.HTTPServer
	rpcServer    *oprpc.Server
}

var _ cliapp.Lifecycle = (*SupervisorService)(nil)

func SupervisorFromConfig(ctx context.Context, cfg *config.Config, logger log.Logger) (*SupervisorService, error) {
	su := &SupervisorService{log: logger}
	if err := su.initFromCLIConfig(ctx, cfg); err != nil {
		return nil, errors.Join(err, su.Stop(ctx)) // try to clean up our failed initialization attempt
	}
	return su, nil
}

func (su *SupervisorService) initFromCLIConfig(ctx context.Context, cfg *config.Config) error {
	su.initMetrics(cfg)
	if err := su.initPProf(cfg); err != nil {
		return fmt.Errorf("failed to start PProf server: %w", err)
	}
	if err := su.initMetricsServer(cfg); err != nil {
		return fmt.Errorf("failed to start Metrics server: %w", err)
	}
	if err := su.initBackend(ctx, cfg); err != nil {
		return fmt.Errorf("failed to start backend: %w", err)
	}
	if err := su.initRPCServer(cfg); err != nil {
		return fmt.Errorf("failed to start RPC server: %w", err)
	}
	if err := su.initDBSync(ctx, cfg); err != nil {
		return fmt.Errorf("failed to start DB sync server: %w", err)
	}
	return nil
}

func (su *SupervisorService) initBackend(ctx context.Context, cfg *config.Config) error {
	// In the future we may introduce other executors.
	// For now, we just use a synchronous executor, and poll the drain function of it.
	ex := event.NewGlobalSynchronous(ctx)
	su.poller = tasks.NewPoller(func() {
		if err := ex.Drain(); err != nil {
			su.log.Warn("Failed to execute events", "err", err)
		}
	}, clock.SystemClock, time.Millisecond*100)

	if cfg.MockRun {
		su.backend = backend.NewMockBackend()
		return nil
	}

	be, err := backend.NewSupervisorBackend(ctx, su.log, su.metrics, cfg, ex)
	if err != nil {
		return fmt.Errorf("failed to create supervisor backend: %w", err)
	}
	su.backend = be
	return nil
}

func (su *SupervisorService) initMetrics(cfg *config.Config) {
	if cfg.MetricsConfig.Enabled {
		procName := "default"
		su.metrics = metrics.NewMetrics(procName)
		su.metrics.RecordInfo(cfg.Version)
	} else {
		su.metrics = metrics.NoopMetrics
	}
}

func (su *SupervisorService) initPProf(cfg *config.Config) error {
	su.pprofService = oppprof.New(
		cfg.PprofConfig.ListenEnabled,
		cfg.PprofConfig.ListenAddr,
		cfg.PprofConfig.ListenPort,
		cfg.PprofConfig.ProfileType,
		cfg.PprofConfig.ProfileDir,
		cfg.PprofConfig.ProfileFilename,
	)

	if err := su.pprofService.Start(); err != nil {
		return fmt.Errorf("failed to start pprof service: %w", err)
	}

	return nil
}

func (su *SupervisorService) initMetricsServer(cfg *config.Config) error {
	if !cfg.MetricsConfig.Enabled {
		su.log.Info("Metrics disabled")
		return nil
	}
	m, ok := su.metrics.(opmetrics.RegistryMetricer)
	if !ok {
		return fmt.Errorf("metrics were enabled, but metricer %T does not expose registry for metrics-server", su.metrics)
	}
	su.log.Debug("Starting metrics server", "addr", cfg.MetricsConfig.ListenAddr, "port", cfg.MetricsConfig.ListenPort)
	metricsSrv, err := opmetrics.StartServer(m.Registry(), cfg.MetricsConfig.ListenAddr, cfg.MetricsConfig.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	su.log.Info("Started metrics server", "addr", metricsSrv.Addr())
	su.metricsSrv = metricsSrv
	return nil
}

func (su *SupervisorService) initRPCServer(cfg *config.Config) error {
	server := oprpc.NewServer(
		cfg.RPC.ListenAddr,
		cfg.RPC.ListenPort,
		cfg.Version,
		oprpc.WithLogger(su.log),
		// oprpc.WithHTTPRecorder(su.metrics), // TODO(protocol-quest#286) hook up metrics to RPC server
	)
	RegisterRPCs(su.log, cfg, server, su.backend)
	su.rpcServer = server
	return nil
}

type RpcServer interface {
	AddAPI(rpc.API)
}

func RegisterRPCs(logger log.Logger, cfg *config.Config, server RpcServer, backend Backend) {
	if cfg.RPC.EnableAdmin {
		logger.Info("Admin RPC enabled")
		server.AddAPI(rpc.API{
			Namespace:     "admin",
			Service:       &frontend.AdminFrontend{Supervisor: backend},
			Authenticated: true, // TODO(protocol-quest#286): enforce auth on this or not?
		})
	}
	server.AddAPI(rpc.API{
		Namespace:     "supervisor",
		Service:       &frontend.QueryFrontend{Supervisor: backend},
		Authenticated: false,
	})
}

func (su *SupervisorService) initDBSync(ctx context.Context, cfg *config.Config) error {
	syncCfg := sync.Config{
		DataDir: cfg.Datadir,
		Logger:  su.log,
	}
	depSet, err := cfg.DependencySetSource.LoadDependencySet(ctx)
	if err != nil {
		return fmt.Errorf("failed to load dependency set: %w", err)
	}
	handler, err := sync.NewServer(syncCfg, depSet.Chains())
	if err != nil {
		return fmt.Errorf("failed to create db sync handler: %w", err)
	}
	su.rpcServer.AddHandler("/dbsync", handler)
	return nil
}

func (su *SupervisorService) Start(ctx context.Context) error {
	su.log.Info("Starting JSON-RPC server")
	if err := su.rpcServer.Start(); err != nil {
		return fmt.Errorf("unable to start RPC server: %w", err)
	}

	su.poller.Start()

	if err := su.backend.Start(ctx); err != nil {
		return fmt.Errorf("unable to start backend: %w", err)
	}

	su.metrics.RecordUp()
	su.log.Info("JSON-RPC Server started", "endpoint", su.rpcServer.Endpoint())
	return nil
}

func (su *SupervisorService) Stop(ctx context.Context) error {
	if !su.closing.CompareAndSwap(false, true) {
		su.log.Warn("Supervisor is already closing")
		return nil // already closing
	}
	su.log.Info("Stopping JSON-RPC server")
	var result error
	if su.rpcServer != nil {
		if err := su.rpcServer.Stop(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop RPC server: %w", err))
		}
	}
	su.log.Info("Stopped RPC Server")
	if su.backend != nil {
		if err := su.backend.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close supervisor backend: %w", err))
		}
	}
	su.log.Info("Stopped Backend")
	if su.pprofService != nil {
		if err := su.pprofService.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop PProf server: %w", err))
		}
	}
	su.log.Info("Stopped PProf")
	if su.metricsSrv != nil {
		if err := su.metricsSrv.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop metrics server: %w", err))
		}
	}
	su.log.Info("JSON-RPC server stopped")
	if su.poller != nil {
		su.poller.Stop()
	}
	su.log.Info("Event processing stopped")
	return result
}

func (su *SupervisorService) Stopped() bool {
	return su.closing.Load()
}

func (su *SupervisorService) RPC() string {
	// the RPC endpoint is assumed to be HTTP
	// TODO(#11032): make this flexible for ws if the server supports it
	return "http://" + su.rpcServer.Endpoint()
}
