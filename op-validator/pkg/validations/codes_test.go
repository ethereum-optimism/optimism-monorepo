package validations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorDescription(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "known error code",
			code:     "SPRCFG-10",
			expected: "SuperchainConfig is paused",
		},
		{
			name:     "another known error code",
			code:     "PORTAL-10",
			expected: "OptimismPortal version mismatch",
		},
		{
			name:     "unknown error code",
			code:     "INVALID-CODE",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorDescription(tt.code)
			require.Equal(t, tt.expected, got, "ErrorDescription returned unexpected value for code %q", tt.code)
		})
	}
}
