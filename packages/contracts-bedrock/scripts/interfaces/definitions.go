package main

import (
	"fmt"
	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"strings"
)

func GenerateTypeDefinition(udtype solc.AstNode) string {
	return fmt.Sprintf("type %s is %s;", udtype.Name, udtype.UnderlyingType.Name)
}

func GenerateFunctionSignature(fn solc.AstNode, abi abi.ABI) string {
	signature := "function "

	if fn.NodeType == "VariableDeclaration" {
		signature += fn.Name + "() external view"

		if fn.TypeDescriptions != nil {
			var returnType = fn.TypeDescriptions.TypeString
			if fn.StorageLocation == "memory" || fn.StorageLocation == "calldata" {
				returnType += " " + fn.StorageLocation
			}

			signature += " returns (" + returnType + ")"
		}

		signature += ";"
		return signature
	}

	if fn.Kind == "constructor" {
		fn.Name = "__constructor__"
	}
	signature += fn.Name + "("

	if fn.Parameters != nil {
		params := []string{}
		for _, param := range fn.Parameters.Parameters {
			paramType := param.TypeDescriptions.TypeString
			paramName := param.Name
			if paramName == "" {
				paramName = "_"
			}

			if param.StorageLocation == "memory" || param.StorageLocation == "calldata" {
				paramType += " " + param.StorageLocation
			}

			params = append(params, fmt.Sprintf("%s %s", paramType, paramName))
		}
		signature += strings.Join(params, ", ")
	}

	signature += ") external"
	if fn.StateMutability == "view" || fn.StateMutability == "pure" {
		signature += " " + fn.StateMutability
	}

	if fn.ReturnParameters != nil && len(fn.ReturnParameters.Parameters) > 0 {
		var returns []string
		for _, ret := range fn.ReturnParameters.Parameters {
			returnType := ret.TypeDescriptions.TypeString

			if ret.StorageLocation == "memory" || ret.StorageLocation == "calldata" {
				returnType += " " + ret.StorageLocation
			}

			returns = append(returns, returnType)
		}
		signature += " returns (" + strings.Join(returns, ", ") + ")"
	}

	signature += ";"
	return signature
}

func GenerateEventDefinition(event solc.AstNode) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("event %s(", event.Name))

	if event.Parameters != nil {
		params := []string{}
		for _, param := range event.Parameters.Parameters {
			paramType := param.TypeDescriptions.TypeString
			paramName := param.Name
			if paramName == "" {
				paramName = "_"
			}

			if strings.HasPrefix(paramType, "enum ") {
				paramType = paramType[strings.LastIndex(paramType, ".")+1:]
			}

			params = append(params, fmt.Sprintf("%s %s", paramType, paramName))
		}
		builder.WriteString(strings.Join(params, ", "))
	}

	// Close the event definition
	builder.WriteString(");")

	return builder.String()
}

func GenerateErrorDefinition(errorDef solc.AstNode) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("error %s(", errorDef.Name))

	if errorDef.Parameters != nil && len(errorDef.Parameters.Parameters) > 0 {
		for i, param := range errorDef.Parameters.Parameters {
			if i > 0 {
				builder.WriteString(", ")
			}
			paramType := param.TypeDescriptions.TypeString
			paramName := param.Name
			if paramName == "" {
				paramName = "_"
			}
			builder.WriteString(fmt.Sprintf("%s %s", paramType, paramName))
		}
	}

	builder.WriteString(");")

	return builder.String()
}

func GenerateStructDefinition(structDef StructDefinition) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("struct %s {\n", structDef.Name))

	for _, member := range structDef.Members {
		builder.WriteString(fmt.Sprintf("    %s %s;\n", member.Type, member.Name))
	}

	builder.WriteString("}\n")

	return builder.String()
}

func GenerateEnumSignature(enumDef EnumDefinition) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("enum %s {\n", enumDef.Name))

	for i, member := range enumDef.Members {
		if i > 0 {
			builder.WriteString(",\n")
		}
		builder.WriteString(fmt.Sprintf("\t\t%s", member.Name))
	}

	builder.WriteString("\n\t}\n")

	return builder.String()
}
