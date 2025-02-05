package serialize

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestRoundTripWithReader(t *testing.T) {
	// Test that reader can read the data written by BinaryWriter.
	// The writer tests check that the generated data is what is expected, so simply check that the reader correctly
	// parses a range of data here rather than duplicating the expected binary serialization.
	buf := new(bytes.Buffer)
	out := NewBinaryWriter(buf)
	require.NoError(t, out.WriteBool(true))
	require.NoError(t, out.WriteBool(false))
	require.NoError(t, out.WriteUInt(uint8(5)))
	require.NoError(t, out.WriteUInt(uint32(76)))
	require.NoError(t, out.WriteUInt(uint64(24824424)))
	expectedHash := common.HexToHash("0x5a8f75b8e1c1529d1d1c596464d17b99763604f4c00b280436fc0dffacc60efd")
	require.NoError(t, out.WriteHash(expectedHash))
	expectedBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	require.NoError(t, out.WriteBytes(expectedBytes))

	in := NewBinaryReader(buf)
	var b bool
	require.NoError(t, in.ReadBool(&b))
	require.True(t, b)
	require.NoError(t, in.ReadBool(&b))
	require.False(t, b)
	var vUInt8 uint8
	require.NoError(t, in.ReadUInt(&vUInt8))
	require.Equal(t, uint8(5), vUInt8)
	var vUInt32 uint32
	require.NoError(t, in.ReadUInt(&vUInt32))
	require.Equal(t, uint32(76), vUInt32)
	var vUInt64 uint64
	require.NoError(t, in.ReadUInt(&vUInt64))
	require.Equal(t, uint64(24824424), vUInt64)
	var hash common.Hash
	require.NoError(t, in.ReadHash(&hash))
	require.Equal(t, expectedHash, hash)
	var data []byte
	require.NoError(t, in.ReadBytes(&data))
	require.Equal(t, expectedBytes, data)
}
