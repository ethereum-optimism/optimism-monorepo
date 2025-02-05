package altda

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/params"
	"github.com/stretchr/testify/require"
)

// TestCommitmentData tests the CommitmentData type and its implementations,
// by encoding and decoding the commitment data and verifying the input data.
func TestCommitmentData(t *testing.T) {

	type tcase struct {
		name        string
		commType    CommitmentType
		commData    []byte
		expectedErr error
	}

	testCases := []tcase{
		{
			name:        "valid keccak256 commitment",
			commType:    Keccak256CommitmentType,
			commData:    []byte("abcdefghijklmnopqrstuvwxyz012345"),
			expectedErr: ErrInvalidCommitment,
		},
		{
			name:        "invalid keccak256 commitment",
			commType:    Keccak256CommitmentType,
			commData:    []byte("ab_baddata_yz012345"),
			expectedErr: ErrInvalidCommitment,
		},
		{
			name:        "valid generic commitment",
			commType:    GenericCommitmentType,
			commData:    []byte("any length of data! wow, that's so generic!"),
			expectedErr: ErrInvalidCommitment,
		},
		{
			name:        "invalid commitment type",
			commType:    9,
			commData:    []byte("abcdefghijklmnopqrstuvwxyz012345"),
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
				require.NoError(t, comm.Verify(tc.commData))
				// Test that Verify() returns error for the incorrect data
				// don't do this for GenericCommitmentType, which does not do any verification
				if tc.commType != GenericCommitmentType {
					require.ErrorIs(t, ErrCommitmentMismatch, comm.Verify([]byte("wrong data")))
				}
			}
		})
	}
}
