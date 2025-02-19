package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

const (
	storageLayoutDir = "snapshots/storageLayout"
	abiDir           = "snapshots/abi"
)

type SnapshotResult struct {
	ContractName  string
	Abi           interface{}
	StorageLayout []solc.AbiSpecStorageLayoutEntry
}

func main() {
	if err := resetDirectory(storageLayoutDir); err != nil {
		fmt.Printf("failed to reset storage layout directory: %v\n", err)
		os.Exit(1)
	}
	if err := resetDirectory(abiDir); err != nil {
		fmt.Printf("failed to reset abi directory: %v\n", err)
		os.Exit(1)
	}

	results, err := common.ProcessFilesGlob(
		[]string{"forge-artifacts/**/*.json"},
		[]string{},
		processFile,
	)
	if err != nil {
		fmt.Printf("Failed to generate snapshots: %v\n", err)
		os.Exit(1)
	}

	for _, result := range results {
		if result == nil {
			continue
		}

		err := common.WriteJSON(result.Abi, filepath.Join(abiDir, fmt.Sprintf("%s.json", result.ContractName)))
		if err != nil {
			fmt.Printf("failed to write abi: %v\n", err)
			os.Exit(1)
		}

		err = common.WriteJSON(result.StorageLayout, filepath.Join(storageLayoutDir, fmt.Sprintf("%s.json", result.ContractName)))
		if err != nil {
			fmt.Printf("failed to write storage layout: %v\n", err)
			os.Exit(1)
		}
	}
}

func processFile(file string) (*SnapshotResult, []error) {
	artifact, err := common.ReadForgeArtifact(file)
	if err != nil {
		return nil, []error{err}
	}

	contractName, err := parseArtifactName(file)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to parse artifact name %q: %w", file, err)}
	}

	// Skip anything that isn't in the src directory, with the exception of
	// GnosisSafe because it's used for decoding storage changes in superchain-ops.
	if !strings.HasPrefix(artifact.Ast.AbsolutePath, "src/") && contractName != "GnosisSafe" {
		return nil, nil
	}

	// Skip anything that isn't a proper contract.
	isContract := false
	for _, node := range artifact.Ast.Nodes {
		if node.NodeType == "ContractDefinition" &&
			node.Name == contractName &&
			node.ContractKind == "contract" &&
			!node.Abstract {
			isContract = true
			break
		}
	}
	if !isContract {
		return nil, nil
	}

	storageLayout := make([]solc.AbiSpecStorageLayoutEntry, 0, len(artifact.StorageLayout.Storage))
	for _, storageEntry := range artifact.StorageLayout.Storage {
		// Convert ast-based type to Solidity type.
		typ, ok := artifact.StorageLayout.Types[storageEntry.Type]
		if !ok {
			return nil, []error{fmt.Errorf("undefined type for %s:%s", contractName, storageEntry.Label)}
		}

		// Convert to Solidity storage layout entry.
		storageLayout = append(storageLayout, solc.AbiSpecStorageLayoutEntry{
			Label:  storageEntry.Label,
			Bytes:  typ.NumberOfBytes,
			Offset: storageEntry.Offset,
			Slot:   storageEntry.Slot,
			Type:   typ.Label,
		})
	}

	return &SnapshotResult{
		ContractName:  contractName,
		Abi:           artifact.Abi.Raw,
		StorageLayout: storageLayout,
	}, nil
}

// ContractName.0.9.8.json -> ContractName.sol
// ContractName.json -> ContractName.sol
func parseArtifactName(artifactVersionFile string) (string, error) {
	re := regexp.MustCompile(`(.*?)\.([0-9]+\.[0-9]+\.[0-9]+)?`)
	baseName := filepath.Base(artifactVersionFile)
	match := re.FindStringSubmatch(baseName)
	if len(match) < 2 {
		return "", fmt.Errorf("invalid artifact file name: %q", artifactVersionFile)
	}
	return match[1], nil
}

func resetDirectory(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove directory %q: %w", dir, err)
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}
	return nil
}
