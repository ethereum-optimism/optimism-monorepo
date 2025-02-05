package derive

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/log"
)

func TestDataAndHashesFromTxs(t *testing.T) {
	// test setup
	rng := rand.New(rand.NewSource(12345))
	privateKey := testutils.InsecureRandomKey(rng)
	publicKey, _ := privateKey.Public().(*ecdsa.PublicKey)
	batcherAddr := crypto.PubkeyToAddress(*publicKey)
	batchInboxAddr := testutils.RandomAddress(rng)
	logger := testlog.Logger(t, log.LvlInfo)

	chainId := new(big.Int).SetUint64(rng.Uint64())
	signer := types.NewPragueSigner(chainId)
	config := DataSourceConfig{
		l1Signer:          signer,
		batchInboxAddress: batchInboxAddr,
	}

	// create a valid non-blob batcher transaction and make sure it's picked up
	txData := &types.LegacyTx{
		Nonce:    rng.Uint64(),
		GasPrice: new(big.Int).SetUint64(rng.Uint64()),
		Gas:      2_000_000,
		To:       &batchInboxAddr,
		Value:    big.NewInt(10),
		Data:     testutils.RandomData(rng, rng.Intn(1000)),
	}
	calldataTx, _ := types.SignNewTx(privateKey, signer, txData)
	txs := types.Transactions{calldataTx}
	data, blobHashes := dataAndHashesFromTxs(txs, &config, batcherAddr, logger)
	require.Equal(t, 1, len(data))
	require.Equal(t, 0, len(blobHashes))

	// create a valid blob batcher tx and make sure it's picked up
	blobHash := testutils.RandomHash(rng)
	blobTxData := &types.BlobTx{
		Nonce:      rng.Uint64(),
		Gas:        2_000_000,
		To:         batchInboxAddr,
		Data:       testutils.RandomData(rng, rng.Intn(1000)),
		BlobHashes: []common.Hash{blobHash},
	}
	blobTx, _ := types.SignNewTx(privateKey, signer, blobTxData)
	txs = types.Transactions{blobTx}
	data, blobHashes = dataAndHashesFromTxs(txs, &config, batcherAddr, logger)
	require.Equal(t, 1, len(data))
	require.Equal(t, 1, len(blobHashes))
	require.Nil(t, data[0].calldata)

	// try again with both the blob & calldata transactions and make sure both are picked up
	txs = types.Transactions{blobTx, calldataTx}
	data, blobHashes = dataAndHashesFromTxs(txs, &config, batcherAddr, logger)
	require.Equal(t, 2, len(data))
	require.Equal(t, 1, len(blobHashes))
	require.NotNil(t, data[1].calldata)

	// make sure blob tx to the batch inbox is ignored if not signed by the batcher
	blobTx, _ = types.SignNewTx(testutils.RandomKey(), signer, blobTxData)
	txs = types.Transactions{blobTx}
	data, blobHashes = dataAndHashesFromTxs(txs, &config, batcherAddr, logger)
	require.Equal(t, 0, len(data))
	require.Equal(t, 0, len(blobHashes))

	// make sure blob tx ignored if the tx isn't going to the batch inbox addr, even if the
	// signature is valid.
	blobTxData.To = testutils.RandomAddress(rng)
	blobTx, _ = types.SignNewTx(privateKey, signer, blobTxData)
	txs = types.Transactions{blobTx}
	data, blobHashes = dataAndHashesFromTxs(txs, &config, batcherAddr, logger)
	require.Equal(t, 0, len(data))
	require.Equal(t, 0, len(blobHashes))

	// make sure SetCode transactions are ignored.
	setCodeTxData := &types.SetCodeTx{
		Nonce: rng.Uint64(),
		Gas:   2_000_000,
		To:    batchInboxAddr,
		Data:  testutils.RandomData(rng, rng.Intn(1000)),
	}
	setCodeTx, err := types.SignNewTx(privateKey, signer, setCodeTxData)
	require.NoError(t, err)
	txs = types.Transactions{setCodeTx}
	data, blobHashes = dataAndHashesFromTxs(txs, &config, batcherAddr, logger)
	require.Equal(t, 0, len(data))
	require.Equal(t, 0, len(blobHashes))
}

func TestFillBlobPointers(t *testing.T) {
	blob := eth.Blob{}
	rng := rand.New(rand.NewSource(1234))
	calldata := eth.Data{}

	for i := 0; i < 100; i++ {
		// create a random length input data array w/ len = [0-10)
		dataLen := rng.Intn(10)
		data := make([]blobOrCalldata, dataLen)

		// pick some subset of those to be blobs, and the rest calldata
		blobLen := 0
		if dataLen != 0 {
			blobLen = rng.Intn(dataLen)
		}
		calldataLen := dataLen - blobLen

		// fill in the calldata entries at random indices
		for j := 0; j < calldataLen; j++ {
			randomIndex := rng.Intn(dataLen)
			for data[randomIndex].calldata != nil {
				randomIndex = (randomIndex + 1) % dataLen
			}
			data[randomIndex].calldata = &calldata
		}

		// create the input blobs array and call fillBlobPointers on it
		blobs := make([]*eth.Blob, blobLen)
		for j := 0; j < blobLen; j++ {
			blobs[j] = &blob
		}
		err := fillBlobPointers(data, blobs)
		require.NoError(t, err)

		// check that we get the expected number of calldata vs blobs results
		blobCount := 0
		calldataCount := 0
		for j := 0; j < dataLen; j++ {
			if data[j].calldata != nil {
				calldataCount++
			}
			if data[j].blob != nil {
				blobCount++
			}
		}
		require.Equal(t, blobLen, blobCount)
		require.Equal(t, calldataLen, calldataCount)
	}
}
