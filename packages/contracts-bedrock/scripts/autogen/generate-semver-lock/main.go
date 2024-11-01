package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	// find semver files
	files, err := findSemverFiles()
	if err != nil {
		return err
	}

	// Call the function to write semver lock (implementation needed)
	if err := writeSemverLock(files); err != nil {
		return fmt.Errorf("failed to write semver lock: %w", err)
	}

	return nil
}

func findSemverFiles() ([]string, error) {
	// Execute grep command to find files with @custom:semver
	var cmd = exec.Command("bash", "-c", "grep -rl '@custom:semver' src | jq -Rs 'split(\"\n\") | map(select(length > 0))'")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse the JSON array of files
	var files []string
	if err := json.Unmarshal(output, &files); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	return files, nil
}

func writeSemverLock(files []string) error {
	// Map to store our JSON output
	output := make(map[string]map[string]string)

	for _, file := range files {
		// Read file contents
		fileContents, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", file, err)
		}

		// Extract contract name from file path using regex
		re := regexp.MustCompile(`src/.*/(.+)\.sol`)
		matches := re.FindStringSubmatch(file)
		if len(matches) < 2 {
			return fmt.Errorf("invalid file path format: %s", file)
		}
		contractName := matches[1]

		// Get artifacts directory
		cmd := exec.Command("forge", "config", "--json")
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get forge config: %w", err)
		}
		var config struct {
			Out string `json:"out"`
		}
		if err := json.Unmarshal(out, &config); err != nil {
			return fmt.Errorf("failed to parse forge config: %w", err)
		}

		// Get artifact files
		artifactDir := filepath.Join(config.Out, contractName+".sol")
		files, err := os.ReadDir(artifactDir)
		if err != nil {
			return fmt.Errorf("failed to read artifact directory: %w", err)
		}
		if len(files) == 0 {
			return fmt.Errorf("no artifacts found for %s", contractName)
		}

		// Read initcode from artifact
		artifactPath := filepath.Join(artifactDir, files[0].Name())
		artifact, err := os.ReadFile(artifactPath)
		if err != nil {
			return fmt.Errorf("failed to read initcode: %w", err)
		}
		artifactJson := json.RawMessage(artifact)
		var artifactObj struct {
			Bytecode struct {
				Object string `json:"object"`
			} `json:"bytecode"`
		}
		if err := json.Unmarshal(artifactJson, &artifactObj); err != nil {
			return fmt.Errorf("failed to parse artifact: %w", err)
		}

		// convert the hex bytecode to a uint8 array / bytes
		bytes, err := hex.DecodeString(strings.TrimPrefix(artifactObj.Bytecode.Object, "0x"))
		if err != nil {
			return fmt.Errorf("failed to decode hex: %w", err)
		}

		// Calculate hashes using Keccak256
		var sourceCode = []byte(strings.TrimSuffix(string(fileContents), "\n"))
		initCodeHash := fmt.Sprintf("0x%x", crypto.Keccak256Hash(bytes))
		sourceCodeHash := fmt.Sprintf("0x%x", crypto.Keccak256Hash(sourceCode))

		// Store in output map
		output[file] = map[string]string{
			"initCodeHash":   initCodeHash,
			"sourceCodeHash": sourceCodeHash,
		}
	}

	// Write to JSON file
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	if err := os.WriteFile("semver-lock.json", jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write semver lock file: %w", err)
	}

	fmt.Println("Wrote semver lock file to \"semver-lock.json\".")
	return nil
}
