# Devnet Descriptors

The devnet descriptor is a standardized format that describes the complete topology and configuration of an Optimism devnet deployment. This standard serves as a bridge between different devnet implementations and the higher-level tooling provided by the devnet-sdk.

## Universal Descriptor Format

Both `kurtosis-devnet` and `netchef` emit the same descriptor format, despite being completely different devnet implementations:

- **kurtosis-devnet**: Uses Kurtosis to orchestrate containerized devnet deployments
- **netchef**: Provides a lightweight, local devnet deployment

This standardization enables a powerful ecosystem where tools can be built independently of the underlying devnet implementation.

## Descriptor Structure

A devnet descriptor provides a complete view of a running devnet:

```json
{
  "l1": {
    "name": "l1",
    "id": "900",
    "services": {
      "geth": {
        "name": "geth",
        "endpoints": {
          "rpc": {
            "host": "localhost",
            "port": 8545
          }
        }
      }
    },
    "nodes": [...],
    "addresses": {
      "deployer": "0x...",
      "admin": "0x..."
    },
    "wallets": {
      "admin": {
        "address": "0x...",
        "private_key": "0x..."
      }
    }
  },
  "l2": [
    {
      "name": "op-sepolia",
      "id": "11155420",
      "services": {...},
      "nodes": [...],
      "addresses": {...},
      "wallets": {...}
    }
  ],
  "features": ["eip1559", "shanghai"]
}
```

## Enabling Devnet-Agnostic Tooling

The power of the descriptor format lies in its ability to make any compliant devnet implementation immediately accessible to the entire devnet-sdk toolset:

1. **Universal Interface**: Any devnet that can emit this descriptor format can be managed through devnet-sdk's tools
2. **Automatic Integration**: New devnet implementations only need to implement the descriptor format to gain access to:
   - System interface for chain interaction
   - Testing framework
   - Shell integration tools
   - Wallet management
   - Transaction processing

## Benefits

This standardization provides several key advantages:

- **Portability**: Tools built against the devnet-sdk work with any compliant devnet implementation
- **Consistency**: Developers get the same experience regardless of the underlying devnet
- **Extensibility**: New devnet implementations can focus on deployment mechanics while leveraging existing tooling
- **Interoperability**: Tools can be built that work across different devnet implementations

## Implementation Requirements

To make a devnet implementation compatible with devnet-sdk, it needs to:

1. Provide a mechanism to output the descriptor (typically as JSON)
2. Ensure all required services and endpoints are properly described

Once these requirements are met, the devnet automatically gains access to the full suite of devnet-sdk capabilities.

## Status

The descriptor format is currently in active development, particularly regarding endpoint specifications:

### Endpoint Requirements

- **Current State**: The format does not strictly specify which endpoints must be included in a compliant descriptor
- **Minimum Known Requirements**: 
  - RPC endpoints are essential for basic chain interaction
  - Other endpoints may be optional depending on use case

### Implementation Notes

- `kurtosis-devnet` currently outputs all service endpoints by default, including many that may not be necessary for testing
- Other devnet implementations can be more selective about which endpoints they expose
- Different testing scenarios may require different sets of endpoints

### Future Development

We plan to develop more specific recommendations for:
- Required vs optional endpoints
- Standard endpoint naming conventions
- Service-specific endpoint requirements
- Best practices for endpoint exposure

Until these specifications are finalized, devnet implementations should:
1. Always include RPC endpoints
2. Document which additional endpoints they expose
3. Consider their specific use cases when deciding which endpoints to include

## Example Usage

Here's how a tool might use the descriptor to interact with any compliant devnet:

```go
// Load descriptor from any compliant devnet
descriptor, err := descriptors.Load("devnet.json")
if err != nil {
    log.Fatal(err)
}

// Use the descriptor with devnet-sdk tools
system, err := system.FromDescriptor(descriptor)
if err != nil {
    log.Fatal(err)
}

// Now you can use all devnet-sdk features
l1 := system.L1()
l2 := system.L2(descriptor.L2[0].ID)
```

This standardization enables a rich ecosystem of tools that work consistently across different devnet implementations, making development and testing more efficient and reliable.
