package altda

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/params"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func encodeCommitmentData(commitmentType CommitmentType, data []byte) []byte {
	return append([]byte{byte(commitmentType)}, data...)
}

// TestCommitmentData tests the CommitmentData type and its implementations,
// by encoding and decoding the commitment data and verifying the input data.
func TestCommitmentData(t *testing.T) {

	type tcase struct {
		name        string
		commType    CommitmentType
		commInput   []byte
		commData    []byte
		expectedErr error
	}

	input := []byte{0}
	hash := crypto.Keccak256(input)

	testCases := []tcase{
		{
			name:        "valid keccak256 commitment",
			commType:    Keccak256CommitmentType,
			commInput:   input,
			commData:    encodeCommitmentData(Keccak256CommitmentType, hash),
			expectedErr: nil,
		},
		{
			name:        "invalid keccak256 commitment",
			commType:    Keccak256CommitmentType,
			commInput:   input,
			commData:    encodeCommitmentData(Keccak256CommitmentType, []byte("ab_baddata_yz012345")),
			expectedErr: ErrInvalidCommitment,
		},
		{
			name:        "valid generic commitment",
			commType:    GenericCommitmentType,
			commInput:   []byte("any input works"),
			commData:    encodeCommitmentData(GenericCommitmentType, []byte("any length of data! wow, that's so generic!")),
			expectedErr: nil, // This should actually be valid now
		},
		{
			name:        "invalid commitment type",
			commType:    9,
			commInput:   []byte("some input"),
			commData:    encodeCommitmentData(CommitmentType(9), []byte("abcdefghijklmnopqrstuvwxyz012345")),
			expectedErr: ErrInvalidCommitment,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			comm, err := DecodeCommitmentData(tc.commData)
			require.ErrorIs(t, err, tc.expectedErr)
			if err == nil {
				// Test that the commitment type is correct
				require.Equal(t, tc.commType, comm.CommitmentType())
				// Test that reencoding the commitment returns the same data
				require.Equal(t, tc.commData, comm.Encode())
				// Test that TxData() returns the same data as the original, prepended with a version byte
				require.Equal(t, append([]byte{params.DerivationVersion1}, tc.commData...), comm.TxData())

				// Test that Verify() returns no error for the correct data
				require.NoError(t, comm.Verify(tc.commInput))
				// Test that Verify() returns error for the incorrect data
				// don't do this for GenericCommitmentType, which does not do any verification
				if tc.commType != GenericCommitmentType {
					require.ErrorIs(t, ErrCommitmentMismatch, comm.Verify([]byte("wrong data")))
				}
			}
		})
	}
}
