package proposer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/op-proposer/metrics"
	"github.com/ethereum-optimism/optimism/op-proposer/proposer/rpc"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

var ErrAlreadyStopped = errors.New("already stopped")

type ProposerConfig struct {
	// How frequently to poll L2 for new finalized outputs
	PollInterval   time.Duration
	NetworkTimeout time.Duration

	// How frequently to post L2 outputs when the DisputeGameFactory is configured
	ProposalInterval time.Duration

	L2OutputOracleAddr     *common.Address
	DisputeGameFactoryAddr *common.Address
	DisputeGameType        uint32

	// AllowNonFinalized enables the proposal of safe, but non-finalized L2 blocks.
	// The L1 block-hash embedded in the proposal TX is checked and should ensure the proposal
	// is never valid on an alternative L1 chain that would produce different L2 data.
	// This option is not necessary when higher proposal latency is acceptable and L1 is healthy.
	AllowNonFinalized bool

	WaitNodeSync bool
}

type ProposerService struct {
	Log     log.Logger
	Metrics metrics.Metricer

	ProposerConfig

	TxManager      txmgr.TxManager
	L1Client       *ethclient.Client
	RollupProvider dial.RollupProvider

	driver *L2OutputSubmitter

	Version string

	pprofService *oppprof.Service
	metricsSrv   *httputil.HTTPServer
	rpcServer    *oprpc.Server

	balanceMetricer io.Closer

	stopped atomic.Bool
}

// ProposerServiceFromCLIConfig creates a new ProposerService from a CLIConfig.
// The service components are fully started, except for the driver,
// which will not be submitting state (if it was configured to) until the Start part of the lifecycle.
func ProposerServiceFromCLIConfig(ctx context.Context, version string, cfg *CLIConfig, log log.Logger) (*ProposerService, error) {
	var ps ProposerService
	if err := ps.initFromCLIConfig(ctx, version, cfg, log); err != nil {
		return nil, errors.Join(err, ps.Stop(ctx)) // try to clean up our failed initialization attempt
	}
	return &ps, nil
}

func (ps *ProposerService) initFromCLIConfig(ctx context.Context, version string, cfg *CLIConfig, log log.Logger) error {
	ps.Version = version
	ps.Log = log

	ps.initMetrics(cfg)

	ps.PollInterval = cfg.PollInterval
	ps.NetworkTimeout = cfg.TxMgrConfig.NetworkTimeout
	ps.AllowNonFinalized = cfg.AllowNonFinalized
	ps.WaitNodeSync = cfg.WaitNodeSync

	ps.initL2ooAddress(cfg)
	ps.initDGF(cfg)

	if err := ps.initRPCClients(ctx, cfg); err != nil {
		return err
	}
	if err := ps.initTxManager(cfg); err != nil {
		return fmt.Errorf("failed to init Tx manager: %w", err)
	}
	ps.initBalanceMonitor(cfg)
	if err := ps.initMetricsServer(cfg); err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	if err := ps.initPProf(cfg); err != nil {
		return fmt.Errorf("failed to init profiling: %w", err)
	}
	if err := ps.initDriver(); err != nil {
		return fmt.Errorf("failed to init Driver: %w", err)
	}
	if err := ps.initRPCServer(cfg); err != nil {
		return fmt.Errorf("failed to start RPC server: %w", err)
	}

	ps.Metrics.RecordInfo(ps.Version)
	ps.Metrics.RecordUp()
	return nil
}

func (ps *ProposerService) initRPCClients(ctx context.Context, cfg *CLIConfig) error {
	l1Client, err := dial.DialEthClientWithTimeout(ctx, dial.DefaultDialTimeout, ps.Log, cfg.L1EthRpc)
	if err != nil {
		return fmt.Errorf("failed to dial L1 RPC: %w", err)
	}
	ps.L1Client = l1Client

	var rollupProvider dial.RollupProvider
	if strings.Contains(cfg.RollupRpc, ",") {
		rollupUrls := strings.Split(cfg.RollupRpc, ",")
		rollupProvider, err = dial.NewActiveL2RollupProvider(ctx, rollupUrls, cfg.ActiveSequencerCheckDuration, dial.DefaultDialTimeout, ps.Log)
	} else {
		rollupProvider, err = dial.NewStaticL2RollupProvider(ctx, ps.Log, cfg.RollupRpc)
	}
	if err != nil {
		return fmt.Errorf("failed to build L2 endpoint provider: %w", err)
	}
	ps.RollupProvider = rollupProvider
	return nil
}

func (ps *ProposerService) initMetrics(cfg *CLIConfig) {
	if cfg.MetricsConfig.Enabled {
		procName := "default"
		ps.Metrics = metrics.NewMetrics(procName)
	} else {
		ps.Metrics = metrics.NoopMetrics
	}
}

// initBalanceMonitor depends on Metrics, L1Client and TxManager to start background-monitoring of the Proposer balance.
func (ps *ProposerService) initBalanceMonitor(cfg *CLIConfig) {
	if cfg.MetricsConfig.Enabled {
		ps.balanceMetricer = ps.Metrics.StartBalanceMetrics(ps.Log, ps.L1Client, ps.TxManager.From())
	}
}

func (ps *ProposerService) initTxManager(cfg *CLIConfig) error {
	txManager, err := txmgr.NewSimpleTxManager("proposer", ps.Log, ps.Metrics, cfg.TxMgrConfig)
	if err != nil {
		return err
	}
	ps.TxManager = txManager
	return nil
}

func (ps *ProposerService) initPProf(cfg *CLIConfig) error {
	ps.pprofService = oppprof.New(
		cfg.PprofConfig.ListenEnabled,
		cfg.PprofConfig.ListenAddr,
		cfg.PprofConfig.ListenPort,
		cfg.PprofConfig.ProfileType,
		cfg.PprofConfig.ProfileDir,
		cfg.PprofConfig.ProfileFilename,
	)

	if err := ps.pprofService.Start(); err != nil {
		return fmt.Errorf("failed to start pprof service: %w", err)
	}

	return nil
}

func (ps *ProposerService) initMetricsServer(cfg *CLIConfig) error {
	if !cfg.MetricsConfig.Enabled {
		ps.Log.Info("Metrics disabled")
		return nil
	}
	m, ok := ps.Metrics.(opmetrics.RegistryMetricer)
	if !ok {
		return fmt.Errorf("metrics were enabled, but metricer %T does not expose registry for metrics-server", ps.Metrics)
	}
	ps.Log.Debug("Starting metrics server", "addr", cfg.MetricsConfig.ListenAddr, "port", cfg.MetricsConfig.ListenPort)
	metricsSrv, err := opmetrics.StartServer(m.Registry(), cfg.MetricsConfig.ListenAddr, cfg.MetricsConfig.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	ps.Log.Info("Started metrics server", "addr", metricsSrv.Addr())
	ps.metricsSrv = metricsSrv
	return nil
}

func (ps *ProposerService) initL2ooAddress(cfg *CLIConfig) {
	l2ooAddress, err := opservice.ParseAddress(cfg.L2OOAddress)
	if err != nil {
		// Return no error & set no L2OO related configuration fields.
		return
	}
	ps.L2OutputOracleAddr = &l2ooAddress
}

func (ps *ProposerService) initDGF(cfg *CLIConfig) {
	dgfAddress, err := opservice.ParseAddress(cfg.DGFAddress)
	if err != nil {
		// Return no error & set no DGF related configuration fields.
		return
	}
	ps.DisputeGameFactoryAddr = &dgfAddress
	ps.ProposalInterval = cfg.ProposalInterval
	ps.DisputeGameType = cfg.DisputeGameType
}

func (ps *ProposerService) initDriver() error {
	driver, err := NewL2OutputSubmitter(DriverSetup{
		Log:            ps.Log,
		Metr:           ps.Metrics,
		Cfg:            ps.ProposerConfig,
		Txmgr:          ps.TxManager,
		L1Client:       ps.L1Client,
		Multicaller:    batching.NewMultiCaller(ps.L1Client.Client(), batching.DefaultBatchSize),
		RollupProvider: ps.RollupProvider,
	})
	if err != nil {
		return err
	}
	ps.driver = driver
	return nil
}

func (ps *ProposerService) initRPCServer(cfg *CLIConfig) error {
	server := oprpc.NewServer(
		cfg.RPCConfig.ListenAddr,
		cfg.RPCConfig.ListenPort,
		ps.Version,
		oprpc.WithLogger(ps.Log),
	)
	if cfg.RPCConfig.EnableAdmin {
		adminAPI := rpc.NewAdminAPI(ps.driver, ps.Metrics, ps.Log)
		server.AddAPI(rpc.GetAdminAPI(adminAPI))
		server.AddAPI(ps.TxManager.API())
		ps.Log.Info("Admin RPC enabled")
	}
	ps.Log.Info("Starting JSON-RPC server")
	if err := server.Start(); err != nil {
		return fmt.Errorf("unable to start RPC server: %w", err)
	}
	ps.rpcServer = server
	return nil
}

// Start runs once upon start of the proposer lifecycle,
// and starts L2Output-submission work if the proposer is configured to start submit data on startup.
func (ps *ProposerService) Start(_ context.Context) error {
	ps.Log.Info("Starting Proposer")
	return ps.driver.StartL2OutputSubmitting()
}

func (ps *ProposerService) Stopped() bool {
	return ps.stopped.Load()
}

// Kill is a convenience method to forcefully, non-gracefully, stop the ProposerService.
func (ps *ProposerService) Kill() error {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ps.Stop(ctx)
}

// Stop fully stops the L2Output-submitter and all its resources gracefully. After stopping, it cannot be restarted.
// See driver.StopL2OutputSubmitting to temporarily stop the L2Output submitter.
func (ps *ProposerService) Stop(ctx context.Context) error {
	if ps.stopped.Load() {
		return ErrAlreadyStopped
	}
	ps.Log.Info("Stopping Proposer")

	var result error
	if ps.driver != nil {
		if err := ps.driver.StopL2OutputSubmittingIfRunning(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop L2Output submitting: %w", err))
		}
	}

	if ps.rpcServer != nil {
		// TODO(7685): the op-service RPC server is not built on top of op-service httputil Server, and has poor shutdown
		if err := ps.rpcServer.Stop(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop RPC server: %w", err))
		}
	}
	if ps.pprofService != nil {
		if err := ps.pprofService.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop PProf server: %w", err))
		}
	}
	if ps.balanceMetricer != nil {
		if err := ps.balanceMetricer.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to close balance metricer: %w", err))
		}
	}

	if ps.TxManager != nil {
		ps.TxManager.Close()
	}

	if ps.metricsSrv != nil {
		if err := ps.metricsSrv.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("failed to stop metrics server: %w", err))
		}
	}

	if ps.L1Client != nil {
		ps.L1Client.Close()
	}

	if ps.RollupProvider != nil {
		ps.RollupProvider.Close()
	}

	if result == nil {
		ps.stopped.Store(true)
		ps.Log.Info("L2Output Submitter stopped")
	}

	return result
}

var _ cliapp.Lifecycle = (*ProposerService)(nil)

// Driver returns the handler on the L2Output-submitter driver element,
// to start/stop/restart the L2Output-submission work, for use in testing.
func (ps *ProposerService) Driver() rpc.ProposerDriver {
	return ps.driver
}
