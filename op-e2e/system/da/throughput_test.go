package da

import (
	"context"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/stretchr/testify/require"
)

// TestBatcherThroughput tests the throughput of the batcher by creating a large amount of L2 data
// before starting the batcher. This causes a backlog which must be cleared quickly by the batcher.
func TestBatcherThroughput(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)
	cfg.BatcherMaxPendingTransactions = 0 // no limit on parallel txs
	cfg.DisableBatcher = true             // disable initially
	sys, err := cfg.Start(t,
		e2esys.WithBatcherThrottling(500*time.Millisecond, 1, 100, 0))
	require.NoError(t, err, "Error starting up system")

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")

	numL2Txs := 400
	finalTxHash := common.Hash{}
	for nonce := range numL2Txs {
		finalTxHash = sendTx(t, cfg.Secrets.Alice, uint64(nonce), bigTxSize, cfg.L2ChainIDBig(), l2Seq)
	}
	waitForReceipt(t, finalTxHash, l2Seq)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// start batch submission
	// driver := sys.BatchSubmitter.TestDriver()
	driver := sys.BatchSubmitter.ThrottlingTestDriver()
	err = driver.StartBatchSubmitting()
	require.NoError(t, err)

	targetBatcherBytes := 4_000_000
	totalBatcherBytes := int64(0)

	// count how many L1 blocks are required to get targetBatcherBytes on chain
	targetNumL1Blocks := uint64(10)
	currentBlock := uint64(0)
	for {
		currentBlock++
		block, err := l1Client.BlockByNumber(ctx, big.NewInt(int64(currentBlock)))
		require.NoError(t, err)

		batcherBytes, err := transactions.TransactionsBytesBySender(block, cfg.DeployConfig.BatchSenderAddress)
		// TODO also track number of transactions?
		require.NoError(t, err)
		totalBatcherBytes += batcherBytes
		t.Log("totalBatcherBytes", totalBatcherBytes)
		if totalBatcherBytes >= int64(targetBatcherBytes) {
			require.LessOrEqual(t, currentBlock, targetNumL1Blocks)
			t.Log("got to targetBatcherBytes in", currentBlock, "L1 blocks (targetBatcherBytes: ", targetBatcherBytes, ")")
			return
		}
		_, err = geth.WaitForBlock(big.NewInt(int64(currentBlock+1)), l1Client)
		require.NoError(t, err)
	}
}
