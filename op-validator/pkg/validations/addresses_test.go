package validations

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestValidatorAddress(t *testing.T) {
	tests := []struct {
		name        string
		chainID     uint64
		version     string
		want        common.Address
		expectError bool
	}{
		{
			name:        "Valid Sepolia v1.8.0",
			chainID:     11155111,
			version:     VersionV180,
			want:        common.HexToAddress("0x2A788Bb1D32AD0dcEC1A51B7156015Aa90548d8C"),
			expectError: false,
		},
		{
			name:        "Valid Sepolia v2.0.0",
			chainID:     11155111,
			version:     VersionV200,
			want:        common.HexToAddress("0x34FFEEF9D42E0EF0d999fBF01E006f745083Fd9b"),
			expectError: false,
		},
		{
			name:        "Invalid Chain ID",
			chainID:     999,
			version:     VersionV180,
			want:        common.Address{},
			expectError: true,
		},
		{
			name:        "Invalid Version",
			chainID:     11155111,
			version:     "v3.0.0",
			want:        common.Address{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatorAddress(tt.chainID, tt.version)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.want, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
