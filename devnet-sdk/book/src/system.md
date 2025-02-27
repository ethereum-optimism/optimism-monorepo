# System Interfaces

The devnet-sdk provides a set of Go interfaces that abstract away the specifics of devnet deployments, enabling automation solutions to work consistently across different deployment types and implementations.

## Core Philosophy

While the Descriptor interfaces provide a common way to describe actual devnet deployments (like production-like or Kurtosis-based deployments), the System interfaces operate at a higher level of abstraction. They are designed to support both real deployments and lightweight testing environments.

The key principles are:

- **Deployment-Agnostic Automation**: Code written against these interfaces works with any implementation - from full deployments described by Descriptors to in-memory stacks or completely fake environments
- **Flexible Testing Options**: Enables testing against:
  - Complete devnet deployments
  - Partial mock implementations
  - Fully simulated environments
- **One-Way Abstraction**: While Descriptors can be converted into System interfaces, System interfaces can represent additional constructs beyond what Descriptors describe
- **Implementation Freedom**: New deployment types or testing environments can be added without modifying existing automation code

## Interface Purity

A critical design principle of these interfaces is their **purity**. This means that interfaces:

1. **Only Reference Other Pure Interfaces**: Each interface method can only return or accept:
   - Other pure interfaces from this package
   - Simple data objects that can be fully instantiated
   - Standard Go types and primitives

2. **Avoid Backend-Specific Types**: The interfaces never expose types that would create dependencies on specific implementations:
   ```go
   // BAD: Creates dependency on specific client implementation
   func (c Chain) GetNodeClient() *specific.NodeClient
   
   // GOOD: Returns pure interface that can be implemented by any backend
   func (c Chain) Client() (ChainClient, error)
   ```

3. **Use Generic Data Types**: When complex data structures are needed, they are defined as pure data objects:
   ```go
   // Pure data type that any implementation can create
   type TransactionData interface {
       From() common.Address
       To() *common.Address
       Value() *big.Int
       Data() []byte
   }
   ```

### Why Purity Matters

Interface purity is crucial because it:
- Preserves implementation freedom
- Prevents accidental coupling to specific backends
- Enables creation of new implementations without constraints
- Allows mixing different implementation types (e.g., partial fakes)

### Example: Maintaining Purity

```go
// IMPURE: Forces dependency on eth client
type Chain interface {
    GetEthClient() *ethclient.Client  // 👎 Locks us to specific client
}

// PURE: Allows any implementation
type Chain interface {
    Client() (ChainClient, error)     // 👍 Implementation-agnostic
}

type ChainClient interface {
    BlockNumber(ctx context.Context) (uint64, error)
    // ... other methods
}
```

## Interface Hierarchy

### System

The top-level interface representing a complete Optimism deployment:

```go
type System interface {
    // Unique identifier for this system
    Identifier() string
    
    // Access to L1 chain
    L1() Chain
    
    // Access to L2 chain(s)
    L2(chainID uint64) Chain
}
```

### Chain

Represents an individual chain (L1 or L2) within the system:

```go
type Chain interface {
    // Chain identification
    RPCURL() string
    ID() types.ChainID
    
    // Core functionality
    Client() (*ethclient.Client, error)
    Wallets(ctx context.Context) ([]Wallet, error)
    ContractsRegistry() interfaces.ContractsRegistry
    
    // Chain capabilities
    SupportsEIP(ctx context.Context, eip uint64) bool
    
    // Transaction management
    GasPrice(ctx context.Context) (*big.Int, error)
    GasLimit(ctx context.Context, tx TransactionData) (uint64, error)
    PendingNonceAt(ctx context.Context, address common.Address) (uint64, error)
}
```

### Wallet

Manages accounts and transaction signing:

```go
type Wallet interface {
    // Account management
    PrivateKey() types.Key
    Address() types.Address
    Balance() types.Balance
    Nonce() uint64
    
    // Transaction operations
    Sign(tx Transaction) (Transaction, error)
    Send(ctx context.Context, tx Transaction) error
    
    // Convenience methods
    SendETH(to types.Address, amount types.Balance) types.WriteInvocation[any]
    Transactor() *bind.TransactOpts
}
```

## Implementation Types

The interfaces can be implemented in various ways to suit different needs:

### 1. Real Deployments
- **Kurtosis-based**: Full containerized deployment
- **Netchef**: Remote devnet deployment
-
### 2. Testing Implementations
- **In-memory**: Fast, lightweight implementation for unit tests
- **Mocks**: Controlled behavior for specific test scenarios
- **Recording**: Record and replay real interactions

### 3. Specialized Implementations
- **Partial**: Combining pieces from fake and real deployments
- **Filtered**: Limited functionality for specific use cases
- **Instrumented**: Added logging/metrics for debugging

## Usage Examples

### Writing Tests

The System interfaces are primarily used through our testing framework. See the [Testing Framework](./testing.md) documentation for detailed examples and best practices.

### Creating a Mock Implementation

```go
type MockSystem struct {
    l1 *MockChain
    l2Map map[uint64]*MockChain
}

func NewMockSystem() *MockSystem {
    return &MockSystem{
        l1: NewMockChain(),
        l2Map: make(map[uint64]*MockChain),
    }
}

// Implement System interface...
```

## Benefits

- **Abstraction**: Automation code is isolated from deployment details
- **Flexibility**: Easy to add new deployment types
- **Testability**: Support for various testing approaches
- **Consistency**: Same interface across all implementations
- **Extensibility**: Can add specialized implementations for specific needs

## Best Practices

1. **Write Against Interfaces**: Never depend on specific implementations
2. **Use Context**: For proper cancellation and timeouts
3. **Handle Errors**: All operations can fail
4. **Test Multiple Implementations**: Ensure code works across different types
5. **Consider Performance**: Choose appropriate implementation for use case

The System interfaces provide a powerful abstraction layer that enables writing robust, deployment-agnostic automation code while supporting a wide range of implementation types for different use cases.
