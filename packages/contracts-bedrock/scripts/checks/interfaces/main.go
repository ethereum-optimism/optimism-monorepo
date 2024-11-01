package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/go-cmp/cmp"
)

var excludeContracts = []string{
	// External dependencies
	"IERC20", "IERC721", "IERC721Enumerable", "IERC721Upgradeable", "IERC721Metadata",
	"IERC165", "IERC165Upgradeable", "ERC721TokenReceiver", "ERC1155TokenReceiver",
	"ERC777TokensRecipient", "Guard", "IProxy", "Vm", "VmSafe", "IMulticall3",
	"IERC721TokenReceiver", "IProxyCreationCallback", "IBeacon",

	// EAS
	"IEAS", "ISchemaResolver", "ISchemaRegistry",

	// TODO: Interfaces that need to be fixed
	"IInitializable", "IOptimismMintableERC20", "ILegacyMintableERC20",
	"KontrolCheatsBase", "ISystemConfigInterop", "IResolvedDelegateProxy",
}

type ContractDefinition struct {
	ContractKind string `json:"contractKind"`
	Name         string `json:"name"`
}

type ASTNode struct {
	NodeType string   `json:"nodeType"`
	Literals []string `json:"literals,omitempty"`
	ContractDefinition
}

type ArtifactAST struct {
	Nodes []ASTNode `json:"nodes"`
}

type Artifact struct {
	AST ArtifactAST     `json:"ast"`
	ABI json.RawMessage `json:"abi"`
}

func main() {
	if err := run(); err != nil {
		writeStderr("an error occurred: %v", err)
		os.Exit(1)
	}
}

func writeStderr(msg string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	artifactsDir := filepath.Join(cwd, "forge-artifacts")

	artifactFiles, err := glob(artifactsDir, ".json")
	if err != nil {
		return fmt.Errorf("failed to get artifact files: %w", err)
	}

	// Remove duplicates from artifactFiles
	uniqueArtifacts := make(map[string]string)
	for contractName, artifactPath := range artifactFiles {
		baseName := strings.Split(contractName, ".")[0]
		uniqueArtifacts[baseName] = artifactPath
	}

	var hasErr int32
	var outMtx sync.Mutex
	fail := func(msg string, args ...any) {
		outMtx.Lock()
		writeStderr("❌  "+msg, args...)
		outMtx.Unlock()
		atomic.StoreInt32(&hasErr, 1)
	}

	sem := make(chan struct{}, runtime.NumCPU())
	for contractName, artifactPath := range uniqueArtifacts {
		contractName := contractName
		artifactPath := artifactPath

		sem <- struct{}{}

		go func() {
			defer func() {
				<-sem
			}()

			if err := processArtifact(contractName, artifactPath, artifactsDir, fail); err != nil {
				fail("%s: %v", contractName, err)
			}
		}()
	}

	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}

	if atomic.LoadInt32(&hasErr) == 1 {
		return errors.New("interface check failed, see logs above")
	}

	return nil
}

func processArtifact(contractName, artifactPath, artifactsDir string, fail func(string, ...any)) error {
	if isExcluded(contractName) {
		return nil
	}

	artifact, err := readArtifact(artifactPath)
	if err != nil {
		return fmt.Errorf("failed to read artifact: %w", err)
	}

	contractDef := getContractDefinition(artifact, contractName)
	if contractDef == nil {
		return nil // Skip processing if contract definition is not found
	}

	if contractDef.ContractKind != "interface" {
		return nil
	}

	if !strings.HasPrefix(contractName, "I") {
		fail("%s: Interface does not start with 'I'", contractName)
	}

	semver, err := getContractSemver(artifact)
	if err != nil {
		return err
	}

	if semver != "solidity^0.8.0" {
		fail("%s: Interface does not have correct compiler version (MUST be exactly solidity ^0.8.0)", contractName)
	}

	contractBasename := contractName[1:]
	correspondingContractFile := filepath.Join(artifactsDir, contractBasename+".sol", contractBasename+".json")

	if _, err := os.Stat(correspondingContractFile); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	contractArtifact, err := readArtifact(correspondingContractFile)
	if err != nil {
		return fmt.Errorf("failed to read corresponding contract artifact: %w", err)
	}

	interfaceABI := artifact.ABI
	contractABI := contractArtifact.ABI

	normalizedInterfaceABI, err := normalizeABI(interfaceABI)
	if err != nil {
		return fmt.Errorf("failed to normalize interface ABI: %w", err)
	}

	normalizedContractABI, err := normalizeABI(contractABI)
	if err != nil {
		return fmt.Errorf("failed to normalize contract ABI: %w", err)
	}

	match, err := compareABIs(normalizedInterfaceABI, normalizedContractABI)
	if err != nil {
		return fmt.Errorf("failed to compare ABIs: %w", err)
	}
	if !match {
		fail("%s: Differences found in ABI between interface and actual contract", contractName)
	}

	return nil
}

func readArtifact(path string) (*Artifact, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open artifact file: %w", err)
	}
	defer file.Close()

	var artifact Artifact
	if err := json.NewDecoder(file).Decode(&artifact); err != nil {
		return nil, fmt.Errorf("failed to parse artifact file: %w", err)
	}

	return &artifact, nil
}

func getContractDefinition(artifact *Artifact, contractName string) *ContractDefinition {
	for _, node := range artifact.AST.Nodes {
		if node.NodeType == "ContractDefinition" && node.Name == contractName {
			return &node.ContractDefinition
		}
	}
	return nil
}

func getContractSemver(artifact *Artifact) (string, error) {
	for _, node := range artifact.AST.Nodes {
		if node.NodeType == "PragmaDirective" {
			return strings.Join(node.Literals, ""), nil
		}
	}
	return "", errors.New("semver not found")
}

func normalizeABI(abi json.RawMessage) (json.RawMessage, error) {
	var abiData []map[string]interface{}
	if err := json.Unmarshal(abi, &abiData); err != nil {
		return nil, err
	}

	hasConstructor := false
	for i := range abiData {
		normalizeABIItem(abiData[i])
		if abiData[i]["type"] == "constructor" {
			hasConstructor = true
		}
	}

	// Add an empty constructor if it doesn't exist
	if !hasConstructor {
		emptyConstructor := map[string]interface{}{
			"type":            "constructor",
			"stateMutability": "nonpayable",
			"inputs":          []interface{}{},
		}
		abiData = append(abiData, emptyConstructor)
	}

	return json.Marshal(abiData)
}

func normalizeABIItem(item map[string]interface{}) {
	for key, value := range item {
		switch v := value.(type) {
		case string:
			if key == "internalType" {
				item[key] = normalizeInternalType(v)
			}
		case map[string]interface{}:
			normalizeABIItem(v)
		case []interface{}:
			for _, elem := range v {
				if elemMap, ok := elem.(map[string]interface{}); ok {
					normalizeABIItem(elemMap)
				}
			}
		}
	}

	if item["type"] == "function" && item["name"] == "__constructor__" {
		item["type"] = "constructor"
		delete(item, "name")
		delete(item, "outputs")
	}
}

func normalizeInternalType(internalType string) string {
	internalType = strings.ReplaceAll(internalType, "contract I", "contract ")
	internalType = strings.ReplaceAll(internalType, "enum I", "enum ")
	internalType = strings.ReplaceAll(internalType, "struct I", "struct ")
	return internalType
}

func compareABIs(abi1, abi2 json.RawMessage) (bool, error) {
	var data1, data2 []map[string]interface{}

	if err := json.Unmarshal(abi1, &data1); err != nil {
		return false, fmt.Errorf("error unmarshalling first ABI: %w", err)
	}

	if err := json.Unmarshal(abi2, &data2); err != nil {
		return false, fmt.Errorf("error unmarshalling second ABI: %w", err)
	}

	// Sort the ABI data
	sort.Slice(data1, func(i, j int) bool {
		return abiItemLess(data1[i], data1[j])
	})
	sort.Slice(data2, func(i, j int) bool {
		return abiItemLess(data2[i], data2[j])
	})

	// Compare using go-cmp
	diff := cmp.Diff(data1, data2)
	if diff != "" {
		log.Printf("ABI diff: %s", diff)
		return false, nil
	}
	return true, nil
}

func abiItemLess(a, b map[string]interface{}) bool {
	aType := getString(a, "type")
	bType := getString(b, "type")

	if aType != bType {
		return aType < bType
	}

	aName := getString(a, "name")
	bName := getString(b, "name")
	return aName < bName
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func isExcluded(contractName string) bool {
	for _, exclude := range excludeContracts {
		if exclude == contractName {
			return true
		}
	}
	return false
}

func glob(dir string, ext string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == ext {
			out[strings.TrimSuffix(filepath.Base(path), ext)] = path
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
	return out, nil
}
