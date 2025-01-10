package eth

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalSuperRoot_UnknownVersion(t *testing.T) {
	_, err := UnmarshalSuperRoot([]byte{0: 0xA, 32: 0xA})
	require.ErrorIs(t, err, ErrInvalidSuperRootVersion)
}

func TestUnmarshalSuperRoot_TooShortForVersion(t *testing.T) {
	_, err := UnmarshalSuperRoot([]byte{})
	require.ErrorIs(t, err, ErrInvalidSuperRoot)
}

func TestSuperRootV1Codec(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		chainA := ChainIdOutputPair{ChainID: 11, Output: Bytes32{0x01}}
		chainB := ChainIdOutputPair{ChainID: 12, Output: Bytes32{0x02}}
		chainC := ChainIdOutputPair{ChainID: 13, Output: Bytes32{0x03}}
		superRoot := SuperV1{
			Timestamp: 7000,
			Chains:    []ChainIdOutputPair{chainA, chainB, chainC},
		}
		marshaled := superRoot.Marshal()
		unmarshaled, err := UnmarshalSuperRoot(marshaled)
		require.NoError(t, err)
		unmarshaledV1 := unmarshaled.(*SuperV1)
		require.Equal(t, superRoot, *unmarshaledV1)
	})

	t.Run("BelowMinLength", func(t *testing.T) {
		_, err := UnmarshalSuperRoot(append([]byte{SuperRootVersionV1}, 0x01))
		require.ErrorIs(t, err, ErrInvalidSuperRoot)
	})

	t.Run("NoChainsIncluded", func(t *testing.T) {
		_, err := UnmarshalSuperRoot(binary.BigEndian.AppendUint64([]byte{SuperRootVersionV1}, 134058))
		require.ErrorIs(t, err, ErrInvalidSuperRoot)
	})

	t.Run("PartialChainSuperRoot", func(t *testing.T) {
		input := binary.BigEndian.AppendUint64([]byte{SuperRootVersionV1}, 134058)
		input = append(input, 0x01, 0x02, 0x03)
		_, err := UnmarshalSuperRoot(input)
		require.ErrorIs(t, err, ErrInvalidSuperRoot)
	})
}
