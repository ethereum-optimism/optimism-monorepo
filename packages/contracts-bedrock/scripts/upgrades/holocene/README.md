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
#
#    The first argument must be the absolute path to your deploy-config.json.
#    You can optionally specify an output folder path different from the default `output/` as a
#    second argument to `just run`, also as an absolute path.
just run $(realpath path/to/deploy-config.json)
```

Note that in order to build the Docker image, you have to allow Docker to use at least 16GB of
memory, or the Solidity compilations may fail. Docker's default is only 8GB.

The `deploy-config.json` that you use for your chain must set the latest `faultGameAbsolutePrestate`
value, not the one at deployment. There's currently one available that includes the Sepolia
Superchain Holocene activations for Base, OP, Mode and Zora:
`0x03925193e3e89f87835bbdf3a813f60b2aa818a36bbe71cd5d8fd7e79f5e8afe`

If you want to make local modifications to the scripts in `scripts/`, you need to build the Docker
image again with `just build-image` before running `just run`.
