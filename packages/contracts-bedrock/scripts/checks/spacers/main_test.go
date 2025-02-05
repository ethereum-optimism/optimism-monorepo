package main

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/stretchr/testify/require"
)

func Test_parseVariableLength(t *testing.T) {
	tests := []struct {
		name         string
		variableType string
		types        map[string]solc.StorageLayoutType
		expected     int
		expectError  bool
	}{
		{
			name:         "uses type from map",
			variableType: "t_custom",
			types: map[string]solc.StorageLayoutType{
				"t_custom": {NumberOfBytes: 16},
			},
			expected: 16,
		},
		{
			name:         "mapping type",
			variableType: "t_mapping(address,uint256)",
			expected:     32,
		},
		{
			name:         "uint type",
			variableType: "t_uint256",
			expected:     32,
		},
		{
			name:         "bytes_ type",
			variableType: "t_bytes_storage",
			expected:     32,
		},
		{
			name:         "bytes type",
			variableType: "t_bytes32",
			expected:     32,
		},
		{
			name:         "address type",
			variableType: "t_address",
			expected:     20,
		},
		{
			name:         "bool type",
			variableType: "t_bool",
			expected:     1,
		},
		{
			name:         "array type",
			variableType: "t_array(t_uint256)2",
			expected:     64, // 2 * 32
		},
		{
			name:         "unsupported type",
			variableType: "t_unknown",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			length, err := parseVariableLength(tt.variableType, tt.types)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, length)
			}
		})
	}
}

func Test_validateSpacer(t *testing.T) {
	tests := []struct {
		name          string
		variable      solc.StorageLayoutEntry
		types         map[string]solc.StorageLayoutType
		expectedErrs  int
		errorContains string
	}{
		{
			name: "valid spacer",
			variable: solc.StorageLayoutEntry{
				Contract: "TestContract",
				Label:    "spacer_1_2_32",
				Slot:     1,
				Offset:   2,
				Type:     "t_uint256",
			},
			types: map[string]solc.StorageLayoutType{
				"t_uint256": {NumberOfBytes: 32},
			},
			expectedErrs: 0,
		},
		{
			name: "invalid name format",
			variable: solc.StorageLayoutEntry{
				Label: "spacer_invalid",
			},
			expectedErrs:  1,
			errorContains: "invalid spacer name format",
		},
		{
			name: "wrong slot",
			variable: solc.StorageLayoutEntry{
				Contract: "TestContract",
				Label:    "spacer_1_2_32",
				Slot:     2,
				Offset:   2,
				Type:     "t_uint256",
			},
			types: map[string]solc.StorageLayoutType{
				"t_uint256": {NumberOfBytes: 32},
			},
			expectedErrs:  1,
			errorContains: "is in slot",
		},
		{
			name: "wrong offset",
			variable: solc.StorageLayoutEntry{
				Contract: "TestContract",
				Label:    "spacer_1_2_32",
				Slot:     1,
				Offset:   3,
				Type:     "t_uint256",
			},
			types: map[string]solc.StorageLayoutType{
				"t_uint256": {NumberOfBytes: 32},
			},
			expectedErrs:  1,
			errorContains: "is at offset",
		},
		{
			name: "wrong length",
			variable: solc.StorageLayoutEntry{
				Contract: "TestContract",
				Label:    "spacer_1_2_32",
				Slot:     1,
				Offset:   2,
				Type:     "t_uint128",
			},
			types: map[string]solc.StorageLayoutType{
				"t_uint128": {NumberOfBytes: 16},
			},
			expectedErrs:  1,
			errorContains: "bytes long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateSpacer(tt.variable, tt.types)
			require.Len(t, errors, tt.expectedErrs)
			if tt.errorContains != "" {
				require.Contains(t, errors[0].Error(), tt.errorContains)
			}
		})
	}
}
