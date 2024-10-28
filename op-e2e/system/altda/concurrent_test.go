package altda

import (
	"context"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/stretchr/testify/require"
)

func TestBatcherConcurrentAltDARequests(t *testing.T) {
	op_e2e.InitParallel(t)

	numL1TxsExpected := int64(10)

	cfg := e2esys.DefaultSystemConfig(t)
	cfg.DeployConfig.UseAltDA = true
	cfg.BatcherMaxPendingTransactions = 0 // no limit on parallel txs
	// ensures that batcher txs are as small as possible
	cfg.BatcherMaxL1TxSizeBytes = derive.FrameV0OverHeadSize + 1 /*version bytes*/ + 1
	cfg.BatcherBatchType = 0
	cfg.DataAvailabilityType = flags.CalldataType
	cfg.BatcherMaxConcurrentDARequest = uint64(numL1TxsExpected)

	// disable batcher because we start it manually below
	cfg.DisableBatcher = true
	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")
	defer sys.Close()

	// make every request take 5 seconds, such that only concurrent requests will be able to make progress fast enough
	sys.FakeAltDAServer.SetPutRequestLatency(5 * time.Second)

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")

	// we wait for numL1TxsExpected L2 blocks to have been produced, just to make sure the sequencer is working properly
	_, err = geth.WaitForBlock(big.NewInt(numL1TxsExpected), l2Seq)
	require.NoError(t, err, "Waiting for L2 blocks")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	startingL1BlockNum, err := l1Client.BlockNumber(ctx)
	require.NoError(t, err)

	// start batch submission
	driver := sys.BatchSubmitter.TestDriver()
	err = driver.StartBatchSubmitting()
	require.NoError(t, err)

	// Iterate over up to 10 blocks. The number of transactions sent by the batcher should
	// exceed the number of blocks.
	checkBlocks := 10
	for i := 0; i < checkBlocks; i++ {
		block, err := geth.WaitForBlock(big.NewInt(int64(startingL1BlockNum)+int64(i)), l1Client)
		require.NoError(t, err, "Waiting for l1 blocks")
		// there are possibly other services (proposer/challenger) in the background sending txs
		// so we only count the batcher txs
		batcherTxCount, err := transactions.TransactionsBySender(block, cfg.DeployConfig.BatchSenderAddress)
		require.NoError(t, err)
		if batcherTxCount > 1 {
			return
		}
	}

	t.Fatalf("did not find more than 1 batcher tx per block in %d blocks", checkBlocks)
}
