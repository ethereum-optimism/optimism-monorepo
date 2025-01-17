package derive_test

import (
	"context"
	"io"
	"math/big"
	"math/rand"
	"testing"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	. "github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockFinalitySignal struct {
	mock.Mock
}

func (m *MockFinalitySignal) OnFinalized(blockRef eth.L1BlockRef) {
	m.MethodCalled("OnFinalized", blockRef)
}

func (m *MockFinalitySignal) ExpectFinalized(blockRef eth.L1BlockRef) {
	m.On("OnFinalized", blockRef).Once()
}

// TestAltDADataSource verifies that commitments are correctly read from l1 and then
// forwarded to the AltDA to return the correct inputs in the iterator.
// First it generates some L1 refs containing a random number of commitments, challenges
// the first 4 commitments then generates enough blocks to expire the challenge.
// Then it simulates rederiving while verifying it does skip the expired input until the next
// challenge expires.
func TestAltDADataSource(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	rng := rand.New(rand.NewSource(1234))

	l1F := &testutils.MockL1Source{}

	storage := altda.NewMockDAClient(logger)

	pcfg := altda.Config{
		ChallengeWindow: 90, ResolveWindow: 90,
	}
	metrics := &altda.NoopMetrics{}

	daState := altda.NewState(logger, metrics, pcfg)

	da := altda.NewAltDAWithState(logger, pcfg, storage, metrics, daState)

	finalitySignal := &MockFinalitySignal{}
	da.OnFinalizedHeadSignal(finalitySignal.OnFinalized)

	// Create rollup genesis and config
	l1Time := uint64(2)
	refA := testutils.RandomBlockRef(rng)
	refA.Number = 1
	l1Refs := []eth.L1BlockRef{refA}
	refA0 := eth.L2BlockRef{
		Hash:           testutils.RandomHash(rng),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           refA.Time,
		L1Origin:       refA.ID(),
		SequenceNumber: 0,
	}
	batcherPriv := testutils.RandomKey()
	batcherAddr := crypto.PubkeyToAddress(batcherPriv.PublicKey)
	batcherInbox := common.Address{42}
	cfg := &rollup.Config{
		Genesis: rollup.Genesis{
			L1:     refA.ID(),
			L2:     refA0.ID(),
			L2Time: refA0.Time,
		},
		BlockTime:         1,
		SeqWindowSize:     20,
		BatchInboxAddress: batcherInbox,
		AltDAConfig: &rollup.AltDAConfig{
			DAChallengeWindow: pcfg.ChallengeWindow,
			DAResolveWindow:   pcfg.ResolveWindow,
			CommitmentType:    altda.KeccakCommitmentString,
		},
	}
	// keep track of random input data to validate against
	var inputs [][]byte
	var comms []altda.CommitmentData
	var inclusionBlocks []eth.L1BlockRef

	signer := cfg.L1Signer()

	factory := NewDataSourceFactory(logger, cfg, l1F, nil, da)

	nc := 0
	firstChallengeExpirationBlock := uint64(95)

	for i := uint64(0); i <= pcfg.ChallengeWindow+pcfg.ResolveWindow; i++ {
		parent := l1Refs[len(l1Refs)-1]
		// create a new mock l1 ref
		ref := eth.L1BlockRef{
			Hash:       testutils.RandomHash(rng),
			Number:     parent.Number + 1,
			ParentHash: parent.Hash,
			Time:       parent.Time + l1Time,
		}
		l1Refs = append(l1Refs, ref)
		logger.Info("new l1 block", "ref", ref)
		// called for each l1 block to sync challenges
		l1F.ExpectFetchReceipts(ref.Hash, nil, types.Receipts{}, nil)

		// pick a random number of commitments to include in the l1 block
		c := rng.Intn(4)
		var txs []eth.GenericTx

		for j := 0; j < c; j++ {
			// mock input commitments in l1 transactions
			input := testutils.RandomData(rng, 2000)
			comm, _ := storage.SetInput(ctx, input)
			// altDA tests are designed for keccak256 commitments, so we type assert here
			kComm := comm.(altda.Keccak256Commitment)
			inputs = append(inputs, input)
			comms = append(comms, kComm)
			inclusionBlocks = append(inclusionBlocks, ref)

			tx, err := types.SignNewTx(batcherPriv, signer, &types.DynamicFeeTx{
				ChainID:   signer.ChainID(),
				Nonce:     0,
				GasTipCap: big.NewInt(2 * params.GWei),
				GasFeeCap: big.NewInt(30 * params.GWei),
				Gas:       100_000,
				To:        &batcherInbox,
				Value:     big.NewInt(int64(0)),
				Data:      comm.TxData(),
			})
			require.NoError(t, err)

			txs = append(txs, eth.MustToOpaqueTransaction(tx))

		}
		logger.Info("included commitments", "count", c)
		l1F.ExpectInfoAndTxsByHash(ref.Hash, testutils.RandomBlockInfo(rng), txs, nil)
		// called once per derivation
		l1F.ExpectInfoAndTxsByHash(ref.Hash, testutils.RandomBlockInfo(rng), txs, nil)

		if ref.Number == 2 {
			l1F.ExpectL1BlockRefByNumber(ref.Number, ref, nil)
			finalitySignal.ExpectFinalized(ref)
		}

		// challenge the first 4 commitments as soon as we have collected them all
		if len(comms) >= 4 && nc < 7 {
			// skip a block between each challenge transaction
			if nc%2 == 0 {
				daState.CreateChallenge(comms[nc/2], ref.ID(), inclusionBlocks[nc/2].Number)
				logger.Info("setting active challenge", "comm", comms[nc/2])
			}
			nc++
		}

		// create a new data source for each block
		src, err := factory.OpenData(ctx, ref, batcherAddr)
		require.NoError(t, err)

		// first challenge expires
		if i == firstChallengeExpirationBlock {
			_, err := src.Next(ctx)
			require.ErrorIs(t, err, ErrReset)
			break
		}

		for j := 0; j < c; j++ {
			data, err := src.Next(ctx)
			// check that each commitment is resolved
			require.NoError(t, err)
			require.Equal(t, hexutil.Bytes(inputs[len(inputs)-(c-j)]), data)
		}
		// returns EOF once done
		_, err = src.Next(ctx)
		require.ErrorIs(t, err, io.EOF)
	}

	logger.Info("pipeline reset ..................................")

	// start at 1 since first input should be skipped
	nc = 1
	secondChallengeExpirationBlock := 98

	for i := 1; i <= len(l1Refs)+2; i++ {

		var ref eth.L1BlockRef
		// first we run through all the existing l1 blocks
		if i < len(l1Refs) {
			ref = l1Refs[i]
			logger.Info("re deriving block", "ref", ref, "i", i)

			if i == len(l1Refs)-1 {
				l1F.ExpectFetchReceipts(ref.Hash, nil, types.Receipts{}, nil)
			}
			// once past the l1 head, continue generating new l1 refs
		} else {
			parent := l1Refs[len(l1Refs)-1]
			// create a new mock l1 ref
			ref = eth.L1BlockRef{
				Hash:       testutils.RandomHash(rng),
				Number:     parent.Number + 1,
				ParentHash: parent.Hash,
				Time:       parent.Time + l1Time,
			}
			l1Refs = append(l1Refs, ref)
			logger.Info("new l1 block", "ref", ref)
			// called for each l1 block to sync challenges
			l1F.ExpectFetchReceipts(ref.Hash, nil, types.Receipts{}, nil)

			// pick a random number of commitments to include in the l1 block
			c := rng.Intn(4)
			var txs []eth.GenericTx

			for j := 0; j < c; j++ {
				// mock input commitments in l1 transactions
				input := testutils.RandomData(rng, 2000)
				comm, _ := storage.SetInput(ctx, input)
				// altDA tests are designed for keccak256 commitments, so we type assert here
				kComm := comm.(altda.Keccak256Commitment)
				inputs = append(inputs, input)
				comms = append(comms, kComm)

				tx, err := types.SignNewTx(batcherPriv, signer, &types.DynamicFeeTx{
					ChainID:   signer.ChainID(),
					Nonce:     0,
					GasTipCap: big.NewInt(2 * params.GWei),
					GasFeeCap: big.NewInt(30 * params.GWei),
					Gas:       100_000,
					To:        &batcherInbox,
					Value:     big.NewInt(int64(0)),
					Data:      comm.TxData(),
				})
				require.NoError(t, err)

				txs = append(txs, eth.MustToOpaqueTransaction(tx))

			}
			logger.Info("included commitments", "count", c)
			l1F.ExpectInfoAndTxsByHash(ref.Hash, testutils.RandomBlockInfo(rng), txs, nil)
		}

		// create a new data source for each block
		src, err := factory.OpenData(ctx, ref, batcherAddr)
		require.NoError(t, err)

		// next challenge expires
		if i == secondChallengeExpirationBlock {
			_, err := src.Next(ctx)
			require.ErrorIs(t, err, ErrReset)
			break
		}

		for data, err := src.Next(ctx); err != io.EOF; data, err = src.Next(ctx) {
			logger.Info("yielding data")
			// check that each commitment is resolved
			require.NoError(t, err)
			require.Equal(t, hexutil.Bytes(inputs[nc]), data)

			nc++
		}

	}

	// finalize based on the second to last block, which will prune the commitment on block 2, and make it finalized
	da.Finalize(l1Refs[len(l1Refs)-2])
	finalitySignal.AssertExpectations(t)
}

// This tests makes sure the pipeline returns a temporary error if data is not found.
func TestAltDADataSourceStall(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	rng := rand.New(rand.NewSource(1234))

	l1F := &testutils.MockL1Source{}

	storage := altda.NewMockDAClient(logger)

	pcfg := altda.Config{
		ChallengeWindow: 90, ResolveWindow: 90,
	}

	metrics := &altda.NoopMetrics{}

	daState := altda.NewState(logger, metrics, pcfg)

	da := altda.NewAltDAWithState(logger, pcfg, storage, metrics, daState)

	finalitySignal := &MockFinalitySignal{}
	da.OnFinalizedHeadSignal(finalitySignal.OnFinalized)

	// Create rollup genesis and config
	l1Time := uint64(2)
	refA := testutils.RandomBlockRef(rng)
	refA.Number = 1
	l1Refs := []eth.L1BlockRef{refA}
	refA0 := eth.L2BlockRef{
		Hash:           testutils.RandomHash(rng),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           refA.Time,
		L1Origin:       refA.ID(),
		SequenceNumber: 0,
	}
	batcherPriv := testutils.RandomKey()
	batcherAddr := crypto.PubkeyToAddress(batcherPriv.PublicKey)
	batcherInbox := common.Address{42}
	cfg := &rollup.Config{
		Genesis: rollup.Genesis{
			L1:     refA.ID(),
			L2:     refA0.ID(),
			L2Time: refA0.Time,
		},
		BlockTime:         1,
		SeqWindowSize:     20,
		BatchInboxAddress: batcherInbox,
		AltDAConfig: &rollup.AltDAConfig{
			DAChallengeWindow: pcfg.ChallengeWindow,
			DAResolveWindow:   pcfg.ResolveWindow,
			CommitmentType:    altda.KeccakCommitmentString,
		},
	}

	signer := cfg.L1Signer()

	factory := NewDataSourceFactory(logger, cfg, l1F, nil, da)

	parent := l1Refs[0]
	// create a new mock l1 ref
	ref := eth.L1BlockRef{
		Hash:       testutils.RandomHash(rng),
		Number:     parent.Number + 1,
		ParentHash: parent.Hash,
		Time:       parent.Time + l1Time,
	}
	l1F.ExpectFetchReceipts(ref.Hash, nil, types.Receipts{}, nil)
	// mock input commitments in l1 transactions
	input := testutils.RandomData(rng, 2000)
	comm, _ := storage.SetInput(ctx, input)

	tx, err := types.SignNewTx(batcherPriv, signer, &types.DynamicFeeTx{
		ChainID:   signer.ChainID(),
		Nonce:     0,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: big.NewInt(30 * params.GWei),
		Gas:       100_000,
		To:        &batcherInbox,
		Value:     big.NewInt(int64(0)),
		Data:      comm.TxData(),
	})
	require.NoError(t, err)

	txs := []eth.GenericTx{eth.MustToOpaqueTransaction(tx)}

	l1F.ExpectInfoAndTxsByHash(ref.Hash, testutils.RandomBlockInfo(rng), txs, nil)

	// delete the input from the DA provider so it returns not found
	require.NoError(t, storage.DeleteData(comm.Encode()))

	// next block is fetched to look ahead challenges but is not yet available
	l1F.ExpectL1BlockRefByNumber(ref.Number+1, eth.L1BlockRef{}, ethereum.NotFound)

	src, err := factory.OpenData(ctx, ref, batcherAddr)
	require.NoError(t, err)

	// data is not found so we return a temporary error
	_, err = src.Next(ctx)
	require.ErrorIs(t, err, ErrTemporary)

	// next block is available with no challenge events
	nextRef := eth.L1BlockRef{
		Number: ref.Number + 1,
		Hash:   testutils.RandomHash(rng),
	}
	l1F.ExpectL1BlockRefByNumber(nextRef.Number, nextRef, nil)
	l1F.ExpectFetchReceipts(nextRef.Hash, nil, types.Receipts{}, nil)

	// not enough data
	_, err = src.Next(ctx)
	require.ErrorIs(t, err, NotEnoughData)

	// create and resolve a challenge
	daState.CreateChallenge(comm, ref.ID(), ref.Number)
	// now challenge is resolved
	err = daState.ResolveChallenge(comm, eth.BlockID{Number: ref.Number + 2}, ref.Number, input)
	require.NoError(t, err)

	// derivation can resume
	data, err := src.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, hexutil.Bytes(input), data)

	l1F.AssertExpectations(t)
}

// TestAltDADataSourceInvalidData tests that the pipeline skips invalid data and continues
// this includes invalid commitments and oversized inputs.
func TestAltDADataSourceInvalidData(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	rng := rand.New(rand.NewSource(1234))

	l1F := &testutils.MockL1Source{}

	storage := altda.NewMockDAClient(logger)

	pcfg := altda.Config{
		ChallengeWindow: 90, ResolveWindow: 90,
	}

	da := altda.NewAltDAWithStorage(logger, pcfg, storage, &altda.NoopMetrics{})

	// Create rollup genesis and config
	l1Time := uint64(2)
	refA := testutils.RandomBlockRef(rng)
	refA.Number = 1
	l1Refs := []eth.L1BlockRef{refA}
	refA0 := eth.L2BlockRef{
		Hash:           testutils.RandomHash(rng),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           refA.Time,
		L1Origin:       refA.ID(),
		SequenceNumber: 0,
	}
	batcherPriv := testutils.RandomKey()
	batcherAddr := crypto.PubkeyToAddress(batcherPriv.PublicKey)
	batcherInbox := common.Address{42}
	cfg := &rollup.Config{
		Genesis: rollup.Genesis{
			L1:     refA.ID(),
			L2:     refA0.ID(),
			L2Time: refA0.Time,
		},
		BlockTime:         1,
		SeqWindowSize:     20,
		BatchInboxAddress: batcherInbox,
		AltDAConfig: &rollup.AltDAConfig{
			DAChallengeWindow: pcfg.ChallengeWindow,
			DAResolveWindow:   pcfg.ResolveWindow,
			CommitmentType:    altda.KeccakCommitmentString,
		},
	}

	signer := cfg.L1Signer()

	factory := NewDataSourceFactory(logger, cfg, l1F, nil, da)

	parent := l1Refs[0]
	// create a new mock l1 ref
	ref := eth.L1BlockRef{
		Hash:       testutils.RandomHash(rng),
		Number:     parent.Number + 1,
		ParentHash: parent.Hash,
		Time:       parent.Time + l1Time,
	}
	l1F.ExpectFetchReceipts(ref.Hash, nil, types.Receipts{}, nil)
	// mock input commitments in l1 transactions with an oversized input
	input := testutils.RandomData(rng, altda.MaxInputSize+1)
	comm, _ := storage.SetInput(ctx, input)

	tx1, err := types.SignNewTx(batcherPriv, signer, &types.DynamicFeeTx{
		ChainID:   signer.ChainID(),
		Nonce:     0,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: big.NewInt(30 * params.GWei),
		Gas:       100_000,
		To:        &batcherInbox,
		Value:     big.NewInt(int64(0)),
		Data:      comm.TxData(),
	})
	require.NoError(t, err)

	// valid data
	input2 := testutils.RandomData(rng, 2000)
	comm2, _ := storage.SetInput(ctx, input2)
	tx2, err := types.SignNewTx(batcherPriv, signer, &types.DynamicFeeTx{
		ChainID:   signer.ChainID(),
		Nonce:     0,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: big.NewInt(30 * params.GWei),
		Gas:       100_000,
		To:        &batcherInbox,
		Value:     big.NewInt(int64(0)),
		Data:      comm2.TxData(),
	})
	require.NoError(t, err)

	// regular input instead of commitment
	input3 := testutils.RandomData(rng, 32)
	tx3, err := types.SignNewTx(batcherPriv, signer, &types.DynamicFeeTx{
		ChainID:   signer.ChainID(),
		Nonce:     0,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: big.NewInt(30 * params.GWei),
		Gas:       100_000,
		To:        &batcherInbox,
		Value:     big.NewInt(int64(0)),
		Data:      input3,
	})
	require.NoError(t, err)

	txs := []eth.GenericTx{
		eth.MustToOpaqueTransaction(tx1),
		eth.MustToOpaqueTransaction(tx2),
		eth.MustToOpaqueTransaction(tx3)}

	l1F.ExpectInfoAndTxsByHash(ref.Hash, testutils.RandomBlockInfo(rng), txs, nil)

	src, err := factory.OpenData(ctx, ref, batcherAddr)
	require.NoError(t, err)

	// oversized input is skipped and returns input2 directly
	data, err := src.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, hexutil.Bytes(input2), data)

	// regular input is passed through
	data, err = src.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, hexutil.Bytes(input3), data)

	_, err = src.Next(ctx)
	require.ErrorIs(t, err, io.EOF)

	l1F.AssertExpectations(t)
}
