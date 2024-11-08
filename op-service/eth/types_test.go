package eth

import (
	"encoding/json"
	"errors"
	"math"
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

func FuzzEncodeScalar(f *testing.F) {
	f.Fuzz(func(t *testing.T, blobBaseFeeScalar uint32, baseFeeScalar uint32) {
		encoded := EncodeScalar(EcotoneScalars{BlobBaseFeeScalar: blobBaseFeeScalar, BaseFeeScalar: baseFeeScalar})
		scalars, err := DecodeScalar(encoded)
		require.NoError(t, err)
		require.Equal(t, blobBaseFeeScalar, scalars.BlobBaseFeeScalar)
		require.Equal(t, baseFeeScalar, scalars.BaseFeeScalar)
	})
}

func TestSystemConfigMarshaling(t *testing.T) {
	sysConfig := SystemConfig{
		BatcherAddr: common.Address{'A'},
		Overhead:    Bytes32{0x4, 0x5, 0x6},
		Scalar:      Bytes32{0x7, 0x8, 0x9},
		GasLimit:    1234,
		// Leave EIP1559 params empty to prove that the
		// zero value is sent.
	}
	j, err := json.Marshal(sysConfig)
	require.NoError(t, err)
	require.Equal(t, `{"batcherAddr":"0x4100000000000000000000000000000000000000","overhead":"0x0405060000000000000000000000000000000000000000000000000000000000","scalar":"0x0708090000000000000000000000000000000000000000000000000000000000","gasLimit":1234,"eip1559Params":"0x0000000000000000"}`, string(j))
	sysConfig.MarshalPreHolocene = true
	j, err = json.Marshal(sysConfig)
	require.NoError(t, err)
	require.Equal(t, `{"batcherAddr":"0x4100000000000000000000000000000000000000","overhead":"0x0405060000000000000000000000000000000000000000000000000000000000","scalar":"0x0708090000000000000000000000000000000000000000000000000000000000","gasLimit":1234}`, string(j))
}
