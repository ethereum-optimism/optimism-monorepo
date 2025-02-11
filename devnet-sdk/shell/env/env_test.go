package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDevnetEnv(t *testing.T) {
	// Create a temporary test file
	content := `{
		"l1": {
			"name": "l1",
			"nodes": [{
				"services": {
					"el": {
						"endpoints": {
							"rpc": {
								"host": "localhost",
								"port": 8545
							}
						}
					}
				}
			}],
			"jwt": "0x1234567890abcdef",
			"addresses": {
				"deployer": "0x1234567890123456789012345678901234567890"
			}
		},
		"l2": [{
			"name": "op",
			"nodes": [{
				"services": {
					"el": {
						"endpoints": {
							"rpc": {
								"host": "localhost",
								"port": 9545
							}
						}
					}
				}
			}],
			"jwt": "0xdeadbeef",
			"addresses": {
				"deployer": "0x2345678901234567890123456789012345678901"
			}
		}]
	}`

	tmpfile, err := os.CreateTemp("", "devnet-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(content))
	require.NoError(t, err)
	err = tmpfile.Close()
	require.NoError(t, err)

	// Test successful load
	t.Run("successful load", func(t *testing.T) {
		env, err := LoadDevnetEnv(tmpfile.Name())
		require.NoError(t, err)
		assert.Equal(t, "l1", env.config.L1.Name)
		assert.Equal(t, "op", env.config.L2[0].Name)
	})

	// Test loading non-existent file
	t.Run("non-existent file", func(t *testing.T) {
		_, err := LoadDevnetEnv("non-existent.json")
		assert.Error(t, err)
	})

	// Test loading invalid JSON
	t.Run("invalid JSON", func(t *testing.T) {
		invalidFile := filepath.Join(t.TempDir(), "invalid.json")
		err := os.WriteFile(invalidFile, []byte("{invalid json}"), 0644)
		require.NoError(t, err)

		_, err = LoadDevnetEnv(invalidFile)
		assert.Error(t, err)
	})
}

func TestGetChain(t *testing.T) {
	devnet := &DevnetEnv{
		config: descriptors.DevnetEnvironment{
			L1: &descriptors.Chain{
				Name: "l1",
				Nodes: []descriptors.Node{
					{
						Services: descriptors.ServiceMap{
							"el": {
								Endpoints: descriptors.EndpointMap{
									"rpc": {
										Host: "localhost",
										Port: 8545,
									},
								},
							},
						},
					},
				},
				JWT: "0x1234",
			},
			L2: []*descriptors.Chain{
				{
					Name: "op",
					Nodes: []descriptors.Node{
						{
							Services: descriptors.ServiceMap{
								"el": {
									Endpoints: descriptors.EndpointMap{
										"rpc": {
											Host: "localhost",
											Port: 9545,
										},
									},
								},
							},
						},
					},
					JWT: "0x5678",
				},
			},
		},
		fname: "test.json",
	}

	// Test getting L1 chain
	t.Run("get L1 chain", func(t *testing.T) {
		chain, err := devnet.GetChain("l1")
		require.NoError(t, err)
		assert.Equal(t, "l1", chain.name)
		assert.Equal(t, "0x1234", chain.chain.JWT)
	})

	// Test getting L2 chain
	t.Run("get L2 chain", func(t *testing.T) {
		chain, err := devnet.GetChain("op")
		require.NoError(t, err)
		assert.Equal(t, "op", chain.name)
		assert.Equal(t, "0x5678", chain.chain.JWT)
	})

	// Test getting non-existent chain
	t.Run("get non-existent chain", func(t *testing.T) {
		_, err := devnet.GetChain("invalid")
		assert.Error(t, err)
	})
}

func TestChainConfig(t *testing.T) {
	chain := &ChainConfig{
		chain: &descriptors.Chain{
			Name: "test",
			Nodes: []descriptors.Node{
				{
					Services: descriptors.ServiceMap{
						"el": {
							Endpoints: descriptors.EndpointMap{
								"rpc": {
									Host: "localhost",
									Port: 8545,
								},
							},
						},
					},
				},
			},
			JWT: "0x1234",
			Addresses: map[string]types.Address{
				"deployer": common.HexToAddress("0x1234567890123456789012345678901234567890"),
			},
		},
		devnetFile: "test.json",
		name:       "test",
	}

	// Test getting environment variables
	t.Run("get environment variables", func(t *testing.T) {
		env, err := chain.GetEnv()
		require.NoError(t, err)

		assert.Equal(t, "http://localhost:8545", env.EnvVars["ETH_RPC_URL"])
		assert.Equal(t, "1234", env.EnvVars["ETH_RPC_JWT_SECRET"])
		assert.Equal(t, "test.json", filepath.Base(env.EnvVars[EnvFileVar]))
		assert.Equal(t, "test", env.EnvVars[ChainNameVar])
		assert.Contains(t, env.Motd, "deployer")
		assert.Contains(t, env.Motd, "0x1234567890123456789012345678901234567890")
	})

	// Test chain with no nodes
	t.Run("chain with no nodes", func(t *testing.T) {
		noNodesChain := &ChainConfig{
			chain: &descriptors.Chain{
				Name:  "test",
				Nodes: []descriptors.Node{},
			},
		}
		_, err := noNodesChain.GetEnv()
		assert.Error(t, err)
	})

	// Test chain with missing service
	t.Run("chain with missing service", func(t *testing.T) {
		missingServiceChain := &ChainConfig{
			chain: &descriptors.Chain{
				Name: "test",
				Nodes: []descriptors.Node{
					{
						Services: descriptors.ServiceMap{},
					},
				},
			},
		}
		_, err := missingServiceChain.GetEnv()
		assert.Error(t, err)
	})

	// Test chain with missing endpoint
	t.Run("chain with missing endpoint", func(t *testing.T) {
		missingEndpointChain := &ChainConfig{
			chain: &descriptors.Chain{
				Name: "test",
				Nodes: []descriptors.Node{
					{
						Services: descriptors.ServiceMap{
							"el": {
								Endpoints: descriptors.EndpointMap{},
							},
						},
					},
				},
			},
		}
		_, err := missingEndpointChain.GetEnv()
		assert.Error(t, err)
	})
}
