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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// start batch submission
	driver := sys.BatchSubmitter.TestDriver()
	err = driver.StartBatchSubmitting()
	require.NoError(t, err)

	totalBatcherTxsCount := int64(0)

	headNum, err := l1Client.BlockNumber(ctx)
	require.NoError(t, err)
	stopNum := headNum + 10
	startBlock := uint64(1)

	for {
		for i := startBlock; i <= headNum; i++ {
			block, err := l1Client.BlockByNumber(ctx, big.NewInt(int64(i)))
			require.NoError(t, err)

			batcherTxCount, err := transactions.TransactionsBySender(block, cfg.DeployConfig.BatchSenderAddress)
			require.NoError(t, err)
			totalBatcherTxsCount += batcherTxCount

			if totalBatcherTxsCount >= 10 {
				return
			}
		}

		headNum++
		if headNum > stopNum {
			break
		}
		startBlock = headNum
		_, err = geth.WaitForBlock(big.NewInt(int64(headNum)), l1Client)
		require.NoError(t, err)
	}

	t.Fatal("Expected at least 10 transactions from the batcher")
}
