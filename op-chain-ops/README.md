# `op-chain-ops`

Issues: [monorepo](https://github.com/ethereum-optimism/optimism/issues?q=is%3Aissue%20state%3Aopen%20label%3AA-op-chain-ops)

Pull requests: [monorepo](https://github.com/ethereum-optimism/optimism/pulls?q=is%3Aopen+is%3Apr+label%3AA-op-chain-ops)

This is an OP Stack utils package for chain operations,
ranging from EVM tooling to chain generation.

Packages:
- `clients`: utils for chain checker tools.
- `cmd`: upgrade validation tools, debug tools, attributes formatting tools.
- `crossdomain`: utils to interact with L1 <> L2 cross-domain messages.
- `devkeys`: generate OP-Stack development keys from a common source.
- `foundry`: utils to read foundry artifacts.
- `genesis`: OP Stack genesis-configs generation, pre OPCM.
- `interopgen`: interop test-chain genesis config generation.
- `script`: foundry-like solidity scripting environment in Go.
- `solc`: utils to read solidity compiler artifacts data.
- `srcmap`: utils for solidity source-maps loaded from foundry-artifacts.

## Usage

Upgrade checks and chain utilities can be found in `./cmd`:
these are not officially published in OP-Stack monorepo releases,
but can be built from source.

Utils:
```text
cmd/
├── check-canyon                  - Checks for Canyon network upgrade
├── check-delta                   - Checks for Delta network upgrade
├── check-deploy-config           - Checks of the (legacy) Deploy Config
├── check-derivation              - Check that transactions can be confirmed and safety can be consolidated
├── check-ecotone                 - Checks for Ecotone network upgrade
├── check-fjord                   - Checks for Fjord network upgrade
├── deposit-hash                  - Determine the L2 deposit tx hash, based on log event(s) emitted by a L1 tx.
├── ecotone-scalar                - Translate between serialized and human-readable L1 fee scalars (introduced in Ecotone upgrade).
├── op-simulate                   - Simulate a remote transaction in a local Geth EVM for block-processing debugging.
├── protocol-version              - Translate between serialized and human-readable protocol versions.
├── receipt-reference-builder     - Receipt data collector for pre-Canyon deposit-nonce metadata.
└── unclaimed-credits             - Utility to inspect credits of resolved fault-proof games.
```

## Product

### Optimization target

Provide tools for chain-setup and inspection tools for deployment, upgrades, and testing.
This includes `op-deployer`, OP-Contracts-Manager (OPCM), upgrade-check scripts, and `op-e2e` testing.

### Vision

- Upgrade checking scripts should become more extensible, and maybe be bundled in a single check-script CLI tool.
- Serve chain inspection/processing building-blocks for test setups and tooling like op-deployer.
- `interopgen` is meant to be temporary, and consolidate with `op-deployer`.
  This change depends largely on the future of `op-e2e`,
  where system tests may be replaced in favor of tests set up by `op-e2e`.
- `script` is a Go version of `forge` script, with hooks and customization options,
  for better integration into tooling such as `op-deployer`.
  This package should evolve to serve testing and `op-deployer` as best as possible,
  it is not a full `forge` replacement.
- `genesis` will shrink over time, as more of the genesis responsibilities are automated away into
  the protocol through system-transactions, and tooling such as `op-deployer` and OPCM.

## Design principles

- Provide high-quality bindings to accelerate testing and tooling development.
- Minimal introspection into fragile solidity details.

There is a trade-off here in how minimal the tooling is:
generally we aim to provide dedicated functionality in Go for better integration,
if the target tool is significant Go service of its own.
If not, then `op-chain-ops` should not be extended, and the design of the target tool should be adjusted instead.

## Testing

- Upgrade checks are tested against live devnet/testnet upgrades, before testing against mainnet.
  Testing here is aimed to expand to end-to-end testing, for better integrated test feedback of these tools.
- Utils have unit-test coverage of their own, and are used widely in end-to-end testing itself.
