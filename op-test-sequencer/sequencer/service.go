package sequencer

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
)

type serviceBackend interface {
	frontend.AdminBackend
	Builder() frontend.BuildBackend
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
	rpcServer    *oprpc.Server
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
	if err := s.initRPCServer(cfg); err != nil {
		return fmt.Errorf("failed to start RPC server: %w", err)
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

func (s *Service) initBackend(ctx context.Context, cfg *config.Config) error {
	if cfg.MockRun {
		s.backend = backend.NewMockBackend()
		return nil
	}
	s.backend = backend.NewBackend(s.log, s.metrics)
	return nil
}

func (s *Service) initRPCServer(cfg *config.Config) error {
	secret, err := oprpc.ObtainJWTSecret(s.log, cfg.JWTSecretPath, true)
	if err != nil {
		return err
	}
	server := oprpc.NewServer(
		cfg.RPC.ListenAddr,
		cfg.RPC.ListenPort,
		cfg.Version,
		oprpc.WithLogger(s.log),
		oprpc.WithJWTSecret(secret[:]),
	)
	if cfg.RPC.EnableAdmin {
		s.log.Info("Admin RPC enabled")
		server.AddAPI(rpc.API{
			Namespace:     "admin",
			Service:       &frontend.AdminFrontend{Backend: s.backend},
			Authenticated: true,
		})
	}
	server.AddAPI(rpc.API{
		Namespace: "test",
		Service:   &frontend.BuildFrontend{Backend: s.backend},
	})
	s.jwtSecret = secret
	s.rpcServer = server
	return nil
}

func (s *Service) Start(ctx context.Context) error {
	s.log.Info("Starting JSON-RPC server")
	if err := s.rpcServer.Start(); err != nil {
		return fmt.Errorf("unable to start RPC server: %w", err)
	}

	if err := s.backend.Start(ctx); err != nil {
		return fmt.Errorf("unable to start backend: %w", err)
	}

	s.metrics.RecordUp()
	s.log.Info("JSON-RPC Server started", "endpoint", s.rpcServer.Endpoint())
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if !s.closing.CompareAndSwap(false, true) {
		s.log.Warn("Already closing")
		return nil // already closing
	}
	s.log.Info("Stopping JSON-RPC server")
	var result error
	if s.rpcServer != nil {
		if err := s.rpcServer.Stop(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop RPC server: %w", err))
		}
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
	// the RPC endpoint is assumed to be HTTP
	// TODO(#11032): make this flexible for ws if the server supports it
	return "http://" + s.rpcServer.Endpoint()
}
