# Optimism Monorepo Contributing Guide

## What to Contribute

Welcome to the Optimism Monorepo Contributing Guide!
If you're reading this then you might be interested in contributing to the Optimism Monorepo.
Before diving into the specifics of this repository, you might be interested in taking a quick look at just a few of the ways that you can contribute.
You can:

- Report issues in this repository. Great bug reports are detailed and give clear instructions for how a developer can reproduce the problem. Write good bug reports and developers will love you.
  - **IMPORTANT**: If you believe your report impacts the security of this repository, refer to the canonical [Security Policy](https://github.com/ethereum-optimism/.github/blob/master/SECURITY.md) document.
- Fix issues that are tagged as [`D-good-first-issue`](https://github.com/ethereum-optimism/optimism/labels/D-good-first-issue) or [`S-confirmed`](https://github.com/ethereum-optimism/optimism/labels/S-confirmed).
- Larger projects are listed on [this project board](https://github.com/orgs/ethereum-optimism/projects/31/views/9). Please talk to us if you're considering working on one of these, they may not be fully specified so it will reduce risk to discuss the approach and ensure that it's still relevant.
- Help improve the [Optimism Developer Docs](https://github.com/ethereum-optimism/docs) by reporting issues, fixing typos, or adding missing sections.
- Get involved in the protocol design process by joining discussions within the [OP Stack Specs](https://github.com/ethereum-optimism/specs/discussions) repository.

## Code of Conduct

Interactions within this repository are subject to a [Code of Conduct](https://github.com/ethereum-optimism/.github/blob/master/CODE_OF_CONDUCT.md) adapted from the [Contributor Covenant](https://www.contributor-covenant.org/version/1/4/code-of-conduct/).

## Development Quick Start

### Software Dependencies

| Dependency                                                    | Version  | Version Check Command    |
| ------------------------------------------------------------- | -------- | ------------------------ |
| [git](https://git-scm.com/)                                   | `^2`     | `git --version`          |
| [go](https://go.dev/)                                         | `^1.21`  | `go version`             |
| [node](https://nodejs.org/en/)                                | `^20`    | `node --version`         |
| [nvm](https://github.com/nvm-sh/nvm)                          | `^0.39`  | `nvm --version`          |
| [just](https://github.com/casey/just)                         | `^1.34.0`| `just --version`         |
| [foundry](https://github.com/foundry-rs/foundry#installation) | `^0.2.0` | `forge --version`        |
| [make](https://linux.die.net/man/1/make)                      | `^3`     | `make --version`         |
| [jq](https://github.com/jqlang/jq)                            | `^1.6`   | `jq --version`           |
| [direnv](https://direnv.net)                                  | `^2`     | `direnv --version`       |
| [docker](https://docs.docker.com/get-docker/)                 | `^24`    | `docker --version`       |
| [docker compose](https://docs.docker.com/compose/install/)    | `^2.23`  | `docker compose version` |

### Notes on Specific Dependencies

#### `node`

Make sure to use the version of `node` specified within [`.nvmrc`](./.nvmrc).
You can use [`nvm`](https://github.com/nvm-sh/nvm) to manage multiple versions of Node.js on your machine and automatically switch to the correct version when you enter this repository.

#### `foundry`

`foundry` is updated frequently and occasionally contains breaking changes.
This repository pins a specific version of `foundry` inside of [`versions.json`](./versions.json).
Use the command `just update-foundry` at the root of the monorepo to make sure that your version of `foundry` is the same as the one currently being used in CI.

#### `direnv`

[`direnv`](https://direnv.net) is a tool used to load environment variables from [`.envrc`](./.envrc) into your shell so you don't have to manually export variables every time you want to use them.
`direnv` only has access to files that you explicitly allow it to see.
After [installing `direnv`](https://direnv.net/docs/installation.html), you will need to **make sure that [`direnv` is hooked into your shell](https://direnv.net/docs/hook.html)**.
Make sure you've followed [the guide on the `direnv` website](https://direnv.net/docs/hook.html), then **close your terminal and reopen it** so that the changes take effect (or `source` your config file if you know how to do that).

#### `docker compose`

[Docker Desktop](https://docs.docker.com/get-docker/) should come with `docker compose` installed by default.
You'll have to install the `compose` plugin if you're not using Docker Desktop or you're on linux.

### Setting Up

Clone the repository and open it:

```bash
git clone git@github.com:ethereum-optimism/optimism.git
cd optimism
```

### Building the Monorepo

Make sure that you've installed all of the required [Software Dependencies](#software-dependencies) before you continue.
You will need [foundry](https://github.com/foundry-rs/foundry) to build the smart contracts found within this repository.
Refer to the note on [foundry as a dependency](#foundry) for instructions.

Install dependencies and build all packages within the monorepo by running:

```bash
make build
```

Packages built on one branch may not be compatible with packages on a different branch.
**You should rebuild the monorepo whenever you move from one branch to another.**
Use the above command to rebuild the monorepo.

### Running tests

Before running tests: **follow the above instructions to get everything built.**

#### Running unit tests (solidity)

```bash
cd packages/contracts-bedrock
just test
```

#### Running unit tests (Go)

Change directory to the package you want to run tests for, then:

```shell
go test ./...
```

#### Running e2e tests (Go)

See [this document](./op-e2e/README.md)

#### Running contract static analysis

We perform static analysis with [`slither`](https://github.com/crytic/slither).
You must have Python 3.x installed to run `slither`.
To run `slither` locally, do:

```bash
cd packages/contracts-bedrock
pip3 install slither-analyzer
just slither
```

## Labels

Labels are divided into categories with their descriptions annotated as `<Category Name>: <description>`.

The following are a comprehensive list of label categories.

- **Area labels** ([`A-`][area]): Denote the general area for the related issue or PR changes.
- **Category labels** ([`C-`][category]): Contextualize the type of issue or change.
- **Meta labels** ([`M-`][meta]): These add context to the issues or prs themselves primarily relating to process.
- **Difficulty labels** ([`D-`][difficulty]): Describe the associated implementation's difficulty level.
- **Status labels** ([`S-`][status]): Specify the status of an issue or pr.

Labels also provide a versatile filter for finding tickets that need help or are open for assignment.
This makes them a great tool for contributors!

[area]: https://github.com/ethereum-optimism/optimism/labels?q=a-
[category]: https://github.com/ethereum-optimism/optimism/labels?q=c-
[meta]: https://github.com/ethereum-optimism/optimism/labels?q=m-
[difficulty]: https://github.com/ethereum-optimism/optimism/labels?q=d-
[status]: https://github.com/ethereum-optimism/optimism/labels?q=s-

### Filtering for Work

To find tickets available for external contribution, take a look at the https://github.com/ethereum-optimism/optimism/labels/M-community label.

You can filter by the https://github.com/ethereum-optimism/optimism/labels/D-good-first-issue
label to find issues that are intended to be easy to implement or fix.

Also, all labels can be seen by visiting the [labels page][labels]

[labels]: https://github.com/ethereum-optimism/optimism/labels

### Modifying Labels

When altering label names or deleting labels there are a few things you must be aware of.

- If the https://github.com/ethereum-optimism/optimism/labels/S-stale label is altered, the [close-stale](.github/workflows/close-stale.yml) workflow should be updated.
- If the https://github.com/ethereum-optimism/optimism/labels/M-dependabot label is altered, the [dependabot config](.github/dependabot.yml) file should be adjusted.
- Saved label filters for project boards will not automatically update. These should be updated if label names change.

## Workflow for Pull Requests

🚨 Before making any non-trivial change, please first open an issue describing the change to solicit feedback and guidance. This will increase the likelihood of the PR getting merged.

In general, the smaller the diff the easier it will be for us to review quickly.

In order to contribute, fork the appropriate branch, for non-breaking changes to production that is `develop` and for the next release that is normally `release/X.X.X` branch, see [details about our branching model](https://github.com/ethereum-optimism/optimism/blob/develop/README.md#branching-model-and-releases).

Additionally, if you are writing a new feature, please ensure you add appropriate test cases.

Follow the [Development Quick Start](#development-quick-start) to set up your local development environment.

We recommend using the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format on commit messages.

Unless your PR is ready for immediate review and merging, please mark it as 'draft' (or simply do not open a PR yet).

Once ready for review, make sure to include a thorough PR description to help reviewers. You can read more about the guidelines for opening PRs in the [PR Guidelines](docs/handbook/pr-guidelines.md) file.

**Bonus:** Add comments to the diff under the "Files Changed" tab on the PR page to clarify any sections where you think we might have questions about the approach taken.

### Response time

We aim to provide a meaningful response to all PRs and issues from external contributors within 2 business days.

### Rebasing

We use the `git rebase` command to keep our commit history tidy.
Rebasing is an easy way to make sure that each PR includes a series of clean commits with descriptive commit messages
See [this tutorial](https://docs.gitlab.com/ee/topics/git/git_rebase.html) for a detailed explanation of `git rebase` and how you should use it to maintain a clean commit history.
