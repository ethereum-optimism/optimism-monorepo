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
2. Deploy and upgrade `op-contracts/v1.3.0` -> `op-contracts/v1.8.0`, with the `L2OutputOracle` still active.
  - Only upgrade the `SystemConfig`

```sh
# 1. Clone the monorepo and navigate to this directory.
git clone --branch proposal/op-contracts/v1.8.0 --depth 1 git@github.com:ethereum-optimism/monorepo.git && \
  cd monorepo/packages/contracts-bedrock/scripts/upgrades/holocene

# 2. Set up the `.env` file
#
# Read the documentation carefully, and when in doubt, reach out to the OP Labs team.
cp .env.example .env && vim .env

# 3. Build the upgrade script Docker image
just build-image

# 4. Run the upgrade task.
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

:warning: The `deploy-config.json` that you use for your chain must set the latest `faultGameAbsolutePrestate`
value, not the original value that was set during deployment of the chain.

You can use `0x03f89406817db1ed7fd8b31e13300444652cdb0b9c509a674de43483b2f83568`, which is based on
`op-program/v1.4.0-rc.3` and includes Holocene activations for
* Sepolia: Base, OP, Metal, Mode, Zora, Ethernity, Unichain, Ink
* Mainnet: Base, OP, Orderly, Lyra, Metal, Mode, Zora, Lisk, Ethernity, Binary

If you want to make local modifications to the scripts in `scripts/`, you need to build the Docker
image again with `just build-image` before running `just run`.

### Upgrading `SystemConfig` only
There is a more direct route available for those wanting to focus on generating the bundle and task for upgrading the `SystemConfig`. This route does not require a deploy config file. Instead of using `just run` as above you can do:
```sh
# You can optionally specify an output folder path different from the default `output/` as a
# second argument to these commands, also as an absolute path. You should use the same path for both commands.
just sys-cfg-bundle
just sys-cfg-task
```
