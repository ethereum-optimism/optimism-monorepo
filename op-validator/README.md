# op-validator

The op-validator is a tool for validating Optimism chain configurations and deployments. It works by calling into the
StandardValidator smart contracts (StandardValidatorV180 and StandardValidatorV200). These then perform a set of checks,
and return error codes for any issues found. These checks include:

- Contract implementations and versions
- Proxy configurations
- System parameters
- Cross-component relationships
- Security settings

## Usage

The validator supports different protocol versions through subcommands:

```bash
op-validator validate [version] [flags]
```

Where version is one of:

- `v1.8.0` - For validating protocol version 1.8.0
- `v2.0.0` - For validating protocol version 2.0.0

### Required Flags

- `--l1-rpc-url`: L1 RPC URL (can also be set via L1_RPC_URL environment variable)
- `--absolute-prestate`: Absolute prestate as hex string
- `--proxy-admin`: Proxy admin address as hex string. This should be a specific chain's proxy admin contract on L1. 
  It is  not the proxy admin owner or the superchain proxy admin.
- `--system-config`: System config proxy address as hex string
- `--l2-chain-id`: L2 chain ID

### Optional Flags

- `--fail`: Exit with non-zero code if validation errors are found (defaults to true)

### Example

```bash
op-validator validate v2.0.0 \
  --l1-rpc-url "https://mainnet.infura.io/v3/YOUR-PROJECT-ID" \
  --absolute-prestate "0x1234..." \
  --proxy-admin "0xabcd..." \
  --system-config "0xefgh..." \
  --l2-chain-id "10" \
  --fail
```

