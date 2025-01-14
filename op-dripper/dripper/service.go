package dripper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/op-dripper/metrics"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

var ErrAlreadyStopped = errors.New("already stopped")

type DripExecutorDriver interface {
	Start() error
	Stop() error
}

type DripExecutorConfig struct {
	PollInterval   time.Duration
	NetworkTimeout time.Duration

	DrippieAddr *common.Address
}

type DripExecutorService struct {
	Log     log.Logger
	Metrics metrics.Metricer

	DripExecutorConfig

	TxManager txmgr.TxManager
	Client    *ethclient.Client

	driver *DripExecutor

	Version string

	pprofService *oppprof.Service
	metricsSrv   *httputil.HTTPServer
	rpcServer    *oprpc.Server

	balanceMetricer io.Closer

	stopped atomic.Bool
}

func DripExecutorServiceFromCLIConfig(ctx context.Context, version string, cfg *CLIConfig, log log.Logger) (*DripExecutorService, error) {
	var ds DripExecutorService
	if err := ds.initFromCLIConfig(ctx, version, cfg, log); err != nil {
		return nil, errors.Join(err, ds.Stop(ctx))
	}
	return &ds, nil
}

func (ds *DripExecutorService) initFromCLIConfig(ctx context.Context, version string, cfg *CLIConfig, log log.Logger) error {
	ds.Version = version
	ds.Log = log

	ds.initMetrics(cfg)

	ds.PollInterval = cfg.PollInterval
	ds.NetworkTimeout = cfg.TxMgrConfig.NetworkTimeout

	ds.initDrippieAddress(cfg)

	if err := ds.initRPCClients(ctx, cfg); err != nil {
		return err
	}
	if err := ds.initTxManager(cfg); err != nil {
		return fmt.Errorf("failed to init tx manager: %w", err)
	}
	if err := ds.initMetricsServer(cfg); err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	if err := ds.initPProf(cfg); err != nil {
		return fmt.Errorf("failed to init pprof server: %w", err)
	}
	if err := ds.initDriver(); err != nil {
		return fmt.Errorf("failed to init driver: %w", err)
	}
	if err := ds.initRPCServer(cfg); err != nil {
		return fmt.Errorf("failed to start rpc server: %w", err)
	}

	ds.initBalanceMonitor(cfg)

	ds.Metrics.RecordInfo(ds.Version)
	ds.Metrics.RecordUp()
	return nil
}

func (ds *DripExecutorService) initRPCClients(ctx context.Context, cfg *CLIConfig) error {
	client, err := dial.DialEthClientWithTimeout(ctx, dial.DefaultDialTimeout, ds.Log, cfg.L1EthRpc)
	if err != nil {
		return fmt.Errorf("failed to dial rpc: %w", err)
	}
	ds.Client = client
	return nil
}

func (ds *DripExecutorService) initMetrics(cfg *CLIConfig) {
	if cfg.MetricsConfig.Enabled {
		procName := "default"
		ds.Metrics = metrics.NewMetrics(procName)
	} else {
		ds.Metrics = metrics.NoopMetrics
	}
}

func (ds *DripExecutorService) initBalanceMonitor(cfg *CLIConfig) {
	if cfg.MetricsConfig.Enabled {
		ds.balanceMetricer = ds.Metrics.StartBalanceMetrics(ds.Log, ds.Client, ds.TxManager.From())
	}
}

func (ds *DripExecutorService) initTxManager(cfg *CLIConfig) error {
	txManager, err := txmgr.NewSimpleTxManager("dripper", ds.Log, ds.Metrics, cfg.TxMgrConfig)
	if err != nil {
		return err
	}
	ds.TxManager = txManager
	return nil
}

func (ds *DripExecutorService) initPProf(cfg *CLIConfig) error {
	ds.pprofService = oppprof.New(
		cfg.PprofConfig.ListenEnabled,
		cfg.PprofConfig.ListenAddr,
		cfg.PprofConfig.ListenPort,
		cfg.PprofConfig.ProfileType,
		cfg.PprofConfig.ProfileDir,
		cfg.PprofConfig.ProfileFilename,
	)

	if err := ds.pprofService.Start(); err != nil {
		return fmt.Errorf("failed to start pprof service: %w", err)
	}

	return nil
}

func (ds *DripExecutorService) initMetricsServer(cfg *CLIConfig) error {
	if !cfg.MetricsConfig.Enabled {
		ds.Log.Info("metrics disabled")
		return nil
	}
	m, ok := ds.Metrics.(opmetrics.RegistryMetricer)
	if !ok {
		return fmt.Errorf("metrics were enabled, but metricer %T does not expose registry for metrics-server", ds.Metrics)
	}
	ds.Log.Debug("starting metrics server", "addr", cfg.MetricsConfig.ListenAddr, "port", cfg.MetricsConfig.ListenPort)
	metricsSrv, err := opmetrics.StartServer(m.Registry(), cfg.MetricsConfig.ListenAddr, cfg.MetricsConfig.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	ds.Log.Info("started metrics server", "addr", metricsSrv.Addr())
	ds.metricsSrv = metricsSrv
	return nil
}

func (ds *DripExecutorService) initDrippieAddress(cfg *CLIConfig) {
	drippieAddress, err := opservice.ParseAddress(cfg.DrippieAddress)
	if err != nil {
		return
	}
	ds.DrippieAddr = &drippieAddress
}

func (ds *DripExecutorService) initDriver() error {
	driver, err := NewDripExecutor(DriverSetup{
		Log:    ds.Log,
		Metr:   ds.Metrics,
		Cfg:    ds.DripExecutorConfig,
		Txmgr:  ds.TxManager,
		Client: ds.Client,
	})
	if err != nil {
		return err
	}
	ds.driver = driver
	return nil
}

func (ds *DripExecutorService) initRPCServer(cfg *CLIConfig) error {
	server := oprpc.NewServer(
		cfg.RPCConfig.ListenAddr,
		cfg.RPCConfig.ListenPort,
		ds.Version,
		oprpc.WithLogger(ds.Log),
	)
	if cfg.RPCConfig.EnableAdmin {
		server.AddAPI(ds.TxManager.API())
		ds.Log.Info("admin rpc enabled")
	}
	ds.Log.Info("starting json-rpc server")
	if err := server.Start(); err != nil {
		return fmt.Errorf("unable to start rpc server: %w", err)
	}
	ds.rpcServer = server
	return nil
}

func (ds *DripExecutorService) Start(ctx context.Context) error {
	return ds.driver.Start()
}

func (ds *DripExecutorService) Stopped() bool {
	return ds.stopped.Load()
}

func (ds *DripExecutorService) Kill() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return ds.Stop(ctx)
}

func (ds *DripExecutorService) Stop(ctx context.Context) error {
	if ds.Stopped() {
		return ErrAlreadyStopped
	}
	ds.Log.Info("stopping executor")

	var result error
	if ds.driver != nil {
		if err := ds.driver.Stop(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop executor: %w", err))
		}
	}

	if ds.rpcServer != nil {
		if err := ds.rpcServer.Stop(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop rpc server: %w", err))
		}
	}

	if ds.pprofService != nil {
		if err := ds.pprofService.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop pprof server: %w", err))
		}
	}

	if ds.balanceMetricer != nil {
		if err := ds.balanceMetricer.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close balance metricer: %w", err))
		}
	}

	if ds.TxManager != nil {
		ds.TxManager.Close()
	}

	if ds.metricsSrv != nil {
		if err := ds.metricsSrv.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop metrics server: %w", err))
		}
	}

	if ds.Client != nil {
		ds.Client.Close()
	}

	if result == nil {
		ds.stopped.Store(true)
		ds.Log.Info("stopped executor")
	}

	return result
}

var _ cliapp.Lifecycle = (*DripExecutorService)(nil)

func (ds *DripExecutorService) Driver() DripExecutorDriver {
	return ds.driver
}
