package eth

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestBlobEncodeDecode(t *testing.T) {
	cases := []string{
		"this is a test of blob encoding/decoding",
		"short",
		"\x00",
		"\x00\x01\x00",
		"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00",
		"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00",
		"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00",
		"",
	}

	var b Blob
	for _, c := range cases {
		data := Data(c)
		err := b.FromData(data)
		require.NoError(t, err)
		decoded, err := b.ToData()
		require.NoError(t, err)
		require.Equal(t, c, string(decoded))
	}
}

func TestSmallBlobEncoding(t *testing.T) {
	// the first field element is filled and no data remains
	data := Data(make([]byte, 128))
	data[127] = 0xFF

	var b Blob
	err := b.FromData(data)
	require.NoError(t, err)

	decoded, err := b.ToData()
	require.NoError(t, err)
	require.Equal(t, data, decoded)

	// only 10 bytes of data
	data = Data(make([]byte, 10))
	data[9] = 0xFF
	err = b.FromData(data)
	require.NoError(t, err)
	decoded, err = b.ToData()
	require.NoError(t, err)
	require.Equal(t, data, decoded)

	// no 3 bytes of extra data left to encode after the first 4 field elements
	data = Data(make([]byte, 27+31*3))
	data[27+31*3-1] = 0xFF
	err = b.FromData(data)
	require.NoError(t, err)
	decoded, err = b.ToData()
	require.NoError(t, err)
	require.Equal(t, data, decoded)
}

func TestBigBlobEncoding(t *testing.T) {
	r := rand.New(rand.NewSource(99))
	bigData := Data(make([]byte, MaxBlobDataSize))
	for i := range bigData {
		bigData[i] = byte(r.Intn(256))
	}
	var b Blob
	// test the maximum size of data that can be encoded
	err := b.FromData(bigData)
	require.NoError(t, err)
	decoded, err := b.ToData()
	require.NoError(t, err)
	require.Equal(t, bigData, decoded)

	// perform encode/decode test on progressively smaller inputs to exercise boundary conditions
	// pertaining to length of the input data
	for i := 1; i < 256; i++ {
		tempBigData := bigData[i:]
		err := b.FromData(tempBigData)
		require.NoError(t, err)
		decoded, err := b.ToData()
		require.NoError(t, err)
		require.Equal(t, len(tempBigData), len(decoded))
		require.Equal(t, tempBigData, decoded)
	}
}

func TestInvalidBlobDecoding(t *testing.T) {
	data := Data("this is a test of invalid blob decoding")
	var b Blob
	if err := b.FromData(data); err != nil {
		t.Fatalf("failed to encode bytes: %v", err)
	}

	b[32] = 0b10000000 // field elements should never have their highest order bit set
	_, err := b.ToData()
	require.ErrorIs(t, err, ErrBlobInvalidFieldElement)
	b[32] = 0x0

	b[32] = 0b01000000 // field elements should never have their second highest order bit set
	_, err = b.ToData()
	require.ErrorIs(t, err, ErrBlobInvalidFieldElement)
	b[32] = 0x0

	b[VersionOffset] = 0x01 // invalid encoding version
	_, err = b.ToData()
	require.ErrorIs(t, err, ErrBlobInvalidEncodingVersion)
	b[VersionOffset] = EncodingVersion

	b[2] = 0xFF // encode an invalid (much too long) length prefix
	_, err = b.ToData()
	require.ErrorIs(t, err, ErrBlobInvalidLength)
}

func TestTooLongDataEncoding(t *testing.T) {
	// should never be able to encode data that has size the same as that of the blob due to < 256
	// bit precision of each field element
	data := Data(make([]byte, BlobSize))
	var b Blob
	err := b.FromData(data)
	require.ErrorIs(t, err, ErrBlobInputTooLarge)
}

func FuzzEncodeDecodeBlob(f *testing.F) {
	var b Blob
	f.Fuzz(func(t *testing.T, d []byte) {
		b.Clear()
		data := Data(d)
		err := b.FromData(data)
		require.NoError(t, err)
		decoded, err := b.ToData()
		require.NoError(t, err)
		require.Equal(t, data, decoded)
	})
}

func FuzzDetectNonBijectivity(f *testing.F) {
	var b Blob
	r := rand.New(rand.NewSource(99))
	f.Fuzz(func(t *testing.T, d []byte) {
		b.Clear()
		data := Data(d)
		err := b.FromData(data)
		require.NoError(t, err)
		// randomly flip a bit and make sure the data either fails to decode or decodes differently
		byteToFlip := r.Intn(BlobSize)
		bitToFlip := r.Intn(8)
		mask := byte(1 << bitToFlip)
		b[byteToFlip] = b[byteToFlip] ^ mask
		decoded, err := b.ToData()
		if err != nil {
			require.NotEqual(t, data, decoded)
		}
	})
}

func TestDecodeTestVectors(t *testing.T) {
	cases := []struct {
		input, output string
		err           error
	}{
		{
			// an empty blob has version 0 and length 0, so is valid and will decode as empty output
			input:  "",
			output: "",
		},
		{
			// encode len==1, so should get one zero byte output
			input:  "\x00\x00\x00\x00\x01",
			output: "\x00",
		},
		{
			// encode len==130044 (0x01FBFC), max blob capacity, so should get 130044 zero bytes
			// for output
			input:  "\x00\x00\x01\xFB\xFC",
			output: string(make([]byte, 130044)),
		},
		{
			// encode len==130045 (0x01FBFD) which is greater than blob capacity, blob invalid
			input: "\x00\x00\x01\xFB\xFD",
			err:   ErrBlobInvalidLength,
		},
		{
			// encode len=10
			input:  "\x00\x00\x00\x00\x0a\x00\x00\x00\x00\x00\x00\x00\x00\x00\xff",
			output: "\x00\x00\x00\x00\x00\x00\x00\x00\x00\xff",
		},
		{
			// decode what should be 27 0xFF bytes
			input:  "\x00\x00\x00\x00\x1b\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff",
			output: "\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff",
		},
		{
			// decode what should be 32 0xFF bytes
			input:  "\x3f\x00\x00\x00\x20\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\x30\xff\xff\xff\xff",
			output: "\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff",
		},
	}

	var b Blob
	for _, c := range cases {
		b.Clear()
		copy(b[:], []byte(c.input))
		decoded, err := b.ToData()
		if c.err != nil {
			require.ErrorIs(t, err, c.err)
		} else {
			require.NoError(t, err)
			require.Equal(t, len(c.output), len(decoded))
			require.Equal(t, c.output, string(decoded))
		}
	}
}

func TestEncodeTestVectors(t *testing.T) {
	cases := []struct {
		input, output string
		err           error
	}{
		{
			// empty (all zeros) blob should decode as empty string
			input:  "",
			output: "",
		},
		{
			// max input data
			input:  string(make([]byte, MaxBlobDataSize)),
			output: "\x00\x00\x01\xfb\xfc",
		},
		{
			// input data too big
			input:  string(make([]byte, MaxBlobDataSize+1)),
			output: "",
			err:    ErrBlobInputTooLarge,
		},
		{
			// 27 bytes each with high order bits set (should cleanly fit in the first FE along with the version+length)
			input:  "\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff",
			output: "\x00\x00\x00\x00\x1b\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff",
		},
		{
			// 28 bytes each with high order bits set, requires high bits spilling into byte 0 and last byte
			input:  "\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff",
			output: "\x3f\x00\x00\x00\x1c\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\x30",
		},
	}

	var b, testBlob Blob
	for _, c := range cases {
		err := b.FromData(Data(c.input))
		if c.err != nil {
			require.ErrorIs(t, err, c.err)
		} else {
			require.NoError(t, err)
			testBlob.Clear()
			copy(testBlob[:], []byte(c.output))
			require.Equal(t, testBlob, b)
		}
	}
}

func TestExtraneousData(t *testing.T) {
	var b Blob
	// make sure 0 length blob with non-zero bits in upper byte are detected & rejected
	input := "\x30\x00\x00\x00\x00"
	copy(b[:], []byte(input))
	_, err := b.ToData()
	require.ErrorIs(t, err, ErrBlobExtraneousDataFieldElement)
	input = "\x01\x00\x00\x00\x00"
	copy(b[:], []byte(input))
	_, err = b.ToData()
	require.ErrorIs(t, err, ErrBlobExtraneousDataFieldElement)
	b.Clear()
	i := len(input)

	// make sure non-zero bytes in blob following the encoded length are detected & rejected
	for ; i < 128; i++ {
		b[i-1] = 0
		b[i] = 0x01
		decoded, err := b.ToData()
		require.ErrorIs(t, err, ErrBlobExtraneousDataFieldElement, len(decoded))
	}
	for ; i < BlobSize; i += 7 { // increment by 7 bytes each iteration so the test isn't too slow
		b[i-1] = 0
		b[i] = 1
		decoded, err := b.ToData()
		require.ErrorIs(t, err, ErrBlobExtraneousData, len(decoded))
	}
}

// TestCalcBlobFeeDefault ensures that the best-effort implementation of CalcBlobFeeDefault
// works as expected. In particular, this test will quickly fail and help detect any changes
// made to the internals of the upstream eip4844.CalcBlobFee function, on which
// CalcBlobFeeDefault relies on with certain assumptions.
func TestCalcBlobFeeDefault(t *testing.T) {
	header := &types.Header{
		ExcessBlobGas: ptr(uint64(20 * params.DefaultCancunBlobConfig.UpdateFraction)),
	}
	cancunBlobFee := CalcBlobFeeDefault(header)
	require.Equal(t, big.NewInt(485165195), cancunBlobFee)

	header.RequestsHash = &(common.Hash{})
	pragueBlobFee := CalcBlobFeeDefault(header)
	require.Equal(t, big.NewInt(617436), pragueBlobFee)
}
