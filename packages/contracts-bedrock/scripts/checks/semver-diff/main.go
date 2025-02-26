package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

var (
	semverLock = "snapshots/semver-lock.json"
	excluded   = []string{
		"src/vendor/asterisc/RISCV.sol",
	}
	hasErrors = false
	mutex     sync.Mutex
	tempDir   string
)

func main() {
	var err error
	tempDir, err = os.MkdirTemp("", "semver-check")
	if err != nil {
		fmt.Printf("failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	hasChanged := hasSemverLockChanged()
	if !hasChanged {
		fmt.Println("No changes detected in semver-lock.json")
		return
	}

	upstreamSemverLock := filepath.Join(tempDir, "upstream_semver_lock.json")
	if err := getUpstreamSemverLock(upstreamSemverLock); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	localSemverLock := filepath.Join(tempDir, "local_semver_lock.json")
	if err := copyFile(semverLock, localSemverLock); err != nil {
		fmt.Printf("failed to copy semver-lock.json: %v\n", err)
		os.Exit(1)
	}

	changedContracts, err := getChangedContracts(localSemverLock, upstreamSemverLock)
	if err != nil {
		fmt.Printf("failed to get changed contracts: %v\n", err)
		os.Exit(1)
	}

	if err := common.ProcessFilesGlob(changedContracts, excluded, processFile); err != nil {
		fmt.Printf("processing failed: %v\n", err)
		os.Exit(1)
	}

	if hasErrors {
		fmt.Println("processing failed")
		os.Exit(1)
	}
}

func processFile(contract string) []error {
	if _, err := os.Stat(contract); os.IsNotExist(err) {
		fmt.Printf("❌ Error: Contract file %s not found\n", contract)
		setHasError()
		return nil
	}

	oldSourceFile := filepath.Join(tempDir, "old_"+filepath.Base(contract))
	newSourceFile := filepath.Join(tempDir, "new_"+filepath.Base(contract))

	if err := getOldSourceFile(contract, oldSourceFile); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		setHasError()
		return nil
	}

	if err := copyFile(contract, newSourceFile); err != nil {
		fmt.Printf("Failed to copy new source file: %v\n", err)
		setHasError()
		return nil
	}

	oldVersion, newVersion := extractVersion(oldSourceFile), extractVersion(newSourceFile)
	if oldVersion == "N/A" || newVersion == "N/A" {
		fmt.Printf("❌ Error: unable to extract version for %s\n", contract)
		fmt.Println("          this is probably a bug in check-semver-diff.sh")
		fmt.Println("          please report or fix the issue if possible")
		setHasError()
	} else if oldVersion == newVersion {
		fmt.Printf("❌ Error: %s has changes in semver-lock.json but no version change\n", contract)
		fmt.Printf("   Old version: %s\n", oldVersion)
		fmt.Printf("   New version: %s\n", newVersion)
		setHasError()
		return nil
	} else {
		fmt.Printf("✅ %s: version changed from %s to %s\n", contract, oldVersion, newVersion)
	}

	return nil
}

func setHasError() {
	mutex.Lock()
	hasErrors = true
	mutex.Unlock()
}

func hasSemverLockChanged() bool {
	// Define the git commands to check for changes
	commands := []string{
		"git diff origin/develop...HEAD --name-only",
		"git diff --name-only",
		"git diff --cached --name-only",
	}

	// Execute each command and capture the output
	for _, cmdStr := range commands {
		var output bytes.Buffer

		cmd := exec.Command("bash", "-c", cmdStr)
		cmd.Stdout = &output
		if err := cmd.Run(); err != nil {
			fmt.Printf("error executing command: %v\n", err)
			os.Exit(1)
		}

		// Check if the semver-lock.json file is in the output
		if bytes.Contains(output.Bytes(), []byte(semverLock)) {
			return true
		}
	}

	return false
}

func getUpstreamSemverLock(dest string) error {
	cmd := exec.Command("git", "show", "origin/develop:packages/contracts-bedrock/"+semverLock)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("could not find semver-lock.json in the snapshots/ directory of develop branch")
	}
	if err := os.WriteFile(dest, out, 0644); err != nil {
		return fmt.Errorf("failed to write semver-lock.json to %s: %w", dest, err)
	}
	return nil
}

func copyFile(src, dest string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, input, 0644)
}

func getChangedContracts(local, upstream string) ([]string, error) {
	localData, err := os.ReadFile(local)
	if err != nil {
		return nil, err
	}
	upstreamData, err := os.ReadFile(upstream)
	if err != nil {
		return nil, err
	}

	var localMap, upstreamMap map[string]interface{}
	if err := json.Unmarshal(localData, &localMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(upstreamData, &upstreamMap); err != nil {
		return nil, err
	}

	var changedContracts []string
	for key, localValue := range localMap {
		upstreamValue, exists := upstreamMap[key]
		if !exists {
			changedContracts = append(changedContracts, key)
			continue
		}

		localJSON, err := json.Marshal(localValue)
		if err != nil {
			continue
		}
		upstreamJSON, err := json.Marshal(upstreamValue)
		if err != nil {
			continue
		}

		if string(localJSON) != string(upstreamJSON) {
			changedContracts = append(changedContracts, key)
		}
	}
	return changedContracts, nil
}

func getOldSourceFile(contract, dest string) error {
	cmd := exec.Command("git", "show", "origin/develop:packages/contracts-bedrock/"+contract)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("could not find old source file for %s", contract)
	}
	return os.WriteFile(dest, out, 0644)
}

// Function to extract version from either constant or function
func extractVersion(file string) string {
	version := extractConstantVersion(file)
	if version == "" {
		version = extractFunctionVersion(file)
	}
	return version
}

// Function to extract version from contract source as a constant
func extractConstantVersion(file string) string {
	f, err := os.Open(file)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	re := regexp.MustCompile(`string.*constant.*version.*=.*"([^"]*)"`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			return matches[1]
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
	return ""
}

// Function to extract version from contract source as a function
func extractFunctionVersion(file string) string {
	f, err := os.Open(file)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	re := regexp.MustCompile(`"([^"]*)"`)

	inFunction := false
	for scanner.Scan() {
		line := scanner.Text()
		if inFunction {
			if matches := re.FindStringSubmatch(line); matches != nil {
				return matches[1]
			}
			if line == "}" {
				inFunction = false
			}
		} else if regexp.MustCompile(`function.*version\(\)`).MatchString(line) {
			inFunction = true
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
	return ""
}
