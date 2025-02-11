package da

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	gethutils "github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// TestSystem4844E2E* run the SystemE2E test with 4844 enabled on L1, and active on the rollup in
// the op-batcher and verifier.  It submits a txpool-blocking transaction before running
// each test to ensure the batcher is able to clear it.
func TestSystem4844E2E_Calldata(t *testing.T) {
	testSystem4844E2E(t, false, batcherFlags.CalldataType)
}

func TestSystem4844E2E_SingleBlob(t *testing.T) {
	testSystem4844E2E(t, false, batcherFlags.BlobsType)
}

func TestSystem4844E2E_MultiBlob(t *testing.T) {
	testSystem4844E2E(t, true, batcherFlags.BlobsType)
}

func testSystem4844E2E(t *testing.T, multiBlob bool, daType batcherFlags.DataAvailabilityType) {
	op_e2e.InitParallel(t)

	cfg := e2esys.EcotoneSystemConfig(t, new(hexutil.Uint64))
	cfg.DataAvailabilityType = daType
	cfg.BatcherBatchType = derive.SpanBatchType
	cfg.DeployConfig.L1GenesisBlockBaseFeePerGas = (*hexutil.Big)(big.NewInt(7000))

	var maxL1TxSize int
	if multiBlob {
		cfg.BatcherTargetNumFrames = eth.MaxBlobsPerBlobTx
		cfg.BatcherUseMaxTxSizeForBlobs = true
		// leads to eth.MaxBlobsPerBlobTx blobs for an L2 block with a user tx with 400 random bytes
		// while all other L2 blocks take 1 blob (deposit tx)
		maxL1TxSize = derive.FrameV0OverHeadSize + 100
		cfg.BatcherMaxL1TxSizeBytes = uint64(maxL1TxSize)
	}

	// For each test we intentionally block the batcher by submitting an incompatible tx type up
	// front. This lets us test the ability for the batcher to clear out the incompatible
	// transaction. The hook used here makes sure we make the jamming call before batch submission
	// is started, as is required by the function.
	var jamChan chan error
	jamCtx, jamCancel := context.WithTimeout(context.Background(), 20*time.Second)
	action := e2esys.StartOption{
		Key: "beforeBatcherStart",
		Action: func(cfg *e2esys.SystemConfig, s *e2esys.System) {
			driver := s.BatchSubmitter.TestDriver()
			err := driver.JamTxPool(jamCtx)
			require.NoError(t, err)
			jamChan = make(chan error)
			go func() {
				jamChan <- driver.WaitOnJammingTx(jamCtx)
			}()
		},
	}
	defer func() {
		if jamChan != nil { // only check if we actually got to a successful batcher start
			jamCancel()
			require.NoError(t, <-jamChan, "jam tx error")
		}
	}()

	cfg.DisableProposer = true // disable L2 output submission for this test
	sys, err := cfg.Start(t, action)
	require.NoError(t, err, "Error starting up system")

	log := testlog.Logger(t, log.LevelInfo)
	log.Info("genesis", "l2", sys.RollupConfig.Genesis.L2, "l1", sys.RollupConfig.Genesis.L1, "l2_time", sys.RollupConfig.Genesis.L2Time)

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")
	l2Verif := sys.NodeClient("verifier")

	// Transactor Account
	ethPrivKey := cfg.Secrets.Alice

	// Send Transaction & wait for success
	fromAddr := cfg.Secrets.Addresses().Alice
	log.Info("alice", "addr", fromAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	startBalance, err := l2Verif.BalanceAt(ctx, fromAddr, nil)
	require.NoError(t, err)

	// Send deposit transaction
	opts, err := bind.NewKeyedTransactorWithChainID(ethPrivKey, cfg.L1ChainIDBig())
	require.NoError(t, err)
	mintAmount := big.NewInt(1_000_000_000_000)
	opts.Value = mintAmount
	helpers.SendDepositTx(t, cfg, l1Client, l2Verif, opts, nil)

	// Confirm balance
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel2()
	endBalance, err := wait.ForBalanceChange(ctx2, l2Verif, fromAddr, startBalance)
	require.NoError(t, err)

	diff := new(big.Int).Sub(endBalance, startBalance)
	require.Equal(t, mintAmount, diff, "Did not get expected balance change")

	// Submit TX to L2 sequencer node
	receipt := helpers.SendL2Tx(t, cfg, l2Seq, ethPrivKey, func(opts *helpers.TxOpts) {
		opts.Value = big.NewInt(1_000_000_000)
		opts.Nonce = 1 // Already have deposit
		opts.ToAddr = &common.Address{0xff, 0xff}
		// put some random data in the tx to make it fill up eth.MaxBlobsPerBlobTx blobs (multi-blob case)
		opts.Data = testutils.RandomData(rand.New(rand.NewSource(420)), 400)
		opts.Gas, err = core.IntrinsicGas(opts.Data, nil, nil, false, true, true, false)
		require.NoError(t, err)
		opts.VerifyOnClients(l2Verif)
	})

	// Verify blocks match after batch submission on verifiers and sequencers
	verifBlock, err := l2Verif.BlockByNumber(context.Background(), receipt.BlockNumber)
	require.NoError(t, err)
	require.Equal(t, verifBlock.Hash(), receipt.BlockHash, "must be same block")
	seqBlock, err := l2Seq.BlockByNumber(context.Background(), receipt.BlockNumber)
	require.NoError(t, err)
	require.Equal(t, seqBlock.Hash(), receipt.BlockHash, "must be same block")
	require.Equal(t, verifBlock.NumberU64(), seqBlock.NumberU64(), "Verifier and sequencer blocks not the same after including a batch tx")
	require.Equal(t, verifBlock.ParentHash(), seqBlock.ParentHash(), "Verifier and sequencer blocks parent hashes not the same after including a batch tx")
	require.Equal(t, verifBlock.Hash(), seqBlock.Hash(), "Verifier and sequencer blocks not the same after including a batch tx")

	rollupClient := sys.RollupClient("sequencer")
	// basic check that sync status works
	seqStatus, err := rollupClient.SyncStatus(context.Background())
	require.NoError(t, err)
	require.LessOrEqual(t, seqBlock.NumberU64(), seqStatus.UnsafeL2.Number)
	// basic check that version endpoint works
	seqVersion, err := rollupClient.Version(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, "", seqVersion)

	// quick check that the batch submitter works
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		// wait for chain to be marked as "safe" (i.e. confirm batch-submission works)
		stat, err := rollupClient.SyncStatus(context.Background())
		require.NoError(ct, err)
		require.GreaterOrEqual(ct, stat.SafeL2.Number, receipt.BlockNumber.Uint64())
	}, time.Second*20, time.Second, "expected L2 to be batch-submitted and labeled as safe")

	// check that the L2 tx is still canonical
	seqBlock, err = l2Seq.BlockByNumber(context.Background(), receipt.BlockNumber)
	require.NoError(t, err)
	require.Equal(t, seqBlock.Hash(), receipt.BlockHash, "receipt block must match canonical block at tx inclusion height")

	// find L1 block that contained the blob(s) batch tx
	tip, err := l1Client.HeaderByNumber(context.Background(), nil)
	require.NoError(t, err)
	var blobTx *types.Transaction
	blobBlock, err := gethutils.FindBlock(l1Client, int(tip.Number.Int64()), 0, 5*time.Second,
		func(b *types.Block) (bool, error) {
			for _, tx := range b.Transactions() {
				if tx.To().Cmp(cfg.DeployConfig.BatchInboxAddress) != 0 {
					continue
				}
				switch daType {
				case batcherFlags.CalldataType:
					if len(tx.BlobHashes()) == 0 {
						return true, nil
					}
				case batcherFlags.BlobsType:
					if len(tx.BlobHashes()) == 0 {
						continue
					}
					if !multiBlob || len(tx.BlobHashes()) > 1 {
						blobTx = tx
						return true, nil
					}
				}
			}
			return false, nil
		})
	require.NoError(t, err)

	if daType == batcherFlags.CalldataType {
		return
	}
	// make sure blobs are as expected
	numBlobs := len(blobTx.BlobHashes())
	if !multiBlob {
		require.NotZero(t, numBlobs, "single-blob: expected to find L1 blob tx")
	} else {
		const maxBlobs = eth.MaxBlobsPerBlobTx
		require.Equal(t, maxBlobs, numBlobs, fmt.Sprintf("multi-blob: expected to find L1 blob tx with %d blobs", maxBlobs))
		// blob tx should have filled up all but last blob
		bcl := sys.L1BeaconHTTPClient()
		hashes := toIndexedBlobHashes(blobTx.BlobHashes()...)
		sidecars, err := bcl.BeaconBlobSideCars(context.Background(), false, sys.L1Slot(blobBlock.Time()), hashes)
		require.NoError(t, err)
		require.Len(t, sidecars.Data, maxBlobs)
		for i := 0; i < maxBlobs-1; i++ {
			data, err := sidecars.Data[i].Blob.ToData()
			require.NoError(t, err)
			require.Len(t, data, maxL1TxSize)
		}
		// last blob should only be partially filled
		data, err := sidecars.Data[maxBlobs-1].Blob.ToData()
		require.NoError(t, err)
		require.Less(t, len(data), maxL1TxSize)
	}
}

func toIndexedBlobHashes(hs ...common.Hash) []eth.IndexedBlobHash {
	hashes := make([]eth.IndexedBlobHash, 0, len(hs))
	for i, hash := range hs {
		hashes = append(hashes, eth.IndexedBlobHash{Index: uint64(i), Hash: hash})
	}
	return hashes
}

// TestBatcherAutoDA tests that the batcher with Auto data availability type
// correctly chooses the cheaper Ethereum-DA type (calldata or blobs).
// The L1 chain is set up with a genesis block that has an excess blob gas that leads
// to a slightly higher blob base fee than 16x the regular base fee.
// So in the first few L1 blocks, calldata will be cheaper than blobs.
// We then send a couple of expensive Deposit transactions, which drives up the
// gas price. The L1 blob gas limit is set to a low value to speed up this process.
func TestBatcherAutoDA(t *testing.T) {
	op_e2e.InitParallel(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// System setup
	cfg := e2esys.HoloceneSystemConfig(t, new(hexutil.Uint64))
	cfg.DeployConfig.L1PragueTimeOffset = new(hexutil.Uint64) // activate prague to get higher calldata cost
	cfg.DataAvailabilityType = batcherFlags.AutoType
	// We set the genesis fee values and block gas limit such that calldata txs are initially cheaper,
	// but then manipulate the fee markets over the coming L1 blocks such that blobs become cheaper again.
	cfg.DeployConfig.L1GenesisBlockBaseFeePerGas = (*hexutil.Big)(big.NewInt(3100))
	// 100 blob targets leads to 130_393 starting blob base fee, which is ~ 42 * 2_000 (equilibrium is ~16x or ~40x under Pectra)
	cfg.DeployConfig.L1GenesisBlockExcessBlobGas = (*hexutil.Uint64)(u64Ptr(100 * params.BlobTxTargetBlobGasPerBlock))
	cfg.DeployConfig.L1GenesisBlockBlobGasUsed = (*hexutil.Uint64)(u64Ptr(0))
	cfg.DeployConfig.L1GenesisBlockGasLimit = 2_500_000
	cfg.BatcherTargetNumFrames = eth.MaxBlobsPerBlobTx
	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")
	log := testlog.Logger(t, log.LevelInfo)
	log.Info("genesis", "l2", sys.RollupConfig.Genesis.L2, "l1", sys.RollupConfig.Genesis.L1, "l2_time", sys.RollupConfig.Genesis.L2Time)

	// Constants
	l1Client := sys.NodeClient("l1")
	ethPrivKey := cfg.Secrets.Alice
	depositContract, err := bindings.NewOptimismPortal(cfg.L1Deployments.OptimismPortalProxy, l1Client)
	require.NoError(t, err)
	depAmount := big.NewInt(1_000_000_000_000)
	opts, err := bind.NewKeyedTransactorWithChainID(ethPrivKey, cfg.L1ChainIDBig())
	require.NoError(t, err)
	opts.Value = depAmount
	fromAddr := cfg.Secrets.Addresses().Alice
	const numTxs = 25

	// Helpers
	mustGetFees := func() (*big.Int, *big.Int, *big.Int, float64) {
		tip, baseFee, blobFee, err := txmgr.DefaultGasPriceEstimatorFn(ctx, l1Client)
		require.NoError(t, err)
		feeRatio := float64(blobFee.Int64()) / float64(baseFee.Int64()+tip.Int64())
		t.Logf("L1 fees are: baseFee(%d), tip(%d), blobBaseFee(%d). feeRatio: %f", baseFee, tip, blobFee, feeRatio)
		return tip, baseFee, blobFee, feeRatio
	}
	requireEventualBatcherTxType := func(txType uint8, timeout time.Duration, strict bool) {
		var foundOtherTxType bool
		require.Eventually(t, func() bool {
			b, err := l1Client.BlockByNumber(ctx, nil)
			require.NoError(t, err)
			for _, tx := range b.Transactions() {
				if tx.To() == nil || tx.To().Cmp(cfg.DeployConfig.BatchInboxAddress) != 0 {
					continue
				}
				if typ := tx.Type(); typ == txType {
					return true
				} else if strict {
					foundOtherTxType = true
				}
			}
			return false
		}, timeout, time.Second, "expected batcher tx type didn't arrive")
		require.False(t, foundOtherTxType, "unexpected batcher tx type found")
	}

	// Check markets are set up as expected.
	_, _, _, feeRatio := mustGetFees()
	require.Greater(t, feeRatio, 41.0, "expected feeRatio to be greater than 41 (calldata should be cheaper, even with Pectra)")

	// Market manipulations:
	// Send deposit transactions in a loop to shore up L1 base fee
	// as blobBaseFee drops (batcher uses calldata initially so the blob market is quiet).
	txs := make([]*types.Transaction, 0, numTxs)
	t.Logf("Sending %d l1 txs...", numTxs)
	for i := int64(0); i < numTxs; i++ {
		opts.Nonce = big.NewInt(i)
		tx, err := transactions.PadGasEstimate(opts, 2, func(opts *bind.TransactOpts) (*types.Transaction, error) {
			return depositContract.DepositTransaction(opts, fromAddr, depAmount, 800_000, false, nil)
		})
		require.NoErrorf(t, err, "failed to send deposit tx[%d]", i)
		t.Logf("Deposit submitted[%d]: tx hash: %v", i, tx.Hash())
		txs = append(txs, tx)
	}

	// At this point, we didn't wait on any blocks yet, so we can check that
	// the first batcher tx used calldata.
	requireEventualBatcherTxType(types.DynamicFeeTxType, 8*time.Second, true)

	// Now wait for txs to confirm on L1:
	t.Logf("Confirming %d txs on L1...", numTxs)
	blockNum := 0
	for i, tx := range txs {
		rec, err := wait.ForReceiptOK(ctx, l1Client, tx.Hash())
		require.NoErrorf(t, err, "Waiting for tx[%d] on L1", i)
		t.Logf("Tx confirmed[%d]: L1 block num: %v, gas used: %d", i, rec.BlockNumber, rec.GasUsed)
		if rec.BlockNumber.Int64() > int64(blockNum) {
			blockNum = int(rec.BlockNumber.Int64())
			block, err := l1Client.BlockByNumber(ctx, rec.BlockNumber)
			require.NoError(t, err)
			t.Logf("gas used %d/%d", block.GasUsed(), block.GasLimit())
			_, _, _, feeRatio = mustGetFees()
			if feeRatio < 16.0 {
				break
			}
		}
	}

	// Check we managed to manipulate the markets correctly.
	require.Less(t, feeRatio, 16.0, "expected fee ratio to be less than 16 (blobspace should be cheaper, even without Pectra)")

	// Now wait for batcher to have switched to blob txs.
	requireEventualBatcherTxType(types.BlobTxType, 8*time.Second, false)
}

func u64Ptr(v uint64) *uint64 {
	return &v
}
