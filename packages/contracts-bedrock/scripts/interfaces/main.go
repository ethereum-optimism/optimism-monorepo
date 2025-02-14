package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GenerateSolidityInterface(contractName string, astData ContractData) string {
	if len(astData.Functions) == 0 && len(astData.Events) == 0 && len(astData.Errors) == 0 &&
		len(astData.Types) == 0 && len(astData.Structs) == 0 && len(astData.Enums) == 0 &&
		len(astData.Imports) == 0 && len(astData.Inherited) == 0 && len(astData.OutEnums) == 0 && len(astData.OutStructs) == 0 || contractName == "L1ChugSplashProxy" {
		return ""
	}

	seenImports := make(map[string]bool)
	seenTypes := make(map[string]bool)
	seenStructs := make(map[string]bool)
	seenEnums := make(map[string]bool)
	seenErrors := make(map[string]bool)
	seenEvents := make(map[string]bool)
	seenFunctions := make(map[string]bool)

	usedTypes := map[string]bool{}
	localTypes := map[string]bool{}

	// Collect used types from all components
	collectUsedTypes := func(typeString string) {
		if typeString == "" {
			return
		}
		parts := strings.Fields(typeString)
		if len(parts) > 1 {
			if strings.Contains(parts[1], ".") {
				part := strings.Split(parts[1], ".")
				usedTypes[part[0]] = true
				return
			}
			usedTypes[parts[1]] = true
		}
	}

	if contractName == "L2ToL2CrossDomainMessenger" {
		print("")
	}

	aliasMapping := map[string]string{}

	for _, importNode := range astData.Imports {
		if importNode.NodeType == "ImportDirective" {
			for _, alias := range importNode.SymbolAliases {
				if alias.Local != "" {
					aliasMapping[alias.Foreign.Name] = alias.Local
				} else {
					aliasMapping[alias.Foreign.Name] = alias.Foreign.Name
				}
			}
		}
	}

	// Analyze Enums
	for _, enumDef := range astData.Enums {
		localTypes[enumDef.Name] = true
	}

	// Analyze Structs
	for _, structDef := range astData.Structs {
		localTypes[structDef.Name] = true
		for _, member := range structDef.Members {
			collectUsedTypes(member.Type)
		}
	}

	// Analyze Inheritance
	for _, inherited := range astData.Inherited {
		usedTypes[inherited.BaseName.Name] = true
	}

	// Analyze Functions
	for _, fn := range astData.Functions {
		if fn.Parameters != nil {
			for _, param := range fn.Parameters.Parameters {
				collectUsedTypes(param.TypeDescriptions.TypeString)
			}
		}
		if fn.ReturnParameters != nil {
			for _, ret := range fn.ReturnParameters.Parameters {
				collectUsedTypes(ret.TypeDescriptions.TypeString)
			}
		}
	}

	// Analyze Events
	for _, event := range astData.Events {
		if event.Parameters != nil {
			for _, param := range event.Parameters.Parameters {
				collectUsedTypes(param.TypeDescriptions.TypeString)
			}
		}
	}

	// Analyze Errors
	for _, errDef := range astData.Errors {
		if errDef.Parameters != nil {
			for _, param := range errDef.Parameters.Parameters {
				collectUsedTypes(param.TypeDescriptions.TypeString)
			}
		}
	}

	// Add SPDX license and pragma version
	var builder strings.Builder
	builder.WriteString("// SPDX-License-Identifier: MIT\n")

	//builder.WriteString(fmt.Sprintf("pragma solidity %s;\n\n", astData.Version))
	builder.WriteString(fmt.Sprintf("pragma solidity ^0.8.0;\n\n"))

	// Add imports
	for _, importNode := range astData.Imports {
		if importNode.NodeType == "ImportDirective" && isImportUsed(importNode, usedTypes) {
			importDefinition := GenerateImportDefinition(importNode)
			if importDefinition != "" && !seenImports[importDefinition] {
				builder.WriteString(fmt.Sprintf("%s\n", importDefinition))
				seenImports[importDefinition] = true
			}
		}
	}

	// Add out structs
	for _, structDef := range astData.OutStructs {
		structDefinition := GenerateStructDefinition(structDef, aliasMapping, contractName, localTypes)
		if !seenStructs[structDefinition] {
			builder.WriteString(fmt.Sprintf("\n    %s", structDefinition))
			seenStructs[structDefinition] = true
		}
	}

	// Add out enums
	for _, enumDef := range astData.OutEnums {
		enumSignature := GenerateEnumSignature(enumDef, aliasMapping)
		if !seenEnums[enumSignature] {
			builder.WriteString(fmt.Sprintf("\n    %s", enumSignature))
			seenEnums[enumSignature] = true
		}
	}

	// Add user-defined types
	for _, typeDef := range astData.Types {
		typeDefinition := GenerateTypeDefinition(typeDef)
		if !seenTypes[typeDefinition] {
			builder.WriteString(fmt.Sprintf("\n%s", typeDefinition))
			seenTypes[typeDefinition] = true
		}
	}

	// Start the interface declaration
	builder.WriteString(GenerateInterfaceDeclaration(contractName, astData.Inherited))

	// Add structs
	for _, structDef := range astData.Structs {
		structDefinition := GenerateStructDefinition(structDef, aliasMapping, contractName, localTypes)
		if !seenStructs[structDefinition] {
			builder.WriteString(fmt.Sprintf("\n    %s", structDefinition))
			seenStructs[structDefinition] = true
		}
	}

	// Add enums
	for _, enumDef := range astData.Enums {
		enumSignature := GenerateEnumSignature(enumDef, aliasMapping)
		if !seenEnums[enumSignature] {
			builder.WriteString(fmt.Sprintf("\n    %s", enumSignature))
			seenEnums[enumSignature] = true
		}
	}

	// Add errors
	for _, errDef := range astData.Errors {
		errorDefinition := GenerateErrorDefinition(errDef, aliasMapping)
		if !seenErrors[errorDefinition] {
			builder.WriteString(fmt.Sprintf("\n    %s", errorDefinition))
			seenErrors[errorDefinition] = true
		}
	}

	// Add events
	for _, event := range astData.Events {
		eventDefinition := GenerateEventDefinition(event, aliasMapping, contractName, localTypes)
		if !seenEvents[eventDefinition] {
			builder.WriteString(fmt.Sprintf("\n    %s", eventDefinition))
			seenEvents[eventDefinition] = true
		}
	}

	// Add public function signatures (including public variable getters)
	for _, fn := range astData.Functions {
		functionSignature := GenerateFunctionSignature(fn, aliasMapping, contractName, localTypes)
		if !seenFunctions[functionSignature] {
			builder.WriteString(fmt.Sprintf("\n    %s", functionSignature))
			seenFunctions[functionSignature] = true
		}
	}

	builder.WriteString("\n}\n")

	return builder.String()
}

func main() {
	err := run()
	if err != nil {
		return
	}
}

func run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	sourceDir := filepath.Join(cwd, "packages/contracts-bedrock/src")
	interfaceDir := filepath.Join(cwd, "packages/contracts-bedrock/interfaces")

	tree, err := buildDirectoryTree(sourceDir)
	if err != nil {
		return err
	}

	processFilesAndGenerateInterfaces(tree, sourceDir, interfaceDir)

	return nil
}

func processFilesAndGenerateInterfaces(tree *TreeNode, sourceDir, targetBaseDir string) {
	processFiles(tree, sourceDir, func(fileNode *TreeNode, parentDir string) {
		name := strings.TrimSuffix(fileNode.Name, ".sol")

		if name == "CannonTypes" || name == "LibUDT" || strings.HasPrefix(name, "I") {
			return
		}

		artifact := mapArtifact(name, "")
		if artifact == nil {
			fmt.Printf("Artifact not found for: %s\n", name)
			return
		}

		data := ExtractASTData(artifact.Ast, false, "", map[string]bool{})

		// Generate the Solidity interface as a string
		interfaceContent := GenerateSolidityInterface(name, data)

		// Determine the relative path from the source directory to the current parent directory
		relativeDir, err := filepath.Rel(sourceDir, parentDir)
		if err != nil {
			fmt.Printf("Error determining relative path for %s: %v\n", parentDir, err)
			return
		}
		cleanedRelativeDir := strings.TrimPrefix(relativeDir, "src/")

		// Combine the target base directory with the relative path
		targetDir := filepath.Join(targetBaseDir, cleanedRelativeDir)

		// Ensure the target directory exists
		err = os.MkdirAll(targetDir, os.ModePerm)
		if err != nil {
			fmt.Printf("Error creating directory %s: %v\n", targetDir, err)
			return
		}

		// Define the output file path
		outputFile := filepath.Join(targetDir, "I"+name+".sol")

		if interfaceContent != "" {
			// Write the interface content to the file
			err = os.WriteFile(outputFile, []byte(interfaceContent), 0644)
			if err != nil {
				fmt.Printf("Error writing file %s: %v\n", outputFile, err)
				return
			}
		}
	})
}

func processFiles(node *TreeNode, parentDir string, processor FileProcessor) {
	if node.IsDir && (strings.EqualFold(node.Name, "lib") || strings.EqualFold(node.Name, "libraries")) {
		fmt.Printf("Skipping directory: %s\n", filepath.Join(parentDir, node.Name))
		return
	}

	if !node.IsDir {
		processor(node, parentDir)
		return
	}

	for _, child := range node.Children {
		childPath := filepath.Join(parentDir, node.Name)
		processFiles(child, childPath, processor)
	}
}
