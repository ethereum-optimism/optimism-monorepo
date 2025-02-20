# Introduction

OP Deployer is a CLI tool that simplifies deploying and upgrading smart contracts for OP Stack chains. It also
exposes a suite of libraries that allow developers to easily manage smart contracts from their applications.

## Goals

### Declarative

With OP Deployer, developers define their chain's desired configuration in a declarative configuration file. The tool
then makes the minimum number of smart contract calls required to make the deployment match the configuration. This
ensures that the implementation details of the deployment are abstracted away, and allows complex configurations to be
expressed cleanly without concern for the underlying deployment process.

### Portable

OP Deployer is designed to be small, portable, and easily installed. As such it is distributed as a standalone binary
with no additional dependencies. This allows it to be used in a variety of contexts, including as a CLI tool, in CI
pipelines, and as part of local development environments like [Kurtosis][kurtosis].

[kurtosis]: https://github.com/ethpandaops/optimism-package

### Standard, But Extensible

OP Deployer aims to make doing the right thing easy, and doing dangerous things hard. As such its configuration and
API are optimized for deploying and upgrading Standard OP Chains. However, it also exposes a lower-level set of
primitives and configuration directives which users can use to deploy more complex configurations if the need arises.

## Development Status

OP Deployer is undergoing active development and has been used for several mainnet deployments. It is considered
production-ready. However, please keep in mind that **OP Deployer has not been audited** and that any chains
deployed using OP Deployer should be checked thoroughly for correctness prior to launch.