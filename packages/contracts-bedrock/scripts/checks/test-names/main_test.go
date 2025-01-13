package main

import (
	"reflect"
	"slices"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestCamelCaseCheck(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected bool
	}{
		{"valid single part", []string{"test"}, true},
		{"valid multiple parts", []string{"test", "something", "succeeds"}, true},
		{"invalid uppercase", []string{"Test"}, false},
		{"invalid middle uppercase", []string{"test", "Something", "succeeds"}, false},
		{"empty parts", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checks["camelCase"].check(tt.parts); got != tt.expected {
				t.Errorf("checkCamelCase error for %v = %v, want %v", tt.parts, got, tt.expected)
			}
		})
	}
}

func TestPartsCountCheck(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected bool
	}{
		{"three parts", []string{"test", "something", "succeeds"}, true},
		{"four parts", []string{"test", "something", "reason", "fails"}, true},
		{"too few parts", []string{"test", "fails"}, false},
		{"too many parts", []string{"test", "a", "b", "c", "fails"}, false},
		{"empty parts", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checks["partsCount"].check(tt.parts); got != tt.expected {
				t.Errorf("checkPartsCount error for %v = %v, want %v", tt.parts, got, tt.expected)
			}
		})
	}
}

func TestPrefixCheck(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected bool
	}{
		{"valid test", []string{"test", "something", "succeeds"}, true},
		{"valid testFuzz", []string{"testFuzz", "something", "succeeds"}, true},
		{"valid testDiff", []string{"testDiff", "something", "succeeds"}, true},
		{"invalid prefix", []string{"testing", "something", "succeeds"}, false},
		{"empty parts", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checks["prefix"].check(tt.parts); got != tt.expected {
				t.Errorf("checkPrefix error for %v = %v, want %v", tt.parts, got, tt.expected)
			}
		})
	}
}

func TestSuffixCheck(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected bool
	}{
		{"valid succeeds", []string{"test", "something", "succeeds"}, true},
		{"valid reverts", []string{"test", "something", "reverts"}, true},
		{"valid fails", []string{"test", "something", "fails"}, true},
		{"valid works", []string{"test", "something", "works"}, true},
		{"valid benchmark", []string{"test", "something", "benchmark"}, true},
		{"valid benchmark_num", []string{"test", "something", "benchmark", "123"}, true},
		{"invalid suffix", []string{"test", "something", "invalid"}, false},
		{"invalid benchmark_text", []string{"test", "something", "benchmark", "abc"}, false},
		{"empty parts", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checks["suffix"].check(tt.parts); got != tt.expected {
				t.Errorf("checkSuffix error for %v = %v, want %v", tt.parts, got, tt.expected)
			}
		})
	}
}

func TestFailurePartsCheck(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected bool
	}{
		{"valid failure with reason", []string{"test", "something", "reason", "fails"}, true},
		{"valid failure with reason", []string{"test", "something", "reason", "reverts"}, true},
		{"invalid failure without reason", []string{"test", "something", "fails"}, false},
		{"invalid failure without reason", []string{"test", "something", "reverts"}, false},
		{"valid non-failure with three parts", []string{"test", "something", "succeeds"}, true},
		{"empty parts", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checks["failureParts"].check(tt.parts); got != tt.expected {
				t.Errorf("checkFailureParts error for %v = %v, want %v", tt.parts, got, tt.expected)
			}
		})
	}
}

func TestCheckTestName(t *testing.T) {
	tests := []struct {
		name          string
		testName      string
		shouldSucceed bool
	}{
		// Valid test names - Basic patterns
		{"valid basic test succeeds", "test_something_succeeds", true},
		{"valid basic test fails with reason", "test_something_reason_fails", true},
		{"valid basic test reverts with reason", "test_something_reason_reverts", true},
		{"valid basic test works", "test_something_works", true},

		// Valid test names - Fuzz variants
		{"valid fuzz test succeeds", "testFuzz_something_succeeds", true},
		{"valid fuzz test fails with reason", "testFuzz_something_reason_fails", true},
		{"valid fuzz test reverts with reason", "testFuzz_something_reason_reverts", true},
		{"valid fuzz test works", "testFuzz_something_works", true},

		// Valid test names - Diff variants
		{"valid diff test succeeds", "testDiff_something_succeeds", true},
		{"valid diff test fails with reason", "testDiff_something_reason_fails", true},
		{"valid diff test reverts with reason", "testDiff_something_reason_reverts", true},
		{"valid diff test works", "testDiff_something_works", true},

		// Valid test names - Benchmark variants
		{"valid benchmark test", "test_something_benchmark", true},
		{"valid benchmark with number", "test_something_benchmark_123", true},
		{"valid benchmark with large number", "test_something_benchmark_999999", true},
		{"valid benchmark with zero", "test_something_benchmark_0", true},

		// Valid test names - Complex middle parts
		{"valid complex middle part", "test_complexOperation_succeeds", true},
		{"valid multiple word middle", "test_veryComplexOperation_succeeds", true},
		{"valid numbers in middle", "test_operation123_succeeds", true},
		{"valid special case", "test_specialCase_reason_fails", true},

		// Invalid test names - Prefix issues
		{"invalid empty string", "", false},
		{"invalid prefix Test", "Test_something_succeeds", false},
		{"invalid prefix testing", "testing_something_succeeds", false},
		{"invalid prefix testfuzz", "testfuzz_something_succeeds", false},
		{"invalid prefix testdiff", "testdiff_something_succeeds", false},
		{"invalid prefix TEST", "TEST_something_succeeds", false},

		// Invalid test names - Suffix issues
		{"invalid suffix succeed", "test_something_succeed", false},
		{"invalid suffix revert", "test_something_revert", false},
		{"invalid suffix fail", "test_something_fail", false},
		{"invalid suffix work", "test_something_work", false},
		{"invalid suffix benchmarks", "test_something_benchmarks", false},
		{"invalid benchmark suffix text", "test_something_benchmark_abc", false},
		{"invalid benchmark suffix special", "test_something_benchmark_123abc", false},

		// Invalid test names - Case issues
		{"invalid uppercase middle", "test_Something_succeeds", false},
		{"invalid multiple uppercase", "test_SomethingHere_succeeds", false},
		{"invalid all caps middle", "test_SOMETHING_succeeds", false},
		{"invalid mixed case suffix", "test_something_Succeeds", false},

		// Invalid test names - Structure issues
		{"invalid single part", "test", false},
		{"invalid two parts", "test_succeeds", false},
		{"invalid five parts", "test_this_that_those_succeeds", false},
		{"invalid six parts", "test_this_that_those_these_succeeds", false},
		{"invalid failure without reason", "test_something_fails", false},
		{"invalid revert without reason", "test_something_reverts", false},

		// Invalid test names - Special cases
		{"invalid empty parts", "test__succeeds", false},
		{"invalid multiple underscores", "test___succeeds", false},
		{"invalid trailing underscore", "test_something_succeeds_", false},
		{"invalid leading underscore", "_test_something_succeeds", false},
		{"invalid benchmark no number", "test_something_benchmark_", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTestName(tt.testName)
			if (err != nil) == tt.shouldSucceed {
				t.Errorf("checkTestName(%q) error = %v, shouldSucceed %v", tt.testName, err, tt.shouldSucceed)
			}
		})
	}
}

func TestExtractTestNames(t *testing.T) {
	tests := []struct {
		name     string
		artifact *solc.ForgeArtifact
		want     []string
	}{
		{
			name: "valid test contract",
			artifact: &solc.ForgeArtifact{
				Abi: solc.AbiType{
					Parsed: abi.ABI{
						Methods: map[string]abi.Method{
							"IS_TEST":                  {Name: "IS_TEST"},
							"test_something_succeeds":  {Name: "test_something_succeeds"},
							"test_other_fails":         {Name: "test_other_fails"},
							"not_a_test":               {Name: "not_a_test"},
							"testFuzz_something_works": {Name: "testFuzz_something_works"},
						},
					},
				},
			},
			want: []string{
				"test_something_succeeds",
				"test_other_fails",
				"testFuzz_something_works",
			},
		},
		{
			name: "non-test contract",
			artifact: &solc.ForgeArtifact{
				Abi: solc.AbiType{
					Parsed: abi.ABI{
						Methods: map[string]abi.Method{
							"test_something_succeeds": {Name: "test_something_succeeds"},
							"not_a_test":              {Name: "not_a_test"},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "empty contract",
			artifact: &solc.ForgeArtifact{
				Abi: solc.AbiType{
					Parsed: abi.ABI{
						Methods: map[string]abi.Method{},
					},
				},
			},
			want: nil,
		},
		{
			name: "test contract with no test methods",
			artifact: &solc.ForgeArtifact{
				Abi: solc.AbiType{
					Parsed: abi.ABI{
						Methods: map[string]abi.Method{
							"IS_TEST":        {Name: "IS_TEST"},
							"not_a_test":     {Name: "not_a_test"},
							"another_method": {Name: "another_method"},
						},
					},
				},
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTestNames(tt.artifact)
			slices.Sort(got)
			slices.Sort(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractTestNames() = %v, want %v", got, tt.want)
			}
		})
	}
}
