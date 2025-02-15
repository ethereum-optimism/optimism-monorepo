package main

import (
	"fmt"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
)

func isTrivialType(typeString string) bool {
	// TODO: Add types or find a way to generate them
	trivialTypes := []string{"uint256", "int256", "bool", "address", "bytes32", "uint", "int"}

	for _, t := range trivialTypes {
		if typeString == t {
			return true
		}
	}

	return false
}

func isImportUsed(importNode solc.AstNode, usedTypes map[string]bool) bool {

	if importNode.AbsolutePath != "" {
		baseName := getBaseName(importNode.AbsolutePath)
		return usedTypes[baseName]
	}
	return false
}

func getBaseName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		fileName := parts[len(parts)-1]
		return strings.TrimSuffix(fileName, ".sol")
	}
	return path
}

func normaliseParam(param string, context string, localTypes map[string]bool) string {
	if strings.Contains(param, "payable") {
		return param
	}

	if strings.Contains(param, " ") {
		parts := strings.SplitN(param, " ", 2)
		if len(parts) == 2 {

			if strings.Contains(param, ".") {
				parts1 := strings.Split(param, ".")

				if localTypes[parts1[1]] {
					return parts1[1]
				}
			}

			return parts[1]
		}
	}
	return param
}

func buildDirectoryTree(root string) (*TreeNode, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", root, err)
	}

	// Create the root node
	node := &TreeNode{
		Name:  info.Name(),
		IsDir: info.IsDir(),
	}

	if info.IsDir() {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", root, err)
		}

		for _, entry := range entries {
			childPath := filepath.Join(root, entry.Name())
			childNode, err := buildDirectoryTree(childPath)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, childNode)
		}
	}

	return node, nil
}

func mapArtifact(name string, canonicalName string) *solc.ForgeArtifact {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get current working directory: %v\n", err)
		return nil
	}

	artifactsDir := filepath.Join(cwd, "packages/contracts-bedrock/forge-artifacts/")

	possiblePaths := []string{
		filepath.Join(artifactsDir, name+".sol", name+".json"),
		filepath.Join(artifactsDir, name+".sol", name+".0.8.25.json"),
		filepath.Join(artifactsDir, canonicalName+".sol", name+".json"),
		filepath.Join(artifactsDir, canonicalName+".sol", name+".0.8.25.json"),
		filepath.Join(artifactsDir, "draft-"+name+".sol", name+".0.8.15.json"),
		filepath.Join(artifactsDir, "draft-"+name+".sol", name+".json"),
	}

	for _, path := range possiblePaths {
		artifact, err := common.ReadForgeArtifact(path)
		if err == nil {
			return artifact
		}
	}

	println(fmt.Sprintf("\nFailed to map: %s", name))

	return nil
}

func extractMappingDetails(mappingString string) ([]string, string) {
	// Recursive function to parse mapping structure
	var parseMapping func(string) ([]string, string)
	parseMapping = func(mappingStr string) ([]string, string) {
		var localMappingTypes []string

		// Regular expression to match the mapping structure
		pattern := `mapping\(([^=>]+)\s*=>\s*(.+)\)`
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(strings.TrimSpace(mappingStr))

		if len(match) == 3 {
			keyType := strings.TrimSpace(match[1])
			valueType := strings.TrimSpace(match[2])
			localMappingTypes = append(localMappingTypes, keyType)

			if strings.HasPrefix(valueType, "mapping") {
				// Recursively parse the inner mapping
				innerTypes, finalReturnType := parseMapping(valueType)
				localMappingTypes = append(localMappingTypes, innerTypes...)
				return localMappingTypes, finalReturnType
			}
			// Base case: value type is the return type
			return localMappingTypes, valueType
		}
		return localMappingTypes, ""
	}

	// Remove anything after "public" or similar modifiers
	cleanString := strings.Split(mappingString, "public")[0]
	return parseMapping(cleanString)
}

func resolveAlias(typeName string, aliasMapping map[string]string) string {
	if alias, exists := aliasMapping[typeName]; exists {
		return alias
	}
	return typeName
}
