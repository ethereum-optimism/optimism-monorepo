package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_findImports(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "finds single named import",
			content: `
				pragma solidity ^0.8.0;
				import { Contract } from "./Contract.sol";
				contract Test {}
			`,
			expected: []string{"Contract"},
		},
		{
			name: "finds multiple named imports",
			content: `
				pragma solidity ^0.8.0;
				import { Contract1, Contract2 } from "./Contracts.sol";
				contract Test {}
			`,
			expected: []string{"Contract1", "Contract2"},
		},
		{
			name: "handles import with as keyword",
			content: `
				pragma solidity ^0.8.0;
				import { Contract as Renamed } from "./Contract.sol";
				contract Test {}
			`,
			expected: []string{"Renamed"},
		},
		{
			name: "handles multiple imports with as keyword",
			content: `
				pragma solidity ^0.8.0;
				import { Contract1 as C1, Contract2 as C2 } from "./Contracts.sol";
				contract Test {}
			`,
			expected: []string{"C1", "C2"},
		},
		{
			name: "ignores regular imports",
			content: `
				pragma solidity ^0.8.0;
				import "./Contract.sol";
				contract Test {}
			`,
			expected: nil,
		},
		{
			name:     "empty content",
			content:  "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findImports(tt.content)
			require.Equal(t, tt.expected, result)
		})
	}
}

func Test_isImportUsed(t *testing.T) {
	tests := []struct {
		name         string
		importedName string
		content      string
		expected     bool
	}{
		{
			name:         "import used in contract",
			importedName: "UsedContract",
			content: `
				contract Test {
					UsedContract used;
				}
			`,
			expected: true,
		},
		{
			name:         "import used in inheritance",
			importedName: "BaseContract",
			content: `
				contract Test is BaseContract {
				}
			`,
			expected: true,
		},
		{
			name:         "import used in function",
			importedName: "Utility",
			content: `
				contract Test {
					function test() {
						Utility.doSomething();
					}
				}
			`,
			expected: true,
		},
		{
			name:         "import not used",
			importedName: "UnusedContract",
			content: `
				contract Test {
					OtherContract other;
				}
			`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isImportUsed(tt.importedName, tt.content)
			require.Equal(t, tt.expected, result)
		})
	}
}
