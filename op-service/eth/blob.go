package eth

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/params"
)

const (
	BlobSize        = 4096 * 32
	MaxBlobDataSize = (4*31+3)*1024 - 4
	EncodingVersion = 0
	VersionOffset   = 1    // offset of the version byte in the blob encoding
	Rounds          = 1024 // number of encode/decode rounds
)

var (
	ErrBlobInvalidFieldElement        = errors.New("invalid field element")
	ErrBlobInvalidEncodingVersion     = errors.New("invalid encoding version")
	ErrBlobInvalidLength              = errors.New("invalid length for blob")
	ErrBlobInputTooLarge              = errors.New("too much data to encode in one blob")
	ErrBlobExtraneousData             = errors.New("non-zero data encountered where blob should be empty")
	ErrBlobExtraneousDataFieldElement = errors.New("non-zero data encountered where field element should be empty")
)

type Blob [BlobSize]byte

func (b *Blob) KZGBlob() *kzg4844.Blob {
	return (*kzg4844.Blob)(b)
}

func (b *Blob) UnmarshalJSON(text []byte) error {
	return hexutil.UnmarshalFixedJSON(reflect.TypeOf(b), text, b[:])
}

func (b *Blob) UnmarshalText(text []byte) error {
	return hexutil.UnmarshalFixedText("Bytes32", text, b[:])
}

func (b *Blob) MarshalText() ([]byte, error) {
	return hexutil.Bytes(b[:]).MarshalText()
}

func (b *Blob) String() string {
	return hexutil.Encode(b[:])
}

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (b *Blob) TerminalString() string {
	return fmt.Sprintf("%x..%x", b[:3], b[BlobSize-3:])
}

func (b *Blob) ComputeKZGCommitment() (kzg4844.Commitment, error) {
	return kzg4844.BlobToCommitment(b.KZGBlob())
}

// KZGToVersionedHash computes the "blob hash" (a.k.a. versioned-hash) of a blob-commitment, as used in a blob-tx.
// We implement it here because it is unfortunately not (currently) exposed by geth.
func KZGToVersionedHash(commitment kzg4844.Commitment) (out common.Hash) {
	hasher := sha256.New()
	return kzg4844.CalcBlobHashV1(hasher, &commitment)
}

// VerifyBlobProof verifies that the given blob and proof corresponds to the given commitment,
// returning error if the verification fails.
func VerifyBlobProof(blob *Blob, commitment kzg4844.Commitment, proof kzg4844.Proof) error {
	return kzg4844.VerifyBlobProof(blob.KZGBlob(), commitment, proof)
}

// FromData encodes the given input data into this blob. The encoding scheme is as follows:
//
// In each round we perform 7 reads of input of lengths (31,1,31,1,31,1,31) bytes respectively for
// a total of 127 bytes. This data is encoded into the next 4 field elements of the output by
// placing each of the 4x31 byte chunks into bytes [1:32] of its respective field element. The
// three single byte chunks (24 bits) are split into 4x6-bit chunks, each of which is written into
// the top most byte of its respective field element, leaving the top 2 bits of each field element
// empty to avoid modulus overflow.  This process is repeated for up to 1024 rounds until all data
// is encoded.
//
// For only the very first output field, bytes [1:5] are used to encode the version and the length
// of the data.
func (b *Blob) FromData(data Data) error {
	if len(data) > MaxBlobDataSize {
		return fmt.Errorf("%w: len=%v", ErrBlobInputTooLarge, data)
	}
	b.Clear()

	readOffset := 0

	// read 1 byte of input, 0 if there is no input left
	read1 := func() byte {
		if readOffset >= len(data) {
			return 0
		}
		out := data[readOffset]
		readOffset += 1
		return out
	}

	writeOffset := 0
	var buf31 [31]byte
	var zero31 [31]byte

	// Read up to 31 bytes of input (left-aligned), into buf31.
	read31 := func() {
		if readOffset >= len(data) {
			copy(buf31[:], zero31[:])
			return
		}
		n := copy(buf31[:], data[readOffset:]) // copy as much data as we can
		copy(buf31[n:], zero31[:])             // pad with zeroes (since there might not be enough data)
		readOffset += n
	}
	// Write a byte, updates the write-offset.
	// Asserts that the write-offset matches encoding-algorithm expectations.
	// Asserts that the value is 6 bits.
	write1 := func(v byte) {
		if writeOffset%32 != 0 {
			panic(fmt.Errorf("blob encoding: invalid byte write offset: %d", writeOffset))
		}
		if v&0b1100_0000 != 0 {
			panic(fmt.Errorf("blob encoding: invalid 6 bit value: 0b%b", v))
		}
		b[writeOffset] = v
		writeOffset += 1
	}
	// Write buf31 to the blob, updates the write-offset.
	// Asserts that the write-offset matches encoding-algorithm expectations.
	write31 := func() {
		if writeOffset%32 != 1 {
			panic(fmt.Errorf("blob encoding: invalid bytes31 write offset: %d", writeOffset))
		}
		copy(b[writeOffset:], buf31[:])
		writeOffset += 31
	}

	for round := 0; round < Rounds && readOffset < len(data); round++ {
		// The first field element encodes the version and the length of the data in [1:5].
		// This is a manual substitute for read31(), preparing the buf31.
		if round == 0 {
			buf31[0] = EncodingVersion
			// Encode the length as big-endian uint24.
			// The length check at the start above ensures we can always fit the length value into only 3 bytes.
			ilen := uint32(len(data))
			buf31[1] = byte(ilen >> 16)
			buf31[2] = byte(ilen >> 8)
			buf31[3] = byte(ilen)

			readOffset += copy(buf31[4:], data[:])
		} else {
			read31()
		}

		x := read1()
		A := x & 0b0011_1111
		write1(A)
		write31()

		read31()
		y := read1()
		B := (y & 0b0000_1111) | ((x & 0b1100_0000) >> 2)
		write1(B)
		write31()

		read31()
		z := read1()
		C := z & 0b0011_1111
		write1(C)
		write31()

		read31()
		D := ((z & 0b1100_0000) >> 2) | ((y & 0b1111_0000) >> 4)
		write1(D)
		write31()
	}

	if readOffset < len(data) {
		panic(fmt.Errorf("expected to fit data but failed, read offset: %d, data: %d", readOffset, len(data)))
	}
	return nil
}

// ToData decodes the blob into raw byte data. See FromData above for details on the encoding
// format. If error is returned it will be one of InvalidFieldElementError,
// InvalidEncodingVersionError and InvalidLengthError.
func (b *Blob) ToData() (Data, error) {
	// check the version
	if b[VersionOffset] != EncodingVersion {
		return nil, fmt.Errorf(
			"%w: expected version %d, got %d", ErrBlobInvalidEncodingVersion, EncodingVersion, b[VersionOffset])
	}

	// decode the 3-byte big-endian length value into a 4-byte integer
	outputLen := uint32(b[2])<<16 | uint32(b[3])<<8 | uint32(b[4])
	if outputLen > MaxBlobDataSize {
		return nil, fmt.Errorf("%w: got %d", ErrBlobInvalidLength, outputLen)
	}

	// round 0 is special cased to copy only the remaining 27 bytes of the first field element into
	// the output due to version/length encoding already occupying its first 5 bytes.
	output := make(Data, MaxBlobDataSize)
	copy(output[0:27], b[5:])

	// now process remaining 3 field elements to complete round 0
	opos := 28 // current position into output buffer
	ipos := 32 // current position into the input blob
	var err error
	encodedByte := make([]byte, 4) // buffer for the 4 6-bit chunks
	encodedByte[0] = b[0]
	for i := 1; i < 4; i++ {
		encodedByte[i], opos, ipos, err = b.decodeFieldElement(opos, ipos, output)
		if err != nil {
			return nil, err
		}
	}
	opos = reassembleBytes(opos, encodedByte, output)

	// in each remaining round we decode 4 field elements (128 bytes) of the input into 127 bytes
	// of output
	for i := 1; i < Rounds && opos < int(outputLen); i++ {
		for j := 0; j < 4; j++ {
			// save the first byte of each field element for later re-assembly
			encodedByte[j], opos, ipos, err = b.decodeFieldElement(opos, ipos, output)
			if err != nil {
				return nil, err
			}
		}
		opos = reassembleBytes(opos, encodedByte, output)
	}
	for i := int(outputLen); i < len(output); i++ {
		if output[i] != 0 {
			return nil, fmt.Errorf("fe=%d: %w", opos/32, ErrBlobExtraneousDataFieldElement)
		}
	}
	output = output[:outputLen]
	for ; ipos < BlobSize; ipos++ {
		if b[ipos] != 0 {
			return nil, fmt.Errorf("pos=%d: %w", ipos, ErrBlobExtraneousData)
		}
	}
	return output, nil
}

// decodeFieldElement decodes the next input field element by writing its lower 31 bytes into its
// appropriate place in the output and checking the high order byte is valid. Returns an
// InvalidFieldElementError if a field element is seen with either of its two high order bits set.
func (b *Blob) decodeFieldElement(opos, ipos int, output []byte) (byte, int, int, error) {
	// two highest order bits of the first byte of each field element should always be 0
	if b[ipos]&0b1100_0000 != 0 {
		return 0, 0, 0, fmt.Errorf("%w: field element: %d", ErrBlobInvalidFieldElement, ipos)
	}
	copy(output[opos:], b[ipos+1:ipos+32])
	return b[ipos], opos + 32, ipos + 32, nil
}

// reassembleBytes takes the 4x6-bit chunks from encodedByte, reassembles them into 3 bytes of
// output, and places them in their appropriate output positions.
func reassembleBytes(opos int, encodedByte []byte, output []byte) int {
	opos-- // account for fact that we don't output a 128th byte
	x := (encodedByte[0] & 0b0011_1111) | ((encodedByte[1] & 0b0011_0000) << 2)
	y := (encodedByte[1] & 0b0000_1111) | ((encodedByte[3] & 0b0000_1111) << 4)
	z := (encodedByte[2] & 0b0011_1111) | ((encodedByte[3] & 0b0011_0000) << 2)
	// put the re-assembled bytes in their appropriate output locations
	output[opos-32] = z
	output[opos-(32*2)] = y
	output[opos-(32*3)] = x
	return opos
}

func (b *Blob) Clear() {
	for i := 0; i < BlobSize; i++ {
		b[i] = 0
	}
}

// CalcBlobFeeDefault calculates the blob fee for the given header using eip4844.CalcBlobFee,
// using the requests hash field of the header as a best-effort heuristic whether
// Prague is active, and the default Ethereum blob schedule.
//
// This is to deal in a best-effort way with situations where the chain config is not
// available, but it can be assumed that per the definition of the Prague fork that
// Prague is active iff the requests hash field is present.
func CalcBlobFeeDefault(header *types.Header) *big.Int {
	// We make the assumption that eip4844.CalcBlobFee only needs
	// - London and Cancun to be active
	// - the Prague time to be set relative to the header time
	// and that the caller assumes the default prod Ethereum Blob schedule config.
	dummyChainCfg := &params.ChainConfig{
		LondonBlock:        common.Big0,
		CancunTime:         ptr(uint64(0)),
		BlobScheduleConfig: params.DefaultBlobSchedule,
	}
	// We assume that the requests hash is set iff Prague is active.
	if header.RequestsHash != nil {
		dummyChainCfg.PragueTime = ptr(uint64(0))
	}
	return eip4844.CalcBlobFee(dummyChainCfg, header)
}

func CalcBlobFeeCancun(excessBlobGas uint64) *big.Int {
	// Dummy Cancun header for calculation.
	cancunHeader := &types.Header{
		ExcessBlobGas: &excessBlobGas,
	}
	return CalcBlobFeeDefault(cancunHeader)
}

func ptr[T any](t T) *T { return &t }
