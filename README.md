<div align="center">
  <a href="https://community.optimism.io"><img alt="Optimism" src="https://user-images.githubusercontent.com/14298799/122151157-0b197500-ce2d-11eb-89d8-6240e3ebe130.png" width=280></a>
  <br />
  <h1> The Optimism Monorepo</h1>
</div>
<p align="center">
  <a href="https://github.com/ethereum-optimism/optimism/actions/workflows/ts-packages.yml?query=branch%3Amaster"><img src="https://github.com/ethereum-optimism/optimism/workflows/typescript%20/%20contracts/badge.svg" /></a>
  <a href="https://github.com/ethereum-optimism/optimism/actions/workflows/integration.yml?query=branch%3Amaster"><img src="https://github.com/ethereum-optimism/optimism/workflows/integration/badge.svg" /></a>
  <a href="https://github.com/ethereum-optimism/optimism/actions/workflows/geth.yml?query=branch%3Amaster"><img src="https://github.com/ethereum-optimism/optimism/workflows/geth%20unit%20tests/badge.svg" /></a>
</p>

## TL;DR

This is the primary place where [Optimism](https://optimism.io) works on stuff related to [Optimistic Ethereum](https://research.paradigm.xyz/optimism).

## Documentation

Extensive documentation is available [here](http://community.optimism.io/docs/).

## Community

* [Come hang on discord](https://discord.optimism.io)

## Directory Structure

* [`packages`](./packages): Contains all the typescript packages and contracts
  * [`contracts`](./packages/contracts): Solidity smart contracts implementing the OVM
  * [`core-utils`](./packages/core-utils): Low-level utilities and encoding packages
  * [`common-ts`](./packages/common-ts): Common tools for TypeScript code that runs in Node
  * [`hardhat-ovm`](./packages/hardhat-ovm): Hardhat plugin which enables the [OVM Compiler](https://github.com/ethereum-optimism/solidity)
  * [`smock`](./packages/smock): Testing utility for mocking smart contract return values and storage
  * [`data-transport-layer`](./packages/data-transport-layer): Event indexer, allowing the `l2geth` node to access L1 data
  * [`batch-submitter`](./packages/batch-submitter): Daemon for submitting L2 transaction and state root batches to L1
  * [`message-relayer`](./packages/message-relayer): Service for relaying L2 messages to L1
  * [`replica-healthcheck`](./packages/replica-healthcheck): Service to monitor the health of different replica deployments
* [`l2geth`](./l2geth): Fork of [go-ethereum v1.9.10](https://github.com/ethereum/go-ethereum/tree/v1.9.10) implementing the [OVM](https://research.paradigm.xyz/optimism#optimistic-geth).
* [`integration-tests`](./integration-tests): Integration tests between a L1 testnet, `l2geth`,
* [`ops`](./ops): Contains Dockerfiles for containerizing each service involved in the protocol,
as well as a docker-compose file for bringing up local testnets easily

## Contributing

Read through [CONTRIBUTING.md](./CONTRIBUTING.md) for a general overview of our contribution process.
Follow the [Development Quick Start](#development-quick-start) to set up your local development environment.

### Good First Issues

You can find good first issues by filtering for the ["good first issue" tag on our issues page](https://github.com/ethereum-optimism/optimism/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) or alternatively by taking a look at our [Good First Issues project board](https://github.com/orgs/ethereum-optimism/projects/23).
If you'd like to tackle one of these issues, please leave a comment and [assign yourself to the issue](https://docs.github.com/en/issues/tracking-your-work-with-issues/assigning-issues-and-pull-requests-to-other-github-users#assigning-an-individual-issue-or-pull-request).
This helps prevent two people accidentally working on the same task at the same time.

### Changesets

We use [changesets](https://github.com/atlassian/changesets) to manage releases of our various packages.
You *must* include a `changeset` file in your PR when making a change that would require a new package release.

Adding a `changeset` file is easy:

1. Navigate to the root of the monorepo.
2. Run `yarn changeset`. You'll be prompted to select packages to include in the changeset. Use the arrow keys to move the cursor up and down, hit the `spacebar` to select a package, and hit `enter` to confirm your selection. Select *all* packages that require a new release as a result of your PR.
3. Once you hit `enter` you'll be prompted to decide whether your selected packages need a `major`, `minor`, or `patch` release. We follow the [Semantic Versioning](https://semver.org/) scheme. Please avoid using `major` releases for any packages that are still in version `0.y.z`.
4. Commit your changeset and push it into your PR. The changeset bot will notice your changeset file and leave a little comment to this effect on GitHub.
5. Voilà, c'est fini!

### Rebasing

We use the `git rebase` command to keep our commit history tidy.
Rebasing is an easy way to make sure that each PR includes a series of clean commits with descriptive commit messages
See [this tutorial](https://docs.gitlab.com/ee/topics/git/git_rebase.html) for a detailed explanation of `git rebase` and how you should use it to maintain a clean commit history.

## Development Quick Start

### Dependencies

You'll need the following:

* [Git](https://git-scm.com/downloads)
* [NodeJS](https://nodejs.org/en/download/)
* [Yarn](https://classic.yarnpkg.com/en/docs/install)
* [Docker](https://docs.docker.com/get-docker/)
* [Docker Compose](https://docs.docker.com/compose/install/)

### Setup

Clone the repository, open it, and install nodejs packages with `yarn`:

```bash
git clone git@github.com:ethereum-optimism/optimism.git
cd optimism
yarn install
```

### Building the TypeScript packages

To build all of the [TypeScript packages](./packages), run:

```bash
yarn clean
yarn build
```

Packages compiled when on one branch may not be compatible with packages on a different branch.
**You should recompile all packages whenever you move from one branch to another.**
Use the above commands to recompile the packages.

### Building the rest of the system

If you want to run an Optimistic Ethereum node OR **if you want to run the integration tests**, you'll need to build the rest of the system.

```bash
cd ops
export COMPOSE_DOCKER_CLI_BUILD=1 # these environment variables significantly speed up build time
export DOCKER_BUILDKIT=1
docker-compose build
```

This will build the following containers:

* [`builder`](https://hub.docker.com/r/ethereumoptimism/builder): used to build the TypeScript packages
* [`l1_chain`](https://hub.docker.com/r/ethereumoptimism/hardhat): simulated L1 chain using hardhat-evm as a backend
* [`deployer`](https://hub.docker.com/r/ethereumoptimism/deployer): process that deploys L1 smart contracts to the L1 chain
* [`dtl`](https://hub.docker.com/r/ethereumoptimism/data-transport-layer): service that indexes transaction data from the L1 chain
* [`l2geth`](https://hub.docker.com/r/ethereumoptimism/l2geth): L2 geth node running in Sequencer mode
* [`verifier`](https://hub.docker.com/r/ethereumoptimism/go-ethereum): L2 geth node running in Verifier mode
* [`relayer`](https://hub.docker.com/r/ethereumoptimism/message-relayer): helper process that relays messages between L1 and L2
* [`batch_submitter`](https://hub.docker.com/r/ethereumoptimism/batch-submitter): service that submits batches of Sequencer transactions to the L1 chain
* [`integration_tests`](https://hub.docker.com/r/ethereumoptimism/integration-tests): integration tests in a box

If you want to make a change to a container, you'll need to take it down and rebuild it.
For example, if you make a change in l2geth:

```bash
cd ops
docker-compose stop -- l2geth
docker-compose build -- l2geth
docker-compose start l2geth
```

For the typescript services, you'll need to rebuild the `builder` so that the compiled
files are re-generated, and then your service, e.g. for the batch submitter

```bash
cd ops
docker-compose stop -- batch_submitter
docker-compose build -- builder batch_submitter
docker-compose start batch_submitter
```

Source code changes can have an impact on more than one container.
**If you're unsure about which containers to rebuild, just rebuild them all**:

```bash
cd ops
docker-compose down
docker-compose build
docker-compose up
```

Finally, **if you're running into weird problems and nothing seems to be working**, run:

```bash
cd optimism
yarn clean
yarn build
cd ops
docker-compose down -v
docker-compose build
docker-compose up
```

#### Viewing docker container logs

By default, the `docker-compose up` command will show logs from all services, and that
can be hard to filter through. In order to view the logs from a specific service, you can run:

```bash
docker-compose logs --follow <service name>
```

### Running tests

Before running tests: **follow the above instructions to get everything built.**

#### Running unit tests

Run unit tests for all packages in parallel via:

```bash
yarn test
```

To run unit tests for a specific package:

```bash
cd packages/package-to-test
yarn test
```

#### Running integration tests

Follow above instructions for building the whole stack.
Build and run the integration tests:

```bash
cd integration-tests
yarn build:integration
yarn test:integration
```

### Generating test coverage:
```shell
yarn test:coverage
```

Coverage is generated using a combination of `solidity-coverage` and `istanbuljs`'s `nyc` depending on which directory you're in (`solidity-coverage` inside `contracts` and `nyc` in ts packages).


## Branching Model and Releases

<!-- TODO: explain about changesets + how we do npm publishing + docker publishing -->

### Active Branches

| Branch          | Status                                                                           |
| --------------- | -------------------------------------------------------------------------------- |
| [master](https://github.com/ethereum-optimism/optimism/tree/master/)                   | Accepts PRs from `develop` when we intend to deploy to mainnet.                                      |
| [develop](https://github.com/ethereum-optimism/optimism/tree/develop/)                 | Accepts PRs that are compatible with `master` OR from `regenesis/X.X.X` branches.                    |
| regenesis/X.X.X                                                                        | Accepts PRs for all changes, particularly those not backwards compatible with `develop` and `master`. |

### Overview

We generally follow [this Git branching model](https://nvie.com/posts/a-successful-git-branching-model/).
Please read the linked post if you're planning to make frequent PRs into this repository (e.g., people working at/with Optimism).

### The `master` branch

The `master` branch contains the code for our latest "stable" releases.
Updates from `master` always come from the `develop` branch.
We only ever update the `master` branch when we intend to deploy code within the `develop` to the Optimistic Ethereum mainnet.
Our update process takes the form of a PR merging the `develop` branch into the `master` branch.

### The `develop` branch

Our primary development branch is [`develop`](https://github.com/ethereum-optimism/optimism/tree/develop/).
`develop` contains the most up-to-date software that remains backwards compatible with our latest experimental [network deployments](https://community.optimism.io/docs/developers/networks.html).
If you're making a backwards compatible change, please direct your pull request towards `develop`.

**Changes to contracts within `packages/contracts/contracts/optimistic-ethereum` are usually NOT considered backwards compatible and SHOULD be made against a release candidate branch**.
Some exceptions to this rule exist for cases in which we absolutely must deploy some new contract after a release candidate branch has already been fully deployed.
If you're changing or adding a contract and you're unsure about which branch to make a PR into, default to using the latest release candidate branch.
See below for info about release candidate branches.

### Release new versions

Developers can release new versions of the software by adding changesets to their pull requests using `yarn changeset`. Changesets will persist over time on the `develop` branch without triggering new version bumps to be proposed by the Changesets bot. Once changesets are merged into `master`, the bot will create a new pull request called "Version Packages" which bumps the versions of packages. The correct flow for triggering releases is to re-base these pull requests onto `develop` and merge them, and then create a new pull request to merge `develop` onto `master`. Then, the `release` workflow will trigger the actual publishing to `npm` and Docker hub.

### Release candidate branches

Branches marked `regenesis/X.X.X` are **release candidate branches**.
Changes that are not backwards compatible and all changes to contracts within `packages/contracts/contracts/optimistic-ethereum` MUST be directed towards a release candidate branch.
Release candidates are merged into `develop` and then into `master` once they've been fully deployed.
We may sometimes have more than one active `regenesis/X.X.X` branch if we're in the middle of a deployment.
See table in the **Active Branches** section above to find the right branch to target.

## Additional Reference Material

### Running contract static analysis

We perform static analysis with [`slither`](https://github.com/crytic/slither).
You must have Python 3.x installed to run `slither`.
To run `slither` locally, do:

```bash
cd packages/contracts
pip3 install slither-analyzer
yarn test:slither
```

## License

Code forked from [`go-ethereum`](https://github.com/ethereum/go-ethereum) under the name [`l2geth`](https://github.com/ethereum-optimism/optimism/tree/master/l2geth) is licensed under the [GNU GPLv3](https://gist.github.com/kn9ts/cbe95340d29fc1aaeaa5dd5c059d2e60) in accordance with the [original license](https://github.com/ethereum/go-ethereum/blob/master/COPYING).

All other files within this repository are licensed under the [MIT License](https://github.com/ethereum-optimism/optimism/blob/master/LICENSE) unless stated otherwise.
