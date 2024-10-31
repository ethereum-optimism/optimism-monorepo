package attributes

import (
	"context"
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

func TestAttributesHandler(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	refA := testutils.RandomBlockRef(rng)

	refB := eth.L1BlockRef{
		Hash:       testutils.RandomHash(rng),
		Number:     refA.Number + 1,
		ParentHash: refA.Hash,
		Time:       refA.Time + 12,
	}
	// Copy with different hash, as alternative where the alt-L2 block may come from
	refBAlt := refB
	refBAlt.Hash = testutils.RandomHash(rng)

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
	refA0Alt := eth.L2BlockRef{
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
				Overhead:    [32]byte{31: 123},
				Scalar:      [32]byte{0: 0, 31: 42},
				GasLimit:    20_000_000,
			},
		},
		BlockTime:     1,
		SeqWindowSize: 2,
		RegolithTime:  new(uint64),
		CanyonTime:    new(uint64),
		EcotoneTime:   new(uint64),
	}

	a1L1Info, err := derive.L1InfoDepositBytes(cfg, cfg.Genesis.SystemConfig, 1, aL1Info, refA0.Time+cfg.BlockTime)
	require.NoError(t, err)
	parentBeaconBlockRoot := testutils.RandomHash(rng)
	payloadA1 := &eth.ExecutionPayloadEnvelope{ExecutionPayload: &eth.ExecutionPayload{
		ParentHash:    refA0.Hash,
		FeeRecipient:  common.Address{},
		StateRoot:     eth.Bytes32{},
		ReceiptsRoot:  eth.Bytes32{},
		LogsBloom:     eth.Bytes256{},
		PrevRandao:    eth.Bytes32{},
		BlockNumber:   eth.Uint64Quantity(refA0.Number + 1),
		GasLimit:      gasLimit,
		GasUsed:       0,
		Timestamp:     eth.Uint64Quantity(refA0.Time + cfg.BlockTime),
		ExtraData:     nil,
		BaseFeePerGas: eth.Uint256Quantity(*uint256.NewInt(7)),
		BlockHash:     common.Hash{},
		Transactions:  []eth.Data{a1L1Info},
	}, ParentBeaconBlockRoot: &parentBeaconBlockRoot}
	// fix up the block-hash
	payloadA1.ExecutionPayload.BlockHash, _ = payloadA1.CheckBlockHash()

	attrA1 := &derive.AttributesWithParent{
		Attributes: &eth.PayloadAttributes{
			Timestamp:             payloadA1.ExecutionPayload.Timestamp,
			PrevRandao:            payloadA1.ExecutionPayload.PrevRandao,
			SuggestedFeeRecipient: payloadA1.ExecutionPayload.FeeRecipient,
			Withdrawals:           payloadA1.ExecutionPayload.Withdrawals,
			ParentBeaconBlockRoot: payloadA1.ParentBeaconBlockRoot,
			Transactions:          []eth.Data{a1L1Info},
			NoTxPool:              false,
			GasLimit:              &payloadA1.ExecutionPayload.GasLimit,
		},
		Parent:      refA0,
		Concluding:  true,
		DerivedFrom: refB,
	}
	refA1, err := derive.PayloadToBlockRef(cfg, payloadA1.ExecutionPayload)
	require.NoError(t, err)

	payloadA1Alt := &eth.ExecutionPayloadEnvelope{ExecutionPayload: &eth.ExecutionPayload{
		ParentHash:    refA0.Hash,
		FeeRecipient:  common.Address{0xde, 0xea}, // change of the alternative payload
		StateRoot:     eth.Bytes32{},
		ReceiptsRoot:  eth.Bytes32{},
		LogsBloom:     eth.Bytes256{},
		PrevRandao:    eth.Bytes32{},
		BlockNumber:   eth.Uint64Quantity(refA0.Number + 1),
		GasLimit:      gasLimit,
		GasUsed:       0,
		Timestamp:     eth.Uint64Quantity(refA0.Time + cfg.BlockTime),
		ExtraData:     nil,
		BaseFeePerGas: eth.Uint256Quantity(*uint256.NewInt(7)),
		BlockHash:     common.Hash{},
		Transactions:  []eth.Data{a1L1Info},
	}, ParentBeaconBlockRoot: &parentBeaconBlockRoot}
	// fix up the block-hash
	payloadA1Alt.ExecutionPayload.BlockHash, _ = payloadA1Alt.CheckBlockHash()

	attrA1Alt := &derive.AttributesWithParent{
		Attributes: &eth.PayloadAttributes{
			Timestamp:             payloadA1Alt.ExecutionPayload.Timestamp,
			PrevRandao:            payloadA1Alt.ExecutionPayload.PrevRandao,
			SuggestedFeeRecipient: payloadA1Alt.ExecutionPayload.FeeRecipient,
			Withdrawals:           payloadA1Alt.ExecutionPayload.Withdrawals,
			ParentBeaconBlockRoot: payloadA1Alt.ParentBeaconBlockRoot,
			Transactions:          []eth.Data{a1L1Info},
			NoTxPool:              false,
			GasLimit:              &payloadA1Alt.ExecutionPayload.GasLimit,
		},
		Parent:      refA0,
		Concluding:  true,
		DerivedFrom: refBAlt,
	}

	refA1Alt, err := derive.PayloadToBlockRef(cfg, payloadA1Alt.ExecutionPayload)
	require.NoError(t, err)

	t.Run("drop invalid attributes", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelInfo)
		l2 := &testutils.MockL2Client{}
		emitter := &testutils.MockEmitter{}
		ah := NewAttributesHandler(logger, cfg, context.Background(), l2)
		ah.AttachEmitter(emitter)

		emitter.ExpectOnce(derive.ConfirmReceivedAttributesEvent{})
		emitter.ExpectOnce(engine.PendingSafeRequestEvent{})
		ah.OnEvent(derive.DerivedAttributesEvent{
			Attributes: attrA1,
		})
		emitter.AssertExpectations(t)
		require.NotNil(t, ah.attributes, "queue the invalid attributes")

		emitter.ExpectOnce(engine.PendingSafeRequestEvent{})
		ah.OnEvent(engine.InvalidPayloadAttributesEvent{
			Attributes: attrA1,
		})
		emitter.AssertExpectations(t)
		require.Nil(t, ah.attributes, "drop the invalid attributes")
	})
	t.Run("drop stale attributes", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelInfo)
		l2 := &testutils.MockL2Client{}
		emitter := &testutils.MockEmitter{}
		ah := NewAttributesHandler(logger, cfg, context.Background(), l2)
		ah.AttachEmitter(emitter)

		emitter.ExpectOnce(derive.ConfirmReceivedAttributesEvent{})
		emitter.ExpectOnce(engine.PendingSafeRequestEvent{})
		ah.OnEvent(derive.DerivedAttributesEvent{
			Attributes: attrA1,
		})
		emitter.AssertExpectations(t)
		require.NotNil(t, ah.attributes)
		// New attributes will have to get generated after processing the last ones
		emitter.ExpectOnce(derive.PipelineStepEvent{PendingSafe: refA1Alt})
		ah.OnEvent(engine.PendingSafeUpdateEvent{
			PendingSafe: refA1Alt,
			Unsafe:      refA1Alt,
		})
		l2.AssertExpectations(t)
		emitter.AssertExpectations(t)
		require.Nil(t, ah.attributes, "drop stale attributes")
	})

	t.Run("pending gets reorged", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelInfo)
		l2 := &testutils.MockL2Client{}
		emitter := &testutils.MockEmitter{}
		ah := NewAttributesHandler(logger, cfg, context.Background(), l2)
		ah.AttachEmitter(emitter)

		emitter.ExpectOnce(derive.ConfirmReceivedAttributesEvent{})
		emitter.ExpectOnce(engine.PendingSafeRequestEvent{})
		ah.OnEvent(derive.DerivedAttributesEvent{
			Attributes: attrA1,
		})
		emitter.AssertExpectations(t)
		require.NotNil(t, ah.attributes)

		emitter.ExpectOnceType("ResetEvent")
		ah.OnEvent(engine.PendingSafeUpdateEvent{
			PendingSafe: refA0Alt,
			Unsafe:      refA0Alt,
		})
		l2.AssertExpectations(t)
		emitter.AssertExpectations(t)
		require.NotNil(t, ah.attributes, "detected reorg does not clear state, reset is required")
	})

	t.Run("pending older than unsafe", func(t *testing.T) {
		t.Run("consolidation fails", func(t *testing.T) {
			logger := testlog.Logger(t, log.LevelInfo)
			l2 := &testutils.MockL2Client{}
			emitter := &testutils.MockEmitter{}
			ah := NewAttributesHandler(logger, cfg, context.Background(), l2)
			ah.AttachEmitter(emitter)

			// attrA1Alt does not match block A1, so will cause force-reorg.
			emitter.ExpectOnce(derive.ConfirmReceivedAttributesEvent{})
			emitter.ExpectOnce(engine.PendingSafeRequestEvent{})
			ah.OnEvent(derive.DerivedAttributesEvent{Attributes: attrA1Alt})
			emitter.AssertExpectations(t)
			require.NotNil(t, ah.attributes, "queued up derived attributes")

			// Call during consolidation.
			// The payloadA1 is going to get reorged out in favor of attrA1Alt (turns into payloadA1Alt)
			l2.ExpectPayloadByNumber(refA1.Number, payloadA1, nil)
			// fail consolidation, perform force reorg
			emitter.ExpectOnce(engine.BuildStartEvent{Attributes: attrA1Alt})
			ah.OnEvent(engine.PendingSafeUpdateEvent{
				PendingSafe: refA0,
				Unsafe:      refA1,
			})
			l2.AssertExpectations(t)
			emitter.AssertExpectations(t)
			require.NotNil(t, ah.attributes, "still have attributes, processing still unconfirmed")

			emitter.ExpectOnce(derive.PipelineStepEvent{PendingSafe: refA1Alt})
			// recognize reorg as complete
			ah.OnEvent(engine.PendingSafeUpdateEvent{
				PendingSafe: refA1Alt,
				Unsafe:      refA1Alt,
			})
			emitter.AssertExpectations(t)
			require.Nil(t, ah.attributes, "drop when attributes are successful")
		})
		t.Run("consolidation passes", func(t *testing.T) {
			fn := func(t *testing.T, concluding bool) {
				logger := testlog.Logger(t, log.LevelInfo)
				l2 := &testutils.MockL2Client{}
				emitter := &testutils.MockEmitter{}
				ah := NewAttributesHandler(logger, cfg, context.Background(), l2)
				ah.AttachEmitter(emitter)

				attr := &derive.AttributesWithParent{
					Attributes:  attrA1.Attributes, // attributes will match, passing consolidation
					Parent:      attrA1.Parent,
					Concluding:  concluding,
					DerivedFrom: refB,
				}
				emitter.ExpectOnce(derive.ConfirmReceivedAttributesEvent{})
				emitter.ExpectOnce(engine.PendingSafeRequestEvent{})
				ah.OnEvent(derive.DerivedAttributesEvent{Attributes: attr})
				emitter.AssertExpectations(t)
				require.NotNil(t, ah.attributes, "queued up derived attributes")

				// Call during consolidation.
				l2.ExpectPayloadByNumber(refA1.Number, payloadA1, nil)

				emitter.ExpectOnce(engine.PromotePendingSafeEvent{
					Ref:         refA1,
					Concluding:  concluding,
					DerivedFrom: refB,
				})
				ah.OnEvent(engine.PendingSafeUpdateEvent{
					PendingSafe: refA0,
					Unsafe:      refA1,
				})
				l2.AssertExpectations(t)
				emitter.AssertExpectations(t)
				require.NotNil(t, ah.attributes, "still have attributes, processing still unconfirmed")

				emitter.ExpectOnce(derive.PipelineStepEvent{PendingSafe: refA1})
				ah.OnEvent(engine.PendingSafeUpdateEvent{
					PendingSafe: refA1,
					Unsafe:      refA1,
				})
				emitter.AssertExpectations(t)
				require.Nil(t, ah.attributes, "drop when attributes are successful")
			}
			t.Run("is last span", func(t *testing.T) {
				fn(t, true)
			})

			t.Run("is not last span", func(t *testing.T) {
				fn(t, false)
			})
		})
	})

	t.Run("pending equals unsafe", func(t *testing.T) {
		// no consolidation to do, just force next attributes on tip of chain
		logger := testlog.Logger(t, log.LevelInfo)
		l2 := &testutils.MockL2Client{}
		emitter := &testutils.MockEmitter{}
		ah := NewAttributesHandler(logger, cfg, context.Background(), l2)
		ah.AttachEmitter(emitter)

		emitter.ExpectOnce(derive.ConfirmReceivedAttributesEvent{})
		emitter.ExpectOnce(engine.PendingSafeRequestEvent{})
		ah.OnEvent(derive.DerivedAttributesEvent{Attributes: attrA1Alt})
		emitter.AssertExpectations(t)
		require.NotNil(t, ah.attributes, "queued up derived attributes")

		// sanity check test setup
		require.True(t, attrA1Alt.Concluding, "must be concluding attributes")

		// attrA1Alt will fit right on top of A0
		emitter.ExpectOnce(engine.BuildStartEvent{Attributes: attrA1Alt})
		ah.OnEvent(engine.PendingSafeUpdateEvent{
			PendingSafe: refA0,
			Unsafe:      refA0,
		})
		l2.AssertExpectations(t)
		emitter.AssertExpectations(t)
		require.NotNil(t, ah.attributes)

		emitter.ExpectOnce(derive.PipelineStepEvent{PendingSafe: refA1Alt})
		ah.OnEvent(engine.PendingSafeUpdateEvent{
			PendingSafe: refA1Alt,
			Unsafe:      refA1Alt,
		})
		emitter.AssertExpectations(t)
		require.Nil(t, ah.attributes, "clear attributes after successful processing")
	})

	t.Run("pending ahead of unsafe", func(t *testing.T) {
		// Legacy test case: if attributes fit on top of the pending safe block as expected,
		// but if the unsafe block is older, then we can recover by resetting.
		logger := testlog.Logger(t, log.LevelInfo)
		l2 := &testutils.MockL2Client{}
		emitter := &testutils.MockEmitter{}
		ah := NewAttributesHandler(logger, cfg, context.Background(), l2)
		ah.AttachEmitter(emitter)

		emitter.ExpectOnceType("ResetEvent")
		ah.OnEvent(engine.PendingSafeUpdateEvent{
			PendingSafe: refA1,
			Unsafe:      refA0,
		})
		emitter.AssertExpectations(t)
		l2.AssertExpectations(t)
	})

	t.Run("no attributes", func(t *testing.T) {
		logger := testlog.Logger(t, log.LevelInfo)
		l2 := &testutils.MockL2Client{}
		emitter := &testutils.MockEmitter{}
		ah := NewAttributesHandler(logger, cfg, context.Background(), l2)
		ah.AttachEmitter(emitter)

		// If there are no attributes, we expect the pipeline to be requested to generate attributes.
		emitter.ExpectOnce(derive.PipelineStepEvent{PendingSafe: refA1})
		ah.OnEvent(engine.PendingSafeUpdateEvent{
			PendingSafe: refA1,
			Unsafe:      refA1,
		})
		// no calls to L2 or emitter when there is nothing to process
		l2.AssertExpectations(t)
		emitter.AssertExpectations(t)
	})
}
