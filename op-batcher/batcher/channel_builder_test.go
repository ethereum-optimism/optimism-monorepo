package batcher

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	dtest "github.com/ethereum-optimism/optimism/op-node/rollup/derive/test"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/stretchr/testify/require"
)

const latestL1BlockOrigin = 10

var defaultTestRollupConfig = &rollup.Config{
	Genesis:   rollup.Genesis{L2: eth.BlockID{Number: 0}},
	L2ChainID: big.NewInt(1234),
}

// newChannelBuilder creates a new channel builder or returns an error if the
// channel out could not be created.
// it acts as a factory for either a span or singular channel out
func newChannelBuilder(cfg ChannelConfig, rollupCfg *rollup.Config, latestL1OriginBlockNum uint64) (*ChannelBuilder, error) {
	co, err := NewChannelOut(cfg, rollupCfg)
	if err != nil {
		return nil, fmt.Errorf("creating channel out: %w", err)
	}

	return NewChannelBuilderWithChannelOut(cfg, rollupCfg, latestL1OriginBlockNum, co), nil
}

// addMiniBlock adds a minimal valid L2 block to the channel builder using the
// ChannelBuilder.AddBlock method.
func addMiniBlock(cb *ChannelBuilder) error {
	a := newMiniL2Block(0)
	_, err := cb.AddBlock(a)
	return err
}

// newMiniL2Block returns a minimal L2 block with a minimal valid L1InfoDeposit
// transaction as first transaction. Both blocks are minimal in the sense that
// most fields are left at defaults or are unset.
//
// If numTx > 0, that many empty DynamicFeeTxs will be added to the txs.
func newMiniL2Block(numTx int) *types.Block {
	return newMiniL2BlockWithNumberParent(numTx, new(big.Int), (common.Hash{}))
}

// newMiniL2Block returns a minimal L2 block with a minimal valid L1InfoDeposit
// transaction as first transaction. Both blocks are minimal in the sense that
// most fields are left at defaults or are unset. Block number and parent hash
// will be set to the given parameters number and parent.
//
// If numTx > 0, that many empty DynamicFeeTxs will be added to the txs.
func newMiniL2BlockWithNumberParent(numTx int, number *big.Int, parent common.Hash) *types.Block {
	return newMiniL2BlockWithNumberParentAndL1Information(numTx, number, parent, 100, 0)
}

// newMiniL2BlockWithNumberParentAndL1Information returns a minimal L2 block with a minimal valid L1InfoDeposit
// It allows you to specify the l1 block number and the block time in addition to the parameters exposed in newMiniL2Block.
func newMiniL2BlockWithNumberParentAndL1Information(numTx int, l2Number *big.Int, parent common.Hash, l1Number int64, blockTime uint64) *types.Block {
	l1Block := types.NewBlock(&types.Header{
		BaseFee:    big.NewInt(10),
		Difficulty: common.Big0,
		Number:     big.NewInt(l1Number),
		Time:       blockTime,
	}, nil, nil, trie.NewStackTrie(nil), types.DefaultBlockConfig)
	l1InfoTx, err := derive.L1InfoDeposit(defaultTestRollupConfig, eth.SystemConfig{}, 0, eth.BlockToInfo(l1Block), blockTime)
	if err != nil {
		panic(err)
	}

	txs := make([]*types.Transaction, 0, 1+numTx)
	txs = append(txs, types.NewTx(l1InfoTx))
	for i := 0; i < numTx; i++ {
		txs = append(txs, types.NewTx(&types.DynamicFeeTx{}))
	}

	return types.NewBlock(&types.Header{
		Number:     l2Number,
		ParentHash: parent,
	}, &types.Body{Transactions: txs}, nil, trie.NewStackTrie(nil), types.DefaultBlockConfig)
}

// addTooManyBlocks adds blocks to the channel until it hits an error,
// which is presumably ErrTooManyRLPBytes.
func addTooManyBlocks(cb *ChannelBuilder, blockCount int) (int, error) {
	rng := rand.New(rand.NewSource(1234))
	t := time.Now()

	for i := 0; i < blockCount; i++ {
		block := dtest.RandomL2BlockWithChainIdAndTime(rng, 1000, defaultTestRollupConfig.L2ChainID, t.Add(time.Duration(i)*time.Second))
		_, err := cb.AddBlock(block)
		if err != nil {
			return i + 1, err
		}
	}

	return blockCount, nil
}

// FuzzDurationTimeoutZeroMaxChannelDuration ensures that when whenever the MaxChannelDuration
// is set to 0, the channel builder cannot have a duration timeout.
func FuzzDurationTimeoutZeroMaxChannelDuration(f *testing.F) {
	for i := range [10]int{} {
		f.Add(uint64(i))
	}
	f.Fuzz(func(t *testing.T, l1BlockNum uint64) {
		channelConfig := defaultTestChannelConfig()
		channelConfig.MaxChannelDuration = 0
		cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
		require.NoError(t, err)
		cb.timeout = 0
		cb.updateDurationTimeout(l1BlockNum)
		require.False(t, cb.TimedOut(l1BlockNum))
	})
}

// FuzzChannelBuilder_DurationZero ensures that when whenever the MaxChannelDuration
// is not set to 0, the channel builder will always have a duration timeout
// as long as the channel builder's timeout is set to 0.
func FuzzChannelBuilder_DurationZero(f *testing.F) {
	for i := range [10]int{} {
		f.Add(uint64(i), uint64(i))
	}
	f.Fuzz(func(t *testing.T, l1BlockNum uint64, maxChannelDuration uint64) {
		if maxChannelDuration == 0 {
			t.Skip("Max channel duration cannot be 0")
		}

		// Create the channel builder
		channelConfig := defaultTestChannelConfig()
		channelConfig.MaxChannelDuration = maxChannelDuration
		cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
		require.NoError(t, err)

		// Whenever the timeout is set to 0, the channel builder should have a duration timeout
		cb.timeout = 0
		cb.updateDurationTimeout(l1BlockNum)
		cb.CheckTimeout(l1BlockNum + maxChannelDuration)
		require.ErrorIs(t, cb.FullErr(), ErrMaxDurationReached)
	})
}

// FuzzDurationTimeoutMaxChannelDuration ensures that when whenever the MaxChannelDuration
// is not set to 0, the channel builder will always have a duration timeout
// as long as the channel builder's timeout is greater than the target block number.
func FuzzDurationTimeoutMaxChannelDuration(f *testing.F) {
	// Set multiple seeds in case fuzzing isn't explicitly used
	for i := range [10]int{} {
		f.Add(uint64(i), uint64(i), uint64(i))
	}
	f.Fuzz(func(t *testing.T, l1BlockNum uint64, maxChannelDuration uint64, timeout uint64) {
		if maxChannelDuration == 0 {
			t.Skip("Max channel duration cannot be 0")
		}

		// Create the channel builder
		channelConfig := defaultTestChannelConfig()
		channelConfig.MaxChannelDuration = maxChannelDuration
		cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
		require.NoError(t, err)

		// Whenever the timeout is greater than the l1BlockNum,
		// the channel builder should have a duration timeout
		cb.timeout = timeout
		cb.updateDurationTimeout(l1BlockNum)
		if timeout > l1BlockNum+maxChannelDuration {
			// Notice: we cannot call this outside of the if statement
			// because it would put the channel builder in an invalid state.
			// That is, where the channel builder has a value set for the timeout
			// with no timeoutReason. This subsequently causes a panic when
			// a nil timeoutReason is used as an error (eg when calling FullErr).
			cb.CheckTimeout(l1BlockNum + maxChannelDuration)
			require.ErrorIs(t, cb.FullErr(), ErrMaxDurationReached)
		} else {
			require.NoError(t, cb.FullErr())
		}
	})
}

// FuzzChannelCloseTimeout ensures that the channel builder has a [ErrChannelTimeoutClose]
// as long as the timeout constraint is met and the builder's timeout is greater than
// the calculated timeout
func FuzzChannelCloseTimeout(f *testing.F) {
	// Set multiple seeds in case fuzzing isn't explicitly used
	for i := range [10]int{} {
		f.Add(uint64(i), uint64(i), uint64(i), uint64(i*5))
	}
	f.Fuzz(func(t *testing.T, l1BlockNum uint64, channelTimeout uint64, subSafetyMargin uint64, timeout uint64) {
		// Create the channel builder
		channelConfig := defaultTestChannelConfig()
		channelConfig.ChannelTimeout = channelTimeout
		channelConfig.SubSafetyMargin = subSafetyMargin
		cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
		require.NoError(t, err)

		// Check the timeout
		cb.timeout = timeout
		cb.FramePublished(l1BlockNum)
		calculatedTimeout := l1BlockNum + channelTimeout - subSafetyMargin
		if timeout > calculatedTimeout && calculatedTimeout != 0 {
			cb.CheckTimeout(calculatedTimeout)
			require.ErrorIs(t, cb.FullErr(), ErrChannelTimeoutClose)
		} else {
			require.NoError(t, cb.FullErr())
		}
	})
}

// FuzzChannelZeroCloseTimeout ensures that the channel builder has a [ErrChannelTimeoutClose]
// as long as the timeout constraint is met and the builder's timeout is set to zero.
func FuzzChannelZeroCloseTimeout(f *testing.F) {
	// Set multiple seeds in case fuzzing isn't explicitly used
	for i := range [10]int{} {
		f.Add(uint64(i), uint64(i), uint64(i))
	}
	f.Fuzz(func(t *testing.T, l1BlockNum uint64, channelTimeout uint64, subSafetyMargin uint64) {
		// Create the channel builder
		channelConfig := defaultTestChannelConfig()
		channelConfig.ChannelTimeout = channelTimeout
		channelConfig.SubSafetyMargin = subSafetyMargin
		cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
		require.NoError(t, err)

		// Check the timeout
		cb.timeout = 0
		cb.FramePublished(l1BlockNum)
		calculatedTimeout := l1BlockNum + channelTimeout - subSafetyMargin
		cb.CheckTimeout(calculatedTimeout)
		if cb.timeout != 0 {
			require.ErrorIs(t, cb.FullErr(), ErrChannelTimeoutClose)
		}
	})
}

// FuzzSeqWindowClose ensures that the channel builder has a [ErrSeqWindowClose]
// as long as the timeout constraint is met and the builder's timeout is greater than
// the calculated timeout
func FuzzSeqWindowClose(f *testing.F) {
	// Set multiple seeds in case fuzzing isn't explicitly used
	for i := range [10]int{} {
		f.Add(uint64(i), uint64(i), uint64(i), uint64(i*5))
	}
	f.Fuzz(func(t *testing.T, epochNum uint64, seqWindowSize uint64, subSafetyMargin uint64, timeout uint64) {
		// Create the channel builder
		channelConfig := defaultTestChannelConfig()
		channelConfig.SeqWindowSize = seqWindowSize
		channelConfig.SubSafetyMargin = subSafetyMargin
		cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
		require.NoError(t, err)

		// Check the timeout
		cb.timeout = timeout
		cb.updateSwTimeout(epochNum)
		calculatedTimeout := epochNum + seqWindowSize - subSafetyMargin
		if timeout > calculatedTimeout && calculatedTimeout != 0 {
			cb.CheckTimeout(calculatedTimeout)
			require.ErrorIs(t, cb.FullErr(), ErrSeqWindowClose)
		} else {
			require.NoError(t, cb.FullErr())
		}
	})
}

// FuzzSeqWindowZeroTimeoutClose ensures that the channel builder has a [ErrSeqWindowClose]
// as long as the timeout constraint is met and the builder's timeout is set to zero.
func FuzzSeqWindowZeroTimeoutClose(f *testing.F) {
	// Set multiple seeds in case fuzzing isn't explicitly used
	for i := range [10]int{} {
		f.Add(uint64(i), uint64(i), uint64(i))
	}
	f.Fuzz(func(t *testing.T, epochNum uint64, seqWindowSize uint64, subSafetyMargin uint64) {
		// Create the channel builder
		channelConfig := defaultTestChannelConfig()
		channelConfig.SeqWindowSize = seqWindowSize
		channelConfig.SubSafetyMargin = subSafetyMargin
		cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
		require.NoError(t, err)

		// Check the timeout
		cb.timeout = 0
		cb.updateSwTimeout(epochNum)
		calculatedTimeout := epochNum + seqWindowSize - subSafetyMargin
		cb.CheckTimeout(calculatedTimeout)
		if cb.timeout != 0 {
			require.ErrorIs(t, cb.FullErr(), ErrSeqWindowClose, "Sequence window close should be reached")
		}
	})
}

func TestChannelBuilderBatchType(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T, batchType uint)
	}{
		{"ChannelBuilder_MaxRLPBytesPerChannel", ChannelBuilder_MaxRLPBytesPerChannel},
		{"ChannelBuilder_MaxRLPBytesPerFjord", ChannelBuilder_MaxRLPBytesPerChannelFjord},
		{"ChannelBuilder_OutputFramesMaxFrameIndex", ChannelBuilder_OutputFramesMaxFrameIndex},
		{"ChannelBuilder_AddBlock", ChannelBuilder_AddBlock},
		{"ChannelBuilder_PendingFrames_TotalFrames", ChannelBuilder_PendingFrames_TotalFrames},
		{"ChannelBuilder_InputBytes", ChannelBuilder_InputBytes},
		{"ChannelBuilder_OutputBytes", ChannelBuilder_OutputBytes},
		{"ChannelBuilder_OutputWrongFramePanic", ChannelBuilder_OutputWrongFramePanic},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name+"_SingularBatch", func(t *testing.T) {
			test.f(t, derive.SingularBatchType)
		})
	}

	for _, test := range tests {
		test := test
		t.Run(test.name+"_SpanBatch", func(t *testing.T) {
			test.f(t, derive.SpanBatchType)
		})
	}
}

// TestChannelBuilder_NextFrame tests calling NextFrame on a ChannelBuilder with only one frame
func TestChannelBuilder_NextFrame(t *testing.T) {
	channelConfig := defaultTestChannelConfig()

	// Create a new channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)

	// Mock the internals of `ChannelBuilder.outputFrame`
	// to construct a single frame
	co := cb.co
	var buf bytes.Buffer
	fn, err := co.OutputFrame(&buf, channelConfig.MaxFrameSize)
	require.NoError(t, err)

	// Push one frame into to the channel builder
	expectedTx := txID{frameID{chID: co.ID(), frameNumber: fn}}
	expectedBytes := buf.Bytes()
	frameData := frameData{
		id: frameID{
			chID:        co.ID(),
			frameNumber: fn,
		},
		data: expectedBytes,
	}
	cb.frames = append(cb.frames, frameData)

	// There should only be 1 frame in the channel builder
	require.Equal(t, 1, cb.PendingFrames())

	// We should be able to increment to the next frame
	constructedFrame := cb.NextFrame()
	require.Equal(t, expectedTx[0], constructedFrame.id)
	require.Equal(t, expectedBytes, constructedFrame.data)
	require.Equal(t, 0, cb.PendingFrames())

	// The next call should panic since the length of frames is 0
	require.PanicsWithValue(t, "no next frame", func() { cb.NextFrame() })
}

// TestChannelBuilder_OutputWrongFramePanic tests that a panic is thrown when we try to rewind the cursor with an invalid frame id
func ChannelBuilder_OutputWrongFramePanic(t *testing.T, batchType uint) {
	channelConfig := defaultTestChannelConfig()
	channelConfig.BatchType = batchType

	// Construct a channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)

	// Mock the internals of `ChannelBuilder.outputFrame`
	// to construct a single frame
	// the type of batch does not matter here because we are using it to construct a broken frame
	c, err := channelConfig.CompressorConfig.NewCompressor()
	require.NoError(t, err)
	co, err := derive.NewSingularChannelOut(c, rollup.NewChainSpec(defaultTestRollupConfig))
	require.NoError(t, err)
	var buf bytes.Buffer
	fn, err := co.OutputFrame(&buf, channelConfig.MaxFrameSize)
	require.NoError(t, err)

	// The frame push should panic since we constructed a new channel out
	// so the channel out id won't match
	require.PanicsWithValue(t, "cannot rewind to unknown frame", func() {
		frame := frameData{
			id: frameID{
				chID:        co.ID(),
				frameNumber: fn,
			},
			data: buf.Bytes(),
		}
		cb.RewindFrameCursor(frame)
	})
}

// TestChannelBuilder_OutputFrames tests [ChannelBuilder.OutputFrames] for singular batches.
func TestChannelBuilder_OutputFrames(t *testing.T) {
	channelConfig := defaultTestChannelConfig()
	channelConfig.MaxFrameSize = derive.FrameV0OverHeadSize + 1
	channelConfig.TargetNumFrames = 1000
	channelConfig.InitNoneCompressor()

	// Construct the channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)
	require.False(t, cb.IsFull())
	require.Equal(t, 0, cb.PendingFrames())

	// Calling OutputFrames without having called [AddBlock]
	// should return no error
	require.NoError(t, cb.OutputFrames())

	// There should be no ready bytes yet
	require.Equal(t, 0, cb.co.ReadyBytes())

	// Let's add a block
	require.NoError(t, addMiniBlock(cb))

	// Check how many ready bytes
	require.Greater(t, uint64(cb.co.ReadyBytes()+derive.FrameV0OverHeadSize), channelConfig.MaxFrameSize)

	require.Equal(t, 0, cb.PendingFrames()) // always 0 because non compressor

	// The channel should not be full
	// but we want to output the frames for testing anyways
	require.False(t, cb.IsFull())

	// We should be able to output the frames
	require.NoError(t, cb.OutputFrames())

	// There should be many frames in the channel builder now
	require.Greater(t, cb.PendingFrames(), 1)
	for _, frame := range cb.frames {
		require.Len(t, frame.data, int(channelConfig.MaxFrameSize))
	}
}

func TestChannelBuilder_OutputFrames_SpanBatch(t *testing.T) {
	for _, algo := range derive.CompressionAlgos {
		t.Run("ChannelBuilder_OutputFrames_SpanBatch_"+algo.String(), func(t *testing.T) {
			ChannelBuilder_OutputFrames_SpanBatch(t, algo) // to fill faster for brotli
		})
	}
}

func ChannelBuilder_OutputFrames_SpanBatch(t *testing.T, algo derive.CompressionAlgo) {
	channelConfig := defaultTestChannelConfig()
	channelConfig.MaxFrameSize = 20 + derive.FrameV0OverHeadSize
	if algo.IsBrotli() {
		channelConfig.TargetNumFrames = 3
	} else {
		channelConfig.TargetNumFrames = 5
	}
	channelConfig.BatchType = derive.SpanBatchType
	channelConfig.InitRatioCompressor(1, algo)

	// Construct the channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)
	require.False(t, cb.IsFull())
	require.Equal(t, 0, cb.PendingFrames())

	// Calling OutputFrames without having called [AddBlock]
	// should return no error
	require.NoError(t, cb.OutputFrames())

	// There should be no ready bytes yet
	require.Equal(t, 0, cb.co.ReadyBytes())

	// fill up
	for {
		err = addMiniBlock(cb)
		if err == nil {
			if cb.IsFull() {
				// this happens when the data exactly fills the channel
				break
			}
			require.False(t, cb.IsFull())
			// There should be no ready bytes until the channel is full
			require.Equal(t, cb.co.ReadyBytes(), 0)
		} else {
			require.ErrorIs(t, err, derive.ErrCompressorFull)
			break
		}
	}

	require.True(t, cb.IsFull())
	// Check how many ready bytes
	require.GreaterOrEqual(t,
		cb.co.ReadyBytes()+derive.FrameV0OverHeadSize,
		int(channelConfig.MaxFrameSize))
	require.Equal(t, 0, cb.PendingFrames())

	// We should be able to output the frames
	require.NoError(t, cb.OutputFrames())

	// There should be several frames in the channel builder now
	require.Greater(t, cb.PendingFrames(), 1)
	for i := 0; i < cb.numFrames-1; i++ {
		require.Len(t, cb.frames[i].data, int(channelConfig.MaxFrameSize))
	}
	require.LessOrEqual(t, len(cb.frames[len(cb.frames)-1].data), int(channelConfig.MaxFrameSize))
}

// ChannelBuilder_MaxRLPBytesPerChannel tests the [ChannelBuilder.OutputFrames]
// function errors when the max RLP bytes per channel is reached.
func ChannelBuilder_MaxRLPBytesPerChannel(t *testing.T, batchType uint) {
	t.Parallel()
	channelConfig := defaultTestChannelConfig()
	chainSpec := rollup.NewChainSpec(defaultTestRollupConfig)
	channelConfig.MaxFrameSize = chainSpec.MaxRLPBytesPerChannel(latestL1BlockOrigin) * 2
	channelConfig.InitNoneCompressor()
	channelConfig.BatchType = batchType

	// Construct the channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)

	// Add a block that overflows the [ChannelOut]
	_, err = addTooManyBlocks(cb, 10_000)
	require.ErrorIs(t, err, derive.ErrTooManyRLPBytes)
}

// ChannelBuilder_MaxRLPBytesPerChannelFjord tests the [ChannelBuilder.OutputFrames]
// function works as intended postFjord.
// strategy:
// check preFjord how many blocks to fill the channel
// then check postFjord w/ double the amount of blocks
func ChannelBuilder_MaxRLPBytesPerChannelFjord(t *testing.T, batchType uint) {
	t.Parallel()
	channelConfig := defaultTestChannelConfig()
	chainSpec := rollup.NewChainSpec(defaultTestRollupConfig)
	channelConfig.MaxFrameSize = chainSpec.MaxRLPBytesPerChannel(latestL1BlockOrigin) * 2
	channelConfig.InitNoneCompressor()
	channelConfig.BatchType = batchType

	// Construct the channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)

	// Count how many a block that overflows the [ChannelOut]
	blockCount, err := addTooManyBlocks(cb, 10_000)
	require.ErrorIs(t, err, derive.ErrTooManyRLPBytes)

	// Create a new channel builder with fjord fork
	now := time.Now()
	fjordTime := uint64(now.Add(-1 * time.Second).Unix())
	rollupConfig := &rollup.Config{
		Genesis:   rollup.Genesis{L2: eth.BlockID{Number: 0}},
		L2ChainID: big.NewInt(1234),
		FjordTime: &fjordTime,
	}

	chainSpec = rollup.NewChainSpec(rollupConfig)
	channelConfig.MaxFrameSize = chainSpec.MaxRLPBytesPerChannel(uint64(now.Unix())) * 2
	channelConfig.InitNoneCompressor()
	channelConfig.BatchType = batchType

	cb, err = newChannelBuilder(channelConfig, rollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)

	// try add double the amount of block, it should not error
	_, err = addTooManyBlocks(cb, 2*blockCount)

	require.NoError(t, err)
}

// ChannelBuilder_OutputFramesMaxFrameIndex tests the [ChannelBuilder.OutputFrames]
// function errors when the max frame index is reached.
func ChannelBuilder_OutputFramesMaxFrameIndex(t *testing.T, batchType uint) {
	channelConfig := defaultTestChannelConfig()
	channelConfig.MaxFrameSize = derive.FrameV0OverHeadSize + 1
	channelConfig.TargetNumFrames = math.MaxUint16 + 1
	channelConfig.InitRatioCompressor(.1, derive.Zlib)
	channelConfig.BatchType = batchType

	rng := rand.New(rand.NewSource(123))

	// Continuously add blocks until the max frame index is reached
	// This should cause the [ChannelBuilder.OutputFrames] function
	// to error
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)
	require.False(t, cb.IsFull())
	require.Equal(t, 0, cb.PendingFrames())
	ti := time.Now()
	for i := 0; ; i++ {
		a := dtest.RandomL2BlockWithChainIdAndTime(rng, 1000, defaultTestRollupConfig.L2ChainID, ti.Add(time.Duration(i)*time.Second))
		_, err = cb.AddBlock(a)
		if cb.IsFull() {
			fullErr := cb.FullErr()
			require.ErrorIs(t, fullErr, derive.ErrCompressorFull)
			break
		}
		require.NoError(t, err)
	}

	_ = cb.OutputFrames()
	require.ErrorIs(t, cb.FullErr(), ErrMaxFrameIndex)
}

// TestChannelBuilder_FullShadowCompressor is a regression test testing that
// the shadow compressor is correctly marked as full if adding another block
// would produce a leftover frame.
//
// This test fails in multiple places if the subtraction of
// [derive.FrameV0OverHeadSize] in [MaxDataSize] is omitted, which has been the
// case before it got fixed it #9887.
func TestChannelBuilder_FullShadowCompressor(t *testing.T) {
	require := require.New(t)
	cfg := ChannelConfig{
		MaxFrameSize:    752,
		TargetNumFrames: 1,
		BatchType:       derive.SpanBatchType,
	}

	cfg.InitShadowCompressor(derive.Zlib)
	cb, err := newChannelBuilder(cfg, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(err)

	rng := rand.New(rand.NewSource(420))
	a := dtest.RandomL2BlockWithChainId(rng, 1, defaultTestRollupConfig.L2ChainID)
	_, err = cb.AddBlock(a)
	require.NoError(err)
	_, err = cb.AddBlock(a)
	require.ErrorIs(err, derive.ErrCompressorFull)
	// without fix, adding the second block would succeed and then adding a
	// third block would fail with full error and the compressor would be full.

	require.NoError(cb.OutputFrames())

	require.True(cb.HasPendingFrame())
	f := cb.NextFrame()
	require.Less(len(f.data), int(cfg.MaxFrameSize)) // would fail without fix, full frame

	require.False(cb.HasPendingFrame(), "no leftover frame expected") // would fail without fix
}

func ChannelBuilder_AddBlock(t *testing.T, batchType uint) {
	channelConfig := defaultTestChannelConfig()
	channelConfig.BatchType = batchType

	// Lower the max frame size so that we can batch
	channelConfig.MaxFrameSize = 20 + derive.FrameV0OverHeadSize
	channelConfig.TargetNumFrames = 2
	// Configure the Input Threshold params so we observe a full channel
	channelConfig.InitRatioCompressor(1, derive.Zlib)

	// Construct the channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)

	// Add a nonsense block to the channel builder
	require.NoError(t, addMiniBlock(cb))
	require.NoError(t, cb.co.Flush())

	// Check the fields reset in the AddBlock function
	expectedInputBytes := 74
	if batchType == derive.SpanBatchType {
		expectedInputBytes = 47
	}
	require.Equal(t, expectedInputBytes, cb.co.InputBytes())
	require.Equal(t, 1, cb.blocks.Len())
	require.Equal(t, 0, cb.frames.Len())
	require.True(t, cb.IsFull())

	// Since the channel output is full, the next call to AddBlock
	// should return the channel out full error
	require.ErrorIs(t, addMiniBlock(cb), derive.ErrCompressorFull)
}

func TestChannelBuilder_CheckTimeout(t *testing.T) {
	channelConfig := defaultTestChannelConfig()

	// Construct the channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)

	// Assert timeout is setup correctly
	require.Equal(t, uint64(1), channelConfig.MaxChannelDuration)
	require.Equal(t, latestL1BlockOrigin+channelConfig.MaxChannelDuration, cb.timeout)

	// Check an L1 block which is after the timeout
	blockNum := uint64(100)
	cb.CheckTimeout(blockNum)
	require.Greater(t, blockNum, cb.timeout)

	// Assert params not modified in CheckTimeout
	require.Equal(t, uint64(1), channelConfig.MaxChannelDuration)
	require.Equal(t, latestL1BlockOrigin+channelConfig.MaxChannelDuration, cb.timeout)
	require.ErrorIs(t, cb.FullErr(), ErrMaxDurationReached)
}

func TestChannelBuilder_CheckTimeoutZeroMaxChannelDuration(t *testing.T) {
	channelConfig := defaultTestChannelConfig()

	// Set the max channel duration to 0
	channelConfig.MaxChannelDuration = 0

	// Construct the channel builder
	cb, err := newChannelBuilder(channelConfig, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)

	// Without a max channel duration, timeout should not be set
	require.Equal(t, uint64(0), channelConfig.MaxChannelDuration)
	require.Equal(t, uint64(0), cb.timeout)

	// Check a new L1 block which should not update the timeout
	cb.CheckTimeout(uint64(100))

	// Since the max channel duration is set to 0,
	// the L1 block register should not update the timeout
	require.Equal(t, uint64(0), channelConfig.MaxChannelDuration)
	require.Equal(t, uint64(0), cb.timeout)
}

func TestChannelBuilder_FramePublished(t *testing.T) {
	cfg := defaultTestChannelConfig()
	cfg.MaxChannelDuration = 10_000
	cfg.ChannelTimeout = 1000
	cfg.SubSafetyMargin = 100

	// Construct the channel builder
	cb, err := newChannelBuilder(cfg, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)
	require.Equal(t, latestL1BlockOrigin+cfg.MaxChannelDuration, cb.timeout)

	priorTimeout := cb.timeout

	// Then the frame published will update the timeout
	l1BlockNum := uint64(100)
	require.Less(t, l1BlockNum, cb.timeout)
	cb.FramePublished(l1BlockNum)

	// Now the timeout will be 1000, blockNum + channelTimeout - subSafetyMargin
	require.Equal(t, uint64(1000), cb.timeout)
	require.Less(t, cb.timeout, priorTimeout)
}

func TestChannelBuilder_LatestL1Origin(t *testing.T) {
	cb, err := newChannelBuilder(defaultTestChannelConfig(), defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)
	require.Equal(t, eth.BlockID{}, cb.LatestL1Origin())

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(1), common.Hash{}, 1, 100))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.LatestL1Origin().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(2), common.Hash{}, 1, 100))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.LatestL1Origin().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(3), common.Hash{}, 2, 110))
	require.NoError(t, err)
	require.Equal(t, uint64(2), cb.LatestL1Origin().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(3), common.Hash{}, 1, 110))
	require.NoError(t, err)
	require.Equal(t, uint64(2), cb.LatestL1Origin().Number)
}

func TestChannelBuilder_OldestL1Origin(t *testing.T) {
	cb, err := newChannelBuilder(defaultTestChannelConfig(), defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)
	require.Equal(t, eth.BlockID{}, cb.OldestL1Origin())

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(1), common.Hash{}, 1, 100))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.OldestL1Origin().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(2), common.Hash{}, 1, 100))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.OldestL1Origin().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(3), common.Hash{}, 2, 110))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.OldestL1Origin().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(3), common.Hash{}, 1, 110))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.OldestL1Origin().Number)
}

func TestChannelBuilder_LatestL2(t *testing.T) {
	cb, err := newChannelBuilder(defaultTestChannelConfig(), defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)
	require.Equal(t, eth.BlockID{}, cb.LatestL2())

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(1), common.Hash{}, 1, 100))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.LatestL2().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(2), common.Hash{}, 1, 100))
	require.NoError(t, err)
	require.Equal(t, uint64(2), cb.LatestL2().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(3), common.Hash{}, 2, 110))
	require.NoError(t, err)
	require.Equal(t, uint64(3), cb.LatestL2().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(3), common.Hash{}, 1, 110))
	require.NoError(t, err)
	require.Equal(t, uint64(3), cb.LatestL2().Number)
}

func TestChannelBuilder_OldestL2(t *testing.T) {
	cb, err := newChannelBuilder(defaultTestChannelConfig(), defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(t, err)
	require.Equal(t, eth.BlockID{}, cb.OldestL2())

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(1), common.Hash{}, 1, 100))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.OldestL2().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(2), common.Hash{}, 1, 100))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.OldestL2().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(3), common.Hash{}, 2, 110))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.OldestL2().Number)

	_, err = cb.AddBlock(newMiniL2BlockWithNumberParentAndL1Information(0, big.NewInt(3), common.Hash{}, 1, 110))
	require.NoError(t, err)
	require.Equal(t, uint64(1), cb.OldestL2().Number)
}

func ChannelBuilder_PendingFrames_TotalFrames(t *testing.T, batchType uint) {
	const tnf = 9
	rng := rand.New(rand.NewSource(94572314))
	require := require.New(t)
	cfg := defaultTestChannelConfig()
	cfg.MaxFrameSize = 1000
	cfg.TargetNumFrames = tnf
	cfg.BatchType = batchType
	cfg.InitShadowCompressor(derive.Zlib)
	cb, err := newChannelBuilder(cfg, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(err)

	// initial builder should be empty
	require.Zero(cb.PendingFrames())
	require.Zero(cb.TotalFrames())

	ti := time.Now()
	// fill up
	for i := 0; ; i++ {
		block := dtest.RandomL2BlockWithChainIdAndTime(rng, 4, defaultTestRollupConfig.L2ChainID, ti.Add(time.Duration(i)*time.Second))
		_, err := cb.AddBlock(block)
		if cb.IsFull() {
			break
		}
		require.NoError(err)
	}
	require.NoError(cb.OutputFrames())

	nf := cb.TotalFrames()
	// require 1 < nf < tnf
	// (because of compression we won't necessarily land exactly at tnf, that's ok)
	require.Greater(nf, 1)
	require.LessOrEqual(nf, tnf)
	require.Equal(nf, cb.PendingFrames())

	// empty queue
	for pf := nf - 1; pf >= 0; pf-- {
		require.True(cb.HasPendingFrame())
		_ = cb.NextFrame()
		require.Equal(cb.PendingFrames(), pf)
		require.Equal(cb.TotalFrames(), nf)
	}
}

func ChannelBuilder_InputBytes(t *testing.T, batchType uint) {
	require := require.New(t)
	rng := rand.New(rand.NewSource(4982432))
	cfg := defaultTestChannelConfig()
	cfg.BatchType = batchType
	var spanBatch *derive.SpanBatch
	if batchType == derive.SpanBatchType {
		chainId := big.NewInt(1234)
		spanBatch = derive.NewSpanBatch(uint64(0), chainId)
	}
	cb, err := newChannelBuilder(cfg, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(err)

	require.Zero(cb.InputBytes())

	var l int
	ti := time.Now()
	for i := 0; i < 5; i++ {
		block := dtest.RandomL2BlockWithChainIdAndTime(rng, rng.Intn(32), defaultTestRollupConfig.L2ChainID, ti.Add(time.Duration(i)*time.Second))
		if batchType == derive.SingularBatchType {
			l += blockBatchRlpSize(t, block)
		} else {
			singularBatch, l1Info, err := derive.BlockToSingularBatch(defaultTestRollupConfig, block)
			require.NoError(err)
			err = spanBatch.AppendSingularBatch(singularBatch, l1Info.SequenceNumber)
			require.NoError(err)
			rawSpanBatch, err := spanBatch.ToRawSpanBatch()
			require.NoError(err)
			batch := derive.NewBatchData(rawSpanBatch)
			var buf bytes.Buffer
			require.NoError(batch.EncodeRLP(&buf))
			l = buf.Len()
		}
		_, err := cb.AddBlock(block)
		require.NoError(err)
		require.Equal(cb.InputBytes(), l)
	}
}

func ChannelBuilder_OutputBytes(t *testing.T, batchType uint) {
	require := require.New(t)
	rng := rand.New(rand.NewSource(9860372))
	cfg := defaultTestChannelConfig()
	cfg.MaxFrameSize = 1000
	cfg.TargetNumFrames = 16
	cfg.BatchType = batchType
	cfg.InitRatioCompressor(1.0, derive.Zlib)
	cb, err := newChannelBuilder(cfg, defaultTestRollupConfig, latestL1BlockOrigin)
	require.NoError(err, "NewChannelBuilder")

	require.Zero(cb.OutputBytes())

	ti := time.Now()
	for i := 0; ; i++ {
		block := dtest.RandomL2BlockWithChainIdAndTime(rng, rng.Intn(32), defaultTestRollupConfig.L2ChainID, ti.Add(time.Duration(i)*time.Second))
		_, err := cb.AddBlock(block)
		if errors.Is(err, derive.ErrCompressorFull) {
			break
		}
		require.NoError(err)
	}

	require.NoError(cb.OutputFrames())
	require.True(cb.IsFull())
	require.Greater(cb.PendingFrames(), 1)

	var flen int
	for cb.HasPendingFrame() {
		f := cb.NextFrame()
		flen += len(f.data)
	}

	require.Equal(cb.OutputBytes(), flen)
}

func blockBatchRlpSize(t *testing.T, b *types.Block) int {
	t.Helper()
	singularBatch, _, err := derive.BlockToSingularBatch(defaultTestRollupConfig, b)
	batch := derive.NewBatchData(singularBatch)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, batch.EncodeRLP(&buf), "RLP-encoding batch")
	return buf.Len()
}
