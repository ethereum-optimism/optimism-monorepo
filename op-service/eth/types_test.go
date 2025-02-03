package eth

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/stretchr/testify/require"
)

func TestInputError(t *testing.T) {
	err := InputError{
		Inner: errors.New("test error"),
		Code:  InvalidForkchoiceState,
	}
	var x InputError
	if !errors.As(err, &x) {
		t.Fatalf("need InputError to be detected as such")
	}
	require.ErrorIs(t, err, InputError{}, "need to detect input error with errors.Is")

	var rpcErr rpc.Error
	require.ErrorAs(t, err, &rpcErr, "need input error to be rpc.Error with errors.As")
	require.EqualValues(t, err.Code, rpcErr.ErrorCode())
}

type scalarTest struct {
	name              string
	val               Bytes32
	fail              bool
	blobBaseFeeScalar uint32
	baseFeeScalar     uint32
}

func TestEcotoneScalars(t *testing.T) {
	testCases := []scalarTest{
		{"dirty padding v0 scalar", Bytes32{0: 0, 27: 1, 31: 2}, false, 0, math.MaxUint32},
		{"dirty padding v0 scalar v2", Bytes32{0: 0, 1: 1, 31: 2}, false, 0, math.MaxUint32},
		{"valid v0 scalar", Bytes32{0: 0, 27: 0, 31: 2}, false, 0, 2},
		{"invalid v1 scalar", Bytes32{0: 1, 7: 1, 31: 2}, true, 0, 0},
		{"valid v1 scalar with 0 blob scalar", Bytes32{0: 1, 27: 0, 31: 2}, false, 0, 2},
		{"valid v1 scalar with non-0 blob scalar", Bytes32{0: 1, 27: 123, 31: 2}, false, 123, 2},
		{"valid v1 scalar with non-0 blob scalar and 0 scalar", Bytes32{0: 1, 27: 123, 31: 0}, false, 123, 0},
		{"zero v0 scalar", Bytes32{0: 0}, false, 0, 0},
		{"zero v1 scalar", Bytes32{0: 1}, false, 0, 0},
		{"unknown version", Bytes32{0: 2}, true, 0, 0},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sysConfig := SystemConfig{Scalar: tc.val}
			scalars, err := sysConfig.EcotoneScalars()
			if tc.fail {
				require.NotNil(t, err)
			} else {
				require.Equal(t, tc.blobBaseFeeScalar, scalars.BlobBaseFeeScalar)
				require.Equal(t, tc.baseFeeScalar, scalars.BaseFeeScalar)
				require.NoError(t, err)
			}
		})
	}
}

func TestOperatorFeeScalars(t *testing.T) {
	sysConfig := SystemConfig{OperatorFeeParams: Bytes32{0: 0, 20: 4, 29: 3}}
	params := sysConfig.OperatorFee()
	require.Equal(t, uint32(0x4000000), params.Scalar)
	require.Equal(t, uint64(0x30000), params.Constant)
}

func FuzzEncodeScalar(f *testing.F) {
	f.Fuzz(func(t *testing.T, blobBaseFeeScalar uint32, baseFeeScalar uint32) {
		encoded := EncodeScalar(EcotoneScalars{BlobBaseFeeScalar: blobBaseFeeScalar, BaseFeeScalar: baseFeeScalar})
		scalars, err := DecodeScalar(encoded)
		require.NoError(t, err)
		require.Equal(t, blobBaseFeeScalar, scalars.BlobBaseFeeScalar)
		require.Equal(t, baseFeeScalar, scalars.BaseFeeScalar)
	})
}

func FuzzEncodeOperatorFeeParams(f *testing.F) {
	f.Fuzz(func(t *testing.T, scalar uint32, constant uint64) {
		encoded := EncodeOperatorFeeParams(OperatorFeeParams{Scalar: scalar, Constant: constant})
		scalars := DecodeOperatorFeeParams(encoded)
		require.Equal(t, scalar, scalars.Scalar)
		require.Equal(t, constant, scalars.Constant)
	})
}

func TestSystemConfigMarshaling(t *testing.T) {
	sysConfig := SystemConfig{
		BatcherAddr:       common.Address{'A'},
		Overhead:          Bytes32{0x4, 0x5, 0x6},
		Scalar:            Bytes32{0x7, 0x8, 0x9},
		OperatorFeeParams: Bytes32{0x1, 0x2, 0x3},
		GasLimit:          1234,
		// Leave EIP1559 params empty to prove that the
		// zero value is sent.
	}
	j, err := json.Marshal(sysConfig)
	require.NoError(t, err)
	require.Equal(t, `{"batcherAddr":"0x4100000000000000000000000000000000000000","overhead":"0x0405060000000000000000000000000000000000000000000000000000000000","scalar":"0x0708090000000000000000000000000000000000000000000000000000000000","gasLimit":1234,"eip1559Params":"0x0000000000000000","operatorFeeParams":"0x0102030000000000000000000000000000000000000000000000000000000000"}`, string(j))
	sysConfig.MarshalPreHolocene = true
	j, err = json.Marshal(sysConfig)
	require.NoError(t, err)
	require.Equal(t, `{"batcherAddr":"0x4100000000000000000000000000000000000000","overhead":"0x0405060000000000000000000000000000000000000000000000000000000000","scalar":"0x0708090000000000000000000000000000000000000000000000000000000000","gasLimit":1234}`, string(j))
}

func TestStorageKey(t *testing.T) {
	cases := []struct {
		unmarshaled string
		marshaled   []byte
	}{
		{
			unmarshaled: "0x",
			marshaled:   []byte{},
		},
		{
			unmarshaled: "0x0",
			marshaled:   []byte{0},
		},
		{
			unmarshaled: "0x1",
			marshaled:   []byte{1},
		},
		{
			unmarshaled: "0x01",
			marshaled:   []byte{1},
		},
		{
			unmarshaled: "0x01020304",
			marshaled:   []byte{1, 2, 3, 4},
		},
		{
			unmarshaled: "0xF01FF02",
			marshaled:   []byte{0xF, 0x01, 0xFF, 0x02},
		},
	}

	for _, c := range cases {
		var key StorageKey
		err := key.UnmarshalText([]byte(c.unmarshaled))
		require.NoError(t, err)
		require.Equal(t, c.marshaled, []uint8(key)[:])
	}
}

func TestBytes(t *testing.T) {
	testBytesN(t, 8, &Bytes8{0: 1, 8 - 1: 2}, func() BytesN { return new(Bytes8) })
	testBytesN(t, 32, &Bytes32{0: 1, 32 - 1: 2}, func() BytesN { return new(Bytes32) })
	testBytesN(t, 65, &Bytes65{0: 1, 65 - 1: 2}, func() BytesN { return new(Bytes65) })
	testBytesN(t, 96, &Bytes96{0: 1, 96 - 1: 2}, func() BytesN { return new(Bytes96) })
	testBytesN(t, 256, &Bytes256{0: 1, 256 - 1: 2}, func() BytesN { return new(Bytes256) })
}

type BytesN interface {
	String() string
	TerminalString() string
	UnmarshalJSON(text []byte) error
	UnmarshalText(text []byte) error
	MarshalText() ([]byte, error)
}

func testBytesN(t *testing.T, n int, x BytesN, alloc func() BytesN) {
	t.Run(fmt.Sprintf("Bytes%d", n), func(t *testing.T) {
		xStr := "0x01" + strings.Repeat("00", n-2) + "02"
		require.Equal(t, xStr, x.String())
		if n == 8 { // too short for dots
			require.Equal(t, "0x0100000000000002", x.TerminalString())
		} else {
			require.Equal(t, "0x010000..000002", x.TerminalString())
		}
		out, err := x.MarshalText()
		require.NoError(t, err)
		require.Equal(t, xStr, string(out))

		y := alloc()
		require.NoError(t, y.UnmarshalText([]byte(xStr)))
		require.Equal(t, x, y)

		z := alloc()
		require.NoError(t, z.UnmarshalJSON([]byte(fmt.Sprintf("%q", xStr))))
		require.Equal(t, x, z)
	})
}
