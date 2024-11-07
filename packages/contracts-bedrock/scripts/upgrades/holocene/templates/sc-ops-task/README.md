# Holocene Hardfork Upgrade

Status: DRAFT

## Objective

Upgrades the `SystemConfig` and Fault Proof contracts for the Holocene hardfork.

## Pre-deployments

- `SystemConfig` - `$SYSTEM_CONFIG_IMPL`
- `MIPS` - `$MIPS_IMPL`
- `FaultDisputeGame` - `$FDG_IMPL`
- `PermissionedDisputeGame` - `$PDG_IMPL`

## Simulation

Please see the "Simulating and Verifying the Transaction" instructions in [NESTED.md](../../../NESTED.md).
When simulating, ensure the logs say `Using script /your/path/to/superchain-ops/tasks/<path>/NestedSignFromJson.s.sol`.
This ensures all safety checks are run. If the default `NestedSignFromJson.s.sol` script is shown (without the full path), something is wrong and the safety checks will not run.

## State Validation

Please see the instructions for [validation](./VALIDATION.md).

## Execution

This upgrade
* Changes the implementation of the `SystemConfig` to hold EIP-1559 parameters for the
* Changes dispute game implementation of the `CANNON` and `PERMISSIONED_CANNON` game types to contain the `op-program` release for the Holocene hardfork.
* Upgrades `MIPS.sol` to support the `F_GETFD` syscall, required by the golang 1.22+ runtime.

See the [overview](./OVERVIEW.md) and `input.json` bundle for more details.
