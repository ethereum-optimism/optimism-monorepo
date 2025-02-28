package main

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
)

func TestGetContractDefinition(t *testing.T) {
	tests := []struct {
		name     string
		artifact *solc.ForgeArtifact
		want     *solc.AstNode
	}{
		{
			name: "Find contract",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{NodeType: "ContractDefinition", Name: "Test"},
					},
				},
			},
			want: &solc.AstNode{NodeType: "ContractDefinition", Name: "Test"},
		},
		{
			name: "No contract",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{NodeType: "PragmaDirective"},
					},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getContractDefinition(tt.artifact)
			if (got == nil) != (tt.want == nil) {
				t.Errorf("getContractDefinition() = %v, want %v", got, tt.want)
			}
			if got != nil && got.NodeType != tt.want.NodeType {
				t.Errorf("getContractDefinition() NodeType = %v, want %v", got.NodeType, tt.want.NodeType)
			}
		})
	}
}

func TestGetFunctionByName(t *testing.T) {
	tests := []struct {
		name         string
		artifact     *solc.ForgeArtifact
		functionName string
		want         *solc.AstNode
	}{
		{
			name: "Find function",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{NodeType: "FunctionDefinition", Name: "initialize"},
								{NodeType: "FunctionDefinition", Name: "upgrade"},
							},
						},
					},
				},
			},
			functionName: "initialize",
			want:         &solc.AstNode{NodeType: "FunctionDefinition", Name: "initialize"},
		},
		{
			name: "Function not found",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{NodeType: "FunctionDefinition", Name: "otherFunction"},
							},
						},
					},
				},
			},
			functionName: "initialize",
			want:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFunctionByName(tt.artifact, tt.functionName)
			if (got == nil) != (tt.want == nil) {
				t.Errorf("getFunctionByName() = %v, want %v", got, tt.want)
			}
			if got != nil && got.Name != tt.want.Name {
				t.Errorf("getFunctionByName() Name = %v, want %v", got.Name, tt.want.Name)
			}
		})
	}
}

func TestGetReinitializerValue(t *testing.T) {
	tests := []struct {
		name    string
		node    *solc.AstNode
		want    uint64
		wantErr bool
	}{
		{
			name: "Valid reinitializer",
			node: &solc.AstNode{
				Modifiers: []solc.AstNode{
					{
						ModifierName: &solc.Expression{Name: "reinitializer"},
						Arguments:    []solc.Expression{{Value: "2"}},
					},
				},
			},
			want:    2,
			wantErr: false,
		},
		{
			name:    "No modifiers",
			node:    &solc.AstNode{},
			want:    0,
			wantErr: true,
		},
		{
			name: "No reinitializer modifier",
			node: &solc.AstNode{
				Modifiers: []solc.AstNode{
					{ModifierName: &solc.Expression{Name: "onlyOwner"}},
				},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "Invalid reinitializer value",
			node: &solc.AstNode{
				Modifiers: []solc.AstNode{
					{
						ModifierName: &solc.Expression{Name: "reinitializer"},
						Arguments:    []solc.Expression{{Value: "invalid"}},
					},
				},
			},
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getReinitializerValue(tt.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("getReinitializerValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getReinitializerValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckArtifact(t *testing.T) {
	tests := []struct {
		name     string
		artifact *solc.ForgeArtifact
		wantErr  bool
	}{
		{
			name: "Matching reinitializer values",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "initialize",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "2"}},
										},
									},
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "upgrade",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Mismatched reinitializer values",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "initialize",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "1"}},
										},
									},
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "upgrade",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "No upgrade function",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "initialize",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "1"}},
										},
									},
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "someOtherFunction",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "No initialize function",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "someOtherFunction",
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "upgrade",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Error getting reinitializer value from upgrade function - no modifiers",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "initialize",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "1"}},
										},
									},
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "upgrade",
									// No modifiers
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Error getting reinitializer value from initialize function - no modifiers",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "initialize",
									// No modifiers
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "upgrade",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Error getting reinitializer value - no reinitializer modifier",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "initialize",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "someOtherModifier"},
											Arguments:    []solc.Expression{{Value: "1"}},
										},
									},
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "upgrade",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Error getting reinitializer value - non-integer value",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "initialize",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "not-an-integer"}},
										},
									},
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "upgrade",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Error getting reinitializer value - non-string value",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "ContractDefinition",
							Nodes: []solc.AstNode{
								{
									NodeType: "FunctionDefinition",
									Name:     "initialize",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: 2}}, // Integer instead of string
										},
									},
								},
								{
									NodeType: "FunctionDefinition",
									Name:     "upgrade",
									Modifiers: []solc.AstNode{
										{
											ModifierName: &solc.Expression{Name: "reinitializer"},
											Arguments:    []solc.Expression{{Value: "2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "No contract definition",
			artifact: &solc.ForgeArtifact{
				Ast: solc.Ast{
					Nodes: []solc.AstNode{
						{
							NodeType: "SomeOtherNodeType", // Not a ContractDefinition
							Nodes:    []solc.AstNode{},
						},
					},
				},
			},
			wantErr: false, // Should return nil without error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkArtifact(tt.artifact)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkArtifact() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
