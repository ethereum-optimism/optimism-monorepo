<div align="center">
  <br />
  <br />
  <a href="https://optimism.io"><img alt="Optimism" src="https://raw.githubusercontent.com/ethereum-optimism/brand-kit/main/assets/svg/OPTIMISM-R.svg" width=600></a>
  <br />
  <h3><a href="https://optimism.io">Optimism</a> is Ethereum, scaled.</h3>
  <br />
</div>

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [What is Optimism?](#what-is-optimism)
- [Documentation](#documentation)
- [Specification](#specification)
- [Community](#community)
- [Contributing](#contributing)
- [Security Policy and Vulnerability Reporting](#security-policy-and-vulnerability-reporting)
- [Directory Structure](#directory-structure)
- [Development and Release Process](#development-and-release-process)
  - [Overview](#overview)
  - [Production Releases](#production-releases)
  - [Development branch](#development-branch)
- [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## What is Optimism?

[Optimism](https://www.optimism.io/) is a project dedicated to scaling Ethereum's technology and expanding its ability to coordinate people from across the world to build effective decentralized economies and governance systems. The [Optimism Collective](https://www.optimism.io/vision) builds open-source software that powers scalable blockchains and aims to address key governance and economic challenges in the wider Ethereum ecosystem. Optimism operates on the principle of **impact=profit**, the idea that individuals who positively impact the Collective should be proportionally rewarded with profit. **Change the incentives and you change the world.**

In this repository you'll find numerous core components of the OP Stack, the decentralized software stack maintained by the Optimism Collective that powers Optimism and forms the backbone of blockchains like [OP Mainnet](https://explorer.optimism.io/) and [Base](https://base.org). The OP Stack is designed to be aggressively open-source — you are welcome to explore, modify, and extend this code.

## Documentation

- If you want to build on top of OP Mainnet, refer to the [Optimism Documentation](https://docs.optimism.io)
- If you want to build your own OP Stack based blockchain, refer to the [OP Stack Guide](https://docs.optimism.io/stack/getting-started) and make sure to understand this repository's [Development and Release Process](#development-and-release-process)

## Specification

Detailed specifications for the OP Stack can be found within the [OP Stack Specs](https://github.com/ethereum-optimism/specs) repository.

## Community

General discussion happens most frequently on the [Optimism discord](https://discord.gg/optimism).
Governance discussion can also be found on the [Optimism Governance Forum](https://gov.optimism.io/).

## Contributing

The OP Stack is a collaborative project. By collaborating on free, open software and shared standards, the Optimism Collective aims to prevent siloed software development and rapidly accelerate the development of the Ethereum ecosystem. Come contribute, build the future, and redefine power, together.

[CONTRIBUTING.md](./CONTRIBUTING.md) contains a detailed explanation of the contributing process for this repository. Make sure to use the [Developer Quick Start](./CONTRIBUTING.md#development-quick-start) to properly set up your development environment.

[Good First Issues](https://github.com/ethereum-optimism/optimism/issues?q=is:open+is:issue+label:D-good-first-issue) are a great place to look for tasks to tackle if you're not sure where to start, and see [CONTRIBUTING.md](./CONTRIBUTING.md) for info on larger projects.

## Security Policy and Vulnerability Reporting

Please refer to the canonical [Security Policy](https://github.com/ethereum-optimism/.github/blob/master/SECURITY.md) document for detailed information about how to report vulnerabilities in this codebase.
Bounty hunters are encouraged to check out the [Optimism Immunefi bug bounty program](https://immunefi.com/bounty/optimism/).
The Optimism Immunefi program offers up to $2,000,042 for in-scope critical vulnerabilities.

## Directory Structure

<pre>
├── <a href="./docs">docs</a>: A collection of documents including audits and post-mortems
├── <a href="./op-batcher">op-batcher</a>: L2-Batch Submitter, submits bundles of batches to L1
├── <a href="./op-chain-ops">op-chain-ops</a>: State surgery utilities
├── <a href="./op-challenger">op-challenger</a>: Dispute game challenge agent
├── <a href="./op-e2e">op-e2e</a>: End-to-End testing of all bedrock components in Go
├── <a href="./op-node">op-node</a>: rollup consensus-layer client
├── <a href="./op-preimage">op-preimage</a>: Go bindings for Preimage Oracle
├── <a href="./op-program">op-program</a>: Fault proof program
├── <a href="./op-proposer">op-proposer</a>: L2-Output Submitter, submits proposals to L1
├── <a href="./op-service">op-service</a>: Common codebase utilities
├── <a href="./op-ufm">op-ufm</a>: Simulations for monitoring end-to-end transaction latency
├── <a href="./op-wheel">op-wheel</a>: Database utilities
├── <a href="./ops">ops</a>: Various operational packages
├── <a href="./ops-bedrock">ops-bedrock</a>: Bedrock devnet work
├── <a href="./packages">packages</a>
│   ├── <a href="./packages/contracts-bedrock">contracts-bedrock</a>: OP Stack smart contracts
├── <a href="./semgrep">semgrep</a>: Semgrep rules and tests
</pre>

## Development and Release Process

### Overview

Please read this section carefully if you're planning to fork or make frequent PRs into this repository.

### Production Releases

Production releases are always tags, versioned as `<component-name>/v<semver>`.
For example, an `op-node` release might be versioned as `op-node/v1.1.2`, and  smart contract releases might be versioned as `op-contracts/v1.0.0`.
Release candidates are versioned in the format `op-node/v1.1.2-rc.1`.
We always start with `rc.1` rather than `rc`.

For contract releases, refer to the GitHub release notes for a given release which will list the specific contracts being released. Not all contracts are considered production ready within a release and many are under active development.

Tags of the form `v<semver>`, such as `v1.1.4`, indicate releases of all Go code only, and **DO NOT** include smart contracts.
This naming scheme is required by Golang.
In the above list, this means these `v<semver` releases contain all `op-*` components and exclude all `contracts-*` components.

`op-geth` embeds upstream geth’s version inside its own version as follows: `vMAJOR.GETH_MAJOR GETH_MINOR GETH_PATCH.PATCH`.
Basically, geth’s version is our minor version.
For example if geth is at `v1.12.0`, the corresponding op-geth version would be `v1.101200.0`.
Note that we pad out to three characters for the geth minor version and two characters for the geth patch version.
Since we cannot left-pad with zeroes, the geth major version is not padded.

See the [Node Software Releases](https://docs.optimism.io/builders/node-operators/releases) page of the documentation for more information about releases for the latest node components.

The full set of components that have releases are:

- `ci-builder`
- `op-batcher`
- `op-contracts`
- `op-challenger`
- `op-node`
- `op-proposer`

All other components and packages should be considered development components only and do not have releases.

### Development branch

The primary development branch is [`develop`](https://github.com/ethereum-optimism/optimism/tree/develop/).
`develop` contains the most up-to-date software that remains backwards compatible with the latest experimental [network deployments](https://docs.optimism.io/chain/networks).
If you're making a backwards compatible change, please direct your pull request towards `develop`.

**Changes to contracts within `packages/contracts-bedrock/src` are usually NOT considered backwards compatible.**
Some exceptions to this rule exist for cases in which we absolutely must deploy some new contract after a tag has already been fully deployed.
If you're changing or adding a contract and you're unsure about which branch to make a PR into, default to using a feature branch.
Feature branches are typically used when there are conflicts between 2 projects touching the same code, to avoid conflicts from merging both into `develop`.

## License

All other files within this repository are licensed under the [MIT License](https://github.com/ethereum-optimism/optimism/blob/master/LICENSE) unless stated otherwise.
