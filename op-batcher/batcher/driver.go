package batcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	_ "net/http/pprof"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-batcher/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

var (
	ErrBatcherNotRunning = errors.New("batcher is not running")
	emptyTxData          = txData{
		frames: []frameData{
			{
				data: []byte{},
			},
		},
	}
	SetMaxDASizeMethod = "miner_setMaxDASize"
)

type txRef struct {
	id       txID
	isCancel bool
	isBlob   bool
}

func (r txRef) String() string {
	return r.string(func(id txID) string { return id.String() })
}

func (r txRef) TerminalString() string {
	return r.string(func(id txID) string { return id.TerminalString() })
}

func (r txRef) string(txIDStringer func(txID) string) string {
	if r.isCancel {
		if r.isBlob {
			return "blob-cancellation"
		} else {
			return "calldata-cancellation"
		}
	}
	return txIDStringer(r.id)
}

type L1Client interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
}

type L2Client interface {
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
}

type RollupClient interface {
	SyncStatus(ctx context.Context) (*eth.SyncStatus, error)
}

// DriverSetup is the collection of input/output interfaces and configuration that the driver operates on.
type DriverSetup struct {
	Log               log.Logger
	Metr              metrics.Metricer
	RollupConfig      *rollup.Config
	Config            BatcherConfig
	Txmgr             txmgr.TxManager
	L1Client          L1Client
	EndpointProvider  dial.L2EndpointProvider
	ChannelConfig     ChannelConfigProvider
	AltDA             *altda.DAClient
	ChannelOutFactory ChannelOutFactory
}

// BatchSubmitter encapsulates a service responsible for submitting L2 tx
// batches to L1 for availability.
type BatchSubmitter struct {
	DriverSetup

	wg sync.WaitGroup

	shutdownCtx       context.Context
	cancelShutdownCtx context.CancelFunc
	killCtx           context.Context
	cancelKillCtx     context.CancelFunc

	l2BlockAdded chan struct{} // notifies the throttling loop whenever an l2 block is added

	mutex   sync.Mutex
	running bool

	txpoolMutex       sync.Mutex // guards txpoolState and txpoolBlockedBlob
	txpoolState       int
	txpoolBlockedBlob bool

	// lastStoredBlock is the last block loaded into `state`. If it is empty it should be set to the l2 safe head.
	lastStoredBlock eth.BlockID
	lastL1Tip       eth.L1BlockRef

	state *channelManager
}

// NewBatchSubmitter initializes the BatchSubmitter driver from a preconfigured DriverSetup
func NewBatchSubmitter(setup DriverSetup) *BatchSubmitter {
	state := NewChannelManager(setup.Log, setup.Metr, setup.ChannelConfig, setup.RollupConfig)
	if setup.ChannelOutFactory != nil {
		state.SetChannelOutFactory(setup.ChannelOutFactory)
	}
	return &BatchSubmitter{
		DriverSetup: setup,
		state:       state,
	}
}

func (l *BatchSubmitter) StartBatchSubmitting() error {
	l.Log.Info("Starting Batch Submitter")

	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.running {
		return errors.New("batcher is already running")
	}
	l.running = true

	l.shutdownCtx, l.cancelShutdownCtx = context.WithCancel(context.Background())
	l.killCtx, l.cancelKillCtx = context.WithCancel(context.Background())
	l.clearState(l.shutdownCtx)
	l.lastStoredBlock = eth.BlockID{}

	if err := l.waitForL2Genesis(); err != nil {
		return fmt.Errorf("error waiting for L2 genesis: %w", err)
	}

	if l.Config.WaitNodeSync {
		err := l.waitNodeSync()
		if err != nil {
			return fmt.Errorf("error waiting for node sync: %w", err)
		}
	}

	l.wg.Add(1)
	go l.loop()

	l.Log.Info("Batch Submitter started")
	return nil
}

// waitForL2Genesis waits for the L2 genesis time to be reached.
func (l *BatchSubmitter) waitForL2Genesis() error {
	genesisTime := time.Unix(int64(l.RollupConfig.Genesis.L2Time), 0)
	now := time.Now()
	if now.After(genesisTime) {
		return nil
	}

	l.Log.Info("Waiting for L2 genesis", "genesisTime", genesisTime)

	// Create a ticker that fires every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	genesisTrigger := time.After(time.Until(genesisTime))

	for {
		select {
		case <-ticker.C:
			remaining := time.Until(genesisTime)
			l.Log.Info("Waiting for L2 genesis", "remainingTime", remaining.Round(time.Second))
		case <-genesisTrigger:
			l.Log.Info("L2 genesis time reached")
			return nil
		case <-l.shutdownCtx.Done():
			return errors.New("batcher stopped")
		}
	}
}

func (l *BatchSubmitter) StopBatchSubmittingIfRunning(ctx context.Context) error {
	err := l.StopBatchSubmitting(ctx)
	if errors.Is(err, ErrBatcherNotRunning) {
		return nil
	}
	return err
}

// StopBatchSubmitting stops the batch-submitter loop, and force-kills if the provided ctx is done.
func (l *BatchSubmitter) StopBatchSubmitting(ctx context.Context) error {
	l.Log.Info("Stopping Batch Submitter")

	l.mutex.Lock()
	defer l.mutex.Unlock()

	if !l.running {
		return ErrBatcherNotRunning
	}
	l.running = false

	// go routine will call cancelKill() if the passed in ctx is ever Done
	cancelKill := l.cancelKillCtx
	wrapped, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-wrapped.Done()
		cancelKill()
	}()

	l.cancelShutdownCtx()
	l.wg.Wait()
	l.cancelKillCtx()

	l.Log.Info("Batch Submitter stopped")
	return nil
}

// loadBlocksIntoState loads all blocks since the previous stored block
// It does the following:
//  1. Fetch the sync status of the sequencer
//  2. Check if the sync status is valid or if we are all the way up to date
//  3. Check if it needs to initialize state OR it is lagging (todo: lagging just means race condition?)
//  4. Load all new blocks into the local state.
//
// If there is a reorg, it will reset the last stored block but not clear the internal state so
// the state can be flushed to L1.
func (l *BatchSubmitter) loadBlocksIntoState(ctx context.Context) error {
	start, end, err := l.calculateL2BlockRangeToStore(ctx)
	if err != nil {
		l.Log.Warn("Error calculating L2 block range", "err", err)
		return err
	} else if start.Number >= end.Number {
		return errors.New("start number is >= end number")
	}

	var latestBlock *types.Block
	// Add all blocks to "state"
	for i := start.Number + 1; i < end.Number+1; i++ {
		block, err := l.loadBlockIntoState(ctx, i)
		if errors.Is(err, ErrReorg) {
			l.Log.Warn("Found L2 reorg", "block_number", i)
			l.lastStoredBlock = eth.BlockID{}
			return err
		} else if err != nil {
			l.Log.Warn("Failed to load block into state", "err", err)
			return err
		}
		l.lastStoredBlock = eth.ToBlockID(block)
		latestBlock = block
	}

	l2ref, err := derive.L2BlockToBlockRef(l.RollupConfig, latestBlock)
	if err != nil {
		l.Log.Warn("Invalid L2 block loaded into state", "err", err)
		return err
	}

	l.Metr.RecordL2BlocksLoaded(l2ref)
	return nil
}

// loadBlockIntoState fetches & stores a single block into `state`. It returns the block it loaded.
func (l *BatchSubmitter) loadBlockIntoState(ctx context.Context, blockNumber uint64) (*types.Block, error) {
	l2Client, err := l.EndpointProvider.EthClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting L2 client: %w", err)
	}

	cCtx, cancel := context.WithTimeout(ctx, l.Config.NetworkTimeout)
	defer cancel()

	block, err := l2Client.BlockByNumber(cCtx, new(big.Int).SetUint64(blockNumber))
	if err != nil {
		return nil, fmt.Errorf("getting L2 block: %w", err)
	}

	if err := l.state.AddL2Block(block); err != nil {
		return nil, fmt.Errorf("adding L2 block to state: %w", err)
	}

	// notify the throttling loop it may be time to initiate throttling without blocking
	select {
	case l.l2BlockAdded <- struct{}{}:
	default:
	}

	l.Log.Info("Added L2 block to local state", "block", eth.ToBlockID(block), "tx_count", len(block.Transactions()), "time", block.Time())
	return block, nil
}

// calculateL2BlockRangeToStore determines the range (start,end] that should be loaded into the local state.
// It also takes care of initializing some local state (i.e. will modify l.lastStoredBlock in certain conditions)
func (l *BatchSubmitter) calculateL2BlockRangeToStore(ctx context.Context) (eth.BlockID, eth.BlockID, error) {
	rollupClient, err := l.EndpointProvider.RollupClient(ctx)
	if err != nil {
		return eth.BlockID{}, eth.BlockID{}, fmt.Errorf("getting rollup client: %w", err)
	}

	var (
		syncStatus *eth.SyncStatus
		backoff    = time.Second
		maxBackoff = 30 * time.Second
	)
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	for {
		cCtx, cancel := context.WithTimeout(ctx, l.Config.NetworkTimeout)
		syncStatus, err = rollupClient.SyncStatus(cCtx)
		cancel()

		// Ensure that we have the sync status
		if err != nil {
			return eth.BlockID{}, eth.BlockID{}, fmt.Errorf("failed to get sync status: %w", err)
		}

		// If we have a head, break out of the loop
		if syncStatus.HeadL1 != (eth.L1BlockRef{}) {
			break
		}

		// Empty sync status, implement backoff
		l.Log.Info("Received empty sync status, backing off", "backoff", backoff)
		select {
		case <-timer.C:
			backoff *= 2
			backoff = min(backoff, maxBackoff)
			// Reset timer to tick of the new backoff time again
			timer.Reset(backoff)
		case <-ctx.Done():
			return eth.BlockID{}, eth.BlockID{}, ctx.Err()
		}
	}

	// Check last stored to see if it needs to be set on startup OR set if is lagged behind.
	// It lagging implies that the op-node processed some batches that were submitted prior to the current instance of the batcher being alive.
	if l.lastStoredBlock == (eth.BlockID{}) {
		l.Log.Info("Starting batch-submitter work at safe-head", "safe", syncStatus.SafeL2)
		l.lastStoredBlock = syncStatus.SafeL2.ID()
	} else if l.lastStoredBlock.Number < syncStatus.SafeL2.Number {
		l.Log.Warn("Last submitted block lagged behind L2 safe head: batch submission will continue from the safe head now", "last", l.lastStoredBlock, "safe", syncStatus.SafeL2)
		l.lastStoredBlock = syncStatus.SafeL2.ID()
	}

	// Check if we should even attempt to load any blocks. TODO: May not need this check
	if syncStatus.SafeL2.Number >= syncStatus.UnsafeL2.Number {
		return eth.BlockID{}, eth.BlockID{}, fmt.Errorf("L2 safe head(%d) ahead of L2 unsafe head(%d)", syncStatus.SafeL2.Number, syncStatus.UnsafeL2.Number)
	}

	return l.lastStoredBlock, syncStatus.UnsafeL2.ID(), nil
}

// The following things occur:
// New L2 block (reorg or not)
// L1 transaction is confirmed
//
// What the batcher does:
// Ensure that channels are created & submitted as frames for an L2 range
//
// Error conditions:
// Submitted batch, but it is not valid
// Missed L2 block somehow.

const (
	// Txpool states.  Possible state transitions:
	//   TxpoolGood -> TxpoolBlocked:
	//     happens when ErrAlreadyReserved is ever returned by the TxMgr.
	//   TxpoolBlocked -> TxpoolCancelPending:
	//     happens once the send loop detects the txpool is blocked, and results in attempting to
	//     send a cancellation transaction.
	//   TxpoolCancelPending -> TxpoolGood:
	//     happens once the cancel transaction completes, whether successfully or in error.
	TxpoolGood int = iota
	TxpoolBlocked
	TxpoolCancelPending
)

func (l *BatchSubmitter) loop() {
	defer l.wg.Done()

	queue := txmgr.NewQueue[txRef](l.killCtx, l.Txmgr, l.Config.MaxPendingTransactions)
	daGroup := &errgroup.Group{}
	// errgroup with limit of 0 means no goroutine is able to run concurrently,
	// so we only set the limit if it is greater than 0.
	if l.Config.MaxConcurrentDARequests > 0 {
		daGroup.SetLimit(int(l.Config.MaxConcurrentDARequests))
	}

	l.txpoolMutex.Lock()
	l.txpoolState = TxpoolGood
	l.txpoolMutex.Unlock()

	// start the receipt/result processing loop
	receiptsLoopDone := make(chan struct{})
	defer close(receiptsLoopDone) // shut down receipt loop
	l.l2BlockAdded = make(chan struct{})
	defer close(l.l2BlockAdded)
	receiptsCh := make(chan txmgr.TxReceipt[txRef])
	go l.processReceiptsLoop(receiptsCh, receiptsLoopDone)

	// DA throttling loop should always be started except for testing (indicated by ThrottleInterval == 0)
	if l.Config.ThrottleInterval > 0 {
		throttlingLoopDone := make(chan struct{})
		defer close(throttlingLoopDone)
		go l.throttlingLoop(throttlingLoopDone)
	} else {
		l.Log.Warn("Throttling loop is DISABLED due to 0 throttle-interval. This should not be disabled in prod.")
	}

	ticker := time.NewTicker(l.Config.PollInterval)
	defer ticker.Stop()

	publishAndWait := func() {
		l.publishStateToL1(queue, receiptsCh, daGroup, time.Duration(math.MaxInt64))
		if !l.Txmgr.IsClosed() {
			if l.Config.UseAltDA {
				l.Log.Info("Waiting for altDA writes to complete...")
				err := daGroup.Wait()
				if err != nil {
					l.Log.Error("Error returned by one of the altda goroutines waited on", "err", err)
				}
			}
			l.Log.Info("Waiting for L1 txs to be confirmed...")
			err := queue.Wait()
			if err != nil {
				l.Log.Error("Error returned by one of the txmgr goroutines waited on", "err", err)
			}
		} else {
			l.Log.Info("Txmgr is closed, remaining channel data won't be sent")
		}
	}

	for {
		select {
		case <-ticker.C:
			if !l.checkTxpool(queue, receiptsCh) {
				continue
			}
			if err := l.loadBlocksIntoState(l.shutdownCtx); errors.Is(err, ErrReorg) {
				err := l.state.Close()
				if err != nil {
					if errors.Is(err, ErrPendingAfterClose) {
						l.Log.Warn("Closed channel manager to handle L2 reorg with pending channel(s) remaining - submitting")
					} else {
						l.Log.Error("Error closing the channel manager to handle a L2 reorg", "err", err)
					}
				}
				// on reorg we want to publish all pending state then wait until each result clears before resetting
				// the state.
				publishAndWait()
				l.clearState(l.shutdownCtx)
				continue
			}
			l.publishStateToL1(queue, receiptsCh, daGroup, l.Config.PollInterval)
		case <-l.shutdownCtx.Done():
			if l.Txmgr.IsClosed() {
				l.Log.Info("Txmgr is closed, remaining channel data won't be sent")
				return
			}
			// This removes any never-submitted pending channels, so these do not have to be drained with transactions.
			// Any remaining unfinished channel is terminated, so its data gets submitted.
			err := l.state.Close()
			if err != nil {
				if errors.Is(err, ErrPendingAfterClose) {
					l.Log.Warn("Closed channel manager on shutdown with pending channel(s) remaining - submitting")
				} else {
					l.Log.Error("Error closing the channel manager on shutdown", "err", err)
				}
			}
			publishAndWait()
			l.Log.Info("Finished publishing all remaining channel data")
			return
		}
	}
}

func (l *BatchSubmitter) processReceiptsLoop(receiptsCh chan txmgr.TxReceipt[txRef], receiptsLoopDone chan struct{}) {
	l.Log.Info("Starting receipts processing loop")
	for {
		select {
		case r := <-receiptsCh:
			l.txpoolMutex.Lock()
			if errors.Is(r.Err, txpool.ErrAlreadyReserved) && l.txpoolState == TxpoolGood {
				l.txpoolState = TxpoolBlocked
				l.txpoolBlockedBlob = r.ID.isBlob
				l.Log.Info("incompatible tx in txpool", "is_blob", r.ID.isBlob)
			} else if r.ID.isCancel && l.txpoolState == TxpoolCancelPending {
				// Set state to TxpoolGood even if the cancellation transaction ended in error
				// since the stuck transaction could have cleared while we were waiting.
				l.txpoolState = TxpoolGood
				l.Log.Info("txpool may no longer be blocked", "err", r.Err)
			}
			l.txpoolMutex.Unlock()
			l.Log.Info("Handling receipt", "id", r.ID)
			l.handleReceipt(r)
		case <-receiptsLoopDone:
			l.Log.Info("Receipts processing loop done")
			return
		}
	}
}

// throttlingLoop monitors the backlog in bytes we need to make available, and appropriately enables or disables
// throttling of incoming data prevent the backlog from growing too large. By looping & calling the miner API setter
// continuously, we ensure the engine currently in use is always going to be reset to the proper throttling settings
// even in the event of sequencer failover.
func (l *BatchSubmitter) throttlingLoop(throttlingLoopDone chan struct{}) {
	l.Log.Info("Starting DA throttling loop")
	ticker := time.NewTicker(l.Config.ThrottleInterval)
	defer ticker.Stop()

	updateParams := func() {
		ctx, cancel := context.WithTimeout(l.shutdownCtx, l.Config.NetworkTimeout)
		defer cancel()
		cl, err := l.EndpointProvider.EthClient(ctx)
		if err != nil {
			l.Log.Error("Can't reach sequencer execution RPC", "err", err)
			return
		}
		pendingBytes := l.state.PendingDABytes()
		maxTxSize := uint64(0)
		maxBlockSize := l.Config.ThrottleAlwaysBlockSize
		if pendingBytes > int64(l.Config.ThrottleThreshold) {
			l.Log.Warn("Pending bytes over limit, throttling DA", "bytes", pendingBytes, "limit", l.Config.ThrottleThreshold)
			maxTxSize = l.Config.ThrottleTxSize
			if maxBlockSize == 0 || (l.Config.ThrottleBlockSize != 0 && l.Config.ThrottleBlockSize < maxBlockSize) {
				maxBlockSize = l.Config.ThrottleBlockSize
			}
		}
		var (
			success bool
			rpcErr  rpc.Error
		)
		if err := cl.Client().CallContext(
			ctx, &success, SetMaxDASizeMethod, hexutil.Uint64(maxTxSize), hexutil.Uint64(maxBlockSize),
		); errors.As(err, &rpcErr) && eth.ErrorCode(rpcErr.ErrorCode()).IsGenericRPCError() {
			l.Log.Error("SetMaxDASize rpc unavailable or broken, shutting down. Either enable it or disable throttling.", "err", err)
			// We'd probably hit this error right after startup, so a short shutdown duration should suffice.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			// Always returns nil. An error is only returned to expose this function as an RPC.
			_ = l.StopBatchSubmitting(ctx)
			return
		} else if err != nil {
			l.Log.Error("SetMaxDASize rpc failed, retrying.", "err", err)
			return
		}
		if !success {
			l.Log.Error("Result of SetMaxDASize was false, retrying.")
		}
	}

	for {
		select {
		case <-l.l2BlockAdded:
			updateParams()
		case <-ticker.C:
			updateParams()
		case <-throttlingLoopDone:
			l.Log.Info("DA throttling loop done")
			return
		}
	}
}

// waitNodeSync Check to see if there was a batcher tx sent recently that
// still needs more block confirmations before being considered finalized
func (l *BatchSubmitter) waitNodeSync() error {
	ctx := l.shutdownCtx
	rollupClient, err := l.EndpointProvider.RollupClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get rollup client: %w", err)
	}

	cCtx, cancel := context.WithTimeout(ctx, l.Config.NetworkTimeout)
	defer cancel()

	l1Tip, err := l.l1Tip(cCtx)
	if err != nil {
		return fmt.Errorf("failed to retrieve l1 tip: %w", err)
	}

	l1TargetBlock := l1Tip.Number
	if l.Config.CheckRecentTxsDepth != 0 {
		l.Log.Info("Checking for recently submitted batcher transactions on L1")
		recentBlock, found, err := eth.CheckRecentTxs(cCtx, l.L1Client, l.Config.CheckRecentTxsDepth, l.Txmgr.From())
		if err != nil {
			return fmt.Errorf("failed checking recent batcher txs: %w", err)
		}
		l.Log.Info("Checked for recently submitted batcher transactions on L1",
			"l1_head", l1Tip, "l1_recent", recentBlock, "found", found)
		l1TargetBlock = recentBlock
	}

	return dial.WaitRollupSync(l.shutdownCtx, l.Log, rollupClient, l1TargetBlock, time.Second*12)
}

// publishStateToL1 queues up all pending TxData to be published to the L1, returning when there is no more data to
// queue for publishing or if there was an error queing the data.  maxDuration tells this function to return from state
// publishing after this amount of time has been exceeded even if there is more data remaining.
func (l *BatchSubmitter) publishStateToL1(queue *txmgr.Queue[txRef], receiptsCh chan txmgr.TxReceipt[txRef], daGroup *errgroup.Group, maxDuration time.Duration) {
	start := time.Now()
	for {
		// if the txmgr is closed, we stop the transaction sending
		if l.Txmgr.IsClosed() {
			l.Log.Info("Txmgr is closed, aborting state publishing")
			return
		}
		if !l.checkTxpool(queue, receiptsCh) {
			l.Log.Info("txpool state is not good, aborting state publishing")
			return
		}
		err := l.publishTxToL1(l.killCtx, queue, receiptsCh, daGroup)
		if err != nil {
			if err != io.EOF {
				l.Log.Error("Error publishing tx to l1", "err", err)
			}
			return
		}
		if time.Since(start) > maxDuration {
			l.Log.Warn("Aborting state publishing, max duration exceeded")
			return
		}
	}
}

// clearState clears the state of the channel manager
func (l *BatchSubmitter) clearState(ctx context.Context) {
	l.Log.Info("Clearing state")
	defer l.Log.Info("State cleared")

	clearStateWithL1Origin := func() bool {
		l1SafeOrigin, err := l.safeL1Origin(ctx)
		if err != nil {
			l.Log.Warn("Failed to query L1 safe origin, will retry", "err", err)
			return false
		} else {
			l.Log.Info("Clearing state with safe L1 origin", "origin", l1SafeOrigin)
			l.state.Clear(l1SafeOrigin)
			return true
		}
	}

	// Attempt to set the L1 safe origin and clear the state, if fetching fails -- fall through to an infinite retry
	if clearStateWithL1Origin() {
		return
	}

	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			if clearStateWithL1Origin() {
				return
			}
		case <-ctx.Done():
			l.Log.Warn("Clearing state cancelled")
			l.state.Clear(eth.BlockID{})
			return
		}
	}
}

// publishTxToL1 submits a single state tx to the L1
func (l *BatchSubmitter) publishTxToL1(ctx context.Context, queue *txmgr.Queue[txRef], receiptsCh chan txmgr.TxReceipt[txRef], daGroup *errgroup.Group) error {
	// send all available transactions
	l1tip, err := l.l1Tip(ctx)
	if err != nil {
		l.Log.Error("Failed to query L1 tip", "err", err)
		return err
	}
	l.recordL1Tip(l1tip)

	// Collect next transaction data. This pulls data out of the channel, so we need to make sure
	// to put it back if ever da or txmgr requests fail, by calling l.recordFailedDARequest/recordFailedTx.
	txdata, err := l.state.TxData(l1tip.ID())

	if err == io.EOF {
		l.Log.Trace("No transaction data available")
		return err
	} else if err != nil {
		l.Log.Error("Unable to get tx data", "err", err)
		return err
	}

	if err = l.sendTransaction(txdata, queue, receiptsCh, daGroup); err != nil {
		return fmt.Errorf("BatchSubmitter.sendTransaction failed: %w", err)
	}
	return nil
}

func (l *BatchSubmitter) safeL1Origin(ctx context.Context) (eth.BlockID, error) {
	c, err := l.EndpointProvider.RollupClient(ctx)
	if err != nil {
		log.Error("Failed to get rollup client", "err", err)
		return eth.BlockID{}, fmt.Errorf("safe l1 origin: error getting rollup client: %w", err)
	}

	cCtx, cancel := context.WithTimeout(ctx, l.Config.NetworkTimeout)
	defer cancel()

	status, err := c.SyncStatus(cCtx)
	if err != nil {
		log.Error("Failed to get sync status", "err", err)
		return eth.BlockID{}, fmt.Errorf("safe l1 origin: error getting sync status: %w", err)
	}

	// If the safe L2 block origin is 0, we are at the genesis block and should use the L1 origin from the rollup config.
	if status.SafeL2.L1Origin.Number == 0 {
		return l.RollupConfig.Genesis.L1, nil
	}

	return status.SafeL2.L1Origin, nil
}

// cancelBlockingTx creates an empty transaction of appropriate type to cancel out the incompatible
// transaction stuck in the txpool. In the future we might send an actual batch transaction instead
// of an empty one to avoid wasting the tx fee.
func (l *BatchSubmitter) cancelBlockingTx(queue *txmgr.Queue[txRef], receiptsCh chan txmgr.TxReceipt[txRef], isBlockedBlob bool) {
	var candidate *txmgr.TxCandidate
	var err error
	if isBlockedBlob {
		candidate = l.calldataTxCandidate([]byte{})
	} else if candidate, err = l.blobTxCandidate(emptyTxData); err != nil {
		panic(err) // this error should not happen
	}
	l.Log.Warn("sending a cancellation transaction to unblock txpool", "blocked_blob", isBlockedBlob)
	l.sendTx(txData{}, true, candidate, queue, receiptsCh)
}

// publishToAltDAAndL1 posts the txdata to the DA Provider and then sends the commitment to L1.
func (l *BatchSubmitter) publishToAltDAAndL1(txdata txData, queue *txmgr.Queue[txRef], receiptsCh chan txmgr.TxReceipt[txRef], daGroup *errgroup.Group) {
	// sanity checks
	if nf := len(txdata.frames); nf != 1 {
		l.Log.Crit("Unexpected number of frames in calldata tx", "num_frames", nf)
	}
	if txdata.asBlob {
		l.Log.Crit("Unexpected blob txdata with AltDA enabled")
	}

	// when posting txdata to an external DA Provider, we use a goroutine to avoid blocking the main loop
	// since it may take a while for the request to return.
	goroutineSpawned := daGroup.TryGo(func() error {
		// TODO: probably shouldn't be using the global shutdownCtx here, see https://go.dev/blog/context-and-structs
		// but sendTransaction receives l.killCtx as an argument, which currently is only canceled after waiting for the main loop
		// to exit, which would wait on this DA call to finish, which would take a long time.
		// So we prefer to mimic the behavior of txmgr and cancel all pending DA/txmgr requests when the batcher is stopped.
		comm, err := l.AltDA.SetInput(l.shutdownCtx, txdata.CallData())
		if err != nil {
			l.Log.Error("Failed to post input to Alt DA", "error", err)
			// requeue frame if we fail to post to the DA Provider so it can be retried
			// note: this assumes that the da server caches requests, otherwise it might lead to resubmissions of the blobs
			l.recordFailedDARequest(txdata.ID(), err)
			return nil
		}
		l.Log.Info("Set altda input", "commitment", comm, "tx", txdata.ID())
		candidate := l.calldataTxCandidate(comm.TxData())
		l.sendTx(txdata, false, candidate, queue, receiptsCh)
		return nil
	})
	if !goroutineSpawned {
		// We couldn't start the goroutine because the errgroup.Group limit
		// is already reached. Since we can't send the txdata, we have to
		// return it for later processing. We use nil error to skip error logging.
		l.recordFailedDARequest(txdata.ID(), nil)
	}
}

// sendTransaction creates & queues for sending a transaction to the batch inbox address with the given `txData`.
// This call will block if the txmgr queue is at the  max-pending limit.
// The method will block if the queue's MaxPendingTransactions is exceeded.
func (l *BatchSubmitter) sendTransaction(txdata txData, queue *txmgr.Queue[txRef], receiptsCh chan txmgr.TxReceipt[txRef], daGroup *errgroup.Group) error {
	var err error

	// if Alt DA is enabled we post the txdata to the DA Provider and replace it with the commitment.
	if l.Config.UseAltDA {
		l.publishToAltDAAndL1(txdata, queue, receiptsCh, daGroup)
		// we return nil to allow publishStateToL1 to keep processing the next txdata
		return nil
	}

	var candidate *txmgr.TxCandidate
	if txdata.asBlob {
		if candidate, err = l.blobTxCandidate(txdata); err != nil {
			// We could potentially fall through and try a calldata tx instead, but this would
			// likely result in the chain spending more in gas fees than it is tuned for, so best
			// to just fail. We do not expect this error to trigger unless there is a serious bug
			// or configuration issue.
			return fmt.Errorf("could not create blob tx candidate: %w", err)
		}
	} else {
		// sanity check
		if nf := len(txdata.frames); nf != 1 {
			l.Log.Crit("Unexpected number of frames in calldata tx", "num_frames", nf)
		}
		candidate = l.calldataTxCandidate(txdata.CallData())
	}

	l.sendTx(txdata, false, candidate, queue, receiptsCh)
	return nil
}

// sendTx uses the txmgr queue to send the given transaction candidate after setting its
// gaslimit. It will block if the txmgr queue has reached its MaxPendingTransactions limit.
func (l *BatchSubmitter) sendTx(txdata txData, isCancel bool, candidate *txmgr.TxCandidate, queue *txmgr.Queue[txRef], receiptsCh chan txmgr.TxReceipt[txRef]) {
	intrinsicGas, err := core.IntrinsicGas(candidate.TxData, nil, false, true, true, false)
	if err != nil {
		// we log instead of return an error here because txmgr can do its own gas estimation
		l.Log.Error("Failed to calculate intrinsic gas", "err", err)
	} else {
		candidate.GasLimit = intrinsicGas
	}

	queue.Send(txRef{id: txdata.ID(), isCancel: isCancel, isBlob: txdata.asBlob}, *candidate, receiptsCh)
}

func (l *BatchSubmitter) blobTxCandidate(data txData) (*txmgr.TxCandidate, error) {
	blobs, err := data.Blobs()
	if err != nil {
		return nil, fmt.Errorf("generating blobs for tx data: %w", err)
	}
	size := data.Len()
	lastSize := len(data.frames[len(data.frames)-1].data)
	l.Log.Info("Building Blob transaction candidate",
		"size", size, "last_size", lastSize, "num_blobs", len(blobs))
	l.Metr.RecordBlobUsedBytes(lastSize)
	return &txmgr.TxCandidate{
		To:    &l.RollupConfig.BatchInboxAddress,
		Blobs: blobs,
	}, nil
}

func (l *BatchSubmitter) calldataTxCandidate(data []byte) *txmgr.TxCandidate {
	l.Log.Info("Building Calldata transaction candidate", "size", len(data))
	return &txmgr.TxCandidate{
		To:     &l.RollupConfig.BatchInboxAddress,
		TxData: data,
	}
}

func (l *BatchSubmitter) handleReceipt(r txmgr.TxReceipt[txRef]) {
	// Record TX Status
	if r.Err != nil {
		l.recordFailedTx(r.ID.id, r.Err)
	} else {
		l.recordConfirmedTx(r.ID.id, r.Receipt)
	}
}

func (l *BatchSubmitter) recordL1Tip(l1tip eth.L1BlockRef) {
	if l.lastL1Tip == l1tip {
		return
	}
	l.lastL1Tip = l1tip
	l.Metr.RecordLatestL1Block(l1tip)
}

func (l *BatchSubmitter) recordFailedDARequest(id txID, err error) {
	if err != nil {
		l.Log.Warn("DA request failed", logFields(id, err)...)
	}
	l.state.TxFailed(id)
}

func (l *BatchSubmitter) recordFailedTx(id txID, err error) {
	l.Log.Warn("Transaction failed to send", logFields(id, err)...)
	l.state.TxFailed(id)
}

func (l *BatchSubmitter) recordConfirmedTx(id txID, receipt *types.Receipt) {
	l.Log.Info("Transaction confirmed", logFields(id, receipt)...)
	l1block := eth.ReceiptBlockID(receipt)
	l.state.TxConfirmed(id, l1block)
}

// l1Tip gets the current L1 tip as a L1BlockRef. The passed context is assumed
// to be a lifetime context, so it is internally wrapped with a network timeout.
func (l *BatchSubmitter) l1Tip(ctx context.Context) (eth.L1BlockRef, error) {
	tctx, cancel := context.WithTimeout(ctx, l.Config.NetworkTimeout)
	defer cancel()
	head, err := l.L1Client.HeaderByNumber(tctx, nil)
	if err != nil {
		return eth.L1BlockRef{}, fmt.Errorf("getting latest L1 block: %w", err)
	}
	return eth.InfoToL1BlockRef(eth.HeaderBlockInfo(head)), nil
}

func (l *BatchSubmitter) checkTxpool(queue *txmgr.Queue[txRef], receiptsCh chan txmgr.TxReceipt[txRef]) bool {
	l.txpoolMutex.Lock()
	if l.txpoolState == TxpoolBlocked {
		// txpoolState is set to Blocked only if Send() is returning
		// ErrAlreadyReserved. In this case, the TxMgr nonce should be reset to nil,
		// allowing us to send a cancellation transaction.
		l.txpoolState = TxpoolCancelPending
		isBlob := l.txpoolBlockedBlob
		l.txpoolMutex.Unlock()
		l.cancelBlockingTx(queue, receiptsCh, isBlob)
		return false
	}
	r := l.txpoolState == TxpoolGood
	l.txpoolMutex.Unlock()
	return r
}

func logFields(xs ...any) (fs []any) {
	for _, x := range xs {
		switch v := x.(type) {
		case txID:
			fs = append(fs, "tx_id", v.String())
		case *types.Receipt:
			fs = append(fs, "tx", v.TxHash, "block", eth.ReceiptBlockID(v))
		case error:
			fs = append(fs, "err", v)
		default:
			fs = append(fs, "ERROR", fmt.Sprintf("logFields: unknown type: %T", x))
		}
	}
	return fs
}
