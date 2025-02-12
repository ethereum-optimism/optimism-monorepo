package main

import "github.com/ethereum-optimism/optimism/op-chain-ops/solc"

type TreeNode struct {
	Name     string      `json:"name"`
	IsDir    bool        `json:"isDir"`
	Children []*TreeNode `json:"children,omitempty"`
}

type FileProcessor func(fileNode *TreeNode, parentDir string)

type ContractData struct {
	Imports    []solc.AstNode
	Functions  []solc.AstNode
	Events     []solc.AstNode
	Errors     []solc.AstNode
	Types      []solc.AstNode
	Inherited  []solc.AstBaseContract
	OutStructs []StructDefinition
	Structs    []StructDefinition
	OutEnums   []EnumDefinition
	Enums      []EnumDefinition
	Version    string
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
