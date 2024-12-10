package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

func main() {
	files, err := common.FindFiles([]string{"forge-artifacts/**/*.json"}, nil)
	if err != nil {
		fmt.Printf("Error finding files: %v\n", err)
		return
	}

	for _, path := range files {
		// skip interop labeled contracts since they're to be merged with the main contract (which they inherit) before deployment
		if strings.HasSuffix(filepath.Base(path), "Interop.json") {
			continue
		}

		artifact, err := common.ReadForgeArtifact(path)
		if err != nil {
			fmt.Printf("Error reading artifact %s: %v\n", path, err)
			continue
		}

		if checkIfProxiedAndNotAPredeploy(&artifact.Ast) {
			if !checkIfInitializerWasDisabledInConstructor(&artifact.Ast) {
				fmt.Printf("Proxied contract %s has an initializer that was not disabled in the constructor\n", path)
				os.Exit(1)
			}

			if !checkIfInitializeHasExternalAndInitializerModifiers(&artifact.Ast) {
				fmt.Printf("Proxied contract %s has an initialize function that does not have external and/or initializer modifiers\n", path)
				os.Exit(1)
			}

			fmt.Println("✅", strings.TrimSuffix(filepath.Base(path), ".json"))
		}
	}
}

func checkIfProxiedAndNotAPredeploy(ast *solc.Ast) bool {
	for _, node := range ast.Nodes {
		if node.NodeType == "ContractDefinition" {
			if node.Documentation == nil {
				continue
			}

			doc, ok := node.Documentation.(map[string]interface{})
			if !ok {
				fmt.Printf("Documentation is not of type string: %v\n", node.Documentation)
				os.Exit(1)
			}

			var text string = doc["text"].(string)
			if strings.Contains(text, "@custom:proxied true") && !strings.Contains(text, "@custom:predeploy") {
				return true
			}
		}
	}
	return false
}

func checkIfInitializerWasDisabledInConstructor(ast *solc.Ast) bool {
	for _, node := range ast.Nodes {
		if node.NodeType == "ContractDefinition" {
			for _, _node_ := range node.Nodes {
				if _node_.NodeType == "FunctionDefinition" && _node_.Kind == "constructor" {
					for _, statement := range _node_.Body.Statements {
						// The _disableInitializers() call must be in the top level block of the constructor and not nested within any other blocks e.g a block, conditional statement, loop, etc.
						if statement.NodeType == "ExpressionStatement" && statement.Expression.NodeType == "FunctionCall" && len(statement.Expression.Expression.ArgumentTypes) == 0 {
							return true
						}
					}
					// return early
					return false
				}
			}
		}
	}

	return false
}

func checkIfInitializeHasExternalAndInitializerModifiers(ast *solc.Ast) bool {
	// has external and initializer modifiers and there's only one initialize function
	var pass bool = false
	var foundInitialize bool = false

	for _, node := range ast.Nodes {
		if node.NodeType == "ContractDefinition" {
			for _, _node_ := range node.Nodes {
				if _node_.NodeType == "FunctionDefinition" && _node_.Name == "initialize" {
					if foundInitialize {
						return false
					}

					if _node_.Visibility == "external" {
						for _, modifier := range _node_.Modifiers {
							if modifier.ModifierName.Name == "initializer" {
								pass = true
								foundInitialize = true
							}
						}
					}
				}
			}
		}
	}

	return pass
}
