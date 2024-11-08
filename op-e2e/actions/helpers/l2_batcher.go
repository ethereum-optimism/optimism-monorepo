package helpers

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"io"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-batcher/batcher"
	"github.com/ethereum-optimism/optimism/op-batcher/compressor"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

type SyncStatusAPI interface {
	SyncStatus(ctx context.Context) (*eth.SyncStatus, error)
}

type BlocksAPI interface {
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
}

type L1TxAPI interface {
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

type AltDAInputSetter interface {
	SetInput(ctx context.Context, img []byte) (altda.CommitmentData, error)
}

type BatcherCfg struct {
	// Limit the size of txs
	MinL1TxSize uint64
	MaxL1TxSize uint64

	BatcherKey *ecdsa.PrivateKey

	GarbageCfg *GarbageChannelCfg

	ForceSubmitSingularBatch bool
	ForceSubmitSpanBatch     bool
	UseAltDA                 bool

	DataAvailabilityType batcherFlags.DataAvailabilityType
	AltDA                AltDAInputSetter
}

func DefaultBatcherCfg(dp *e2eutils.DeployParams) *BatcherCfg {
	return &BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           dp.Secrets.Batcher,
		DataAvailabilityType: batcherFlags.CalldataType,
	}
}

func AltDABatcherCfg(dp *e2eutils.DeployParams, altDA AltDAInputSetter) *BatcherCfg {
	return &BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           dp.Secrets.Batcher,
		DataAvailabilityType: batcherFlags.CalldataType,
		AltDA:                altDA,
		UseAltDA:             true,
	}
}

type L2BlockRefs interface {
	L2BlockRefByHash(ctx context.Context, hash common.Hash) (eth.L2BlockRef, error)
}

// L2Batcher buffers and submits L2 batches to L1.
//
// TODO: note the batcher shares little logic/state with actual op-batcher,
// tests should only use this actor to build batch contents for rollup node actors to consume,
// until the op-batcher is refactored and can be covered better.
type L2Batcher struct {
	log log.Logger

	rollupCfg *rollup.Config

	syncStatusAPI SyncStatusAPI
	l2            BlocksAPI
	l1            L1TxAPI
	engCl         L2BlockRefs

	l1Signer types.Signer

	L2ChannelOut     ChannelOutIface
	l2Submitting     bool // when the channel out is being submitted, and not safe to write to without resetting
	L2BufferedBlock  eth.L2BlockRef
	l2SubmittedBlock eth.L2BlockRef
	l2BatcherCfg     *BatcherCfg
	BatcherAddr      common.Address

	LastSubmitted *types.Transaction
}

func NewL2Batcher(log log.Logger, rollupCfg *rollup.Config, batcherCfg *BatcherCfg, api SyncStatusAPI, l1 L1TxAPI, l2 BlocksAPI, engCl L2BlockRefs) *L2Batcher {
	return &L2Batcher{
		log:           log,
		rollupCfg:     rollupCfg,
		syncStatusAPI: api,
		l1:            l1,
		l2:            l2,
		engCl:         engCl,
		l2BatcherCfg:  batcherCfg,
		l1Signer:      types.LatestSignerForChainID(rollupCfg.L1ChainID),
		BatcherAddr:   crypto.PubkeyToAddress(batcherCfg.BatcherKey.PublicKey),
	}
}

// SubmittingData indicates if the actor is submitting buffer data.
// All data must be submitted before it can safely continue buffering more L2 blocks.
func (s *L2Batcher) SubmittingData() bool {
	return s.l2Submitting
}

// Reset the batcher state, clearing any buffered data.
func (s *L2Batcher) Reset() {
	s.L2ChannelOut = nil
	s.l2Submitting = false
	s.L2BufferedBlock = eth.L2BlockRef{}
	s.l2SubmittedBlock = eth.L2BlockRef{}
}

// ActL2BatchBuffer adds the next L2 block to the batch buffer.
// If the buffer is being submitted, the buffer is wiped.
func (s *L2Batcher) ActL2BatchBuffer(t Testing, opts ...BlockModifier) {
	require.NoError(t, s.Buffer(t, opts...), "failed to add block to channel")
}

// ActCreateChannel creates a channel if we don't have one yet.
func (s *L2Batcher) ActCreateChannel(t Testing, useSpanChannelOut bool) {
	var err error
	if s.L2ChannelOut == nil {
		var ch ChannelOutIface
		if s.l2BatcherCfg.GarbageCfg != nil {
			ch, err = NewGarbageChannelOut(s.l2BatcherCfg.GarbageCfg)
		} else {
			target := batcher.MaxDataSize(1, s.l2BatcherCfg.MaxL1TxSize)
			c, e := compressor.NewShadowCompressor(compressor.Config{
				TargetOutputSize: target,
				CompressionAlgo:  derive.Zlib,
			})
			require.NoError(t, e, "failed to create compressor")

			if s.l2BatcherCfg.ForceSubmitSingularBatch && s.l2BatcherCfg.ForceSubmitSpanBatch {
				t.Fatalf("ForceSubmitSingularBatch and ForceSubmitSpanBatch cannot be set to true at the same time")
			} else {
				chainSpec := rollup.NewChainSpec(s.rollupCfg)
				// use span batch if we're forcing it or if we're at/beyond delta
				if s.l2BatcherCfg.ForceSubmitSpanBatch || useSpanChannelOut {
					ch, err = derive.NewSpanChannelOut(target, derive.Zlib, chainSpec)
					// use singular batches in all other cases
				} else {
					ch, err = derive.NewSingularChannelOut(c, chainSpec)
				}
			}
		}
		require.NoError(t, err, "failed to create channel")
		s.L2ChannelOut = ch
	}
}

type BlockModifier = func(block *types.Block) *types.Block

func BlockLogger(t e2eutils.TestingBase) BlockModifier {
	f := func(block *types.Block) *types.Block {
		t.Log("added block", "num", block.Number(), "txs", block.Transactions(), "time", block.Time())
		return block
	}
	return f
}

func (s *L2Batcher) Buffer(t Testing, opts ...BlockModifier) error {
	if s.l2Submitting { // break ongoing submitting work if necessary
		s.L2ChannelOut = nil
		s.l2Submitting = false
	}
	syncStatus, err := s.syncStatusAPI.SyncStatus(t.Ctx())
	require.NoError(t, err, "no sync status error")
	// If we just started, start at safe-head
	if s.l2SubmittedBlock == (eth.L2BlockRef{}) {
		s.log.Info("Starting batch-submitter work at safe-head", "safe", syncStatus.SafeL2)
		s.l2SubmittedBlock = syncStatus.SafeL2
		s.L2BufferedBlock = syncStatus.SafeL2
		s.L2ChannelOut = nil
	}
	// If it's lagging behind, catch it up.
	if s.l2SubmittedBlock.Number < syncStatus.SafeL2.Number {
		s.log.Warn("last submitted block lagged behind L2 safe head: batch submission will continue from the safe head now", "last", s.l2SubmittedBlock, "safe", syncStatus.SafeL2)
		s.l2SubmittedBlock = syncStatus.SafeL2
		s.L2BufferedBlock = syncStatus.SafeL2
		s.L2ChannelOut = nil
	}
	// Add the next unsafe block to the channel
	if s.L2BufferedBlock.Number >= syncStatus.UnsafeL2.Number {
		if s.L2BufferedBlock.Number > syncStatus.UnsafeL2.Number || s.L2BufferedBlock.Hash != syncStatus.UnsafeL2.Hash {
			s.log.Error("detected a reorg in L2 chain vs previous buffered information, resetting to safe head now", "safe_head", syncStatus.SafeL2)
			s.l2SubmittedBlock = syncStatus.SafeL2
			s.L2BufferedBlock = syncStatus.SafeL2
			s.L2ChannelOut = nil
		} else {
			s.log.Info("nothing left to submit")
			return nil
		}
	}

	block, err := s.l2.BlockByNumber(t.Ctx(), big.NewInt(int64(s.L2BufferedBlock.Number+1)))
	require.NoError(t, err, "need l2 block %d from sync status", s.l2SubmittedBlock.Number+1)
	if block.ParentHash() != s.L2BufferedBlock.Hash {
		s.log.Error("detected a reorg in L2 chain vs previous submitted information, resetting to safe head now", "safe_head", syncStatus.SafeL2)
		s.l2SubmittedBlock = syncStatus.SafeL2
		s.L2BufferedBlock = syncStatus.SafeL2
		s.L2ChannelOut = nil
	}

	// Apply modifications to the block
	for _, f := range opts {
		if f != nil {
			block = f(block)
		}
	}

	s.ActCreateChannel(t, s.rollupCfg.IsDelta(block.Time()))

	if _, err := s.L2ChannelOut.AddBlock(s.rollupCfg, block); err != nil {
		return err
	}
	ref, err := s.engCl.L2BlockRefByHash(t.Ctx(), block.Hash())
	require.NoError(t, err, "failed to get L2BlockRef")
	s.L2BufferedBlock = ref
	return nil
}

// ActAddBlockByNumber causes the batcher to pull the block with the provided
// number, and add it to its ChannelOut.
func (s *L2Batcher) ActAddBlockByNumber(t Testing, blockNumber int64, opts ...BlockModifier) {
	block, err := s.l2.BlockByNumber(t.Ctx(), big.NewInt(blockNumber))
	require.NoError(t, err)
	require.NotNil(t, block)

	// cache block hash before we modify the block
	blockHash := block.Hash()

	// Apply modifications to the block
	for _, f := range opts {
		if f != nil {
			block = f(block)
		}
	}

	_, err = s.L2ChannelOut.AddBlock(s.rollupCfg, block)
	require.NoError(t, err)
	ref, err := s.engCl.L2BlockRefByHash(t.Ctx(), blockHash)
	require.NoError(t, err, "failed to get L2BlockRef")
	s.L2BufferedBlock = ref
}

func (s *L2Batcher) ActL2ChannelClose(t Testing) {
	// Don't run this action if there's no data to submit
	if s.L2ChannelOut == nil {
		t.InvalidAction("need to buffer data first, cannot batch submit with empty buffer")
		return
	}
	require.NoError(t, s.L2ChannelOut.Close(), "must close channel before submitting it")
}

func (s *L2Batcher) ReadNextOutputFrame(t Testing) []byte {
	// Don't run this action if there's no data to submit
	if s.L2ChannelOut == nil {
		t.InvalidAction("need to buffer data first, cannot batch submit with empty buffer")
		return nil
	}
	// Collect the output frame
	data := new(bytes.Buffer)
	data.WriteByte(derive.DerivationVersion0)
	// subtract one, to account for the version byte
	if _, err := s.L2ChannelOut.OutputFrame(data, s.l2BatcherCfg.MaxL1TxSize-1); err == io.EOF {
		s.L2ChannelOut = nil
		s.l2Submitting = false
	} else if err != nil {
		s.l2Submitting = false
		t.Fatalf("failed to output channel data to frame: %v", err)
	}

	return data.Bytes()
}

// ActL2BatchSubmit constructs a batch tx from previous buffered L2 blocks, and submits it to L1
func (s *L2Batcher) ActL2BatchSubmit(t Testing, txOpts ...func(tx *types.DynamicFeeTx)) {
	s.ActL2BatchSubmitRaw(t, s.ReadNextOutputFrame(t), txOpts...)
}

func (s *L2Batcher) ActL2BatchSubmitRaw(t Testing, payload []byte, txOpts ...func(tx *types.DynamicFeeTx)) {
	if s.l2BatcherCfg.UseAltDA {
		comm, err := s.l2BatcherCfg.AltDA.SetInput(t.Ctx(), payload)
		require.NoError(t, err, "failed to set input for altda")
		payload = comm.TxData()
	}

	nonce, err := s.l1.PendingNonceAt(t.Ctx(), s.BatcherAddr)
	require.NoError(t, err, "need batcher nonce")

	gasTipCap := big.NewInt(2 * params.GWei)
	pendingHeader, err := s.l1.HeaderByNumber(t.Ctx(), big.NewInt(-1))
	require.NoError(t, err, "need l1 pending header for gas price estimation")
	gasFeeCap := new(big.Int).Add(gasTipCap, new(big.Int).Mul(pendingHeader.BaseFee, big.NewInt(2)))

	var txData types.TxData
	if s.l2BatcherCfg.DataAvailabilityType == batcherFlags.CalldataType {
		rawTx := &types.DynamicFeeTx{
			ChainID:   s.rollupCfg.L1ChainID,
			Nonce:     nonce,
			To:        &s.rollupCfg.BatchInboxAddress,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			Data:      payload,
		}
		for _, opt := range txOpts {
			opt(rawTx)
		}

		gas, err := core.IntrinsicGas(rawTx.Data, nil, false, true, true, false)
		require.NoError(t, err, "need to compute intrinsic gas")
		rawTx.Gas = gas
		txData = rawTx
	} else if s.l2BatcherCfg.DataAvailabilityType == batcherFlags.BlobsType {
		var b eth.Blob
		require.NoError(t, b.FromData(payload), "must turn data into blob")
		sidecar, blobHashes, err := txmgr.MakeSidecar([]*eth.Blob{&b})
		require.NoError(t, err)
		require.NotNil(t, pendingHeader.ExcessBlobGas, "need L1 header with 4844 properties")
		blobBaseFee := eip4844.CalcBlobFee(*pendingHeader.ExcessBlobGas)
		blobFeeCap := new(uint256.Int).Mul(uint256.NewInt(2), uint256.MustFromBig(blobBaseFee))
		if blobFeeCap.Lt(uint256.NewInt(params.GWei)) { // ensure we meet 1 gwei geth tx-pool minimum
			blobFeeCap = uint256.NewInt(params.GWei)
		}
		txData = &types.BlobTx{
			To:         s.rollupCfg.BatchInboxAddress,
			Data:       nil,
			Gas:        params.TxGas, // intrinsic gas only
			BlobHashes: blobHashes,
			Sidecar:    sidecar,
			ChainID:    uint256.MustFromBig(s.rollupCfg.L1ChainID),
			GasTipCap:  uint256.MustFromBig(gasTipCap),
			GasFeeCap:  uint256.MustFromBig(gasFeeCap),
			BlobFeeCap: blobFeeCap,
			Value:      uint256.NewInt(0),
			Nonce:      nonce,
		}
	} else {
		t.Fatalf("unrecognized DA type: %q", string(s.l2BatcherCfg.DataAvailabilityType))
	}

	tx, err := types.SignNewTx(s.l2BatcherCfg.BatcherKey, s.l1Signer, txData)
	require.NoError(t, err, "need to sign tx")

	err = s.l1.SendTransaction(t.Ctx(), tx)
	require.NoError(t, err, "need to send tx")
	s.LastSubmitted = tx
}

func (s *L2Batcher) ActL2BatchSubmitMultiBlob(t Testing, numBlobs int) {
	if s.l2BatcherCfg.DataAvailabilityType != batcherFlags.BlobsType {
		t.InvalidAction("ActL2BatchSubmitMultiBlob only available for Blobs DA type")
		return
	} else if numBlobs > eth.MaxBlobsPerBlobTx || numBlobs < 1 {
		t.InvalidAction("invalid number of blobs %d, must be within [1,%d]", numBlobs, eth.MaxBlobsPerBlobTx)
	}

	// Don't run this action if there's no data to submit
	if s.L2ChannelOut == nil {
		t.InvalidAction("need to buffer data first, cannot batch submit with empty buffer")
		return
	}

	// Collect the output frames into blobs
	blobs := make([]*eth.Blob, numBlobs)
	for i := 0; i < numBlobs; i++ {
		data := new(bytes.Buffer)
		data.WriteByte(derive.DerivationVersion0)
		// write only a few bytes to all but the last blob
		l := uint64(derive.FrameV0OverHeadSize + 4) // 4 bytes content
		if i == numBlobs-1 {
			// write remaining channel to last frame
			// subtract one, to account for the version byte
			l = s.l2BatcherCfg.MaxL1TxSize - 1
		}
		if _, err := s.L2ChannelOut.OutputFrame(data, l); err == io.EOF {
			s.l2Submitting = false
			if i < numBlobs-1 {
				t.Fatalf("failed to fill up %d blobs, only filled %d", numBlobs, i+1)
			}
			s.L2ChannelOut = nil
		} else if err != nil {
			s.l2Submitting = false
			t.Fatalf("failed to output channel data to frame: %v", err)
		}

		blobs[i] = new(eth.Blob)
		require.NoError(t, blobs[i].FromData(data.Bytes()), "must turn data into blob")
	}

	nonce, err := s.l1.PendingNonceAt(t.Ctx(), s.BatcherAddr)
	require.NoError(t, err, "need batcher nonce")

	gasTipCap := big.NewInt(2 * params.GWei)
	pendingHeader, err := s.l1.HeaderByNumber(t.Ctx(), big.NewInt(-1))
	require.NoError(t, err, "need l1 pending header for gas price estimation")
	gasFeeCap := new(big.Int).Add(gasTipCap, new(big.Int).Mul(pendingHeader.BaseFee, big.NewInt(2)))

	sidecar, blobHashes, err := txmgr.MakeSidecar(blobs)
	require.NoError(t, err)
	require.NotNil(t, pendingHeader.ExcessBlobGas, "need L1 header with 4844 properties")
	blobBaseFee := eip4844.CalcBlobFee(*pendingHeader.ExcessBlobGas)
	blobFeeCap := new(uint256.Int).Mul(uint256.NewInt(2), uint256.MustFromBig(blobBaseFee))
	if blobFeeCap.Lt(uint256.NewInt(params.GWei)) { // ensure we meet 1 gwei geth tx-pool minimum
		blobFeeCap = uint256.NewInt(params.GWei)
	}
	txData := &types.BlobTx{
		To:         s.rollupCfg.BatchInboxAddress,
		Data:       nil,
		Gas:        params.TxGas, // intrinsic gas only
		BlobHashes: blobHashes,
		Sidecar:    sidecar,
		ChainID:    uint256.MustFromBig(s.rollupCfg.L1ChainID),
		GasTipCap:  uint256.MustFromBig(gasTipCap),
		GasFeeCap:  uint256.MustFromBig(gasFeeCap),
		BlobFeeCap: blobFeeCap,
		Value:      uint256.NewInt(0),
		Nonce:      nonce,
	}

	tx, err := types.SignNewTx(s.l2BatcherCfg.BatcherKey, s.l1Signer, txData)
	require.NoError(t, err, "need to sign tx")

	err = s.l1.SendTransaction(t.Ctx(), tx)
	require.NoError(t, err, "need to send tx")
	s.LastSubmitted = tx
}

// ActL2BatchSubmitGarbage constructs a malformed channel frame and submits it to the
// batch inbox. This *should* cause the batch inbox to reject the blocks
// encoded within the frame, even if the blocks themselves are valid.
func (s *L2Batcher) ActL2BatchSubmitGarbage(t Testing, kind GarbageKind) {
	outputFrame := s.ReadNextOutputFrame(t)
	s.ActL2BatchSubmitGarbageRaw(t, outputFrame, kind)
}

// ActL2BatchSubmitGarbageRaw constructs a malformed channel frame from `outputFrame` and submits it to the
// batch inbox. This *should* cause the batch inbox to reject the blocks
// encoded within the frame, even if the blocks themselves are valid.
func (s *L2Batcher) ActL2BatchSubmitGarbageRaw(t Testing, outputFrame []byte, kind GarbageKind) {
	// Malform the output frame
	switch kind {
	// Strip the derivation version byte from the output frame
	case STRIP_VERSION:
		outputFrame = outputFrame[1:]
	// Replace the output frame with random bytes of length [1, 512]
	case RANDOM:
		i, err := rand.Int(rand.Reader, big.NewInt(512))
		require.NoError(t, err, "error generating random bytes length")
		buf := make([]byte, i.Int64()+1)
		_, err = rand.Read(buf)
		require.NoError(t, err, "error generating random bytes")
		outputFrame = buf
	// Remove 4 bytes from the tail end of the output frame
	case TRUNCATE_END:
		outputFrame = outputFrame[:len(outputFrame)-4]
	// Append 4 garbage bytes to the end of the output frame
	case DIRTY_APPEND:
		outputFrame = append(outputFrame, []byte{0xBA, 0xD0, 0xC0, 0xDE}...)
	case INVALID_COMPRESSION:
		// Do nothing post frame encoding- the `GarbageChannelOut` used for this case is modified to
		// use gzip compression rather than zlib, which is invalid.
		break
	case MALFORM_RLP:
		// Do nothing post frame encoding- the `GarbageChannelOut` used for this case is modified to
		// write malformed RLP each time a block is added to the channel.
		break
	default:
		t.Fatalf("Unexpected garbage kind: %v", kind)
	}

	s.ActL2BatchSubmitRaw(t, outputFrame)
}

func (s *L2Batcher) ActBufferAll(t Testing) {
	stat, err := s.syncStatusAPI.SyncStatus(t.Ctx())
	require.NoError(t, err)
	for s.L2BufferedBlock.Number < stat.UnsafeL2.Number {
		s.ActL2BatchBuffer(t)
	}
}

func (s *L2Batcher) ActSubmitAll(t Testing) {
	s.ActBufferAll(t)
	s.ActL2ChannelClose(t)
	s.ActL2BatchSubmit(t)
}

func (s *L2Batcher) ActSubmitAllMultiBlobs(t Testing, numBlobs int) {
	s.ActBufferAll(t)
	s.ActL2ChannelClose(t)
	s.ActL2BatchSubmitMultiBlob(t, numBlobs)
}
