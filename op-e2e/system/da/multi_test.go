package da

import (
	"context"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/stretchr/testify/require"
)

func TestBatcherMultiTx(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)
	cfg.BatcherMaxPendingTransactions = 0 // no limit on parallel txs
	// ensures that batcher txs are as small as possible
	cfg.BatcherMaxL1TxSizeBytes = derive.FrameV0OverHeadSize + 1 /*version bytes*/ + 1
	cfg.DisableBatcher = true
	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")

	_, err = geth.WaitForBlock(big.NewInt(10), l2Seq)
	require.NoError(t, err, "Waiting for L2 blocks")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l1Number, err := l1Client.BlockNumber(ctx)
	require.NoError(t, err)

	// start batch submission
	driver := sys.BatchSubmitter.TestDriver()
	err = driver.StartBatchSubmitting()
	require.NoError(t, err)

	totalBatcherTxsCount := int64(0)
	// wait for up to 5 L1 blocks, usually only 3 is required, but it's
	// possible additional L1 blocks will be created before the batcher starts,
	// so we wait additional blocks.
	for i := int64(0); i < 5; i++ {
		block, err := geth.WaitForBlock(big.NewInt(int64(l1Number)+i), l1Client)
		require.NoError(t, err, "Waiting for l1 blocks")
		// there are possibly other services (proposer/challenger) in the background sending txs
		// so we only count the batcher txs
		batcherTxCount, err := transactions.TransactionsBySender(block, cfg.DeployConfig.BatchSenderAddress)
		require.NoError(t, err)
		totalBatcherTxsCount += int64(batcherTxCount)

		if totalBatcherTxsCount >= 10 {
			return
		}
	}

	t.Fatal("Expected at least 10 transactions from the batcher")
}
