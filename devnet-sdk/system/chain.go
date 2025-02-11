package system

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts"
	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// clientManager handles ethclient connections
type clientManager struct {
	mu      sync.RWMutex
	clients map[string]*ethclient.Client
}

func newClientManager() *clientManager {
	return &clientManager{
		clients: make(map[string]*ethclient.Client),
	}
}

func (m *clientManager) getClient(rpcURL string) (*ethclient.Client, error) {
	m.mu.RLock()
	if client, ok := m.clients[rpcURL]; ok {
		m.mu.RUnlock()
		return client, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := m.clients[rpcURL]; ok {
		return client, nil
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ethereum client: %w", err)
	}
	m.clients[rpcURL] = client
	return client, nil
}

type chain struct {
	id     string
	rpcUrl string

	users    map[string]types.Wallet
	clients  *clientManager
	registry interfaces.ContractsRegistry
	mu       sync.Mutex
}

func (c *chain) getClient() (*ethclient.Client, error) {
	return c.clients.getClient(c.rpcUrl)
}

func newChain(chainID string, rpcUrl string, users map[string]types.Wallet) *chain {
	return &chain{
		id:      chainID,
		rpcUrl:  rpcUrl,
		users:   users,
		clients: newClientManager(),
	}
}

func (c *chain) ContractsRegistry() interfaces.ContractsRegistry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.registry != nil {
		return c.registry
	}

	client, err := c.getClient()
	if err != nil {
		return contracts.NewEmptyRegistry()
	}

	c.registry = contracts.NewClientRegistry(client)
	return c.registry
}

func (c *chain) RPCURL() string {
	return c.rpcUrl
}

// Wallet returns the first wallet which meets all provided constraints, or an
// error.
// Typically this will be one of the pre-funded wallets associated with
// the deployed system.
func (c *chain) Wallet(ctx context.Context, constraints ...constraints.WalletConstraint) (types.Wallet, error) {
	// Try each user
	for _, user := range c.users {
		// Check all constraints
		meetsAll := true
		for _, constraint := range constraints {
			if !constraint.CheckWallet(user) {
				meetsAll = false
				break
			}
		}
		if meetsAll {
			return user, nil
		}
	}

	return nil, fmt.Errorf("no user found meeting all constraints")
}

func (c *chain) ID() types.ChainID {
	if c.id == "" {
		return types.ChainID(big.NewInt(0))
	}
	id, ok := new(big.Int).SetString(c.id, 10)
	if !ok {
		return types.ChainID(big.NewInt(0))
	}
	return types.ChainID(id)
}

func chainFromDescriptor(d *descriptors.Chain) Chain {
	// TODO: handle incorrect descriptors better. We could panic here.
	firstNodeRPC := d.Nodes[0].Services["el"].Endpoints["rpc"]
	rpcURL := fmt.Sprintf("http://%s:%d", firstNodeRPC.Host, firstNodeRPC.Port)

	c := newChain(d.ID, rpcURL, nil) // Create chain first

	users := make(map[string]types.Wallet)
	for key, w := range d.Wallets {
		users[key] = newWallet(w.PrivateKey, types.Address(w.Address), c)
	}
	c.users = users // Set users after creation

	return c
}
