package clsync

import (
	"errors"
	"math/big"
	"math/rand" // nosemgrep
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestCLSync(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))

	refA := testutils.RandomBlockRef(rng)

	aL1Info := &testutils.MockBlockInfo{
		InfoParentHash:  refA.ParentHash,
		InfoNum:         refA.Number,
		InfoTime:        refA.Time,
		InfoHash:        refA.Hash,
		InfoBaseFee:     big.NewInt(1),
		InfoBlobBaseFee: big.NewInt(1),
		InfoReceiptRoot: types.EmptyRootHash,
		InfoRoot:        testutils.RandomHash(rng),
		InfoGasUsed:     rng.Uint64(),
	}

	refA0 := eth.L2BlockRef{
		Hash:           testutils.RandomHash(rng),
		Number:         0,
		ParentHash:     common.Hash{},
		Time:           refA.Time,
		L1Origin:       refA.ID(),
		SequenceNumber: 0,
	}
	gasLimit := eth.Uint64Quantity(20_000_000)
	cfg := &rollup.Config{
		Genesis: rollup.Genesis{
			L1:     refA.ID(),
			L2:     refA0.ID(),
			L2Time: refA0.Time,
			SystemConfig: eth.SystemConfig{
				BatcherAddr: common.Address{42},
				Overhead:    [32]byte{123},
				Scalar:      [32]byte{42},
				GasLimit:    20_000_000,
			},
		},
		BlockTime:     1,
		SeqWindowSize: 2,
	}

	refA1 := eth.L2BlockRef{
		Hash:           testutils.RandomHash(rng),
		Number:         refA0.Number + 1,
		ParentHash:     refA0.Hash,
		Time:           refA0.Time + cfg.BlockTime,
		L1Origin:       refA.ID(),
		SequenceNumber: 1,
	}

	altRefA1 := refA1
	altRefA1.Hash = testutils.RandomHash(rng)

	refA2 := eth.L2BlockRef{
		Hash:           testutils.RandomHash(rng),
		Number:         refA1.Number + 1,
		ParentHash:     refA1.Hash,
		Time:           refA1.Time + cfg.BlockTime,
		L1Origin:       refA.ID(),
		SequenceNumber: 2,
	}

	a1L1Info, err := derive.L1InfoDepositBytes(cfg, cfg.Genesis.SystemConfig, refA1.SequenceNumber, aL1Info, refA1.Time)
	require.NoError(t, err)
	payloadA1 := &eth.ExecutionPayloadEnvelope{ExecutionPayload: &eth.ExecutionPayload{
		ParentHash:    refA1.ParentHash,
		FeeRecipient:  common.Address{},
		StateRoot:     eth.Bytes32{},
		ReceiptsRoot:  eth.Bytes32{},
		LogsBloom:     eth.Bytes256{},
		PrevRandao:    eth.Bytes32{},
		BlockNumber:   eth.Uint64Quantity(refA1.Number),
		GasLimit:      gasLimit,
		GasUsed:       0,
		Timestamp:     eth.Uint64Quantity(refA1.Time),
		ExtraData:     nil,
		BaseFeePerGas: eth.Uint256Quantity(*uint256.NewInt(7)),
		BlockHash:     refA1.Hash,
		Transactions:  []eth.Data{a1L1Info},
	}}
	a2L1Info, err := derive.L1InfoDepositBytes(cfg, cfg.Genesis.SystemConfig, refA2.SequenceNumber, aL1Info, refA2.Time)
	require.NoError(t, err)
	payloadA2 := &eth.ExecutionPayloadEnvelope{ExecutionPayload: &eth.ExecutionPayload{
		ParentHash:    refA2.ParentHash,
		FeeRecipient:  common.Address{},
		StateRoot:     eth.Bytes32{},
		ReceiptsRoot:  eth.Bytes32{},
		LogsBloom:     eth.Bytes256{},
		PrevRandao:    eth.Bytes32{},
		BlockNumber:   eth.Uint64Quantity(refA2.Number),
		GasLimit:      gasLimit,
		GasUsed:       0,
		Timestamp:     eth.Uint64Quantity(refA2.Time),
		ExtraData:     nil,
		BaseFeePerGas: eth.Uint256Quantity(*uint256.NewInt(7)),
		BlockHash:     refA2.Hash,
		Transactions:  []eth.Data{a2L1Info},
	}}

	metrics := &testutils.TestDerivationMetrics{}

	// When a previously received unsafe block is older than the tip of the chain, we want to drop it.
	t.Run("drop old", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelError)

		emitter := &testutils.MockEmitter{}
		cl := NewCLSync(logger, cfg, metrics)
		cl.AttachEmitter(emitter)

		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA1})
		emitter.AssertExpectations(t)

		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA2,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t) // no new events expected to be emitted

		require.Nil(t, cl.unsafePayloads.Peek(), "pop because too old")
	})

	// When we already have the exact payload as tip, then no need to process it
	t.Run("drop equal", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelError)

		emitter := &testutils.MockEmitter{}
		cl := NewCLSync(logger, cfg, metrics)
		cl.AttachEmitter(emitter)

		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA1})
		emitter.AssertExpectations(t)

		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA1,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t) // no new events expected to be emitted

		require.Nil(t, cl.unsafePayloads.Peek(), "pop because seen")
	})

	// When we have a different payload, at the same height, then we want to keep it.
	// The unsafe chain consensus preserves the first-seen payload.
	t.Run("ignore conflict", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelError)

		emitter := &testutils.MockEmitter{}
		cl := NewCLSync(logger, cfg, metrics)
		cl.AttachEmitter(emitter)

		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA1})
		emitter.AssertExpectations(t)

		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    altRefA1,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t) // no new events expected to be emitted

		require.Nil(t, cl.unsafePayloads.Peek(), "pop because alternative")
	})

	t.Run("ignore unsafe reorg", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelError)

		emitter := &testutils.MockEmitter{}
		cl := NewCLSync(logger, cfg, metrics)
		cl.AttachEmitter(emitter)

		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA2})
		emitter.AssertExpectations(t)

		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    altRefA1,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t) // no new events expected, since A2 does not fit onto altA1

		require.Nil(t, cl.unsafePayloads.Peek(), "pop because not applicable")
	})

	t.Run("success", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelError)

		emitter := &testutils.MockEmitter{}
		cl := NewCLSync(logger, cfg, metrics)
		cl.AttachEmitter(emitter)
		emitter.AssertExpectations(t) // nothing to process yet

		require.Nil(t, cl.unsafePayloads.Peek(), "no payloads yet")

		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA1})
		emitter.AssertExpectations(t)

		lowest := cl.LowestQueuedUnsafeBlock()
		require.Equal(t, refA1, lowest, "expecting A1 next")

		// payload A1 should be possible to process on top of A0
		emitter.ExpectOnce(engine.ProcessUnsafePayloadEvent{Envelope: payloadA1})
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA0,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t)

		// now pretend the payload was processed: we can drop A1 now
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA1,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		require.Nil(t, cl.unsafePayloads.Peek(), "pop because applied")

		// repeat for A2
		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA2})
		emitter.AssertExpectations(t)

		lowest = cl.LowestQueuedUnsafeBlock()
		require.Equal(t, refA2, lowest, "expecting A2 next")

		emitter.ExpectOnce(engine.ProcessUnsafePayloadEvent{Envelope: payloadA2})
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA1,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t)

		// now pretend the payload was processed: we can drop A2 now
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA2,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		require.Nil(t, cl.unsafePayloads.Peek(), "pop because applied")
	})

	t.Run("double buffer", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelError)

		emitter := &testutils.MockEmitter{}
		cl := NewCLSync(logger, cfg, metrics)
		cl.AttachEmitter(emitter)

		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA1})
		emitter.AssertExpectations(t)
		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA2})
		emitter.AssertExpectations(t)

		lowest := cl.LowestQueuedUnsafeBlock()
		require.Equal(t, refA1, lowest, "expecting A1 next")

		emitter.ExpectOnce(engine.ProcessUnsafePayloadEvent{Envelope: payloadA1})
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA0,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t)
		require.Equal(t, 2, cl.unsafePayloads.Len(), "still holding on to A1, and queued A2")

		// Now pretend the payload was processed: we can drop A1 now.
		// The CL-sync will try to immediately continue with A2.
		emitter.ExpectOnce(engine.ProcessUnsafePayloadEvent{Envelope: payloadA2})
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA1,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t)

		// now pretend the payload was processed: we can drop A2 now
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA2,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		require.Nil(t, cl.unsafePayloads.Peek(), "done")
	})

	t.Run("temporary error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelError)

		emitter := &testutils.MockEmitter{}
		cl := NewCLSync(logger, cfg, metrics)
		cl.AttachEmitter(emitter)

		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA1})
		emitter.AssertExpectations(t)

		emitter.ExpectOnce(engine.ProcessUnsafePayloadEvent{Envelope: payloadA1})
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA0,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t)

		// On temporary errors we don't need any feedback from the engine.
		// We just hold on to what payloads there are in the queue.
		require.NotNil(t, cl.unsafePayloads.Peek(), "no pop because temporary error")

		// Pretend we are still stuck on the same forkchoice. The CL-sync will retry sending the payload.
		emitter.ExpectOnce(engine.ProcessUnsafePayloadEvent{Envelope: payloadA1})
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA0,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t)
		require.NotNil(t, cl.unsafePayloads.Peek(), "no pop because retry still unconfirmed")

		// Now confirm we got the payload this time
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA1,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		require.Nil(t, cl.unsafePayloads.Peek(), "pop because valid")
	})

	t.Run("invalid payload error", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelError)
		emitter := &testutils.MockEmitter{}
		cl := NewCLSync(logger, cfg, metrics)
		cl.AttachEmitter(emitter)

		// CLSync gets payload and requests engine state, to later determine if payload should be forwarded
		emitter.ExpectOnce(engine.ForkchoiceRequestEvent{})
		cl.OnEvent(ReceivedUnsafePayloadEvent{Envelope: payloadA1})
		emitter.AssertExpectations(t)

		// Engine signals, CLSync sends the payload
		emitter.ExpectOnce(engine.ProcessUnsafePayloadEvent{Envelope: payloadA1})
		cl.OnEvent(engine.ForkchoiceUpdateEvent{
			UnsafeL2Head:    refA0,
			SafeL2Head:      refA0,
			FinalizedL2Head: refA0,
		})
		emitter.AssertExpectations(t)

		// Pretend the payload is bad. It should not be retried after this.
		cl.OnEvent(engine.PayloadInvalidEvent{Envelope: payloadA1, Err: errors.New("test err")})
		emitter.AssertExpectations(t)
		require.Nil(t, cl.unsafePayloads.Peek(), "pop because invalid")
	})
}
