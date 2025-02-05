package derive

import (
	"bytes"
	"io"
	"math/big"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var rollupCfg = rollup.Config{
	Genesis: rollup.Genesis{
		L2Time: uint64(1723618465),
	},
	BlockTime: 2,
	L2ChainID: big.NewInt(420),
	L1ChainID: big.NewInt(161),
}

// basic implementation of the Compressor interface that does no compression
type nonCompressor struct {
	bytes.Buffer
}

func (s *nonCompressor) Flush() error {
	return nil
}

func (s *nonCompressor) Close() error {
	return nil
}

func (s *nonCompressor) FullErr() error {
	return nil
}

type channelOut interface {
	ChannelOut
	addSingularBatch(batch *SingularBatch, seqNum uint64) error
}

// channelTypes allows tests to run against different channel types
var channelTypes = []struct {
	ChannelOut func(t *testing.T, rcfg *rollup.Config) channelOut
	Name       string
}{
	{
		Name: "Singular",
		ChannelOut: func(t *testing.T, rcfg *rollup.Config) channelOut {
			cout, err := NewSingularChannelOut(&nonCompressor{}, rollup.NewChainSpec(rcfg))
			require.NoError(t, err)
			return cout
		},
	},
	{
		Name: "Span",
		ChannelOut: func(t *testing.T, rcfg *rollup.Config) channelOut {
			cout, err := NewSpanChannelOut(128_000, Zlib, rollup.NewChainSpec(rcfg))
			require.NoError(t, err)
			return cout
		},
	},
}

func TestChannelOutAddBlock(t *testing.T) {
	for _, tcase := range channelTypes {
		t.Run(tcase.Name, func(t *testing.T) {
			cout := tcase.ChannelOut(t, &rollupCfg)
			header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(100)}
			block := types.NewBlockWithHeader(header).WithBody(
				types.Body{
					Transactions: []*types.Transaction{
						types.NewTx(&types.DynamicFeeTx{}),
					},
				},
			)
			_, err := cout.AddBlock(&rollupCfg, block)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrNotDepositTx)
		})
	}
}

// TestOutputFrameSmallMaxSize tests that calling [OutputFrame] with a small
// max size that is below the fixed frame size overhead of FrameV0OverHeadSize (23),
// will return an error.
func TestOutputFrameSmallMaxSize(t *testing.T) {
	for _, tcase := range channelTypes {
		t.Run(tcase.Name, func(t *testing.T) {
			cout := tcase.ChannelOut(t, &rollupCfg)
			// Call OutputFrame with the range of small max size values that err
			var w bytes.Buffer
			for i := 0; i < FrameV0OverHeadSize; i++ {
				fid, err := cout.OutputFrame(&w, uint64(i))
				require.ErrorIs(t, err, ErrMaxFrameSizeTooSmall)
				require.Zero(t, fid)
			}
		})
	}
}

func TestOutputFrameNoEmptyLastFrame(t *testing.T) {
	for _, tcase := range channelTypes {
		t.Run(tcase.Name, func(t *testing.T) {
			cout := tcase.ChannelOut(t, &rollupCfg)

			rng := rand.New(rand.NewSource(0x543331))
			txCount := 1
			singularBatch := RandomSingularBatch(rng, txCount, rollupCfg.L2ChainID)

			err := cout.addSingularBatch(singularBatch, 0)
			var written uint64
			require.NoError(t, err)

			require.NoError(t, cout.Close())

			// depending on the channel type, determine the size of the written data
			if span, ok := cout.(*SpanChannelOut); ok {
				written = uint64(span.compressor.Len())
			} else if singular, ok := cout.(*SingularChannelOut); ok {
				written = uint64(singular.compress.Len())
			}

			var buf bytes.Buffer
			// Output a frame which needs exactly `written` bytes. This frame is expected to be the last frame.
			_, err = cout.OutputFrame(&buf, written+FrameV0OverHeadSize)
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

// TestRLPByteLimit ensures that stream encoder is properly limiting the length.
// It will decode the input if `len(input) <= inputLimit`.
func TestRLPByteLimit(t *testing.T) {
	// Should succeed if `len(input) == inputLimit`
	enc := []byte("\x8bhello world") // RLP encoding of the string "hello world"
	in := bytes.NewBuffer(enc)
	var out string
	stream := rlp.NewStream(in, 12)
	err := stream.Decode(&out)
	require.Nil(t, err)
	require.Equal(t, out, "hello world")

	// Should fail if the `inputLimit = len(input) - 1`
	enc = []byte("\x8bhello world") // RLP encoding of the string "hello world"
	in = bytes.NewBuffer(enc)
	var out2 string
	stream = rlp.NewStream(in, 11)
	err = stream.Decode(&out2)
	require.Equal(t, err, rlp.ErrValueTooLarge)
	require.Equal(t, out2, "")
}

func TestForceCloseTxData(t *testing.T) {
	id := [16]byte{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0xbe, 0xef}
	tests := []struct {
		frames []Frame
		errors bool
		output string
	}{
		{
			frames: []Frame{},
			errors: true,
			output: "",
		},
		{
			frames: []Frame{{FrameNumber: 0, IsLast: false}, {ID: id, FrameNumber: 1, IsLast: true}},
			errors: true,
			output: "",
		},
		{
			frames: []Frame{{ID: id, FrameNumber: 0, IsLast: false}},
			errors: false,
			output: "00deadbeefdeadbeefdeadbeefdeadbeef00000000000001",
		},
		{
			frames: []Frame{{ID: id, FrameNumber: 0, IsLast: true}},
			errors: false,
			output: "00",
		},
		{
			frames: []Frame{{ID: id, FrameNumber: 1, IsLast: false}},
			errors: false,
			output: "00deadbeefdeadbeefdeadbeefdeadbeef00000000000001",
		},
		{
			frames: []Frame{{ID: id, FrameNumber: 1, IsLast: true}},
			errors: false,
			output: "00deadbeefdeadbeefdeadbeefdeadbeef00000000000000",
		},
		{
			frames: []Frame{{ID: id, FrameNumber: 2, IsLast: true}},
			errors: false,
			output: "00deadbeefdeadbeefdeadbeefdeadbeef00000000000000deadbeefdeadbeefdeadbeefdeadbeef00010000000000",
		},
		{
			frames: []Frame{{ID: id, FrameNumber: 1, IsLast: false}, {ID: id, FrameNumber: 3, IsLast: true}},
			errors: false,
			output: "00deadbeefdeadbeefdeadbeefdeadbeef00000000000000deadbeefdeadbeefdeadbeefdeadbeef00020000000000",
		},
		{
			frames: []Frame{{ID: id, FrameNumber: 1, IsLast: false}, {ID: id, FrameNumber: 3, IsLast: true}, {ID: id, FrameNumber: 5, IsLast: true}},
			errors: false,
			output: "00deadbeefdeadbeefdeadbeefdeadbeef00000000000000deadbeefdeadbeefdeadbeefdeadbeef00020000000000",
		},
	}

	for i, test := range tests {
		out, err := ForceCloseTxData(test.frames)
		if test.errors {
			require.Error(t, err, "Should error on tc %v", i)
			require.Nil(t, out, "Should return no value in tc %v", i)
		} else {
			require.NoError(t, err, "Should not error on tc %v", i)
			require.Equal(t, common.FromHex(test.output), out, "Should match output tc %v", i)
		}
	}
}

func TestBlockToBatchValidity(t *testing.T) {
	block := new(types.Block)
	_, _, err := BlockToSingularBatch(&rollupCfg, block)
	require.ErrorContains(t, err, "has no transactions")
}

func SpanChannelAndBatches(t *testing.T, targetOutputSize uint64, numBatches int, algo CompressionAlgo, opts ...SpanChannelOutOption) (*SpanChannelOut, []*SingularBatch) {
	// target is larger than one batch, but smaller than two batches
	rng := rand.New(rand.NewSource(0x543331))
	chainID := rollupCfg.L2ChainID
	txCount := 1
	genesisTime := rollupCfg.Genesis.L2Time
	cout, err := NewSpanChannelOut(targetOutputSize, algo, rollup.NewChainSpec(&rollupCfg), opts...)
	require.NoError(t, err)
	batches := make([]*SingularBatch, 0, numBatches)
	// adding the first batch should not cause an error
	for i := 0; i < numBatches; i++ {
		singularBatch := RandomSingularBatch(rng, txCount, chainID)
		// use default 2 sec block time
		singularBatch.Timestamp = genesisTime + 420_000 + rollupCfg.BlockTime*uint64(i)
		batches = append(batches, singularBatch)
	}

	return cout, batches
}

func TestSpanChannelOut(t *testing.T) {
	tests := []func(t *testing.T, algo CompressionAlgo){
		SpanChannelOutCompressionOnlyOneBatch,
		SpanChannelOutCompressionUndo,
		SpanChannelOutClose,
	}
	for _, test := range tests {
		test := test
		for _, algo := range CompressionAlgos {
			t.Run(funcName(test)+"_"+algo.String(), func(t *testing.T) {
				test(t, algo)
			})
		}
	}
}

func funcName(fn any) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

// TestSpanChannelOutCompressionOnlyOneBatch tests that the SpanChannelOut compression works as expected when there is only one batch
// and it is larger than the target size. The single batch should be compressed, and the channel should now be full
func SpanChannelOutCompressionOnlyOneBatch(t *testing.T, algo CompressionAlgo) {
	cout, singularBatches := SpanChannelAndBatches(t, 300, 2, algo)

	err := cout.addSingularBatch(singularBatches[0], 0)
	// confirm compression was not skipped
	require.Greater(t, cout.compressor.Len(), 0)
	require.NoError(t, err)

	// confirm the channel is full
	require.ErrorIs(t, cout.FullErr(), ErrCompressorFull)

	// confirm adding another batch would cause the same full error
	err = cout.addSingularBatch(singularBatches[1], 0)
	require.ErrorIs(t, err, ErrCompressorFull)
}

// TestSpanChannelOutCompressionUndo tests that the SpanChannelOut compression rejects a batch that would cause the channel to be overfull
func SpanChannelOutCompressionUndo(t *testing.T, algo CompressionAlgo) {
	// target is larger than one batch, but smaller than two batches
	cout, singularBatches := SpanChannelAndBatches(t, 1100, 2, algo)

	err := cout.addSingularBatch(singularBatches[0], 0)
	require.NoError(t, err)
	// confirm that the first compression was skipped
	if algo == Zlib {
		require.Equal(t, 0, cout.compressor.Len())
	} else {
		require.Equal(t, 1, cout.compressor.Len()) // 1 because of brotli channel version
	}
	// record the RLP length to confirm it doesn't change when adding a rejected batch
	rlp1 := cout.activeRLP().Len()

	err = cout.addSingularBatch(singularBatches[1], 0)
	require.ErrorIs(t, err, ErrCompressorFull)
	// confirm that the second compression was not skipped
	require.Greater(t, cout.compressor.Len(), 0)

	// confirm that the second rlp is tht same size as the first (because the second batch was not added)
	require.Equal(t, rlp1, cout.activeRLP().Len())
}

// TestSpanChannelOutClose tests that the SpanChannelOut compression works as expected when the channel is closed.
// it should compress the batch even if it is smaller than the target size because the channel is closing
func SpanChannelOutClose(t *testing.T, algo CompressionAlgo) {
	target := uint64(1100)
	cout, singularBatches := SpanChannelAndBatches(t, target, 1, algo)

	err := cout.addSingularBatch(singularBatches[0], 0)
	require.NoError(t, err)
	// confirm no compression has happened yet

	if algo == Zlib {
		require.Equal(t, 0, cout.compressor.Len())
	} else {
		require.Equal(t, 1, cout.compressor.Len()) // 1 because of brotli channel version
	}

	// confirm the RLP length is less than the target
	rlpLen := cout.activeRLP().Len()
	require.Less(t, uint64(rlpLen), target)

	// close the channel
	require.NoError(t, cout.Close())

	// confirm that the only batch was compressed, and that the RLP did not change
	require.Greater(t, cout.compressor.Len(), 0)
	require.Equal(t, rlpLen, cout.activeRLP().Len())
}

type maxBlocksTest struct {
	outputSize        uint64
	exactFull         bool // whether the outputSize is exactly hit by the last batch
	numBatches        int  // the last batch should cause the compressor to be full
	maxBlocks         int
	expNumSpanBatches int
	expLastNumBlocks  int
}

// This tests sets a max blocks per span batch and causes multiple span batches
// within a single channel. It then does a full round trip, encoding and decoding
// the channel, confirming that the expected batches were encoded.
func TestSpanChannelOut_MaxBlocksPerSpanBatch(t *testing.T) {
	for i, tt := range []maxBlocksTest{
		{
			outputSize:        10_751,
			exactFull:         true,
			numBatches:        15,
			maxBlocks:         4,
			expNumSpanBatches: 4,
			expLastNumBlocks:  3,
		},
		{
			outputSize:        11_000,
			numBatches:        16,
			maxBlocks:         4,
			expNumSpanBatches: 4,
			expLastNumBlocks:  3,
		},
		{
			outputSize:        11_154,
			exactFull:         true,
			numBatches:        16,
			maxBlocks:         4,
			expNumSpanBatches: 4,
			expLastNumBlocks:  4,
		},
		{
			outputSize:        11_500,
			numBatches:        17,
			maxBlocks:         4,
			expNumSpanBatches: 4,
			expLastNumBlocks:  4,
		},
		{
			outputSize:        11_801,
			exactFull:         true,
			numBatches:        17,
			maxBlocks:         4,
			expNumSpanBatches: 5,
			expLastNumBlocks:  1,
		},
		{
			outputSize:        12_000,
			numBatches:        18,
			maxBlocks:         4,
			expNumSpanBatches: 5,
			expLastNumBlocks:  1,
		},
	} {
		t.Run("test-"+strconv.Itoa(i), func(t *testing.T) {
			testSpanChannelOut_MaxBlocksPerSpanBatch(t, tt)
		})
	}
}

func testSpanChannelOut_MaxBlocksPerSpanBatch(t *testing.T, tt maxBlocksTest) {
	l1Origin := eth.L1BlockRef{Number: rollupCfg.Genesis.L1.Number + 42_000, Hash: common.Hash{0xde, 0xad, 0x42}}
	l2SafeHead := eth.L2BlockRef{Number: rollupCfg.Genesis.L2Time + 40_000}
	cout, bs := SpanChannelAndBatches(t, tt.outputSize, tt.numBatches, Brotli, WithMaxBlocksPerSpanBatch(tt.maxBlocks))
	for i, b := range bs {
		b.EpochNum = rollup.Epoch(l1Origin.Number)
		b.EpochHash = l1Origin.Hash
		err := cout.addSingularBatch(b, uint64(i))
		if i != tt.numBatches-1 || tt.exactFull {
			require.NoErrorf(t, err, "iteration %d", i)
		} else {
			// adding last batch should not succeed, if not making compressor exactly full
			require.ErrorIs(t, err, ErrCompressorFull)
			t.Logf("full compressor length: %d", cout.compressor.Len())
		}

	}
	require.ErrorIs(t, cout.FullErr(), ErrCompressorFull)
	expSpanBatchBlocks := tt.expLastNumBlocks
	if !tt.exactFull {
		// if we didn't fill up exactly, we expect that one more block got
		// added to the current span batch to detect that the compressor is full
		expSpanBatchBlocks = tt.expLastNumBlocks%tt.maxBlocks + 1
	}
	require.Equal(t, expSpanBatchBlocks, cout.spanBatch.GetBlockCount(),
		"last block should still have been added to the span batch")
	require.NoError(t, cout.Close())

	// write cannel into a single frame
	var frameBuf bytes.Buffer
	fn, err := cout.OutputFrame(&frameBuf, tt.outputSize+FrameV0OverHeadSize)
	require.Zero(t, fn)
	require.ErrorIs(t, err, io.EOF)

	// now roundtrip to decode the batches
	var frame Frame
	require.NoError(t, frame.UnmarshalBinary(&frameBuf))
	require.True(t, frame.IsLast)
	spec := rollup.NewChainSpec(&rollupCfg)
	ch := NewChannel(frame.ID, l1Origin, false)
	require.False(t, ch.IsReady())
	require.NoError(t, ch.AddFrame(frame, l1Origin))
	require.True(t, ch.IsReady())
	br, err := BatchReader(ch.Reader(), spec.MaxRLPBytesPerChannel(0), true)
	require.NoError(t, err)

	sbs := make([]*SingularBatch, 0, tt.numBatches-1)
	for i := 0; i < tt.expNumSpanBatches; i++ {
		t.Logf("iteration %d", i)
		expBlocks := tt.maxBlocks
		if i == tt.expNumSpanBatches-1 {
			// last span batch possibly contains less
			expBlocks = tt.expLastNumBlocks
		}

		bd, err := br()
		require.NoError(t, err)
		require.EqualValues(t, SpanBatchType, bd.GetBatchType())
		sb, err := DeriveSpanBatch(bd, rollupCfg.BlockTime, rollupCfg.Genesis.L2Time, cout.spanBatch.ChainID)
		require.NoError(t, err)
		require.Equal(t, expBlocks, sb.GetBlockCount())
		sbs0, err := sb.GetSingularBatches([]eth.L1BlockRef{l1Origin}, l2SafeHead)
		require.NoError(t, err)
		// last span batch contains one less
		require.Len(t, sbs0, expBlocks)
		sbs = append(sbs, sbs0...)
	}

	// batch reader should be exhausted
	_, err = br()
	require.ErrorIs(t, err, io.EOF)

	for i, batch := range sbs {
		batch0 := bs[i]
		// clear the expected parent hash, as GetSingularBatches doesn't set these yet
		// we still compare timestamps and txs, which is enough
		batch0.ParentHash = (common.Hash{})
		require.Equalf(t, batch0, batch, "iteration %d", i)
	}
}
