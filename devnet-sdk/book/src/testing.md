# Testing Framework

The devnet-sdk provides a comprehensive testing framework designed to make testing against Optimism devnets both powerful and developer-friendly.

## Testing Philosophy

Our testing approach is built on several key principles:

### 1. Native Go Tests

Tests are written as standard Go tests, providing:
- Full IDE integration
- Native debugging capabilities
- Familiar testing patterns
- Integration with standard Go tooling

```go
func TestSystemWrapETH(t *testing.T) {
    // Standard Go test function
    systest.SystemTest(t, wrapETHScenario(...))
}
```

### 2. Safe Test Execution

Tests are designed to be safe and self-aware:
- Tests verify their prerequisites before execution
- Tests skip gracefully when prerequisites aren't met
- Clear distinction between precondition failures and test failures

```go
// Test will skip if the system doesn't support required features
walletGetter, fundsValidator := validators.AcquireL2WalletWithFunds(
    chainIdx,
    types.NewBalance(big.NewInt(1.0 * constants.ETH)),
)
```

### 3. Testable Scenarios

Test scenarios themselves are designed to be testable:
- Scenarios work against any compliant System implementation
- Mocks and fakes can be used for scenario validation
- Clear separation between test logic and system interaction

### 4. Framework Integration

The `systest` package provides integration helpers that:
- Handle system acquisition and setup
- Manage test context and cleanup
- Provide precondition validation
- Enable consistent test patterns

## Example Test

Here's a complete example showing these principles in action:

```go
import (
    "log/slog"
    "math/big"
    
    "github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
    "github.com/ethereum-optimism/optimism/devnet-sdk/system"
    "github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
    "github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
    "github.com/ethereum-optimism/optimism/devnet-sdk/types"
    "github.com/stretchr/testify/require"
)

// Define test scenario as a function that works with any System implementation
func wrapETHScenario(chainIdx uint64, walletGetter validators.WalletGetter) systest.SystemTestFunc {
    return func(t systest.T, sys system.System) {
        ctx := t.Context()
        logger := slog.With("test", "WrapETH", "devnet", sys.Identifier())

        // Get the L2 chain we want to test with
        chain := sys.L2(chainIdx)
        logger = logger.With("chain", chain.ID())
        
        // Get a funded wallet for testing
        user := walletGetter(ctx)
        
        // Access contract registry
        wethAddr := constants.SuperchainWETH
        weth, err := chain.ContractsRegistry().SuperchainWETH(wethAddr)
        require.NoError(t, err)
        
        // Test logic using pure interfaces
        funds := types.NewBalance(big.NewInt(0.5 * constants.ETH))
        initialBalance, err := weth.BalanceOf(user.Address()).Call(ctx)
        require.NoError(t, err)
        
        require.NoError(t, user.SendETH(wethAddr, funds).Send(ctx).Wait())
        
        finalBalance, err := weth.BalanceOf(user.Address()).Call(ctx)
        require.NoError(t, err)
        
        require.Equal(t, initialBalance.Add(funds), finalBalance)
    }
}

func TestSystemWrapETH(t *testing.T) {
    chainIdx := uint64(0) // First L2 chain
    
    // Setup wallet with required funds - this acts as a precondition
    walletGetter, fundsValidator := validators.AcquireL2WalletWithFunds(
        chainIdx,
        types.NewBalance(big.NewInt(1.0 * constants.ETH)),
    )
    
    // Run the test with system management handled by framework
    systest.SystemTest(t,
        wrapETHScenario(chainIdx, walletGetter),
        fundsValidator,
    )
}
```

## Framework Components

### 1. Test Context Management

The framework provides context management through `systest.T`:
- Proper test timeouts
- Cleanup handling
- Resource management
- Logging context

### 2. Precondition Validators

Validators ensure test prerequisites are met:
```go
// Validator ensures required funds are available
fundsValidator := validators.AcquireL2WalletWithFunds(...)
```

### 3. System Acquisition

The framework handles system creation and setup:
```go
systest.SystemTest(t, func(t systest.T, sys system.System) {
    // System is ready to use
})
```

### 4. Resource Management

Resources are properly managed:
- Automatic cleanup
- Proper error handling
- Context cancellation

## Best Practices

1. **Use Scenarios**: Write reusable test scenarios that work with any System implementation
2. **Validate Prerequisites**: Always check test prerequisites using validators
3. **Handle Resources**: Use the framework's resource management
4. **Use Pure Interfaces**: Write tests against the interfaces, not specific implementations
5. **Proper Logging**: Use structured logging with test context
6. **Clear Setup**: Keep test setup clear and explicit
7. **Error Handling**: Always handle errors and provide clear failure messages
