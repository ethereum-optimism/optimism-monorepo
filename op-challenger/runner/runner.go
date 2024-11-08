package runner

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrUnexpectedStatusCode = errors.New("unexpected status code")
)

type Metricer interface {
	contractMetrics.ContractMetricer

	RecordVmExecutionTime(vmType string, t time.Duration)
	RecordVmMemoryUsed(vmType string, memoryUsed uint64)
	RecordFailure(vmType string)
	RecordInvalid(vmType string)
	RecordSuccess(vmType string)
}

type RunConfig struct {
	TraceType types.TraceType
	Name      string
	Prestate  common.Hash
}

type Runner struct {
	log        log.Logger
	cfg        *config.Config
	runConfigs []RunConfig
	m          Metricer

	running    atomic.Bool
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	metricsSrv *httputil.HTTPServer
}

func NewRunner(logger log.Logger, cfg *config.Config, runConfigs []RunConfig) *Runner {
	return &Runner{
		log:        logger,
		cfg:        cfg,
		runConfigs: runConfigs,
		m:          NewMetrics(),
	}
}

func (r *Runner) Start(ctx context.Context) error {
	if !r.running.CompareAndSwap(false, true) {
		return errors.New("already started")
	}
	ctx, cancel := context.WithCancel(ctx)
	r.ctx = ctx
	r.cancel = cancel
	if err := r.initMetricsServer(&r.cfg.MetricsConfig); err != nil {
		return fmt.Errorf("failed to start metrics: %w", err)
	}

	rollupClient, err := dial.DialRollupClientWithTimeout(ctx, 1*time.Minute, r.log, r.cfg.RollupRpc)
	if err != nil {
		return fmt.Errorf("failed to dial rollup client: %w", err)
	}

	l1Client, err := dial.DialRPCClientWithTimeout(ctx, 1*time.Minute, r.log, r.cfg.L1EthRpc)
	if err != nil {
		return fmt.Errorf("failed to dial l1 client: %w", err)
	}
	caller := batching.NewMultiCaller(l1Client, batching.DefaultBatchSize)

	for _, runConfig := range r.runConfigs {
		r.wg.Add(1)
		go r.loop(ctx, runConfig, rollupClient, caller)
	}

	r.log.Info("Runners started", "num", len(r.runConfigs))
	return nil
}

func (r *Runner) loop(ctx context.Context, runConfig RunConfig, client *sources.RollupClient, caller *batching.MultiCaller) {
	defer r.wg.Done()
	t := time.NewTicker(1 * time.Minute)
	defer t.Stop()
	for {
		r.runAndRecordOnce(ctx, runConfig, client, caller)
		select {
		case <-t.C:
		case <-ctx.Done():
			return
		}
	}
}

func (r *Runner) runAndRecordOnce(ctx context.Context, runConfig RunConfig, client *sources.RollupClient, caller *batching.MultiCaller) {
	recordError := func(err error, traceType string, m Metricer, log log.Logger) {
		if errors.Is(err, ErrUnexpectedStatusCode) {
			log.Error("Incorrect status code", "type", runConfig.Name, "err", err)
			m.RecordInvalid(traceType)
		} else if err != nil {
			log.Error("Failed to run", "type", runConfig.Name, "err", err)
			m.RecordFailure(traceType)
		} else {
			log.Info("Successfully verified output root", "type", runConfig.Name)
			m.RecordSuccess(traceType)
		}
	}

	prestateHash := runConfig.Prestate
	if prestateHash == (common.Hash{}) {
		hash, err := r.getPrestateHash(ctx, runConfig.TraceType, caller)
		if err != nil {
			recordError(err, runConfig.Name, r.m, r.log)
			return
		}
		prestateHash = hash
	}

	localInputs, err := r.createGameInputs(ctx, client)
	if err != nil {
		recordError(err, runConfig.Name, r.m, r.log)
		return
	}

	inputsLogger := r.log.New("l1", localInputs.L1Head, "l2", localInputs.L2Head, "l2Block", localInputs.L2BlockNumber, "claim", localInputs.L2Claim)
	// Sanitize the directory name.
	safeName := regexp.MustCompile("[^a-zA-Z0-9_-]").ReplaceAllString(runConfig.Name, "")
	dir, err := r.prepDatadir(safeName)
	if err != nil {
		recordError(err, runConfig.Name, r.m, r.log)
		return
	}
	err = r.runOnce(ctx, inputsLogger.With("type", runConfig.Name), runConfig.Name, runConfig.TraceType, prestateHash, localInputs, dir)
	recordError(err, runConfig.Name, r.m, r.log)
}

func (r *Runner) runOnce(ctx context.Context, logger log.Logger, name string, traceType types.TraceType, prestateHash common.Hash, localInputs utils.LocalGameInputs, dir string) error {
	provider, err := createTraceProvider(ctx, logger, metrics.NewVmMetrics(r.m, name), r.cfg, prestateHash, traceType, localInputs, dir)
	if err != nil {
		return fmt.Errorf("failed to create trace provider: %w", err)
	}
	hash, err := provider.Get(ctx, types.RootPosition)
	if err != nil {
		return fmt.Errorf("failed to execute trace provider: %w", err)
	}
	if hash[0] != mipsevm.VMStatusValid {
		return fmt.Errorf("%w: %v", ErrUnexpectedStatusCode, hash)
	}
	return nil
}

func (r *Runner) prepDatadir(name string) (string, error) {
	dir := filepath.Join(r.cfg.Datadir, name)
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("failed to remove old dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data dir (%v): %w", dir, err)
	}
	return dir, nil
}

func (r *Runner) createGameInputs(ctx context.Context, client *sources.RollupClient) (utils.LocalGameInputs, error) {
	status, err := client.SyncStatus(ctx)
	if err != nil {
		return utils.LocalGameInputs{}, fmt.Errorf("failed to get rollup sync status: %w", err)
	}

	if status.FinalizedL2.Number == 0 {
		return utils.LocalGameInputs{}, errors.New("safe head is 0")
	}
	l1Head := status.FinalizedL1
	if status.FinalizedL1.Number > status.CurrentL1.Number {
		// Restrict the L1 head to a block that has actually be processed by op-node.
		// This only matters if op-node is behind and hasn't processed all finalized L1 blocks yet.
		l1Head = status.CurrentL1
	}
	if l1Head.Number == 0 {
		return utils.LocalGameInputs{}, errors.New("l1 head is 0")
	}
	blockNumber, err := r.findL2BlockNumberToDispute(ctx, client, l1Head.Number, status.FinalizedL2.Number)
	if err != nil {
		return utils.LocalGameInputs{}, fmt.Errorf("failed to find l2 block number to dispute: %w", err)
	}
	claimOutput, err := client.OutputAtBlock(ctx, blockNumber)
	if err != nil {
		return utils.LocalGameInputs{}, fmt.Errorf("failed to get claim output: %w", err)
	}
	parentOutput, err := client.OutputAtBlock(ctx, blockNumber-1)
	if err != nil {
		return utils.LocalGameInputs{}, fmt.Errorf("failed to get claim output: %w", err)
	}
	localInputs := utils.LocalGameInputs{
		L1Head:        l1Head.Hash,
		L2Head:        parentOutput.BlockRef.Hash,
		L2OutputRoot:  common.Hash(parentOutput.OutputRoot),
		L2Claim:       common.Hash(claimOutput.OutputRoot),
		L2BlockNumber: new(big.Int).SetUint64(blockNumber),
	}
	return localInputs, nil
}

func (r *Runner) findL2BlockNumberToDispute(ctx context.Context, client *sources.RollupClient, l1HeadNum uint64, l2BlockNum uint64) (uint64, error) {
	// Try to find a L1 block prior to the batch that make l2BlockNum safe
	// Limits how far back we search to 10 * 32 blocks
	const skipSize = uint64(32)
	for i := 0; i < 10; i++ {
		if l1HeadNum < skipSize {
			// Too close to genesis, give up and just use the original block
			r.log.Info("Failed to find prior batch.")
			return l2BlockNum, nil
		}
		l1HeadNum -= skipSize
		priorSafeHead, err := client.SafeHeadAtL1Block(ctx, l1HeadNum)
		if err != nil {
			return 0, fmt.Errorf("failed to get prior safe head at L1 block %v: %w", l1HeadNum, err)
		}
		if priorSafeHead.SafeHead.Number < l2BlockNum {
			// We walked back far enough to be before the batch that included l2BlockNum
			// So use the first block after the prior safe head as the disputed block.
			// It must be the first block in a batch.
			return priorSafeHead.SafeHead.Number + 1, nil
		}
	}
	r.log.Warn("Failed to find prior batch", "l2BlockNum", l2BlockNum, "earliestCheckL1Block", l1HeadNum)
	return l2BlockNum, nil
}

func (r *Runner) getPrestateHash(ctx context.Context, traceType types.TraceType, caller *batching.MultiCaller) (common.Hash, error) {
	gameFactory := contracts.NewDisputeGameFactoryContract(r.m, r.cfg.GameFactoryAddress, caller)
	gameImplAddr, err := gameFactory.GetGameImpl(ctx, traceType.GameType())
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to load game impl: %w", err)
	}
	if gameImplAddr == (common.Address{}) {
		return common.Hash{}, nil // No prestate is set, will only work if a single prestate is specified
	}
	gameImpl, err := contracts.NewFaultDisputeGameContract(ctx, r.m, gameImplAddr, caller)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create fault dispute game contract bindings for %v: %w", gameImplAddr, err)
	}
	prestateHash, err := gameImpl.GetAbsolutePrestateHash(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get absolute prestate hash for %v: %w", gameImplAddr, err)
	}
	return prestateHash, err
}

func (r *Runner) Stop(ctx context.Context) error {
	r.log.Info("Stopping")
	if !r.running.CompareAndSwap(true, false) {
		return errors.New("not started")
	}
	r.cancel()
	r.wg.Wait()

	if r.metricsSrv != nil {
		return r.metricsSrv.Stop(ctx)
	}
	return nil
}

func (r *Runner) Stopped() bool {
	return !r.running.Load()
}

func (r *Runner) initMetricsServer(cfg *opmetrics.CLIConfig) error {
	if !cfg.Enabled {
		return nil
	}
	r.log.Debug("Starting metrics server", "addr", cfg.ListenAddr, "port", cfg.ListenPort)
	m, ok := r.m.(opmetrics.RegistryMetricer)
	if !ok {
		return fmt.Errorf("metrics were enabled, but metricer %T does not expose registry for metrics-server", r.m)
	}
	metricsSrv, err := opmetrics.StartServer(m.Registry(), cfg.ListenAddr, cfg.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	r.log.Info("started metrics server", "addr", metricsSrv.Addr())
	r.metricsSrv = metricsSrv
	return nil
}

var _ cliapp.Lifecycle = (*Runner)(nil)
