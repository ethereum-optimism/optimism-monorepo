# Stage 1.4 Upgrade

Status: DRAFT, NOT READY TO SIGN

## Objective

Registers a new `FaultDisputeGame` with the `DisputeGameFactory` that is backed by the [`asterisc`][asterisc] FPVM and
the [`kona`][kona] fault proof program.

The proposal was:

- [ ] Posted on the governance forum.
- [ ] Approved by Token House voting.
- [ ] Not vetoed by the Citizens' house.
- [ ] Executed on OP Mainnet.

The governance proposal should be treated as the source of truth and used to verify the correctness of the onchain operations.

Governance post of the upgrade can be found at <placeholder>.

This upgrades the Fault Proof contracts in the
[op-contracts/v1.9.0](https://github.com/ethereum-optimism/optimism/tree/op-contracts/v1.9.0-rc.2) release.

## Pre-deployments

- `RISCV` - `0x03021267e939257f8d6ce8c22ef74c3a01a70a22`
- `DelayedWETHProxy` - `0xa89494fba7d1bb5e96aa5c7d7e6d39b5d3973748`
- `FaultDisputeGame` - `0x91ee75c3c0f238652d88b62e29bebddd5a92ed7f`

## Simulation

Please see the "Simulating and Verifying the Transaction" instructions in [NESTED.md](../../../NESTED.md).
When simulating, ensure the logs say `Using script /your/path/to/superchain-ops/tasks/<path>/NestedSignFromJson.s.sol`.
This ensures all safety checks are run. If the default `NestedSignFromJson.s.sol` script is shown (without the full path), something is wrong and the safety checks will not run.

## State Validation

Please see the instructions for [validation](./VALIDATION.md).

## Execution

This upgrade
* Registers the new `FaultDisputeGame` with type `3` to the `DisputeGameFactory`.

See the [overview](./OVERVIEW.md) and `input.json` bundle for more details.

[asterisc]: https://github.com/ethereum-optimism/asterisc
[kona]: https://github.com/anton-rs/kona
