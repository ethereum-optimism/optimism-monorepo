package sequencer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/config"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/metrics"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
)

type serviceBackend interface {
	frontend.AdminBackend
	frontend.BuildBackend
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

var _ serviceBackend = (*backend.Backend)(nil)
var _ serviceBackend = (*backend.MockBackend)(nil)

type Service struct {
	closing atomic.Bool

	log log.Logger

	backend serviceBackend

	metrics      metrics.Metricer
	pprofService *oppprof.Service
	metricsSrv   *httputil.HTTPServer
	rpcHandler   *oprpc.Handler
	httpServer   *httputil.HTTPServer
	jwtSecret    eth.Bytes32
}

var _ cliapp.Lifecycle = (*Service)(nil)

func FromConfig(ctx context.Context, cfg *config.Config, logger log.Logger) (*Service, error) {
	su := &Service{log: logger}
	if err := su.initFromCLIConfig(ctx, cfg); err != nil {
		return nil, errors.Join(err, su.Stop(ctx)) // try to clean up our failed initialization attempt
	}
	return su, nil
}

func (s *Service) initFromCLIConfig(ctx context.Context, cfg *config.Config) error {
	s.initMetrics(cfg)
	if err := s.initPProf(cfg); err != nil {
		return fmt.Errorf("failed to start PProf server: %w", err)
	}
	if err := s.initMetricsServer(cfg); err != nil {
		return fmt.Errorf("failed to start Metrics server: %w", err)
	}
	if err := s.initBackend(ctx, cfg); err != nil {
		return fmt.Errorf("failed to start backend: %w", err)
	}
	if err := s.initRPCHandler(cfg); err != nil {
		return fmt.Errorf("failed to start RPC handler: %w", err)
	}
	if err := s.initHTTPServer(cfg); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (s *Service) initMetrics(cfg *config.Config) {
	if cfg.MetricsConfig.Enabled {
		procName := "default"
		s.metrics = metrics.NewMetrics(procName)
		s.metrics.RecordInfo(cfg.Version)
	} else {
		s.metrics = metrics.NoopMetrics{}
	}
}

func (s *Service) initPProf(cfg *config.Config) error {
	s.pprofService = oppprof.New(
		cfg.PprofConfig.ListenEnabled,
		cfg.PprofConfig.ListenAddr,
		cfg.PprofConfig.ListenPort,
		cfg.PprofConfig.ProfileType,
		cfg.PprofConfig.ProfileDir,
		cfg.PprofConfig.ProfileFilename,
	)

	if err := s.pprofService.Start(); err != nil {
		return fmt.Errorf("failed to start pprof service: %w", err)
	}

	return nil
}

func (s *Service) initMetricsServer(cfg *config.Config) error {
	if !cfg.MetricsConfig.Enabled {
		s.log.Info("Metrics disabled")
		return nil
	}
	m, ok := s.metrics.(opmetrics.RegistryMetricer)
	if !ok {
		return fmt.Errorf("metrics were enabled, but metricer %T does not expose registry for metrics-server", s.metrics)
	}
	s.log.Debug("Starting metrics server", "addr", cfg.MetricsConfig.ListenAddr, "port", cfg.MetricsConfig.ListenPort)
	metricsSrv, err := opmetrics.StartServer(m.Registry(), cfg.MetricsConfig.ListenAddr, cfg.MetricsConfig.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	s.log.Info("Started metrics server", "addr", metricsSrv.Addr())
	s.metricsSrv = metricsSrv
	return nil
}

func (s *Service) initRPCHandler(cfg *config.Config) error {
	secret, err := oprpc.ObtainJWTSecret(s.log, cfg.JWTSecretPath, true)
	if err != nil {
		return err
	}
	s.jwtSecret = secret
	s.rpcHandler = oprpc.NewHandler(cfg.Version,
		oprpc.WithLogger(s.log),
		oprpc.WithJWTSecret(secret[:]),
		oprpc.WithWebsocketEnabled(),
	)
	if cfg.RPC.EnableAdmin {
		s.log.Info("Admin RPC enabled")
		if err := s.rpcHandler.AddAPI(rpc.API{
			Namespace:     "admin",
			Service:       &frontend.AdminFrontend{Backend: s.backend},
			Authenticated: true,
		}); err != nil {
			return fmt.Errorf("failed to add admin API: %w", err)
		}
	}
	if err := s.rpcHandler.AddAPI(rpc.API{
		Namespace: "build",
		Service:   &frontend.BuildFrontend{Backend: s.backend},
	}); err != nil {
		return fmt.Errorf("failed to add build API: %w", err)
	}
	return nil
}

func (s *Service) initBackend(ctx context.Context, cfg *config.Config) error {
	if cfg.MockRun {
		s.backend = backend.NewMockBackend()
		return nil
	}
	setup, err := cfg.Ensemble.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load builder setup: %w", err)
	}
	builders, err := setup.Start(ctx, &work.StartOpts{
		Log:     s.log,
		Metrics: s.metrics,
	})
	if err != nil {
		return fmt.Errorf("failed to setup builders: %w", err)
	}
	s.backend = backend.NewBackend(s.log, s.metrics, builders, s.rpcHandler)
	return nil
}

func (s *Service) initHTTPServer(cfg *config.Config) error {
	endpoint := net.JoinHostPort(cfg.RPC.ListenAddr, strconv.Itoa(cfg.RPC.ListenPort))
	s.httpServer = httputil.NewHTTPServer(endpoint, s.rpcHandler)
	return nil
}

func (s *Service) Start(ctx context.Context) error {
	s.log.Info("Starting JSON-RPC server")
	if err := s.httpServer.Start(); err != nil {
		return fmt.Errorf("unable to start RPC server: %w", err)
	}

	if err := s.backend.Start(ctx); err != nil {
		return fmt.Errorf("unable to start backend: %w", err)
	}

	s.metrics.RecordUp()
	s.log.Info("JSON-RPC Server started", "endpoint", s.httpServer.HTTPEndpoint())
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if !s.closing.CompareAndSwap(false, true) {
		s.log.Warn("Already closing")
		return nil // already closing
	}
	s.log.Info("Stopping JSON-RPC server")
	var result error
	if s.httpServer != nil {
		if err := s.httpServer.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop HTTP server: %w", err))
		}
	}
	if s.rpcHandler != nil {
		s.rpcHandler.Stop()
	}
	s.log.Info("Stopped RPC Server")
	if s.backend != nil {
		if err := s.backend.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close backend: %w", err))
		}
	}
	s.log.Info("Stopped Backend")
	if s.pprofService != nil {
		if err := s.pprofService.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop PProf server: %w", err))
		}
	}
	s.log.Info("Stopped PProf")
	if s.metricsSrv != nil {
		if err := s.metricsSrv.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop metrics server: %w", err))
		}
	}
	s.log.Info("JSON-RPC server stopped")
	return result
}

func (s *Service) Stopped() bool {
	return s.closing.Load()
}

func (s *Service) RPC() string {
	return s.httpServer.HTTPEndpoint()
}
