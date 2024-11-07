# Holocene Upgrade

This directory contains a repeatable task for upgrading an `op-contracts/v1.6.0` deployment to `op-contracts/v1.8.0`.

## Dependencies

- `docker`
- `just`
- `foundry`

## Usage

```sh
# 1. Clone the monorepo and navigate to this directory.
git clone git@github.com:ethereum-optimism/monorepo.git && \
  cd monorepo/packages/contracts-bedrock/scripts/upgrades/holocene

# 2. Set up the `.env` file
cp .env.example .env && vim .env

# 3. Run the upgrade task.
#
#    This task will:
#    - Deploy the new smart contract implementations.
#    - Optionally, generate a safe upgrade bundle.
#    - Optionally, generate a `superchain-ops` upgrade task.
just run
```
