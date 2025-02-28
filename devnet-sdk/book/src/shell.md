# Shell Integration

The devnet-sdk provides powerful shell integration capabilities that allow developers to "enter" a devnet environment, making interactions with the network more intuitive and streamlined.

## Devnet Shell Environment

Using a devnet's descriptor, we can create a shell environment that is automatically configured with all the necessary context to interact with the devnet:

```bash
# Enter a shell configured for your devnet
devnet-sdk shell --descriptor path/to/devnet.json
```

### Automatic Configuration

When you enter a devnet shell, the environment is automatically configured with:

- Environment variables for RPC endpoints
- JWT authentication tokens where required
- Named wallet addresses
- Chain IDs
- Other devnet-specific configuration

### Simplified Tool Usage

This automatic configuration enables seamless use of Ethereum development tools without explicit endpoint configuration:

```bash
# Without devnet shell
cast balance 0x123... --rpc-url http://localhost:8545 --jwt-secret /path/to/jwt

# With devnet shell
cast balance 0x123...  # RPC and JWT automatically configured
```

## Supported Tools

The shell environment enhances the experience with various Ethereum development tools:

- `cast`: For sending transactions and querying state

## Environment Variables

The shell automatically sets up standard Ethereum environment variables based on the descriptor:

```bash
# Chain enpointpoit
export ETH_RPC_URL=...
export ETH_JWT_SECRET=...
```

## Usage Examples

```bash
# Enter devnet shell
go run devnet-sdk/shll/cmd/enter/main.go --descriptor devnet.json --chain ...

# Now you can use tools directly
cast block latest

# Exit the shell
exit
```

## Benefits

- **Simplified Workflow**: No need to manually configure RPC endpoints or authentication
- **Consistent Environment**: Same configuration across all tools and commands
- **Reduced Error Risk**: Eliminates misconfigurations and copy-paste errors
- **Context Awareness**: Shell knows about all chains and services in your devnet

## Implementation Details

The shell integration:
1. Reads the descriptor file
2. Sets up environment variables based on the descriptor content
3. Creates a new shell session with the configured environment
4. Maintains the environment until you exit the shell

This feature makes it significantly easier to work with devnets by removing the need to manually manage connection details and authentication tokens.
