package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
)

func ExtractASTData(ast solc.Ast, inherited bool, version string, processedContracts map[string]bool) ContractData {
	var contractData ContractData

	for _, node := range ast.Nodes {
		switch node.NodeType {
		case "PragmaDirective":
			if !inherited {
				if len(node.Literals) < 3 {
					continue
				}
				version = fmt.Sprint(node.Literals[1], node.Literals[2])
				contractData.Version = version
			}
		case "ImportDirective":
			contractData.Imports = append(contractData.Imports, node)
		case "ContractDefinition":
			// Use BaseName.Name as the unique identifier
			contractKey := node.Name
			if processedContracts[contractKey] {
				continue
			}
			processedContracts[contractKey] = true

			// Process the current contract
			extractedData := ExtractContractASTData(node, inherited)
			contractData.Functions = append(contractData.Functions, extractedData.Functions...)
			contractData.Events = append(contractData.Events, extractedData.Events...)
			contractData.Errors = append(contractData.Errors, extractedData.Errors...)
			contractData.Types = append(contractData.Types, extractedData.Types...)
			contractData.Structs = append(contractData.Structs, extractedData.Structs...)
			contractData.Enums = append(contractData.Enums, extractedData.Enums...)
			contractData.Imports = append(contractData.Imports, extractedData.Imports...)
			contractData.Inherited = append(contractData.Inherited, extractedData.Inherited...)

			// Process base contracts
			for _, baseContract := range node.BaseContracts {
				baseContractKey := baseContract.BaseName.Name
				if processedContracts[baseContractKey] {
					continue
				}
				//  || baseContract.BaseName.Name == "TransientReentrancyAware"
				if baseContract.BaseName.Name == "IGasToken" || baseContract.BaseName.Name == "BaseGuard" || baseContract.BaseName.Name == "TransientReentrancyAware" {
					continue
				}

				artifact := mapArtifact(baseContract.BaseName.Name, node.CanonicalName)
				baseData := ExtractASTData(artifact.Ast, true, version, processedContracts)

				contractData.Functions = append(contractData.Functions, baseData.Functions...)
				contractData.Events = append(contractData.Events, baseData.Events...)
				contractData.Errors = append(contractData.Errors, baseData.Errors...)
				contractData.Types = append(contractData.Types, baseData.Types...)
				contractData.Structs = append(contractData.Structs, baseData.Structs...)
				contractData.Enums = append(contractData.Enums, baseData.Enums...)
				if baseContract.NodeType == "InheritanceSpecifier" {
					if strings.HasPrefix(baseContract.BaseName.Name, "I") && unicode.IsUpper(rune(baseContract.BaseName.Name[1])) {
						contractData.Inherited = append(contractData.Inherited, baseContract)
					}
				}
			}
		case "ErrorDefinition":
			contractData.Errors = append(contractData.Errors, node)
		case "UserDefinedValueTypeDefinition":
			contractData.Types = append(contractData.Types, node)
		case "StructDefinition":
			structDef := StructDefinition{
				Name: node.Name,
			}

			for _, member := range node.Members {
				memberType := member.TypeDescriptions.TypeString

				idx := strings.Index(memberType, ".")
				if idx != -1 {

					memberType = memberType[idx+1:]
				}

				memberName := member.Name
				structDef.Members = append(structDef.Members, StructMember{
					Name: memberName,
					Type: memberType,
				})
			}

			contractData.OutStructs = append(contractData.OutStructs, structDef)
		case "EnumDefinition":
			enumDef := EnumDefinition{
				Name: node.Name,
			}

			for _, member := range node.Members {
				enumDef.Members = append(enumDef.Members, EnumMember{
					Name: member.Name,
				})
			}

			contractData.OutEnums = append(contractData.OutEnums, enumDef)
		default:
			break
		}
	}

	return contractData
}

func ExtractContractASTData(node solc.AstNode, inherited bool) ContractData {
	var data ContractData
	for _, innerNode := range node.Nodes {
		switch innerNode.NodeType {
		case "FunctionDefinition":
			if inherited && innerNode.Kind == "constructor" {
				continue
			}
			if innerNode.Visibility == "public" || innerNode.Visibility == "external" {
				data.Functions = append(data.Functions, innerNode)
			}
		case "VariableDeclaration":
			if innerNode.Visibility == "public" {
				data.Functions = append(data.Functions, innerNode)
			}

		case "EventDefinition":
			data.Events = append(data.Events, innerNode)
		case "ErrorDefinition":
			data.Errors = append(data.Errors, innerNode)
		case "StructDefinition":
			structDef := StructDefinition{
				Name: innerNode.Name,
			}

			for _, member := range innerNode.Members {
				memberType := member.TypeDescriptions.TypeString

				idx := strings.Index(memberType, ".")
				if idx != -1 {

					memberType = memberType[idx+1:]
				}

				memberName := member.Name
				structDef.Members = append(structDef.Members, StructMember{
					Name: memberName,
					Type: memberType,
				})
			}

			data.Structs = append(data.Structs, structDef)
		case "EnumDefinition":
			enumDef := EnumDefinition{
				Name: innerNode.Name,
			}

			for _, member := range innerNode.Members {
				enumDef.Members = append(enumDef.Members, EnumMember{
					Name: member.Name,
				})
			}

			data.Enums = append(data.Enums, enumDef)
		case "UserDefinedValueTypeDefinition":
			data.Types = append(data.Types, innerNode)
		}
	}
	return data
}
