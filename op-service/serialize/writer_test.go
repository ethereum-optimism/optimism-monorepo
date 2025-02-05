package serialize

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestUInt(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		expected []byte
	}{
		{name: "uint8-zero", val: uint8(0), expected: []byte{0}},
		{name: "uint8-one", val: uint8(1), expected: []byte{1}},
		{name: "uint8-big", val: uint8(156), expected: []byte{156}},
		{name: "uint8-max", val: uint8(math.MaxUint8), expected: []byte{255}},

		{name: "uint16-zero", val: uint16(0), expected: []byte{0, 0}},
		{name: "uint16-one", val: uint16(1), expected: []byte{0, 1}},
		{name: "uint16-big", val: uint16(1283), expected: []byte{5, 3}},
		{name: "uint16-max", val: uint16(math.MaxUint16), expected: []byte{255, 255}},

		{name: "uint32-zero", val: uint32(0), expected: []byte{0, 0, 0, 0}},
		{name: "uint32-one", val: uint32(1), expected: []byte{0, 0, 0, 1}},
		{name: "uint32-big", val: uint32(1283424245), expected: []byte{0x4c, 0x7f, 0x7f, 0xf5}},
		{name: "uint32-max", val: uint32(math.MaxUint32), expected: []byte{255, 255, 255, 255}},

		{name: "uint64-zero", val: uint64(0), expected: []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "uint64-one", val: uint64(1), expected: []byte{0, 0, 0, 0, 0, 0, 0, 1}},
		{name: "uint64-big", val: uint64(1283424245242429284), expected: []byte{0x11, 0xcf, 0xa3, 0x8d, 0x19, 0xcc, 0x7f, 0x64}},
		{name: "uint64-max", val: uint64(math.MaxUint64), expected: []byte{255, 255, 255, 255, 255, 255, 255, 255}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			out := new(bytes.Buffer)
			bout := NewBinaryWriter(out)
			require.NoError(t, bout.WriteUInt(test.val))
			result := out.Bytes()
			require.Equal(t, test.expected, result)
		})
	}
}

func TestWriteHash(t *testing.T) {
	out := new(bytes.Buffer)
	bout := NewBinaryWriter(out)
	hash := common.HexToHash("0x5a8f75b8e1c1529d1d1c596464d17b99763604f4c00b280436fc0dffacc60efd")
	require.NoError(t, bout.WriteHash(hash))

	result := out.Bytes()
	require.Equal(t, hash[:], result)
}

func TestWriteBool(t *testing.T) {
	for _, val := range []bool{true, false} {
		val := val
		t.Run(fmt.Sprintf("%t", val), func(t *testing.T) {
			out := new(bytes.Buffer)
			bout := NewBinaryWriter(out)
			require.NoError(t, bout.WriteBool(val))

			result := out.Bytes()
			require.Len(t, result, 1)
			if val {
				require.Equal(t, result[0], uint8(1))
			} else {
				require.Equal(t, result[0], uint8(0))
			}
		})
	}
}

func TestWriteBytes(t *testing.T) {
	tests := []struct {
		name     string
		val      []byte
		expected []byte
	}{
		{name: "nil", val: nil, expected: []byte{0, 0, 0, 0}},
		{name: "empty", val: []byte{}, expected: []byte{0, 0, 0, 0}},
		{name: "non-empty", val: []byte{1, 2, 3, 4, 5}, expected: []byte{0, 0, 0, 5, 1, 2, 3, 4, 5}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			out := new(bytes.Buffer)
			bout := NewBinaryWriter(out)
			require.NoError(t, bout.WriteBytes(test.val))

			result := out.Bytes()
			require.Equal(t, test.expected, result)
		})
	}
}
