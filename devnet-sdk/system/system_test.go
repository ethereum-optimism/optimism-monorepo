package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/devnet-sdk/shell/env"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSystemFromEnv(t *testing.T) {
	// Create a temporary devnet file
	tempDir := t.TempDir()
	devnetFile := filepath.Join(tempDir, "devnet.json")

	devnet := &descriptors.DevnetEnvironment{
		L1: &descriptors.Chain{
			ID: "1",
			Nodes: []descriptors.Node{{
				Services: map[string]descriptors.Service{
					"el": {
						Name: "geth",
						Endpoints: descriptors.EndpointMap{
							"rpc": descriptors.PortInfo{
								Host: "localhost",
								Port: 8545,
							},
						},
					},
				},
			}},
			Wallets: descriptors.WalletMap{
				"default": descriptors.Wallet{
					Address:    "0x123",
					PrivateKey: "0xabc",
				},
			},
		},
		L2: []*descriptors.Chain{{
			ID: "2",
			Nodes: []descriptors.Node{{
				Services: map[string]descriptors.Service{
					"el": {
						Name: "geth",
						Endpoints: descriptors.EndpointMap{
							"rpc": descriptors.PortInfo{
								Host: "localhost",
								Port: 8546,
							},
						},
					},
				},
			}},
			Wallets: descriptors.WalletMap{
				"default": descriptors.Wallet{
					Address:    "0x123",
					PrivateKey: "0xabc",
				},
			},
		}},
		Features: []string{},
	}

	data, err := json.Marshal(devnet)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(devnetFile, data, 0644))

	// Test with valid environment
	envVar := env.EnvFileVar
	os.Setenv(envVar, devnetFile)
	sys, err := NewSystemFromEnv(envVar)
	assert.NoError(t, err)
	assert.NotNil(t, sys)

	// Test with unset environment variable
	os.Unsetenv(envVar)
	sys, err = NewSystemFromEnv(envVar)
	assert.Error(t, err)
	assert.Nil(t, sys)
}

func TestContractAddress(t *testing.T) {
	testWallet := NewWallet("0xabc", "0x123", "http://localhost:8545")
	chain := NewChain("1", "http://localhost:8545", testWallet)

	tests := []struct {
		name       string
		contractID string
		want       types.Address
	}{
		{
			name:       "existing contract",
			contractID: "SuperchainWETH",
			want:       "0x4200000000000000000000000000000000000024",
		},
		{
			name:       "non-existent contract",
			contractID: "NonExistentContract",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chain.ContractAddress(tt.contractID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSystemFromDevnet(t *testing.T) {
	testNode := descriptors.Node{
		Services: map[string]descriptors.Service{
			"el": {
				Name: "geth",
				Endpoints: descriptors.EndpointMap{
					"rpc": descriptors.PortInfo{
						Host: "localhost",
						Port: 8545,
					},
				},
			},
		},
	}

	testWallet := descriptors.Wallet{
		Address:    "0x123",
		PrivateKey: "0xabc",
	}

	tests := []struct {
		name      string
		devnet    descriptors.DevnetEnvironment
		wantErr   bool
		isInterop bool
	}{
		{
			name: "basic system",
			devnet: descriptors.DevnetEnvironment{
				L1: &descriptors.Chain{
					ID:    "1",
					Nodes: []descriptors.Node{testNode},
					Wallets: descriptors.WalletMap{
						"default": testWallet,
					},
				},
				L2: []*descriptors.Chain{{
					ID:    "2",
					Nodes: []descriptors.Node{testNode},
					Wallets: descriptors.WalletMap{
						"default": testWallet,
					},
				}},
			},
			wantErr:   false,
			isInterop: false,
		},
		{
			name: "interop system",
			devnet: descriptors.DevnetEnvironment{
				L1: &descriptors.Chain{
					ID:    "1",
					Nodes: []descriptors.Node{testNode},
					Wallets: descriptors.WalletMap{
						"default": testWallet,
					},
				},
				L2: []*descriptors.Chain{{
					ID:    "2",
					Nodes: []descriptors.Node{testNode},
					Wallets: descriptors.WalletMap{
						"default": testWallet,
					},
				}},
				Features: []string{"interop"},
			},
			wantErr:   false,
			isInterop: true,
		},
		{
			name: "invalid chain ID",
			devnet: descriptors.DevnetEnvironment{
				L1: &descriptors.Chain{
					ID:    "invalid",
					Nodes: []descriptors.Node{testNode},
					Wallets: descriptors.WalletMap{
						"default": testWallet,
					},
				},
			},
			wantErr:   true,
			isInterop: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys, err := systemFromDevnet(tt.devnet, "test")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, sys)

			_, isInterop := sys.(InteropSystem)
			assert.Equal(t, tt.isInterop, isInterop)
		})
	}
}

func TestDevnetFromFile(t *testing.T) {
	// Create a temporary devnet file
	tempDir := t.TempDir()
	validFile := filepath.Join(tempDir, "valid.json")
	invalidFile := filepath.Join(tempDir, "invalid.json")

	validDevnet := &descriptors.DevnetEnvironment{
		L1: &descriptors.Chain{ID: "1"},
		L2: []*descriptors.Chain{{ID: "2"}},
	}

	validData, err := json.Marshal(validDevnet)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(validFile, validData, 0644))

	require.NoError(t, os.WriteFile(invalidFile, []byte("invalid json"), 0644))

	tests := []struct {
		name    string
		file    string
		wantErr bool
	}{
		{
			name:    "valid file",
			file:    validFile,
			wantErr: false,
		},
		{
			name:    "invalid file",
			file:    invalidFile,
			wantErr: true,
		},
		{
			name:    "non-existent file",
			file:    "nonexistent.json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devnet, err := devnetFromFile(tt.file)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, devnet)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, devnet)
			}
		})
	}
}

func TestWallet(t *testing.T) {
	tests := []struct {
		name        string
		privateKey  types.Key
		address     types.Address
		wantAddr    types.Address
		wantPrivKey types.Key
	}{
		{
			name:        "valid wallet",
			privateKey:  "0xabc",
			address:     "0x123",
			wantAddr:    "0x123",
			wantPrivKey: "0xabc",
		},
		{
			name:        "empty wallet",
			privateKey:  "",
			address:     "",
			wantAddr:    "",
			wantPrivKey: "",
		},
		{
			name:        "only address",
			privateKey:  "",
			address:     "0x456",
			wantAddr:    "0x456",
			wantPrivKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWallet(tt.privateKey, tt.address, "http://localhost:8545")
			assert.Equal(t, tt.wantAddr, w.Address())
			assert.Equal(t, tt.wantPrivKey, w.PrivateKey())
		})
	}
}

func TestChainUser(t *testing.T) {
	testWallet := NewWallet("0xabc", "0x123", "http://localhost:8545")
	chain := NewChain("1", "http://localhost:8545", testWallet)

	ctx := context.Background()
	user, err := chain.User(ctx)
	assert.NoError(t, err)
	assert.Equal(t, testWallet.Address(), user.Address())
	assert.Equal(t, testWallet.PrivateKey(), user.PrivateKey())
}
