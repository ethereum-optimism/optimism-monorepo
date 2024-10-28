# Contributing to CONTRIBUTING.md

First off, thanks for taking the time to contribute!

We welcome and appreciate all kinds of contributions. We ask that before contributing you please review the procedures for each type of contribution available in the [Table of Contents](#table-of-contents). This will streamline the process for both maintainers and contributors. To find ways to contribute, view the [I Want To Contribute](#i-want-to-contribute) section below. Larger contributions should [open an issue](https://github.com/ethereum-optimism/optimism/issues/new) before implementation to ensure changes don't go to waste.

We're excited to work with you and your contributions to scaling Ethereum!

## Table of Contents

- [I Have a Question](#i-have-a-question)
- [I Want To Contribute](#i-want-to-contribute)
- [Reporting Bugs](#reporting-bugs)
- [Suggesting Enhancements](#suggesting-enhancements)
- [Your First Code Contribution](#your-first-code-contribution)
- [Improving The Documentation](#improving-the-documentation)
- [Deploying on Devnet](#deploying-on-devnet)
- [Tools](#tools)

## I Have a Question

> **Note**
> Before making an issue, please read the documentation and search the issues to see if your question has already been answered.

If you have any questions about the smart contracts, please feel free to ask them in the Optimism discord developer channels or create a new detailed issue.

## I Want To Contribute

### Reporting Bugs

**Any and all bug reports on production smart contract code should be submitted privately to the Optimism team so that we can mitigate the issue before it is exploited. Please see our security policy document [here](https://github.com/ethereum-optimism/.github/blob/master/SECURITY.md).**

### Suggesting Enhancements

#### Before Submitting an Enhancement

- Read the documentation and the smart contracts themselves to see if the feature already exists.
- Perform a search in the issues to see if the enhancement has already been suggested. If it has, add a comment to the existing issue instead of opening a new one.

#### How Do I Submit a Good Enhancement Suggestion?

Enhancement suggestions are tracked as [GitHub issues](https://github.com/ethereum-optimism/optimism/issues).

- Use a **clear and descriptive title** for the issue to identify the suggestion.
- Provide a **step-by-step** description of the suggested enhancement in as many details as possible.
- Describe the **current** behavior and why the **intended** behavior you expected to see differs. At this point you can also tell which alternatives do not work for you.
- Explain why this enhancement would be useful in Optimism's smart contracts. You may also want to point out the other projects that solved it better and which could serve as inspiration.

### Your First Code Contribution

The best place to begin contributing is by looking through the issues with the `good first issue` label. These are issues that are relatively easy to implement and are a great way to get familiar with the codebase.

Optimism's smart contracts are written in Solidity and we use [foundry](https://github.com/foundry-rs/foundry) as our development framework. To get started, you'll need to install several dependencies:

1. [just](https://github.com/casey/just)
   Make sure to `just install`
1. [foundry](https://getfoundry.sh)
   Foundry is built with [rust](https://www.rust-lang.org/tools/install), and this project uses a pinned version of foundry. Install the rust toolchain with `rustup`.
   Make sure to install the version of foundry used by `ci-builder`, defined in the `versions.json` file in the root of this repo under the `foundry` key. Once you have `foundryup` installed, there is a helper to do this: `just install-foundry`
1. [golang](https://golang.org/doc/install)
1. [python](https://www.python.org/downloads/)

Our [Style Guide](STYLE_GUIDE.md) contains information about the project structure, syntax preferences, naming conventions, and more. Please take a look at it before submitting a PR, and let us know if you spot inconsistencies!

Once you've read the style guide and are ready to work on your PR, there are a plethora of useful `just` scripts to know about that will help you with development.
You can run `just -l` to list them all, some of the key ones are:

1. `just build` Builds the smart contracts.
1. `just test` Runs the full `forge` test suite.
1. `just gas-snapshot` Generates the gas snapshot for the smart contracts.
1. `just semver-lock` Generates the semver lockfile.
1. `just snapshots` Generates the storage and ABI snapshots.
1. `just clean` Removes all build artifacts for `forge` and `go` compilations.
1. `just validate-spacers` Validates the positions of the storage slot spacers.
1. `just validate-deploy-configs` Validates the deployment configurations in `deploy-config`
1. `just lint` Runs the linter on the smart contracts and scripts.
1. `just pre-pr` Runs most checks, generators, and linters prior to a PR. For most PRs, this is sufficient to pass CI if everything is in order.
1. `just pre-pr-full` Runs all checks, generators, and linters prior to a PR.

### Improving The Documentation

Documentation improvements are more than welcome! If you see a typo or feel that a code comment describes something poorly or incorrectly, please submit a PR with a fix.

### Deploying on Devnet

To deploy the smart contracts on a local devnet, run `make devnet-up` in the monorepo root. For more information on the local devnet, see [dev-node](https://docs.optimism.io/chain/testing/dev-node).

### Tools

#### Validate Spacing

In order to make sure that we don't accidentally overwrite storage slots, contract storage layouts are checked to make sure spacing is correct.

This uses the `snapshots/storageLayout` directory to check contract spacing. Run `just validate-spacers` to check the spacing of all contracts.

#### Gas Snapshots

We use forge's `gas-snapshot` subcommand to produce a gas snapshot for tests in `Benchmark.t.sol`. CI will check that the gas snapshot has been updated properly when it runs, so make sure to run `just gas-snapshot`!

#### Semver Locking

Many of our smart contracts are semantically versioned. To make sure that changes are not made to a contract without deliberately bumping its version, we commit to the source code and the creation bytecode of its dependencies in a lockfile. Consult the [Style Guide](./STYLE_GUIDE.md#Versioning) for more information about how our contracts are versioned.

#### Storage Snapshots

Due to the many proxied contracts in Optimism's protocol, we automate tracking the diff to storage layouts of the contracts in the project. This is to ensure that we don't break a proxy by upgrading its implementation to a contract with a different storage layout. To generate the storage lockfile, run `just snapshots`.
