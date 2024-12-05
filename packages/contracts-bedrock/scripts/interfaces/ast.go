package main

import (
	"fmt"
	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

type ContractData struct {
	Functions []solc.AstNode
	Events    []solc.AstNode
	Errors    []solc.AstNode
	Types     []solc.AstNode
	Structs   []StructDefinition
	Enums     []EnumDefinition
}

type StructDefinition struct {
	Name    string
	Members []StructMember
}

type StructMember struct {
	Name string
	Type string
}

type EnumDefinition struct {
	Name    string
	Members []EnumMember
}

type EnumMember struct {
	Name string
}

func ExtractASTData(ast solc.Ast) ContractData {
	var contractData ContractData

	for _, node := range ast.Nodes {
		if node.NodeType == "ContractDefinition" {
			contractData = ExtractContractASTData(node)
			for i := 0; i < len(node.BaseContracts); i++ {
				artifact, _ := common.ReadForgeArtifact(fmt.Sprintf(
					"packages/contracts-bedrock/scripts/interfaces/mockcontracts/forge-artifacts/%s.sol/%s.json",
					node.BaseContracts[i].BaseName.Name,
					node.BaseContracts[i].BaseName.Name,
				))
				data := ExtractASTData(artifact.Ast)
				contractData.Functions = append(contractData.Functions, data.Functions...)
				contractData.Events = append(contractData.Events, data.Events...)
				contractData.Errors = append(contractData.Errors, data.Errors...)
				contractData.Types = append(contractData.Types, data.Types...)
				contractData.Structs = append(contractData.Structs, data.Structs...)
				contractData.Enums = append(contractData.Enums, data.Enums...)
			}
		}
	}

	return contractData
}

func ExtractContractASTData(node solc.AstNode) ContractData {
	var data ContractData
	for _, innerNode := range node.Nodes {
		switch innerNode.NodeType {
		case "FunctionDefinition":
			if (innerNode.Visibility == "public" || innerNode.Visibility == "external") && innerNode.Kind != "receive" {
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
