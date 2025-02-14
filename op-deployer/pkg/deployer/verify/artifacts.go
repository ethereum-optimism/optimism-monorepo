package verify

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
)

type contractArtifact struct {
	ContractName     string
	CompilerVersion  string
	OptimizationUsed bool
	OptimizationRuns int
	EVMVersion       string
	StandardInput    string
	ConstructorArgs  string
}

// Map state.json struct's contract field names to forge artifact names
var contractNameExceptions = map[string]string{
	"OptimismPortalImpl":          "OptimismPortal2",
	"L1StandardBridgeProxy":       "L1ChugSplashProxy",
	"L1CrossDomainMessengerProxy": "ResolvedDelegateProxy",
	"Opcm":                        "OPContractsManager",
}

func getArtifactName(name string) string {
	lookupName := strings.TrimSuffix(name, "Address")

	if artifactName, exists := contractNameExceptions[lookupName]; exists {
		return artifactName
	}

	lookupName = strings.TrimSuffix(lookupName, "Proxy")
	lookupName = strings.TrimSuffix(lookupName, "Impl")
	lookupName = strings.TrimSuffix(lookupName, "Singleton")

	// If it was a proxy and not a special case, return "Proxy"
	if strings.HasSuffix(name, "ProxyAddress") {
		return "Proxy"
	}

	return lookupName
}

func (v *Verifier) getContractArtifact(name string) (*contractArtifact, error) {
	artifactName := getArtifactName(name)
	artifactPath := path.Join(artifactName+".sol", artifactName+".json")

	v.log.Info("Opening artifact", "path", artifactPath, "name", name)
	f, err := v.artifactsFS.Open(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open artifact: %w", err)
	}
	defer f.Close()

	var art foundry.Artifact
	if err := json.NewDecoder(f).Decode(&art); err != nil {
		return nil, fmt.Errorf("failed to decode artifact: %w", err)
	}

	// Add all sources (main contract and dependencies)
	sources := make(map[string]map[string]string)
	for sourcePath, sourceInfo := range art.Metadata.Sources {
		remappedKey := art.SearchRemappings(sourcePath)
		sources[remappedKey] = map[string]string{"content": sourceInfo.Content}
		v.log.Debug("added source contract", "originalPath", sourcePath, "remappedKey", remappedKey)
	}

	var optimizer struct {
		Enabled bool `json:"enabled"`
		Runs    int  `json:"runs"`
	}
	if err := json.Unmarshal(art.Metadata.Settings.Optimizer, &optimizer); err != nil {
		return nil, fmt.Errorf("failed to parse optimizer settings: %w", err)
	}

	standardInput := map[string]interface{}{
		"language": "Solidity",
		"sources":  sources,
		"settings": map[string]interface{}{
			"optimizer": map[string]interface{}{
				"enabled": optimizer.Enabled,
				"runs":    optimizer.Runs,
			},
			"evmVersion": art.Metadata.Settings.EVMVersion,
			"metadata": map[string]interface{}{
				"useLiteralContent": true,
				"bytecodeHash":      "none",
			},
			"outputSelection": map[string]interface{}{
				"*": map[string]interface{}{
					"*": []string{
						"abi",
						"evm.bytecode.object",
						"evm.bytecode.sourceMap",
						"evm.deployedBytecode.object",
						"evm.deployedBytecode.sourceMap",
						"metadata",
					},
				},
			},
		},
	}

	standardInputJSON, err := json.Marshal(standardInput)
	if err != nil {
		return nil, fmt.Errorf("failed to generate standard input: %w", err)
	}

	// Get the contract name from the compilation target
	var contractName string
	for contractFile, name := range art.Metadata.Settings.CompilationTarget {
		contractName = contractFile + ":" + name
		break
	}
	v.log.Info("contractName", "name", contractName)

	constructorArgs, err := v.getEncodedConstructorArgs(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get constructor args: %w", err)
	}
	v.log.Debug("constructorArgs", "args", constructorArgs)

	return &contractArtifact{
		ContractName:     contractName,
		CompilerVersion:  art.Metadata.Compiler.Version,
		OptimizationUsed: optimizer.Enabled,
		OptimizationRuns: optimizer.Runs,
		EVMVersion:       art.Metadata.Settings.EVMVersion,
		StandardInput:    string(standardInputJSON),
		ConstructorArgs:  constructorArgs,
	}, nil
}
