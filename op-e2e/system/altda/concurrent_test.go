package altda

import (
	"context"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/stretchr/testify/require"
)

// TestBatcherConcurrentAltDARequests tests that the batcher can submit parallel requests
// to the alt-da server. It does not check that the requests are correctly ordered and interpreted
// by op nodes.
func TestBatcherConcurrentAltDARequests(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t, e2esys.WithAllocType(config.AllocTypeAltDAGeneric))
	cfg.BatcherMaxPendingTransactions = 0 // no limit on parallel txs
	cfg.BatcherBatchType = 0
	cfg.DataAvailabilityType = flags.CalldataType
	cfg.BatcherMaxConcurrentDARequest = 2

	// disable batcher because we start it manually below
	cfg.DisableBatcher = true
	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")
	t.Cleanup(func() {
		sys.Close()
	})

	// make every request take 5 seconds, such that only if 2 altda requests are made
	// concurrently will 2 batcher txs be able to land in a single L1 block
	sys.FakeAltDAServer.SetPutRequestLatency(5 * time.Second)

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")

	// we wait for 10 L2 blocks to have been produced, just to make sure the sequencer is working properly
	_, err = geth.WaitForBlock(big.NewInt(10), l2Seq)
	require.NoError(t, err, "Waiting for L2 blocks")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	startingL1BlockNum, err := l1Client.BlockNumber(ctx)
	require.NoError(t, err)

	// start batch submission
	driver := sys.BatchSubmitter.TestDriver()
	err = driver.StartBatchSubmitting()
	require.NoError(t, err)

	// We make sure that some block has more than 1 batcher tx
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

// The Holocene fork enforced a new strict batch ordering rule, see https://specs.optimism.io/protocol/holocene/derivation.html
// This test makes sure that concurrent requests to the alt-da server that are responded out of order
// are submitted to the L1 chain in the correct order by the batcher.
func TestBatcherCanHandleOutOfOrderDAServerResponses(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.HoloceneSystemConfig(t, new(hexutil.Uint64), e2esys.WithAllocType(config.AllocTypeAltDAGeneric))
	cfg.BatcherMaxPendingTransactions = 0 // no limit on parallel txs
	cfg.BatcherBatchType = 0
	cfg.DataAvailabilityType = flags.CalldataType
	cfg.BatcherMaxConcurrentDARequest = 2
	cfg.BatcherMaxL1TxSizeBytes = 150               // enough to fit a single compressed empty L1 block, but not 2
	cfg.Nodes["sequencer"].SafeDBPath = t.TempDir() // needed for SafeHeadAtL1Block() below

	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")
	t.Cleanup(func() {
		sys.Close()
	})
	sys.FakeAltDAServer.SetOutOfOrderResponses(true)

	l1Client := sys.NodeClient("l1")
	l2SeqCL := sys.RollupClient("sequencer")

	checkBlocksL1 := int64(15)
	l2SafeHeadMovedCount := 0
	l2SafeHeadMovedCountExpected := 3
	l2SafeHeadCur := uint64(0)
	for i := int64(0); i < checkBlocksL1; i++ {
		_, err := geth.WaitForBlock(big.NewInt(i), l1Client, geth.WithNoChangeTimeout(5*time.Minute))
		require.NoError(t, err, "Waiting for l1 blocks")
		newL2SafeHead, err := l2SeqCL.SafeHeadAtL1Block(context.Background(), uint64(i))
		require.NoError(t, err)
		if newL2SafeHead.SafeHead.Number > l2SafeHeadCur {
			l2SafeHeadMovedCount++
			l2SafeHeadCur = newL2SafeHead.SafeHead.Number
		}
		if l2SafeHeadMovedCount == l2SafeHeadMovedCountExpected {
			return
		}
	}
	t.Fatalf("L2SafeHead only advanced %d times (expected >= %d) in %d L1 blocks", l2SafeHeadMovedCount, l2SafeHeadMovedCountExpected, checkBlocksL1)

}

// TODO: need to add tests for 3 unhappy paths (https://github.com/ethereum-optimism/optimism/tree/develop/op-batcher#happy-path):
// 1. reorgs: what happens?
// 2. tx fails: think we will end up resending the data to alt-da
//    which is fine as long as alt-da has idempotent puts
// 3. tx confirmed but channel timed out: what happens?
