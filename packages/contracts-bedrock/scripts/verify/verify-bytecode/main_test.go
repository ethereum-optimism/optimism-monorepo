package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadArtifact(t *testing.T) {
	// Create a temporary artifact file
	tempDir := t.TempDir()
	artifactPath := filepath.Join(tempDir, "artifact.json")

	// Test case 1: Valid artifact
	validArtifact := map[string]interface{}{
		"deployedBytecode": map[string]interface{}{
			"object": "0x1234",
		},
	}
	artifactJSON, err := json.Marshal(validArtifact)
	require.NoError(t, err)
	err = os.WriteFile(artifactPath, artifactJSON, 0644)
	require.NoError(t, err)

	artifact, err := loadArtifact(artifactPath)
	require.NoError(t, err)
	assert.Equal(t, "0x1234", artifact["deployedBytecode"].(map[string]interface{})["object"])

	// Test case 2: Empty path
	_, err = loadArtifact("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "artifact path is required")

	// Test case 3: Non-existent file
	_, err = loadArtifact(filepath.Join(tempDir, "nonexistent.json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read artifact file")

	// Test case 4: Invalid JSON
	err = os.WriteFile(artifactPath, []byte("invalid json"), 0644)
	require.NoError(t, err)
	_, err = loadArtifact(artifactPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

func TestGetDeployedBytecode(t *testing.T) {
	tests := []struct {
		name     string
		artifact map[string]interface{}
		want     string
		wantErr  bool
	}{
		{
			name: "Forge/Foundry format",
			artifact: map[string]interface{}{
				"deployedBytecode": map[string]interface{}{
					"object": "0x1234",
				},
			},
			want:    "0x1234",
			wantErr: false,
		},
		{
			name: "Standard format with string",
			artifact: map[string]interface{}{
				"deployedBytecode": "0x5678",
			},
			want:    "0x5678",
			wantErr: false,
		},
		{
			name: "Bytecode object format",
			artifact: map[string]interface{}{
				"bytecode": map[string]interface{}{
					"object": "0xabcd",
				},
			},
			want:    "0xabcd",
			wantErr: false,
		},
		{
			name: "Bytecode string format",
			artifact: map[string]interface{}{
				"bytecode": "0xef01",
			},
			want:    "0xef01",
			wantErr: false,
		},
		{
			name:     "No bytecode",
			artifact: map[string]interface{}{},
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDeployedBytecode(tt.artifact)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetVariableNameFromAST(t *testing.T) {
	tests := []struct {
		name     string
		artifact map[string]interface{}
		varID    string
		want     string
	}{
		{
			name: "Find variable by ID",
			artifact: map[string]interface{}{
				"ast": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"id":   float64(123),
							"name": "testVar",
						},
					},
				},
			},
			varID: "123",
			want:  "testVar",
		},
		{
			name: "Find variable with path prefix",
			artifact: map[string]interface{}{
				"ast": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"id":   float64(456),
							"name": "prefixedVar",
						},
					},
				},
			},
			varID: "path:to:456",
			want:  "prefixedVar",
		},
		{
			name: "Variable not found",
			artifact: map[string]interface{}{
				"ast": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"id":   float64(789),
							"name": "otherVar",
						},
					},
				},
			},
			varID: "999",
			want:  "999", // Returns the ID if not found
		},
		{
			name: "Non-numeric ID",
			artifact: map[string]interface{}{
				"ast": map[string]interface{}{
					"nodes": []interface{}{},
				},
			},
			varID: "abc",
			want:  "abc", // Returns the ID if not numeric
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getVariableNameFromAST(tt.artifact, tt.varID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindNodeName(t *testing.T) {
	tests := []struct {
		name     string
		node     interface{}
		targetID int
		want     string
	}{
		{
			name: "Find node in map",
			node: map[string]interface{}{
				"id":   float64(123),
				"name": "testNode",
			},
			targetID: 123,
			want:     "testNode",
		},
		{
			name: "Find node in nested map",
			node: map[string]interface{}{
				"child": map[string]interface{}{
					"id":   float64(456),
					"name": "nestedNode",
				},
			},
			targetID: 456,
			want:     "nestedNode",
		},
		{
			name: "Find node in array",
			node: map[string]interface{}{
				"children": []interface{}{
					map[string]interface{}{
						"id":   float64(789),
						"name": "arrayNode",
					},
				},
			},
			targetID: 789,
			want:     "arrayNode",
		},
		{
			name: "Node not found",
			node: map[string]interface{}{
				"id":   float64(111),
				"name": "wrongNode",
			},
			targetID: 999,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findNodeName(tt.node, tt.targetID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetImmutableReferences(t *testing.T) {
	tests := []struct {
		name     string
		artifact map[string]interface{}
		want     map[string][]ImmutableReference
		wantLen  int
	}{
		{
			name: "Forge/Foundry format",
			artifact: map[string]interface{}{
				"deployedBytecode": map[string]interface{}{
					"immutableReferences": map[string]interface{}{
						"123": []interface{}{
							map[string]interface{}{
								"start":  float64(10),
								"length": float64(32),
							},
						},
					},
				},
				"ast": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"id":   float64(123),
							"name": "testVar",
						},
					},
				},
			},
			want: map[string][]ImmutableReference{
				"testVar": {
					{
						Offset: 10,
						Length: 32,
						Value:  "",
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "Standard format",
			artifact: map[string]interface{}{
				"immutableReferences": map[string]interface{}{
					"456": []interface{}{
						[]interface{}{float64(20), float64(16)},
					},
				},
				"ast": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"id":   float64(456),
							"name": "anotherVar",
						},
					},
				},
			},
			want: map[string][]ImmutableReference{
				"anotherVar": {
					{
						Offset: 20,
						Length: 16,
						Value:  "",
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "No immutable references",
			artifact: map[string]interface{}{
				"deployedBytecode": map[string]interface{}{},
			},
			want:    map[string][]ImmutableReference{},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getImmutableReferences(tt.artifact)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantLen, len(got))

			// Check specific values for non-empty cases
			if tt.wantLen > 0 {
				for k, v := range tt.want {
					assert.Contains(t, got, k)
					assert.Equal(t, v[0].Offset, got[k][0].Offset)
					assert.Equal(t, v[0].Length, got[k][0].Length)
				}
			}
		})
	}
}

func TestIsInImmutableReference(t *testing.T) {
	immutableRefs := map[string][]ImmutableReference{
		"var1": {
			{Offset: 10, Length: 5, Value: ""},
		},
		"var2": {
			{Offset: 20, Length: 10, Value: ""},
			{Offset: 40, Length: 5, Value: ""},
		},
	}

	tests := []struct {
		name        string
		position    int
		wantIn      bool
		wantVarName string
		wantRef     bool
	}{
		{
			name:        "Inside first variable",
			position:    12,
			wantIn:      true,
			wantVarName: "var1",
			wantRef:     true,
		},
		{
			name:        "At start of first variable",
			position:    10,
			wantIn:      true,
			wantVarName: "var1",
			wantRef:     true,
		},
		{
			name:        "At end of first variable (exclusive)",
			position:    15,
			wantIn:      false,
			wantVarName: "",
			wantRef:     false,
		},
		{
			name:        "Inside second variable, first reference",
			position:    25,
			wantIn:      true,
			wantVarName: "var2",
			wantRef:     true,
		},
		{
			name:        "Inside second variable, second reference",
			position:    42,
			wantIn:      true,
			wantVarName: "var2",
			wantRef:     true,
		},
		{
			name:        "Outside any variable",
			position:    30,
			wantIn:      false,
			wantVarName: "",
			wantRef:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inImmutable, varName, ref := isInImmutableReference(tt.position, immutableRefs)
			assert.Equal(t, tt.wantIn, inImmutable)
			assert.Equal(t, tt.wantVarName, varName)
			if tt.wantRef {
				assert.NotNil(t, ref)
			} else {
				assert.Nil(t, ref)
			}
		})
	}
}

func TestFindDifferences(t *testing.T) {
	tests := []struct {
		name             string
		expectedBytecode string
		actualBytecode   string
		immutableRefs    map[string][]ImmutableReference
		wantDiffs        int
		wantImmutable    int
		wantErr          bool
	}{
		{
			name:             "No differences",
			expectedBytecode: "0x1234567890abcdef",
			actualBytecode:   "0x1234567890abcdef",
			immutableRefs:    map[string][]ImmutableReference{},
			wantDiffs:        0,
			wantImmutable:    0,
			wantErr:          false,
		},
		{
			name:             "Difference in immutable reference",
			expectedBytecode: "0x1234000000abcdef",
			actualBytecode:   "0x1234fffffeabcdef",
			immutableRefs: map[string][]ImmutableReference{
				"testVar": {
					{Offset: 2, Length: 3, Value: ""},
				},
			},
			wantDiffs:     1,
			wantImmutable: 1,
			wantErr:       false,
		},
		{
			name:             "Difference outside immutable reference",
			expectedBytecode: "0x1234567890abcdef",
			actualBytecode:   "0x1234567890abcdee", // Last byte different
			immutableRefs:    map[string][]ImmutableReference{},
			wantDiffs:        1,
			wantImmutable:    0,
			wantErr:          false,
		},
		{
			name:             "Multiple differences",
			expectedBytecode: "0x1234000000abcdef",
			actualBytecode:   "0x1234fffffeabcdee", // Immutable and non-immutable differences
			immutableRefs: map[string][]ImmutableReference{
				"testVar": {
					{Offset: 2, Length: 3, Value: ""},
				},
			},
			wantDiffs:     2,
			wantImmutable: 1,
			wantErr:       false,
		},
		{
			name:             "Invalid expected bytecode",
			expectedBytecode: "0xZZZZ",
			actualBytecode:   "0x1234",
			immutableRefs:    map[string][]ImmutableReference{},
			wantDiffs:        0,
			wantImmutable:    0,
			wantErr:          true,
		},
		{
			name:             "Invalid actual bytecode",
			expectedBytecode: "0x1234",
			actualBytecode:   "0xZZZZ",
			immutableRefs:    map[string][]ImmutableReference{},
			wantDiffs:        0,
			wantImmutable:    0,
			wantErr:          true,
		},
		{
			name:             "Different lengths",
			expectedBytecode: "0x1234",
			actualBytecode:   "0x123456",
			immutableRefs:    map[string][]ImmutableReference{},
			wantDiffs:        0, // No differences in the common part
			wantImmutable:    0,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs, err := findDifferences(tt.expectedBytecode, tt.actualBytecode, tt.immutableRefs)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantDiffs, len(diffs))

			// Count immutable differences
			immutableCount := 0
			for _, diff := range diffs {
				if diff.InImmutable {
					immutableCount++
				}
			}
			assert.Equal(t, tt.wantImmutable, immutableCount)
		})
	}
}

func TestFindDifferencesDetailed(t *testing.T) {
	// Test with specific bytecode patterns to verify exact difference detection
	expected := "0x1234567890abcdef"
	actual := "0x1234FF7890abFFef"

	immutableRefs := map[string][]ImmutableReference{
		"testVar": {
			{Offset: 2, Length: 1, Value: ""}, // Covers the "FF" difference
		},
	}

	diffs, err := findDifferences(expected, actual, immutableRefs)
	require.NoError(t, err)

	// Should find 2 differences: one in immutable ref, one outside
	assert.Equal(t, 2, len(diffs))

	// First difference should be in immutable reference
	assert.True(t, diffs[0].InImmutable)
	assert.Equal(t, "testVar", diffs[0].ImmutableName)
	assert.Equal(t, 2, diffs[0].Start) // 0-based index after 0x prefix
	assert.Equal(t, 1, diffs[0].Length)
	assert.Equal(t, "56", diffs[0].Expected)
	assert.Equal(t, "ff", diffs[0].Actual)

	// Second difference should be outside immutable reference
	assert.False(t, diffs[1].InImmutable)
	assert.Equal(t, "", diffs[1].ImmutableName)
	assert.Equal(t, 6, diffs[1].Start) // 0-based index after 0x prefix
	assert.Equal(t, 1, diffs[1].Length)
	assert.Equal(t, "cd", diffs[1].Expected)
	assert.Equal(t, "ff", diffs[1].Actual)

	// Check that immutable reference value was captured
	assert.Equal(t, "ff", immutableRefs["testVar"][0].Value)
}

// TestPrintDifferences doesn't test the actual output (which goes to stdout)
// but ensures the function doesn't panic with various inputs
func TestPrintDifferences(t *testing.T) {
	differences := []BytecodeDifference{
		{
			Start:         10,
			Length:        2,
			Expected:      "1234",
			Actual:        "5678",
			InImmutable:   true,
			ImmutableName: "testVar",
		},
		{
			Start:         20,
			Length:        1,
			Expected:      "ab",
			Actual:        "cd",
			InImmutable:   false,
			ImmutableName: "",
		},
	}

	immutableRefs := map[string][]ImmutableReference{
		"testVar": {
			{Offset: 10, Length: 2, Value: "5678"},
		},
	}

	// This should not panic
	printDifferences(differences, immutableRefs)

	// Test with empty differences
	printDifferences([]BytecodeDifference{}, immutableRefs)

	// Test with empty immutable references
	printDifferences(differences, map[string][]ImmutableReference{})
}

// Test handling of bytecode with and without 0x prefix
func TestBytecodePrefix(t *testing.T) {
	expected := "0x1234"
	actual := "1234" // No prefix

	diffs, err := findDifferences(expected, actual, map[string][]ImmutableReference{})
	require.NoError(t, err)
	assert.Equal(t, 0, len(diffs), "Should handle different prefixes correctly")

	// Test the reverse
	diffs, err = findDifferences(actual, expected, map[string][]ImmutableReference{})
	require.NoError(t, err)
	assert.Equal(t, 0, len(diffs), "Should handle different prefixes correctly")
}

// Test consecutive differences are properly grouped
func TestConsecutiveDifferences(t *testing.T) {
	expected := "0x123456789a"
	actual := "0x12FFFF789a" // Two consecutive bytes different

	diffs, err := findDifferences(expected, actual, map[string][]ImmutableReference{})
	require.NoError(t, err)

	// Should group consecutive differences
	assert.Equal(t, 1, len(diffs), "Consecutive differences should be grouped")
	assert.Equal(t, 2, diffs[0].Length, "Difference should span 2 bytes")
	assert.Equal(t, "3456", diffs[0].Expected)
	assert.Equal(t, "ffff", diffs[0].Actual)
}

// Test with empty bytecode
func TestEmptyBytecode(t *testing.T) {
	_, err := findDifferences("0x", "0x", map[string][]ImmutableReference{})
	assert.NoError(t, err, "Should handle empty bytecode")

	_, err = findDifferences("", "", map[string][]ImmutableReference{})
	assert.NoError(t, err, "Should handle empty bytecode without prefix")
}

// Test with invalid hex characters
func TestInvalidHex(t *testing.T) {
	_, err := findDifferences("0x123Z", "0x1234", map[string][]ImmutableReference{})
	assert.Error(t, err, "Should detect invalid hex in expected bytecode")

	_, err = findDifferences("0x1234", "0x123Z", map[string][]ImmutableReference{})
	assert.Error(t, err, "Should detect invalid hex in actual bytecode")
}
