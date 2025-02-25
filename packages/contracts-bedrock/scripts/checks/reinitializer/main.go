package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

func main() {
	if _, err := common.ProcessFilesGlob(
		[]string{"forge-artifacts/**/*.json"},
		[]string{},
		processFile,
	); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func processFile(path string) (*common.Void, []error) {
	artifact, err := common.ReadForgeArtifact(path)
	if err != nil {
		return nil, []error{err}
	}

	if err := checkArtifact(artifact); err != nil {
		return nil, []error{err}
	}

	return nil, nil
}

func checkArtifact(artifact *solc.ForgeArtifact) error {
	// Skip interfaces.
	if strings.HasPrefix(artifact.Ast.AbsolutePath, "interfaces/") {
		return nil
	}

	// Skip if we have no upgrade function.
	upgradeFn := getFunctionByName(artifact, "upgrade")
	if upgradeFn == nil {
		return nil
	}

	// We can have an upgrade function without an initialize function.
	initializeFn := getFunctionByName(artifact, "initialize")
	if initializeFn == nil {
		return nil
	}

	// Grab the reinitializer value from the upgrade function.
	upgradeFnReinitializerValue, err := getReinitializerValue(upgradeFn)
	if err != nil {
		return fmt.Errorf("error getting reinitializer value from upgrade function: %w", err)
	}

	// Grab the reinitializer value from the initialize function.
	initializeFnReinitializerValue, err := getReinitializerValue(initializeFn)
	if err != nil {
		return fmt.Errorf("error getting reinitializer value from initialize function: %w", err)
	}

	// If the reinitializer values are different, return an error.
	if upgradeFnReinitializerValue != initializeFnReinitializerValue {
		return fmt.Errorf("upgrade function and initialize function have different reinitializer values")
	}

	return nil
}

func getContractDefinition(artifact *solc.ForgeArtifact) *solc.AstNode {
	for _, node := range artifact.Ast.Nodes {
		if node.NodeType == "ContractDefinition" {
			return &node
		}
	}
	return nil
}

func getFunctionByName(artifact *solc.ForgeArtifact, name string) *solc.AstNode {
	contract := getContractDefinition(artifact)
	if contract == nil {
		return nil
	}
	for _, node := range contract.Nodes {
		if node.NodeType == "FunctionDefinition" {
			if node.Name == name {
				return &node
			}
		}
	}
	return nil
}

func getReinitializerValue(node *solc.AstNode) (uint64, error) {
	if node.Modifiers == nil {
		return 0, fmt.Errorf("no modifiers found")
	}

	for _, modifier := range node.Modifiers {
		if modifier.ModifierName.Name == "reinitializer" {
			if modifier.Arguments[0].Kind == "functionCall" {
				if modifier.Arguments[0].Expression.Name == "initVersion" {
					return math.MaxUint64, nil // uint64 max representing initVersion call
				} else {
					return 0, fmt.Errorf("reinitializer value is not a call to initVersion")
				}
			} else {
				valStr, ok := modifier.Arguments[0].Value.(string)
				if !ok {
					return 0, fmt.Errorf("reinitializer value is not a string")
				}
				val, err := strconv.Atoi(valStr)
				if err != nil {
					return 0, fmt.Errorf("reinitializer value is not an integer")
				}
				return uint64(val), nil
			}
		}
	}

	return 0, fmt.Errorf("reinitializer modifier not found")
}
