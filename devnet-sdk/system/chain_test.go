package system

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientManager(t *testing.T) {
	manager := newClientManager()

	t.Run("returns error for invalid URL", func(t *testing.T) {
		_, err := manager.Client("invalid://url")
		assert.Error(t, err)
	})

	t.Run("caches client for same URL", func(t *testing.T) {
		// Use a hostname that's guaranteed to fail DNS resolution
		url := "http://this.domain.definitely.does.not.exist:8545"

		// First call should create new client
		client1, err1 := manager.Client(url)
		// Second call should return cached client
		client2, err2 := manager.Client(url)

		// Both calls should succeed in creating a client
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotNil(t, client1)
		assert.NotNil(t, client2)

		// But the client should fail when used
		ctx := context.Background()
		_, err := client1.ChainID(ctx)
		assert.Error(t, err)

		// And both clients should be the same instance
		assert.Same(t, client1, client2)
	})
}

func TestChainFromDescriptor(t *testing.T) {
	descriptor := &descriptors.Chain{
		ID: "1",
		Nodes: []descriptors.Node{
			{
				Services: descriptors.ServiceMap{
					"el": descriptors.Service{
						Endpoints: descriptors.EndpointMap{
							"rpc": descriptors.PortInfo{
								Host: "localhost",
								Port: 8545,
							},
						},
					},
				},
			},
		},
		Wallets: descriptors.WalletMap{
			"user1": descriptors.Wallet{
				PrivateKey: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				Address:    common.HexToAddress("0x1234567890123456789012345678901234567890"),
			},
		},
	}

	chain, err := chainFromDescriptor(descriptor)
	assert.Nil(t, err)
	assert.NotNil(t, chain)
	lowLevelChain, ok := chain.(LowLevelChain)
	assert.True(t, ok)
	assert.Equal(t, "http://localhost:8545", lowLevelChain.RPCURL())

	// Compare the underlying big.Int values
	chainID := chain.ID()
	expectedID := big.NewInt(1)
	assert.Equal(t, 0, expectedID.Cmp(chainID))
}

func TestChainWallet(t *testing.T) {
	ctx := context.Background()
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	wallet, err := newWallet("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", testAddr, nil)
	assert.Nil(t, err)

	chain := newChain("1", "http://localhost:8545", map[string]Wallet{
		"user1": wallet,
	})

	t.Run("finds wallet meeting constraints", func(t *testing.T) {
		constraint := &addressConstraint{addr: testAddr}
		wallets, err := chain.Wallets(ctx)
		require.NoError(t, err)

		for _, w := range wallets {
			if constraint.CheckWallet(w) {
				assert.NotNil(t, w)
				assert.Equal(t, testAddr, w.Address())
				return
			}
		}
		t.Fatalf("wallet not found")
	})

	t.Run("returns error when no wallet meets constraints", func(t *testing.T) {
		wrongAddr := common.HexToAddress("0x0987654321098765432109876543210987654321")
		constraint := &addressConstraint{addr: wrongAddr}
		wallets, err := chain.Wallets(ctx)
		require.NoError(t, err)

		for _, w := range wallets {
			if constraint.CheckWallet(w) {
				t.Fatalf("wallet found")
			}
		}
	})
}

// addressConstraint implements constraints.WalletConstraint for testing
type addressConstraint struct {
	addr common.Address
}

func (c *addressConstraint) CheckWallet(w Wallet) bool {
	return w.Address() == c.addr
}

func TestChainID(t *testing.T) {
	tests := []struct {
		name     string
		idString string
		want     *big.Int
	}{
		{
			name:     "valid chain ID",
			idString: "1",
			want:     big.NewInt(1),
		},
		{
			name:     "empty chain ID",
			idString: "",
			want:     big.NewInt(0),
		},
		{
			name:     "invalid chain ID",
			idString: "not a number",
			want:     big.NewInt(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := newChain(tt.idString, "", nil)
			got := chain.ID()
			// Compare the underlying big.Int values
			assert.Equal(t, 0, tt.want.Cmp(got))
		})
	}
}

func TestSupportsEIP(t *testing.T) {
	ctx := context.Background()
	chain := newChain("1", "http://localhost:8545", nil)

	// Since we can't reliably test against a live node, we're just testing the error case
	t.Run("returns false for connection error", func(t *testing.T) {
		assert.False(t, chain.SupportsEIP(ctx, 1559))
		assert.False(t, chain.SupportsEIP(ctx, 4844))
	})
}

func TestContractsRegistry(t *testing.T) {
	chain := newChain("1", "http://localhost:8545", nil)

	t.Run("returns empty registry on error", func(t *testing.T) {
		registry := chain.ContractsRegistry()
		assert.NotNil(t, registry)
	})

	t.Run("caches registry", func(t *testing.T) {
		registry1 := chain.ContractsRegistry()
		registry2 := chain.ContractsRegistry()
		assert.Same(t, registry1, registry2)
	})
}
