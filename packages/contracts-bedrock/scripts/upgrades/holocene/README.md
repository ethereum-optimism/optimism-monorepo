# Holocene Upgrade

This directory contains a repeatable task for:
* upgrading an `op-contracts/v1.6.0` deployment to `op-contracts/v1.8.0`.
* upgrading an `op-contracts/v1.3.0` deployment to `op-contracts/v1.8.0`, while retaining the `L2OutputOracle`.

## Dependencies

- [`docker`](https://docs.docker.com/engine/install/)
- [`just`](https://github.com/casey/just)
- [`foundry`](https://getfoundry.sh/)

## Usage

This script has several different modes of operation. Namely:
1. Deploy and upgrade `op-contracts/1.6.0` -> `op-contracts/v1.8.0`
  - Always upgrade the `SystemConfig`
  - FP options:
    - With permissionless fault proofs enabled (incl. `FaultDisputeGame`)
    - With permissioned fault proofs enabled (excl. `FaultDisputeGame`)
1. Deploy and upgrade `op-contracts/v1.3.0` -> `op-contracts/v1.8.0`, with the `L2OutputOracle` still active.
  - Only upgrade the `SystemConfig`

```sh
# 1. Clone the monorepo and navigate to this directory.
git clone git@github.com:ethereum-optimism/monorepo.git && \
  cd monorepo/packages/contracts-bedrock/scripts/upgrades/holocene

# 2. Set up the `.env` file
#
# Read the documentation carefully, and when in doubt, reach out to the OP Labs team.
cp .env.example .env && vim .env

# 3. Run the upgrade task.
#
#    This task will:
#    - Deploy the new smart contract implementations.
#    - Optionally, generate a safe upgrade bundle.
#    - Optionally, generate a `superchain-ops` upgrade task.
just run
```
