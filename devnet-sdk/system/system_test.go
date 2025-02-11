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
	"github.com/ethereum/go-ethereum/common"
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
					Address:    common.HexToAddress("0x123"),
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
					Address:    common.HexToAddress("0x123"),
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
		Address:    common.HexToAddress("0x123"),
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
	chain := newChain("1", "http://localhost:8545", nil)

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
			address:     common.HexToAddress("0x123"),
			wantAddr:    common.HexToAddress("0x123"),
			wantPrivKey: "abc",
		},
		{
			name:        "empty wallet",
			privateKey:  "",
			address:     common.HexToAddress("0x123"),
			wantAddr:    common.HexToAddress("0x123"),
			wantPrivKey: "",
		},
		{
			name:        "only address",
			privateKey:  "",
			address:     common.HexToAddress("0x456"),
			wantAddr:    common.HexToAddress("0x456"),
			wantPrivKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWallet(tt.privateKey, tt.address, chain)
			assert.Equal(t, tt.wantAddr, w.Address())
			assert.Equal(t, tt.wantPrivKey, w.PrivateKey())
		})
	}
}

func TestChainUser(t *testing.T) {
	chain := newChain("1", "http://localhost:8545", nil)
	testWallet := newWallet("0xabc", common.HexToAddress("0x123"), chain)
	chain.users = map[string]types.Wallet{
		"l2Faucet": testWallet,
	}

	ctx := context.Background()
	user, err := chain.Wallet(ctx)
	assert.NoError(t, err)
	assert.Equal(t, testWallet.Address(), user.Address())
	assert.Equal(t, testWallet.PrivateKey(), user.PrivateKey())
}
