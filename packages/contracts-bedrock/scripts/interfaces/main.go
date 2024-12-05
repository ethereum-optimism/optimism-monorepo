package main

import (
	"fmt"
	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
	"strings"
)

func GenerateSolidityInterface(
	contractName string,
	publicFunctions []solc.AstNode,
	events []solc.AstNode,
	structs []StructDefinition,
) string {
	var builder strings.Builder

	//TODO: PEP What version here?
	builder.WriteString(fmt.Sprintf("pragma solidity ^0.8.0;\n\n"))
	builder.WriteString(fmt.Sprintf("interface I%s {\n", contractName))

	for _, structDef := range structs {
		structDefinition := GenerateStructDefinition(structDef)
		builder.WriteString(fmt.Sprintf("\n    %s", structDefinition))
	}

	for _, event := range events {
		eventDefinition := GenerateEventDefinition(event)
		builder.WriteString(fmt.Sprintf("\n    %s", eventDefinition))
	}

	for _, fn := range publicFunctions {
		functionSignature := GenerateFunctionSignature(fn)
		builder.WriteString(fmt.Sprintf("\n    %s;", functionSignature))
	}

	builder.WriteString("\n}\n")

	return builder.String()
}

func main() {
	artifact, _ := common.ReadForgeArtifact("packages/contracts-bedrock/scripts/interfaces/mockcontracts/forge-artifacts/A.sol/A.json")

	astData := ExtractASTData(artifact.Ast)

	for i := 0; i < len(astData.Types); i++ {
		println(GenerateTypeDefinition(astData.Types[i]))
	}

	for i := 0; i < len(astData.Structs); i++ {
		println(GenerateStructDefinition(astData.Structs[i]))
	}

	for i := 0; i < len(astData.Errors); i++ {
		println(GenerateErrorDefinition(astData.Errors[i]))
	}

	for i := 0; i < len(astData.Events); i++ {
		println(GenerateEventDefinition(astData.Events[i]))
	}

	for i := 0; i < len(astData.Functions); i++ {
		println(GenerateFunctionSignature(astData.Functions[i]))
	}

	for i := 0; i < len(astData.Enums); i++ {
		println(GenerateEnumSignature(astData.Enums[i]))
	}
}
