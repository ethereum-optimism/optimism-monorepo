package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

func main() {
	if _, err := common.ProcessFilesGlob(
		[]string{"forge-artifacts/**/*.json"},
		[]string{},
		processFile,
	); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func processFile(path string) (*common.Void, []error) {
	artifact, err := common.ReadForgeArtifact(path)
	if err != nil {
		return nil, []error{err}
	}

	var errors []error
	names := extractTestNames(artifact)
	for _, name := range names {
		if err = checkTestName(name); err != nil {
			errors = append(errors, err)
		}
	}

	return nil, errors
}

func extractTestNames(artifact *solc.ForgeArtifact) []string {
	isTest := false
	for _, entry := range artifact.Abi.Parsed.Methods {
		if entry.Name == "IS_TEST" {
			isTest = true
			break
		}
	}
	if !isTest {
		return nil
	}

	names := []string{}
	for _, entry := range artifact.Abi.Parsed.Methods {
		if !strings.HasPrefix(entry.Name, "test") {
			continue
		}
		names = append(names, entry.Name)
	}

	return names
}

type CheckFunc func(parts []string) bool

type CheckInfo struct {
	error string
	check CheckFunc
}

var checks = map[string]CheckInfo{
	"doubleUnderscores": {
		error: "test names cannot have double underscores",
		check: func(parts []string) bool {
			for _, part := range parts {
				if len(strings.TrimSpace(part)) == 0 {
					return false
				}
			}
			return true
		},
	},
	"camelCase": {
		error: "test name parts should be in camelCase",
		check: func(parts []string) bool {
			for _, part := range parts {
				if len(part) > 0 && unicode.IsUpper(rune(part[0])) {
					return false
				}
			}
			return true
		},
	},
	"partsCount": {
		error: "test names should have either 3 or 4 parts, each separated by underscores",
		check: func(parts []string) bool {
			return len(parts) == 3 || len(parts) == 4
		},
	},
	"prefix": {
		error: "test names should begin with 'test', 'testFuzz', or 'testDiff'",
		check: func(parts []string) bool {
			return len(parts) > 0 && (parts[0] == "test" || parts[0] == "testFuzz" || parts[0] == "testDiff")
		},
	},
	"suffix": {
		error: "test names should end with either 'succeeds', 'reverts', 'fails', 'works', or 'benchmark[_num]'",
		check: func(parts []string) bool {
			if len(parts) == 0 {
				return false
			}
			last := parts[len(parts)-1]
			if last == "succeeds" || last == "reverts" || last == "fails" || last == "works" {
				return true
			}
			if len(parts) >= 2 && parts[len(parts)-2] == "benchmark" {
				_, err := strconv.Atoi(last)
				return err == nil
			}
			return last == "benchmark"
		},
	},
	"failureParts": {
		error: "failure tests should have 4 parts, third part should indicate the reason for failure",
		check: func(parts []string) bool {
			if len(parts) == 0 {
				return false
			}
			last := parts[len(parts)-1]
			return len(parts) == 4 || (last != "reverts" && last != "fails")
		},
	},
}

func checkTestName(name string) error {
	parts := strings.Split(name, "_")
	for _, check := range checks {
		if !check.check(parts) {
			return fmt.Errorf("%s: %s", name, check.error)
		}
	}
	return nil
}
