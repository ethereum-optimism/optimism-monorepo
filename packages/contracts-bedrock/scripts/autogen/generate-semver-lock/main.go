package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const semverLockFile = "snapshots/semver-lock.json"

type SemverLockOutput struct {
	InitCodeHash   string `json:"initCodeHash"`
	SourceCodeHash string `json:"sourceCodeHash"`
}

type SemverLockResult struct {
	SemverLockOutput
	SourceFilePath string
}

func main() {
	results, err := common.ProcessFilesGlob(
		[]string{"forge-artifacts/**/*.json"},
		[]string{},
		processFile,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	// Create the output map
	output := make(map[string]SemverLockOutput)
	for _, result := range results {
		if result == nil {
			continue
		}
		output[result.SourceFilePath] = result.SemverLockOutput
	}

	// Get and sort the keys
	keys := make([]string, 0, len(output))
	for k := range output {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create a sorted map for output
	sortedOutput := make(map[string]SemverLockOutput)
	for _, k := range keys {
		sortedOutput[k] = output[k]
	}

	// Write to JSON file
	jsonData, err := json.MarshalIndent(sortedOutput, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(semverLockFile, jsonData, 0644); err != nil {
		panic(err)
	}

	fmt.Printf("Wrote semver lock file to \"%s\".\n", semverLockFile)
}

func processFile(file string) (*SemverLockResult, []error) {
	artifact, err := common.ReadForgeArtifact(file)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read artifact: %w", err)}
	}

	// Only apply to files in the src directory.
	sourceFilePath := artifact.Ast.AbsolutePath
	if !strings.HasPrefix(sourceFilePath, "src/") {
		return nil, nil
	}

	// Check if the contract uses semver.
	semverRegex := regexp.MustCompile(`custom:semver`)
	semver := semverRegex.FindStringSubmatch(artifact.RawMetadata)
	if len(semver) == 0 {
		return nil, nil
	}

	// Extract the init code from the artifact.
	initCodeBytes, err := hex.DecodeString(strings.TrimPrefix(artifact.Bytecode.Object, "0x"))
	if err != nil {
		return nil, []error{fmt.Errorf("failed to decode hex: %w", err)}
	}

	// Extract the source contents from the AST.
	sourceCode, err := os.ReadFile(sourceFilePath)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read source file: %w", err)}
	}

	// Calculate hashes using Keccak256
	trimmedSourceCode := []byte(strings.TrimSuffix(string(sourceCode), "\n"))
	initCodeHash := fmt.Sprintf("0x%x", crypto.Keccak256Hash(initCodeBytes))
	sourceCodeHash := fmt.Sprintf("0x%x", crypto.Keccak256Hash(trimmedSourceCode))

	return &SemverLockResult{
		SourceFilePath: sourceFilePath,
		SemverLockOutput: SemverLockOutput{
			InitCodeHash:   initCodeHash,
			SourceCodeHash: sourceCodeHash,
		},
	}, nil
}
