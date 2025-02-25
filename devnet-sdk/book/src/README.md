> ⚠️ **UNDER HEAVY DEVELOPMENT** ⚠️
>
> This documentation is actively being developed and may change frequently.

# Introduction

# Devnet SDK

The Devnet SDK is a comprehensive toolkit designed to standardize and simplify interactions with Optimism devnets. It provides a robust set of tools and interfaces for deploying, managing, and testing Optimism networks in development environments.

## Core Components

### 1. Devnet Descriptors

The descriptors package defines a standard interface for describing and interacting with devnets. It provides:

- Structured representation of devnet environments including L1 and L2 chains
- Service discovery and endpoint management
- Wallet and address management
- Standardized configuration for chain components (nodes, services, endpoints)

### 2. Shell Integration

The shell package provides a preconfigured shell environment for interacting with devnets. For example, you can quickly:

- Launch a shell with all environment variables set and run commands like `cast balance <address>` that automatically use the correct RPC endpoints
- Access chain-specific configuration like JWT secrets and contract addresses

This makes it easy to interact with your devnet without manually configuring tools or managing connection details.

### 3. System Interface

The system package provides a devnet-agnostic programmatic interface, constructed for example from the descriptors above, for interacting with Optimism networks. Key features include:

- Unified interface for L1 and L2 chain interactions
- Transaction management and processing
- Wallet management and operations
- Contract interaction capabilities
- Interoperability features between different L2 chains

Core interfaces include:
- `System`: Represents a complete Optimism system with L1 and L2 chains
- `Chain`: Provides access to chain-specific operations
- `Wallet`: Manages accounts, transactions, and signing operations
- `Transaction`: Handles transaction creation and processing

### 4. Testing Framework

The testing package provides a comprehensive framework for testing devnet deployments:

- Standardized testing utilities
- System test capabilities
- Integration test helpers
- Test fixtures and utilities
