package artifacts

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashIntegrityChecker_CheckIntegrity(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		setupHash   [32]byte
		expectError bool
	}{
		{
			name:        "valid hash matches data",
			data:        []byte("test data"),
			setupHash:   sha256.Sum256([]byte("test data")),
			expectError: false,
		},
		{
			name:        "invalid hash doesn't match data",
			data:        []byte("test data"),
			setupHash:   sha256.Sum256([]byte("different data")),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &hashIntegrityChecker{
				hash: tt.setupHash,
			}

			err := checker.CheckIntegrity(tt.data)

			if tt.expectError {
				require.ErrorContains(t, err, "integrity check failed")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
