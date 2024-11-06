package da

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"

	"github.com/ethereum-optimism/optimism/op-batcher/batcher"
	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

const (
	bigTxSize = 10000 // amount of incompressible calldata to put in a "big" transaction
)

func TestDATxThrottling(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg, rollupClient, l2Seq, l2Verif, batcher := setupTest(t, 100, 0)

	sendTx := func(senderKey *ecdsa.PrivateKey, nonce uint64, size int) *types.Receipt {
		hash := sendTx(t, senderKey, nonce, size, cfg.L2ChainIDBig(), l2Seq)
		return waitForReceipt(t, hash, l2Seq)
	}

	// send a big transaction before throttling could have started, this transaction should land
	receipt := sendTx(cfg.Secrets.Alice, 0, bigTxSize)

	// start batch submission, which should trigger throttling future large transactions
	err := batcher.StartBatchSubmitting()
	require.NoError(t, err)

	// wait until the block containing the above tx shows up as safe to confirm batcher is running.
	waitForBlock(t, receipt.BlockNumber, l2Verif, rollupClient)

	// send another big tx, this one should get "stuck" so we wait for its receipt in a parallel goroutine.
	done := make(chan bool, 1)
	var bigReceipt *types.Receipt
	go func() {
		bigReceipt = sendTx(cfg.Secrets.Alice, 1, bigTxSize)
		done <- true
	}()

	safeBlockInclusionDuration := time.Duration(6*cfg.DeployConfig.L1BlockTime) * time.Second
	time.Sleep(safeBlockInclusionDuration)
	require.Nil(t, bigReceipt, "large tx did not get throttled")

	// Send a small tx, it should get included before the earlier one as long as it's from another sender
	r := sendTx(cfg.Secrets.Bob, 0, 0)
	// wait until the block the tx was first included in shows up in the safe chain on the verifier
	waitForBlock(t, r.BlockNumber, l2Verif, rollupClient)

	// second tx should still be throttled
	require.Nil(t, bigReceipt, "large tx did not get throttled")

	// disable throttling to let big tx through
	batcher.Config.ThrottleTxSize = 0
	<-done
	require.NotNil(t, bigReceipt, "large tx did not get throttled")
}

func TestDABlockThrottling(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg, rollupClient, l2Seq, l2Verif, batcher := setupTest(t, 0, bigTxSize+bigTxSize/10)

	sendTx := func(senderKey *ecdsa.PrivateKey, nonce uint64, size int) common.Hash {
		return sendTx(t, senderKey, nonce, size, cfg.L2ChainIDBig(), l2Seq)
	}

	// Send three big transactions before throttling could have started and make sure some eventually appear in the same
	// block to confirm there is no block-level DA throttling active. This usually happens the first try but might
	// require a second iteration in some cases due to stochasticity.
	nonce := uint64(0)
	for {
		h1 := sendTx(cfg.Secrets.Alice, nonce, bigTxSize)
		h2 := sendTx(cfg.Secrets.Bob, nonce, bigTxSize)
		h3 := sendTx(cfg.Secrets.Mallory, nonce, bigTxSize)
		nonce++

		r1 := waitForReceipt(t, h1, l2Seq)
		r2 := waitForReceipt(t, h2, l2Seq)
		r3 := waitForReceipt(t, h3, l2Seq)

		// wait until the blocks containing the above txs show up in the unsafe chain
		waitForBlock(t, r1.BlockNumber, l2Seq, rollupClient)
		waitForBlock(t, r2.BlockNumber, l2Seq, rollupClient)
		waitForBlock(t, r3.BlockNumber, l2Seq, rollupClient)
		t.Log("Some block numbers should be the same:", r1.BlockNumber, r2.BlockNumber, r3.BlockNumber)

		if r1.BlockNumber.Cmp(r2.BlockNumber) == 0 || r1.BlockNumber.Cmp(r3.BlockNumber) == 0 || r2.BlockNumber.Cmp(r3.BlockNumber) == 0 {
			// At least 2 transactions appeared in the same block, so we can exit the loop.
			// But first we start batch submission, which will enabling DA throttling.
			err := batcher.StartBatchSubmitting()
			require.NoError(t, err)
			// wait for a safe block containing one of the above transactions to ensure the batcher is running
			waitForBlock(t, r1.BlockNumber, l2Verif, rollupClient)
			break
		}
		t.Log("Another iteration required:", nonce)
	}

	// Send 3 more big transactions at a time, but this time they must all appear in different blocks due to the
	// block-level DA limit. Repeat the test 3 times to reduce the probability this happened just due to bad luck.
	for i := 0; i < 3; i++ {
		h1 := sendTx(cfg.Secrets.Alice, nonce, bigTxSize)
		h2 := sendTx(cfg.Secrets.Bob, nonce, bigTxSize)
		h3 := sendTx(cfg.Secrets.Mallory, nonce, bigTxSize)
		nonce++

		r1 := waitForReceipt(t, h1, l2Seq)
		r2 := waitForReceipt(t, h2, l2Seq)
		r3 := waitForReceipt(t, h3, l2Seq)
		t.Log("Block numbers should all be different:", r1.BlockNumber, r2.BlockNumber, r3.BlockNumber)

		require.NotEqual(t, 0, r1.BlockNumber.Cmp(r2.BlockNumber))
		require.NotEqual(t, 0, r1.BlockNumber.Cmp(r3.BlockNumber))
		require.NotEqual(t, 0, r2.BlockNumber.Cmp(r3.BlockNumber))
	}
}

func setupTest(t *testing.T, maxTxSize, maxBlockSize uint64) (e2esys.SystemConfig, *sources.RollupClient, *ethclient.Client, *ethclient.Client, *batcher.TestBatchSubmitter) {
	cfg := e2esys.DefaultSystemConfig(t)
	cfg.GethOptions["sequencer"] = append(cfg.GethOptions["sequencer"], []geth.GethOption{
		func(ethCfg *ethconfig.Config, nodeCfg *node.Config) error {
			ethCfg.Miner.GasCeil = 30_000_000
			return nil
		},
	}...)
	// disable batcher because we start it manually later
	cfg.DisableBatcher = true

	sys, err := cfg.Start(t,
		e2esys.WithBatcherThrottling(500*time.Millisecond, 1, maxTxSize, maxBlockSize))
	require.NoError(t, err, "Error starting up system")

	rollupClient := sys.RollupClient("verifier")
	l2Seq := sys.NodeClient("sequencer")
	l2Verif := sys.NodeClient("verifier")
	batcher := sys.BatchSubmitter.ThrottlingTestDriver()

	return cfg, rollupClient, l2Seq, l2Verif, batcher
}

// sendTx sends a tx containing the 'size' amount of random calldata
func sendTx(t *testing.T, senderKey *ecdsa.PrivateKey, nonce uint64, size int, chainID *big.Int, cl *ethclient.Client) common.Hash {
	randomBytes := make([]byte, size)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	tx := types.MustSignNewTx(senderKey, types.LatestSignerForChainID(chainID), &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &common.Address{0xff, 0xff},
		Value:     big.NewInt(1_000_000_000),
		GasTipCap: big.NewInt(10),
		GasFeeCap: big.NewInt(200),
		Gas:       21_000 + uint64(len(randomBytes))*16,
		Data:      randomBytes,
	})
	err = cl.SendTransaction(context.Background(), tx)
	require.NoError(t, err, "sending L2 tx")
	return tx.Hash()
}

func waitForReceipt(t *testing.T, hash common.Hash, cl *ethclient.Client) *types.Receipt {
	receipt, err := wait.ForReceiptOK(context.Background(), cl, hash)
	require.NoError(t, err, "waiting for L2 tx")
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx not successful")
	return receipt
}

func waitForBlock(t *testing.T, blockNumber *big.Int, cl *ethclient.Client, rc *sources.RollupClient) {
	_, err := geth.WaitForBlock(blockNumber, cl)
	require.NoError(t, err, "Waiting for block on verifier")
	require.NoError(t, wait.ForProcessingFullBatch(context.Background(), rc))
}
