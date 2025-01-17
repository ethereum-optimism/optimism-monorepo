package main

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

var pullCommand = &cli.Command{
	Name:   "pull",
	Usage:  "Pull a range of blocks and extract nonces from all user deposits",
	Flags:  []cli.Flag{StartFlag, EndFlag, RPCURLFlag, WorkerFlag, OutputFlag, BackoffFlag, BatchSizeFlag, OutputFormatFlag},
	Action: pull,
}

var MaxBatchSize uint64 = 100

// pull will pull a range of blocks and extract nonces from all user deposits
// it will start a number of workers to process blocks
// and runs an aggregation to collect the results
func pull(ctx *cli.Context) error {
	timeout := 1 * time.Minute
	log := log.New()
	// create a new client
	c, err := dial.DialEthClientWithTimeout(
		ctx.Context,
		timeout,
		log,
		ctx.String("rpc-url"),
	)
	if err != nil {
		log.Error("Failed to dial rollup client", "Err", err)
		return err
	}

	cid, err := c.ChainID(ctx.Context)
	if err != nil {
		log.Error("Failed to Get Chain ID", "Err", err)
		return err
	}
	chainID := cid.Uint64()

	// record start time
	startT := time.Now()

	resultChan := make(chan result)
	errorChan := make(chan error)

	start := ctx.Uint64("start")
	end := ctx.Uint64("end")
	workers := ctx.Uint64("workers")
	batchSize := ctx.Uint64("batch-size")
	writer, ok := formats[ctx.String("output-format")]
	if !ok {
		log.Error("Invalid Output Format. Defaulting to JSON", "Format", ctx.String("output-format"))
		writer = formats["json"]
	}

	if batchSize > MaxBatchSize {
		log.Warn("Batch Size Too Large, Reducing", "BatchSize", batchSize, "MaxBatchSize", MaxBatchSize)
		batchSize = MaxBatchSize
	}

	log.Info("Starting", "First", start, "Last", end, "Workers", workers, "BatchSize", batchSize)

	// first cut the work into ranges for batching
	// and load the work into a channel
	if batchSize > end-start {
		log.Info("More Batch Size Than Required", "BatchSize", batchSize, "Blocks", end-start)
		batchSize = end - start
	}
	batches := toBatches(start, end, batchSize)
	workChan := make(chan batchRange, len(batches))
	for _, b := range batches {
		workChan <- b
	}
	retryWorkChan := make(chan batchRange, len(batches))

	// set the number of workers to the number of batches if there are more workers than batches
	if workers > uint64(len(batches)) {
		log.Info("More Workers Than Batches", "Workers", workers, "Batches", len(batches))
		workers = uint64(len(batches))
	}

	// start workers
	wg := &sync.WaitGroup{}
	for id := uint64(0); id < workers; id++ {
		wg.Add(1)
		go startWorker(
			id, ctx, c,
			workChan,
			retryWorkChan,
			resultChan,
			errorChan,
			log,
			wg)
	}

	// start a worker-waiter to end the aggregation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		log.Info("All Workers Finished")
		done <- struct{}{}
	}()

	// aggregate until the done signal is received
	aggregateResults, err := startAggregator(resultChan, errorChan, done, log)
	if err != nil {
		log.Error("Errors Encountered During Aggregation. All Jobs Retried to Completion")
	}
	aggregateResults.First = start
	aggregateResults.Last = end
	aggregateResults.ChainID = chainID

	err = writer.writeAggregate(aggregateResults, ctx.String("output"))
	if err != nil {
		log.Error("Failed to Write Aggregate Results", "Err", err)
		return err
	}

	log.Info("Finished", "Duration", time.Since(startT))

	return nil
}

type batchRange struct {
	Start uint64
	End   uint64
}

// toBatches is a helper function to split a single large range into smaller batches
func toBatches(start, end, size uint64) []batchRange {
	batches := []batchRange{}
	for i := start; i < end; i += size {
		if i+size > end {
			batches = append(batches, batchRange{i, end})
		} else {
			batches = append(batches, batchRange{i, i + size})
		}
	}
	return batches
}

// splitBatchRange will split a batch range into two smaller ranges
// it is used to reduce pressure from large batches dynamically
func splitBatchRange(b batchRange) []batchRange {
	size := b.End - b.Start
	if size < 2 {
		return []batchRange{b}
	}
	half := size / 2
	return []batchRange{
		{b.Start, b.Start + half},
		{b.Start + half, b.End},
	}
}

// startAggregator will aggregate the results of the workers and return the aggregation once done
// it will receive results on the results channel, and chooses to include them in the aggregation if they are not empty
// it logs errors from the error channel and joins them as part of the return
func startAggregator(results chan result, errorChan chan error, done chan struct{}, log log.Logger) (aggregate, error) {
	aggregateResults := aggregate{
		Results: make(map[uint64][]uint64),
	}
	var errs error
	handled := 0
	errCount := 0
	for {
		select {
		case r := <-results:
			handled += 1
			if len(r.Nonces) > 0 {
				log.Info("Block Has Deposit Transactions", "Block", r.BlockNumber, "Nonces", r.Nonces, "Handled", handled)
				aggregateResults.Results[r.BlockNumber] = r.Nonces
			}
		case err := <-errorChan:
			log.Error("Got Error", "Err", err)
			errCount += 1
			errs = errors.Join(errs, err)
		case <-done:
			// drain the results channel
			// this is not very DRY, but it is the simplest way to do this
			for len(results) > 0 {
				r := <-results
				handled += 1
				if len(r.Nonces) > 0 {
					log.Info("Block Has Deposit Transactions", "Block", r.BlockNumber, "Nonces", r.Nonces, "Handled", handled)
					aggregateResults.Results[r.BlockNumber] = r.Nonces
				}
			}
			log.Info("Finished Aggregation", "ResultsHandled", handled, "ResultsMatched", len(aggregateResults.Results))
			return aggregateResults, errs
		}
	}
}

// startWorker will start a worker to process blocks.
// callers should set up the wait group and call this function as a goroutine
// each worker will process blocks until the work channel is empty
// if the worker fails to process a work item, it will be returned to the work channel and the worker will sleep for the backoff duration
// workers return results to the results channel, from which they will be aggregated
func startWorker(
	id uint64,
	ctx *cli.Context,
	c *ethclient.Client,
	workChan chan batchRange,
	retryWorkChan chan batchRange,
	resultsChan chan result,
	errorsChan chan error,
	log log.Logger,
	wg *sync.WaitGroup) {

	defer wg.Done()
	log.Info("Starting Worker", "ID", id)
	for {
		select {
		case <-ctx.Context.Done():
			log.Info("Context Done")
			return
		// retry work is work that has been tried at least once. it is prioritized equally to new work
		case b := <-retryWorkChan:
			log.Info("Got Retry Work", "Start", b.Start, "End", b.End)
			doWork(*ctx, b, resultsChan, errorsChan, retryWorkChan, c, log)
		case b := <-workChan:
			log.Info("Got Work", "Start", b.Start, "End", b.End)
			doWork(*ctx, b, resultsChan, errorsChan, retryWorkChan, c, log)
		default:
			log.Info("No More Work")
			return
		}
	}
}

func doWork(ctx cli.Context, b batchRange, resultsChan chan result, errorChan chan error, retryChan chan batchRange, c *ethclient.Client, log log.Logger) {
	results, err := processBlockRange(ctx.Context, c, b, log)
	if err != nil {
		log.Error("Failed to Process Blocks")
		errorChan <- err
		newWork := splitBatchRange(b)
		for _, w := range newWork {
			retryChan <- w
		}
		log.Warn("Returned Failed Work to Retry Channel. Sleeping for Backoff Duration", "Backoff", ctx.Duration("backoff"), "Start", b.Start, "End", b.End)
		time.Sleep(ctx.Duration("backoff"))
	} else {
		for _, r := range results {
			resultsChan <- r
		}
	}
}

// processBlockRange will process a range of blocks for user deposits
// it takes a batchRange and constructs a batchRPC request for the blocks
// it then processes each block's transactions for user deposits
// a list of results is returned for each block
func processBlockRange(
	ctx context.Context,
	c *ethclient.Client,
	br batchRange,
	log log.Logger) ([]result, error) {

	// turn the batch range into a list of block numbers
	nums := []rpc.BlockNumber{}
	for i := br.Start; i < br.End; i++ {
		nums = append(nums, rpc.BlockNumber(i))
	}

	// get all blocks in the batch range
	blocks, err := batchBlockByNumber(ctx, c, nums)
	if err != nil {
		log.Error("Failed to Get Batched Blocks", "Err", err)
		return []result{}, err
	}
	log.Info("Got Blocks", "NumBlocks", len(blocks))

	results := []result{}
	// process each block for user deposits
	for i := 0; i < len(blocks); i++ {
		b := blocks[i]
		matches := 0
		blockNumber := b.BlockID().Number
		res := result{
			BlockNumber: blockNumber,
			Nonces:      []uint64{},
		}
		// process each transaction in the block
		for j := 0; j < len(b.Transactions); j++ {
			t := b.Transactions[j]
			tx, err := t.Transaction()
			if err != nil {
				log.Warn("Unsupported Transaction", "Err", err)
				continue
			}
			ok, err := checkTransaction(ctx, c, tx, log)
			if err != nil {
				log.Error("Failed to Check Tx", "Err", err)
				return []result{}, err
			}
			// if the transaction matches the criteria, add it to the results
			if ok {
				matches += 1
				res.Nonces = append(res.Nonces, *tx.EffectiveNonce())
			}
		}
		log.Info("Processed Block", "Block", blockNumber, "TxCount", len(b.Transactions), "UserDeposits", matches)
		results = append(results, res)
	}
	return results, nil
}

// batchBlockByNumber will batch a list of block numbers into a single batch rpc request
// it uses the iterative batch call to make the request
// and returns the results
func batchBlockByNumber(ctx context.Context, c *ethclient.Client, blockNumbers []rpc.BlockNumber) ([]*sources.RPCBlock, error) {
	makeBlockByNumberRequest := func(blockNumber rpc.BlockNumber) (*sources.RPCBlock, rpc.BatchElem) {
		out := new(sources.RPCBlock)
		return out, rpc.BatchElem{
			Method: "eth_getBlockByNumber",
			Args:   []any{blockNumber, true},
			Result: &out,
		}
	}
	batchReq := batching.NewIterativeBatchCall[rpc.BlockNumber, *sources.RPCBlock](
		blockNumbers,
		makeBlockByNumberRequest,
		c.Client().BatchCallContext,
		c.Client().CallContext,
		int(MaxBatchSize),
	)
	for {
		if err := batchReq.Fetch(ctx); err == io.EOF {
			break
		} else if err != nil {
			log.Warn("Failed to Fetch Blocks", "Err", err, "Start", blockNumbers[0], "End", blockNumbers[len(blockNumbers)-1])
			return nil, err
		}
	}
	return batchReq.Result()
}

// checkTransaction will check if a transaction is a user deposit, and not initiated by the system address
func checkTransaction(ctx context.Context, c *ethclient.Client, tx *types.Transaction, log log.Logger) (bool, error) {
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		log.Error("Failed to Get Sender", "Err", err)
		return false, err
	}
	// we are filtering for deposit transactions which are not system transactions
	if tx.Type() == depositType &&
		from != systemAddress {
		log.Info("Got Transaction", "From", from, "Nonce", tx.EffectiveNonce(), "Type", tx.Type())
		return true, nil
	}
	return false, nil
}
