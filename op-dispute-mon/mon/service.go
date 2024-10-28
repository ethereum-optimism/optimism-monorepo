package mon

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/mon/bonds"
	"github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/config"
	"github.com/ethereum-optimism/optimism/op-dispute-mon/metrics"
	"github.com/ethereum-optimism/optimism/op-dispute-mon/mon/extract"
	"github.com/ethereum-optimism/optimism/op-dispute-mon/version"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
)

type Service struct {
	logger       log.Logger
	metrics      metrics.Metricer
	monitor      *gameMonitor
	honestActors types.HonestActors

	factoryContract *contracts.DisputeGameFactoryContract

	cl clock.Clock

	extractor    *extract.Extractor
	forecast     *Forecast
	bonds        *bonds.Bonds
	game         *extract.GameCallerCreator
	resolutions  *ResolutionMonitor
	claims       *ClaimMonitor
	withdrawals  *WithdrawalMonitor
	rollupClient *sources.RollupClient

	l1Client *ethclient.Client

	pprofService *oppprof.Service
	metricsSrv   *httputil.HTTPServer

	stopped atomic.Bool
}

// NewService creates a new Service.
func NewService(ctx context.Context, logger log.Logger, cfg *config.Config) (*Service, error) {
	s := &Service{
		cl:           clock.SystemClock,
		logger:       logger,
		metrics:      metrics.NewMetrics(),
		honestActors: types.NewHonestActors(cfg.HonestActors),
	}

	if err := s.initFromConfig(ctx, cfg); err != nil {
		return nil, errors.Join(fmt.Errorf("failed to init service: %w", err), s.Stop(ctx))
	}

	return s, nil
}

func (s *Service) initFromConfig(ctx context.Context, cfg *config.Config) error {
	if err := s.initL1Client(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init l1 client: %w", err)
	}
	if err := s.initPProf(&cfg.PprofConfig); err != nil {
		return fmt.Errorf("failed to init profiling: %w", err)
	}
	if err := s.initMetricsServer(&cfg.MetricsConfig); err != nil {
		return fmt.Errorf("failed to init metrics server: %w", err)
	}
	if err := s.initFactoryContract(cfg); err != nil {
		return fmt.Errorf("failed to create factory contract bindings: %w", err)
	}
	if err := s.initOutputRollupClient(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init rollup client: %w", err)
	}

	s.initClaimMonitor(cfg)
	s.initResolutionMonitor()
	s.initWithdrawalMonitor()

	s.initGameCallerCreator() // Must be called before initForecast

	s.initExtractor(cfg)

	s.initForecast(cfg)
	s.initBonds()

	s.initMonitor(ctx, cfg) // Monitor must be initialized last

	s.metrics.RecordInfo(version.SimpleWithMeta)
	s.metrics.RecordUp()

	return nil
}

func (s *Service) initClaimMonitor(cfg *config.Config) {
	s.claims = NewClaimMonitor(s.logger, s.cl, s.honestActors, s.metrics)
}

func (s *Service) initResolutionMonitor() {
	s.resolutions = NewResolutionMonitor(s.logger, s.metrics, s.cl)
}

func (s *Service) initWithdrawalMonitor() {
	s.withdrawals = NewWithdrawalMonitor(s.logger, s.cl, s.metrics, s.honestActors)
}

func (s *Service) initGameCallerCreator() {
	s.game = extract.NewGameCallerCreator(s.metrics, batching.NewMultiCaller(s.l1Client.Client(), batching.DefaultBatchSize))
}

func (s *Service) initExtractor(cfg *config.Config) {
	s.extractor = extract.NewExtractor(
		s.logger,
		s.cl,
		s.game.CreateContract,
		s.factoryContract.GetGamesAtOrAfter,
		cfg.IgnoredGames,
		cfg.MaxConcurrency,
		extract.NewClaimEnricher(),
		extract.NewRecipientEnricher(), // Must be called before WithdrawalsEnricher and BondEnricher
		extract.NewWithdrawalsEnricher(),
		extract.NewBondEnricher(),
		extract.NewBalanceEnricher(),
		extract.NewL1HeadBlockNumEnricher(s.l1Client),
		extract.NewAgreementEnricher(s.logger, s.metrics, s.rollupClient),
	)
}

func (s *Service) initForecast(cfg *config.Config) {
	s.forecast = NewForecast(s.logger, s.metrics)
}

func (s *Service) initBonds() {
	s.bonds = bonds.NewBonds(s.logger, s.metrics, s.cl)
}

func (s *Service) initOutputRollupClient(ctx context.Context, cfg *config.Config) error {
	outputRollupClient, err := dial.DialRollupClientWithTimeout(ctx, dial.DefaultDialTimeout, s.logger, cfg.RollupRpc)
	if err != nil {
		return fmt.Errorf("failed to dial rollup client: %w", err)
	}
	s.rollupClient = outputRollupClient
	return nil
}

func (s *Service) initL1Client(ctx context.Context, cfg *config.Config) error {
	l1Client, err := dial.DialEthClientWithTimeout(ctx, dial.DefaultDialTimeout, s.logger, cfg.L1EthRpc)
	if err != nil {
		return fmt.Errorf("failed to dial L1: %w", err)
	}
	s.l1Client = l1Client
	return nil
}

func (s *Service) initPProf(cfg *oppprof.CLIConfig) error {
	s.pprofService = oppprof.New(
		cfg.ListenEnabled,
		cfg.ListenAddr,
		cfg.ListenPort,
		cfg.ProfileType,
		cfg.ProfileDir,
		cfg.ProfileFilename,
	)

	if err := s.pprofService.Start(); err != nil {
		return fmt.Errorf("failed to start pprof service: %w", err)
	}

	return nil
}

func (s *Service) initMetricsServer(cfg *opmetrics.CLIConfig) error {
	if !cfg.Enabled {
		return nil
	}
	s.logger.Debug("starting metrics server", "addr", cfg.ListenAddr, "port", cfg.ListenPort)
	m, ok := s.metrics.(opmetrics.RegistryMetricer)
	if !ok {
		return fmt.Errorf("metrics were enabled, but metricer %T does not expose registry for metrics-server", s.metrics)
	}
	metricsSrv, err := opmetrics.StartServer(m.Registry(), cfg.ListenAddr, cfg.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	s.logger.Info("started metrics server", "addr", metricsSrv.Addr())
	s.metricsSrv = metricsSrv
	return nil
}

func (s *Service) initFactoryContract(cfg *config.Config) error {
	factoryContract := contracts.NewDisputeGameFactoryContract(s.metrics, cfg.GameFactoryAddress,
		batching.NewMultiCaller(s.l1Client.Client(), batching.DefaultBatchSize))
	s.factoryContract = factoryContract
	return nil
}

func (s *Service) initMonitor(ctx context.Context, cfg *config.Config) {
	blockHashFetcher := func(ctx context.Context, blockNumber *big.Int) (common.Hash, error) {
		block, err := s.l1Client.BlockByNumber(ctx, blockNumber)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to fetch block by number: %w", err)
		}
		return block.Hash(), nil
	}
	l2ChallengesMonitor := NewL2ChallengesMonitor(s.logger, s.metrics)
	updateTimeMonitor := NewUpdateTimeMonitor(s.cl, s.metrics)
	s.monitor = newGameMonitor(ctx, s.logger, s.cl, s.metrics, cfg.MonitorInterval, cfg.GameWindow, blockHashFetcher,
		s.l1Client.BlockNumber,
		s.extractor.Extract,
		s.forecast.Forecast,
		s.bonds.CheckBonds,
		s.resolutions.CheckResolutions,
		s.claims.CheckClaims,
		s.withdrawals.CheckWithdrawals,
		l2ChallengesMonitor.CheckL2Challenges,
		updateTimeMonitor.CheckUpdateTimes)
}

func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("Starting scheduler")
	s.logger.Info("Starting monitoring")
	s.monitor.StartMonitoring()
	s.logger.Info("Dispute monitor game service start completed")
	return nil
}

func (s *Service) Stopped() bool {
	return s.stopped.Load()
}

func (s *Service) Stop(ctx context.Context) error {
	s.logger.Info("Stopping dispute mon service")

	var result error
	if s.pprofService != nil {
		if err := s.pprofService.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close pprof server: %w", err))
		}
	}
	if s.metricsSrv != nil {
		if err := s.metricsSrv.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close metrics server: %w", err))
		}
	}
	s.stopped.Store(true)
	s.logger.Info("stopped dispute mon service", "err", result)
	return result
}
