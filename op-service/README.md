# `op-service`

Issues: [monorepo](https://github.com/ethereum-optimism/optimism/issues?q=is%3Aissue%20state%3Aopen%20label%3AA-op-service)

Pull requests: [monorepo](https://github.com/ethereum-optimism/optimism/pulls?q=is%3Aopen+is%3Apr+label%3AA-op-service)

`op-service` is a collection of Go utilities to build OP-Stack services with.

```text
├── cliapp          - Flag and lifecycle handling for a Urfave v2 CLI app.
├── client          - RPC and HTTP client utils
├── clock           - Clock interface, system clock, tickers, mock/test time utils
├── crypto          - Cryptography utils, complements geth crypto package
├── ctxinterrupt    - Blocking/Interrupt handling
├── dial            - Dialing util functions for RPC clients
├── endpoint        - Abstracts away type of RPC endpoint
├── enum            - Utils to create enums
├── errutil         - Utils to work with customized errors
├── eth             - Common Ethereum data types and OP-Stack extension types
├── flags           - Utils and flag types for CLI usage
├── httputil        - Utils to create enhanced HTTP Server
├── ioutil          - File utils, including atomic files and compression
├── jsonutil        - JSON encoding/decoding utils
├── locks           - Lock utils, like read-write wrapped types
├── log             - Logging CLI and middleware utils
├── metrics         - Metrics types, metering abstractions, server utils
├── oppprof         - P-Prof CLI types and server setup
├── predeploys      - OP-Stack predeploy definitions
├── queue           - Generic queue implementation
├── retry           - Function retry utils
├── rpc             - RPC server utils
├── safego          - Utils to make Go memory more safe
├── serialize       - Binary serialization abstractions
├── signer          - CLI flags and bindings to work with a remote signer
├── solabi          - Utils to encode/decode Solidity ABI formatted data
├── sources         - RPC client bindings
├── tasks           - Err-group with panic handling
├── testlog         - Test logger and log-capture utils for testing
├── testutils       - Simplified Ethereum types, mock RPC bindings, utils for testing.
├── tls             - CLI flags and utils to work with TLS connections
├── txmgr           - Transaction manager: automated nonce, fee and confirmation handling.
└── *.go            - Miscellaneous utils (soon to be deprecated / moved)
```

## Usage

From `op-service` dir:
```bash
# Run Go tests
make test
# Run Go fuzz tests
make fuzz
```

## Product

### Optimization target

Provide solid reusable building blocks for all OP-Stack Go services.

### Vision

- Remove unused utilities: `op-service` itself needs to stay maintainable.
- Make all Go services consistent: `op-service` modules can be used to simplify and improve more Go services.

## Design principles

- Reduce boilerplate in Go services: provide service building utils ranging from CLI to testing.
- Protect devs from sharp edges in the Go std-lib: think of providing missing composition,
  proper resource-closing, well set up network-binding, safe concurrency utils.

## Testing

Each op-service package has its own unit-testing.
More advanced utils, such as the transaction manager, are covered in `op-e2e` as well.
