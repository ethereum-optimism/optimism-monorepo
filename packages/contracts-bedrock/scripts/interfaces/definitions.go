package main

import (
	"fmt"
	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"strings"
)

func GenerateFunctionSignature(fn solc.AstNode) string {
	if fn.Kind == "constructor" {
		fn.Name = "__constructor__"
	}
	signature := "function " + fn.Name + "("

	if fn.Parameters != nil {
		params := []string{}
		for _, param := range fn.Parameters.Parameters {
			paramType := param.TypeDescriptions.TypeString
			params = append(params, paramType)
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
			returns = append(returns, returnType)
		}
		signature += " returns (" + strings.Join(returns, ", ") + ")"
	}

	return signature
}

func GenerateEventDefinition(event solc.AstNode) string {
	eventSignature := "event " + event.Name + "("

	if event.Parameters != nil {
		params := []string{}
		for _, param := range event.Parameters.Parameters {
			paramType := param.TypeDescriptions.TypeString
			paramName := param.Name
			if paramName == "" {
				paramName = "_"
			}
			isIndexed := ""
			if param.StorageLocation == "indexed" {
				isIndexed = " indexed"
			}
			params = append(params, fmt.Sprintf("%s%s %s", paramType, isIndexed, paramName))
		}
		eventSignature += strings.Join(params, ", ")
	}

	eventSignature += ");"
	return eventSignature
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
			builder.WriteString(", ")
		}
		builder.WriteString(member.Name)
	}

	builder.WriteString("\n}\n")

	return builder.String()
}
