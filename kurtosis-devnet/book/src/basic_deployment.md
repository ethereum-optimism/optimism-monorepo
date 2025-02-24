# Basic Deployment

The Kurtosis devnet provides several pre-configured devnet templates and convenient commands to deploy and interact with them.

## Built-in Devnets

The following devnet templates are available out of the box:

1. **Simple Devnet** (`simple.yaml`)
   - Basic single-chain setup
   - Ideal for local development and testing
   - Deploy with: `just simple-devnet`

2. **Interop Devnet** (`interop.yaml`)
   - Designed for interop testing
   - Includes test suite for cross-chain interactions
   - Deploy with: `just interop-devnet`
   - Run tests with: `just interop-devnet-test`

3. **Pectra Devnet** (`pectra.yaml`)
   - Specialized configuration for Pectra testing
   - Deploy with: `just pectra-devnet`

## User-Defined Devnets (Experimental)

> **Note**: User-defined devnets are an experimental feature and not actively supported at this time. Use at your own risk.

The user devnet template (`user.yaml`) allows for customizable devnet configurations through a JSON input file. This feature is designed to simplify devnet creation for future devnet-as-a-service scenarios.

### Deployment
```bash
just user-devnet <data-file>
```

### Example Configuration
Here's an example of a user devnet configuration file:

```json
{
    "interop": true,
    "l2s": {
        "2151908": {
            "nodes": ["op-geth", "op-geth"]
        },
        "2151909": {
            "nodes": ["op-reth"]
        }
    },
    "overrides": {
        "flags": {
            "log_level": "--log.level=debug"
        }
    }
}
```

This configuration:
- Enables interop testing features
- Defines two L2 chains:
  - Chain `2151908` with two `op-geth` nodes
  - Chain `2151909` with one `op-reth` node
- Sets custom logging level for all nodes

## Deployment Commands

Arbitrary devnets can be deployed using the general `devnet` command with the following syntax:
```bash
just devnet <template-file> [data-file] [name]
```

Where:
- `template-file`: The YAML template to use (e.g., `simple.yaml`)
- `data-file`: Optional JSON file with configuration data
- `name`: Optional custom name for the devnet (defaults to template name)

For example:
```bash
# Deploy simple devnet with default name
just devnet simple.yaml

# Deploy user devnet with custom data and name
just devnet user.yaml my-config.json my-custom-devnet
```

This can be convenient when experimenting with devnet definitions

## Entering a Devnet Shell

The devnet provides a powerful feature to "enter" a devnet environment, which sets up the necessary environment variables for interacting with the chains.

### Basic Usage
```bash
just enter-devnet <devnet-name> [chain-name]
```

Where:
- `devnet-name`: The name of your deployed devnet
- `chain-name`: Optional chain to connect to (defaults to "Ethereum")

Example:
```bash
# Enter the Ethereum chain environment in the simple devnet
just enter-devnet simple-devnet

# Enter a specific chain environment
just enter-devnet my-devnet l2-chain

# Use exec to replace the current shell process (recommended)
exec just enter-devnet my-devnet l2-chain
```

Note: The enter feature creates a new shell process. To avoid accumulating shell processes, you can use the `exec` command, which replaces the current shell with the new one. This is especially useful in scripts or when you want to maintain a clean process tree.

### Features of the Devnet Shell

When you enter a devnet shell, you get:
1. All necessary environment variables set for the chosen chain
2. Integration with tools like `cast` for blockchain interaction
3. Chain-specific configuration and endpoints
4. A new shell session with the devnet context

The shell inherits your current environment and adds:
- Chain-specific RPC endpoints
- Network identifiers
- Authentication credentials (if any)
- Tool configurations

To exit the devnet shell, simply type `exit` or press `Ctrl+D`.

### Environment Variables

The devnet shell automatically sets up environment variables needed for development and testing:
- `ETH_RPC_URL`: The RPC endpoint for the selected chain
- `ETH_RPC_JWT_SECRET`: JWT secret for authenticated RPC connections (when cast integration is enabled)
- `DEVNET_ENV_URL`: The URL or absolute path to the devnet environment file
- `DEVNET_CHAIN_NAME`: The name of the currently selected chain

These variables are automatically picked up by tools like `cast`, making it easy to interact with the chain directly from the shell.