package backend

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/cross"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/sync"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/l1access"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/processors"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/syncnode"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/frontend"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type SupervisorBackend struct {
	started atomic.Bool
	logger  log.Logger
	m       Metrics
	dataDir string

	// depSet is the dependency set that the backend uses to know about the chains it is indexing
	depSet depset.DependencySet

	// chainDBs is the primary interface to the databases, including logs, derived-from information and L1 finalization
	chainDBs *db.ChainsDB

	// l1Accessor provides access to the L1 chain for the L1 processor and subscribes to new block events
	l1Accessor *l1access.L1Accessor

	// chainProcessors are notified of new unsafe blocks, and add the unsafe log events data into the events DB
	chainProcessors locks.RWMap[types.ChainID, *processors.ChainProcessor]
	// crossProcessors are used to index cross-chain dependency validity data once the log events are indexed
	crossSafeProcessors   locks.RWMap[types.ChainID, *cross.Worker]
	crossUnsafeProcessors locks.RWMap[types.ChainID, *cross.Worker]

	// syncNodesController controls the derivation or reset of the sync nodes
	syncNodesController *syncnode.SyncNodesController

	// synchronousProcessors disables background-workers,
	// requiring manual triggers for the backend to process l2 data.
	synchronousProcessors bool

	// chainMetrics are used to track metrics for each chain
	// they are reused for processors and databases of the same chain
	chainMetrics locks.RWMap[types.ChainID, *chainMetrics]
}

var _ frontend.Backend = (*SupervisorBackend)(nil)

var errAlreadyStopped = errors.New("already stopped")

func NewSupervisorBackend(ctx context.Context, logger log.Logger, m Metrics, cfg *config.Config) (*SupervisorBackend, error) {
	// attempt to prepare the data directory
	if err := db.PrepDataDir(cfg.Datadir); err != nil {
		return nil, err
	}

	// Load the dependency set
	depSet, err := cfg.DependencySetSource.LoadDependencySet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load dependency set: %w", err)
	}

	// Sync the databases from the remote server if configured
	// We only attempt to sync a database if it doesn't exist; we don't update existing databases
	if cfg.DatadirSyncEndpoint != "" {
		syncCfg := sync.Config{DataDir: cfg.Datadir, Logger: logger}
		syncClient, err := sync.NewClient(syncCfg, cfg.DatadirSyncEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to create db sync client: %w", err)
		}
		if err := syncClient.SyncAll(ctx, depSet.Chains(), false); err != nil {
			return nil, fmt.Errorf("failed to sync databases: %w", err)
		}
	}

	// create initial per-chain resources
	chainsDBs := db.NewChainsDB(logger, depSet)

	l1Accessor := l1access.NewL1Accessor(
		logger,
		nil,
		processors.MaybeUpdateFinalizedL1Fn(context.Background(), logger, chainsDBs),
	)

	// create the supervisor backend
	super := &SupervisorBackend{
		logger:     logger,
		m:          m,
		dataDir:    cfg.Datadir,
		depSet:     depSet,
		chainDBs:   chainsDBs,
		l1Accessor: l1Accessor,
		// For testing we can avoid running the processors.
		synchronousProcessors: cfg.SynchronousProcessors,
	}

	// create node controller
	super.syncNodesController = syncnode.NewSyncNodesController(logger, depSet, chainsDBs, super)

	// Initialize the resources of the supervisor backend.
	// Stop the supervisor if any of the resources fails to be initialized.
	if err := super.initResources(ctx, cfg); err != nil {
		err = fmt.Errorf("failed to init resources: %w", err)
		return nil, errors.Join(err, super.Stop(ctx))
	}

	return super, nil
}

// initResources initializes all the resources, such as DBs and processors for chains.
// An error may returned, without closing the thus-far initialized resources.
// Upon error the caller should call Stop() on the supervisor backend to clean up and release resources.
func (su *SupervisorBackend) initResources(ctx context.Context, cfg *config.Config) error {
	chains := su.depSet.Chains()

	// for each chain known to the dependency set, create the necessary DB resources
	for _, chainID := range chains {
		if err := su.openChainDBs(chainID); err != nil {
			return fmt.Errorf("failed to open chain %s: %w", chainID, err)
		}
	}

	// initialize all cross-unsafe processors
	for _, chainID := range chains {
		worker := cross.NewCrossUnsafeWorker(su.logger, chainID, su.chainDBs)
		su.crossUnsafeProcessors.Set(chainID, worker)
	}
	// initialize all cross-safe processors
	for _, chainID := range chains {
		worker := cross.NewCrossSafeWorker(su.logger, chainID, su.chainDBs)
		su.crossSafeProcessors.Set(chainID, worker)
	}
	// For each chain initialize a chain processor service,
	// after cross-unsafe workers are ready to receive updates
	for _, chainID := range chains {
		logProcessor := processors.NewLogProcessor(chainID, su.chainDBs)
		chainProcessor := processors.NewChainProcessor(su.logger, chainID, logProcessor, su.chainDBs, su.onIndexedLocalUnsafeData)
		su.chainProcessors.Set(chainID, chainProcessor)
	}

	if cfg.L1RPC != "" {
		if err := su.attachL1RPC(ctx, cfg.L1RPC); err != nil {
			return fmt.Errorf("failed to create L1 processor: %w", err)
		}
	} else {
		su.logger.Warn("No L1 RPC configured, L1 processor will not be started")
	}

	setups, err := cfg.SyncSources.Load(ctx, su.logger)
	if err != nil {
		return fmt.Errorf("failed to load sync-source setups: %w", err)
	}
	// the config has some sync sources (RPC connections) to attach to the chain-processors
	for _, srcSetup := range setups {
		src, err := srcSetup.Setup(ctx, su.logger)
		if err != nil {
			return fmt.Errorf("failed to set up sync source: %w", err)
		}
		if _, err := su.AttachSyncNode(ctx, src, false); err != nil {
			return fmt.Errorf("failed to attach sync source %s: %w", src, err)
		}
	}
	return nil
}

// onIndexedLocalUnsafeData is called by the event indexing workers.
// This signals to cross-unsafe workers that there's data to index.
func (su *SupervisorBackend) onIndexedLocalUnsafeData() {
	// We signal all workers, since dependencies on a chain may be unblocked
	// by new data on other chains.
	// Busy workers don't block processing.
	// The signal is picked up only if the worker is running in the background.
	su.crossUnsafeProcessors.Range(func(_ types.ChainID, w *cross.Worker) bool {
		w.OnNewData()
		return true
	})
}

// onNewLocalSafeData is called by the safety-indexing.
// This signals to cross-safe workers that there's data to index.
func (su *SupervisorBackend) onNewLocalSafeData() {
	// We signal all workers, since dependencies on a chain may be unblocked
	// by new data on other chains.
	// Busy workers don't block processing.
	// The signal is picked up only if the worker is running in the background.
	su.crossSafeProcessors.Range(func(_ types.ChainID, w *cross.Worker) bool {
		w.OnNewData()
		return true
	})
}

// openChainDBs initializes all the DB resources of a specific chain.
// It is a sub-task of initResources.
func (su *SupervisorBackend) openChainDBs(chainID types.ChainID) error {
	cm := newChainMetrics(chainID, su.m)
	// create metrics and a logdb for the chain
	su.chainMetrics.Set(chainID, cm)

	logDB, err := db.OpenLogDB(su.logger, chainID, su.dataDir, cm)
	if err != nil {
		return fmt.Errorf("failed to open logDB of chain %s: %w", chainID, err)
	}
	su.chainDBs.AddLogDB(chainID, logDB)

	localDB, err := db.OpenLocalDerivedFromDB(su.logger, chainID, su.dataDir, cm)
	if err != nil {
		return fmt.Errorf("failed to open local derived-from DB of chain %s: %w", chainID, err)
	}
	su.chainDBs.AddLocalDerivedFromDB(chainID, localDB)

	crossDB, err := db.OpenCrossDerivedFromDB(su.logger, chainID, su.dataDir, cm)
	if err != nil {
		return fmt.Errorf("failed to open cross derived-from DB of chain %s: %w", chainID, err)
	}
	su.chainDBs.AddCrossDerivedFromDB(chainID, crossDB)

	su.chainDBs.AddCrossUnsafeTracker(chainID)

	su.chainDBs.AddSubscriptions(chainID)

	return nil
}

// AttachSyncNode attaches a node to be managed by the supervisor.
// If noSubscribe, the node is not actively polled/subscribed to, and requires manual Node.PullEvents calls.
func (su *SupervisorBackend) AttachSyncNode(ctx context.Context, src syncnode.SyncNode, noSubscribe bool) (syncnode.Node, error) {
	su.logger.Info("attaching sync source to chain processor", "source", src)

	chainID, err := src.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to identify chain ID of sync source: %w", err)
	}
	if !su.depSet.HasChain(chainID) {
		return nil, fmt.Errorf("chain %s is not part of the interop dependency set: %w", chainID, types.ErrUnknownChain)
	}
	err = su.AttachProcessorSource(chainID, src)
	if err != nil {
		return nil, fmt.Errorf("failed to attach sync source to processor: %w", err)
	}
	return su.syncNodesController.AttachNodeController(chainID, src, noSubscribe)
}

func (su *SupervisorBackend) AttachProcessorSource(chainID types.ChainID, src processors.Source) error {
	proc, ok := su.chainProcessors.Get(chainID)
	if !ok {
		return fmt.Errorf("unknown chain %s, cannot attach RPC to processor", chainID)
	}
	proc.SetSource(src)
	return nil
}

func (su *SupervisorBackend) attachL1RPC(ctx context.Context, l1RPCAddr string) error {
	su.logger.Info("attaching L1 RPC to L1 processor", "rpc", l1RPCAddr)

	logger := su.logger.New("l1-rpc", l1RPCAddr)
	l1RPC, err := client.NewRPC(ctx, logger, l1RPCAddr)
	if err != nil {
		return fmt.Errorf("failed to setup L1 RPC: %w", err)
	}
	l1Client, err := sources.NewL1Client(
		l1RPC,
		su.logger,
		nil,
		// placeholder config for the L1
		sources.L1ClientSimpleConfig(true, sources.RPCKindBasic, 100))
	if err != nil {
		return fmt.Errorf("failed to setup L1 Client: %w", err)
	}
	su.AttachL1Source(l1Client)
	return nil
}

// AttachL1Source attaches an L1 source to the L1 accessor
// if the L1 accessor does not exist, it is created
// if an L1 source is already attached, it is replaced
func (su *SupervisorBackend) AttachL1Source(source l1access.L1Source) {
	su.l1Accessor.AttachClient(source)
}

func (su *SupervisorBackend) Start(ctx context.Context) error {
	// ensure we only start once
	if !su.started.CompareAndSwap(false, true) {
		return errors.New("already started")
	}

	// initiate "ResumeFromLastSealedBlock" on the chains db,
	// which rewinds the database to the last block that is guaranteed to have been fully recorded
	if err := su.chainDBs.ResumeFromLastSealedBlock(); err != nil {
		return fmt.Errorf("failed to resume chains db: %w", err)
	}

	if !su.synchronousProcessors {
		// Make all the chain-processors run automatic background processing
		su.chainProcessors.Range(func(_ types.ChainID, processor *processors.ChainProcessor) bool {
			processor.StartBackground()
			return true
		})
		su.crossUnsafeProcessors.Range(func(_ types.ChainID, worker *cross.Worker) bool {
			worker.StartBackground()
			return true
		})
		su.crossSafeProcessors.Range(func(_ types.ChainID, worker *cross.Worker) bool {
			worker.StartBackground()
			return true
		})
	}

	return nil
}

func (su *SupervisorBackend) Stop(ctx context.Context) error {
	if !su.started.CompareAndSwap(true, false) {
		return errAlreadyStopped
	}
	su.logger.Info("Closing supervisor backend")

	// close all processors
	su.chainProcessors.Range(func(id types.ChainID, processor *processors.ChainProcessor) bool {
		su.logger.Info("stopping chain processor", "chainID", id)
		processor.Close()
		return true
	})
	su.chainProcessors.Clear()

	su.crossUnsafeProcessors.Range(func(id types.ChainID, worker *cross.Worker) bool {
		su.logger.Info("stopping cross-unsafe processor", "chainID", id)
		worker.Close()
		return true
	})
	su.crossUnsafeProcessors.Clear()

	su.crossSafeProcessors.Range(func(id types.ChainID, worker *cross.Worker) bool {
		su.logger.Info("stopping cross-safe processor", "chainID", id)
		worker.Close()
		return true
	})
	su.crossSafeProcessors.Clear()

	su.syncNodesController.Close()

	// close the databases
	return su.chainDBs.Close()
}

// AddL2RPC attaches an RPC as the RPC for the given chain, overriding the previous RPC source, if any.
func (su *SupervisorBackend) AddL2RPC(ctx context.Context, rpc string, jwtSecret eth.Bytes32) error {
	setupSrc := &syncnode.RPCDialSetup{
		JWTSecret: jwtSecret,
		Endpoint:  rpc,
	}
	src, err := setupSrc.Setup(ctx, su.logger)
	if err != nil {
		return fmt.Errorf("failed to set up sync source from RPC: %w", err)
	}
	_, err = su.AttachSyncNode(ctx, src, false)
	return err
}

// Internal methods, for processors
// ----------------------------

func (su *SupervisorBackend) DependencySet() depset.DependencySet {
	return su.depSet
}

// Query methods
// ----------------------------

func (su *SupervisorBackend) CheckMessage(identifier types.Identifier, payloadHash common.Hash) (types.SafetyLevel, error) {
	logHash := types.PayloadHashToLogHash(payloadHash, identifier.Origin)
	chainID := identifier.ChainID
	blockNum := identifier.BlockNumber
	logIdx := identifier.LogIndex
	_, err := su.chainDBs.Check(chainID, blockNum, logIdx, logHash)
	if errors.Is(err, types.ErrFuture) {
		su.logger.Debug("Future message", "identifier", identifier, "payloadHash", payloadHash, "err", err)
		return types.LocalUnsafe, nil
	}
	if errors.Is(err, types.ErrConflict) {
		su.logger.Debug("Conflicting message", "identifier", identifier, "payloadHash", payloadHash, "err", err)
		return types.Invalid, nil
	}
	if err != nil {
		return types.Invalid, fmt.Errorf("failed to check log: %w", err)
	}
	return su.chainDBs.Safest(chainID, blockNum, logIdx)
}

func (su *SupervisorBackend) CheckMessages(
	messages []types.Message,
	minSafety types.SafetyLevel) error {
	su.logger.Debug("Checking messages", "count", len(messages), "minSafety", minSafety)

	for _, msg := range messages {
		su.logger.Debug("Checking message",
			"identifier", msg.Identifier, "payloadHash", msg.PayloadHash.String())
		safety, err := su.CheckMessage(msg.Identifier, msg.PayloadHash)
		if err != nil {
			su.logger.Error("Check message failed", "err", err,
				"identifier", msg.Identifier, "payloadHash", msg.PayloadHash.String())
			return fmt.Errorf("failed to check message: %w", err)
		}
		if !safety.AtLeastAsSafe(minSafety) {
			su.logger.Error("Message is not sufficiently safe",
				"safety", safety, "minSafety", minSafety,
				"identifier", msg.Identifier, "payloadHash", msg.PayloadHash.String())
			return fmt.Errorf("message %v (safety level: %v) does not meet the minimum safety %v",
				msg.Identifier,
				safety,
				minSafety)
		}
	}
	return nil
}

func (su *SupervisorBackend) CrossSafe(ctx context.Context, chainID types.ChainID) (types.DerivedIDPair, error) {
	p, err := su.chainDBs.CrossSafe(chainID)
	if err != nil {
		return types.DerivedIDPair{}, err
	}
	return types.DerivedIDPair{
		DerivedFrom: p.DerivedFrom.ID(),
		Derived:     p.Derived.ID(),
	}, nil
}

func (su *SupervisorBackend) LocalSafe(ctx context.Context, chainID types.ChainID) (types.DerivedIDPair, error) {
	p, err := su.chainDBs.LocalSafe(chainID)
	if err != nil {
		return types.DerivedIDPair{}, err
	}
	return types.DerivedIDPair{
		DerivedFrom: p.DerivedFrom.ID(),
		Derived:     p.Derived.ID(),
	}, nil
}

func (su *SupervisorBackend) LocalUnsafe(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	v, err := su.chainDBs.LocalUnsafe(chainID)
	if err != nil {
		return eth.BlockID{}, err
	}
	return v.ID(), nil
}

func (su *SupervisorBackend) CrossUnsafe(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	v, err := su.chainDBs.CrossUnsafe(chainID)
	if err != nil {
		return eth.BlockID{}, err
	}
	return v.ID(), nil
}

func (su *SupervisorBackend) SafeDerivedAt(ctx context.Context, chainID types.ChainID, derivedFrom eth.BlockID) (eth.BlockID, error) {
	v, err := su.chainDBs.SafeDerivedAt(chainID, derivedFrom)
	if err != nil {
		return eth.BlockID{}, err
	}
	return v.ID(), nil
}

func (su *SupervisorBackend) Finalized(ctx context.Context, chainID types.ChainID) (eth.BlockID, error) {
	v, err := su.chainDBs.Finalized(chainID)
	if err != nil {
		return eth.BlockID{}, err
	}
	return v.ID(), nil
}

func (su *SupervisorBackend) FinalizedL1() eth.BlockRef {
	return su.chainDBs.FinalizedL1()
}

func (su *SupervisorBackend) CrossDerivedFrom(ctx context.Context, chainID types.ChainID, derived eth.BlockID) (derivedFrom eth.BlockRef, err error) {
	v, err := su.chainDBs.CrossDerivedFromBlockRef(chainID, derived)
	if err != nil {
		return eth.BlockRef{}, err
	}
	return v, nil
}

func (su *SupervisorBackend) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	return su.l1Accessor.L1BlockRefByNumber(ctx, number)
}

// Update methods
// ----------------------------

func (su *SupervisorBackend) UpdateLocalUnsafe(ctx context.Context, chainID types.ChainID, head eth.BlockRef) error {
	ch, ok := su.chainProcessors.Get(chainID)
	if !ok {
		return types.ErrUnknownChain
	}
	return ch.OnNewHead(head)
}

func (su *SupervisorBackend) UpdateLocalSafe(ctx context.Context, chainID types.ChainID, derivedFrom eth.BlockRef, lastDerived eth.BlockRef) error {
	err := su.chainDBs.UpdateLocalSafe(chainID, derivedFrom, lastDerived)
	if err != nil {
		return err
	}
	su.onNewLocalSafeData()
	return nil
}

// Access to synchronous processing for tests
// ----------------------------

func (su *SupervisorBackend) SyncEvents(chainID types.ChainID) error {
	ch, ok := su.chainProcessors.Get(chainID)
	if !ok {
		return types.ErrUnknownChain
	}
	ch.ProcessToHead()
	return nil
}

func (su *SupervisorBackend) SyncCrossUnsafe(chainID types.ChainID) error {
	ch, ok := su.crossUnsafeProcessors.Get(chainID)
	if !ok {
		return types.ErrUnknownChain
	}
	return ch.ProcessWork()
}

func (su *SupervisorBackend) SyncCrossSafe(chainID types.ChainID) error {
	ch, ok := su.crossSafeProcessors.Get(chainID)
	if !ok {
		return types.ErrUnknownChain
	}
	return ch.ProcessWork()
}

// SyncFinalizedL1 is a test-only method to update the finalized L1 block without the use of a subscription
func (su *SupervisorBackend) SyncFinalizedL1(ref eth.BlockRef) {
	fn := processors.MaybeUpdateFinalizedL1Fn(context.Background(), su.logger, su.chainDBs)
	fn(context.Background(), ref)
}
