package eth

import (
	"encoding/binary"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalOutput_UnknownVersion(t *testing.T) {
	_, err := UnmarshalOutput([]byte{0: 0xA, 32: 0xA})
	require.ErrorIs(t, err, ErrInvalidOutputVersion)
}

func TestUnmarshalOutput_TooShortForVersion(t *testing.T) {
	_, err := UnmarshalOutput([]byte{0xA})
	require.ErrorIs(t, err, ErrInvalidOutput)
}

func TestOutputV0Codec(t *testing.T) {
	output := OutputV0{
		StateRoot:                Bytes32{1, 2, 3},
		MessagePasserStorageRoot: Bytes32{4, 5, 6},
		BlockHash:                common.Hash{7, 8, 9},
	}
	marshaled := output.Marshal()
	unmarshaled, err := UnmarshalOutput(marshaled)
	require.NoError(t, err)
	unmarshaledV0 := unmarshaled.(*OutputV0)
	require.Equal(t, output, *unmarshaledV0)

	_, err = UnmarshalOutput([]byte{64: 0xA})
	require.ErrorIs(t, err, ErrInvalidOutput)
}

func TestOutputV1Codec(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		chainA := Bytes32{0x01}
		chainB := Bytes32{0x02}
		chainC := Bytes32{0x03}
		output := OutputV1{
			Timestamp: 7000,
			Outputs:   []Bytes32{chainA, chainB, chainC},
		}
		marshaled := output.Marshal()
		unmarshaled, err := UnmarshalOutput(marshaled)
		require.NoError(t, err)
		unmarshaledV1 := unmarshaled.(*OutputV1)
		require.Equal(t, output, *unmarshaledV1)
	})

	t.Run("BelowMinLength", func(t *testing.T) {
		_, err := UnmarshalOutput(append(OutputVersionV1[:], 0x01))
		require.ErrorIs(t, err, ErrInvalidOutput)
	})

	t.Run("NoChainsIncluded", func(t *testing.T) {
		_, err := UnmarshalOutput(binary.BigEndian.AppendUint64(OutputVersionV1[:], 134058))
		require.ErrorIs(t, err, ErrInvalidOutput)
	})

	t.Run("PartialChainOutputRoot", func(t *testing.T) {
		input := binary.BigEndian.AppendUint64(OutputVersionV1[:], 134058)
		input = append(input, 0x01, 0x02, 0x03)
		_, err := UnmarshalOutput(input)
		require.ErrorIs(t, err, ErrInvalidOutput)
	})
}
