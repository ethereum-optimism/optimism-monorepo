# Smart Contract Versioning and Release Process

The Smart Contract Versioning and Release Process closely follows a true [semver](https://semver.org) for both individual contracts and monorepo releases.
However, there are some changes to accommodate the unique nature of smart contract development and governance cycles.

There are five parts to the versioning and release process:

- [Semver Rules](#semver-rules): Follows the rules defined in the [style guide](./STYLE_GUIDE.md#versioning) for when to bump major, minor, and patch versions in individual contracts.
- [Individual Contract Versioning](#individual-contract-versioning): The versioning scheme for individual contracts and includes beta, release candidate, and feature tags.
- [Monorepo Contracts Release Versioning](#monorepo-contracts-release-versioning): The versioning scheme for monorepo smart contract releases.
- [Release Process](#release-process): The process for deploying contracts, creating a governance proposal, and the required associated releases.
  - [Additional Release Candidates](#additional-release-candidates): How to handle additional release candidates after an initial `op-contracts/vX.Y.Z-rc.1` release.
  - [Merging Back to Develop After Governance Approval](#merging-back-to-develop-after-governance-approval): Explains how to choose the resulting contract versions when merging back into `develop`.

> [!NOTE]
> The rules described in this document must be enforced manually.
> Ideally, a check can be added to CI to enforce the conventions defined here, but this is not currently implemented.

## Semver Rules

Version increments follow the [style guide rules](./STYLE_GUIDE.md#versioning) for when to bump major, minor, and patch versions in individual contracts:

> - `patch` releases are to be used only for changes that do NOT modify contract bytecode (such as updating comments).
> - `minor` releases are to be used for changes that modify bytecode OR changes that expand the contract ABI provided that these changes do NOT break the existing interface.
> - `major` releases are to be used for changes that break the existing contract interface OR changes that modify the security model of a contract.
>
> Bumping the patch version does change the bytecode, so another exception is carved out for this.
> In other words, changing comments increments the patch version, which changes bytecode. This bytecode
> change implies a minor version increment is needed, but because it's just a version change, only a
> patch increment should be used.

## Individual Contract Versioning

Versioning for individual contracts works as follows:

- A contract is only `X.Y.Z` on `develop` if it has been governance approved. If it's `X.Y.Z` before that, it must be on a branch. More on this below.
- For contracts undergoing development, a `-beta.n` identifier must be appended to the version number.
- For contracts in a release candidate state, an `-rc.n` identifier must be appended to the version number.
- For contracts with feature-specific changes, a `+feature-name` identifier must be appended to the version number. See the [Smart Contract Feature Development](https://github.com/ethereum-optimism/design-docs/blob/main/smart-contract-feature-development.md) design document to learn more.
- When making changes to a contract, always bump to the lowest possible version based on the specific change you are making. We do not want to e.g. optimistically bump to a major version, because protocol development sequencing may change unexpectedly. Use these examples to know how to bump the version:
  - Example 1: A contract is currently on `1.2.3`.
    - We don't yet know when the next release of this contract will be. However, you are simply fixing typos in comments so you bump the version to `1.2.4-beta.1`.
    - The next PR made to that same contract clarifies some comments, so it bumps the version to `1.2.4-beta.2`.
    - The next PR introduces a breaking change, which bumps the version from `1.2.4-beta.2` to `2.0.0-beta.1`. A `1.2.4-rc.1` and `1.2.4` version both never exist.
  - Example 2: A contract is currently on `2.4.7`.
    - We know the next release of this contract will be a breaking change. Regardless, as you start development by fixing typos in comments, bump the version to `2.4.8-beta.1`. This is because we may end up putting out a release before the breaking change is added.
    - Once you start working on the breaking change, bump the version to `3.0.0-beta.1`.
    - Continue to bump the beta version as you make changes. When the contract is ready for release, bump the version to `3.0.0-rc.1`.
- New contracts start at `1.0.0-beta.1`, increment the `-beta.n` counter during development, and become `1.0.0` when they are ready for production.

## Monorepo Contracts Release Versioning

Versioning for monorepo releases works as follows:

- Monorepo releases continue to follow the `op-contracts/vX.Y.Z` naming convention.
- The version used for the next release is determined by the highest version bump of any individual contract in the release.
  - Example 1: The monorepo is at `op-contracts/v1.5.0`. Clarifying comments are made in contracts, so all contracts only bump the patch version. The next monorepo release will be `op-contracts/v1.5.1`.
  - Example 2: The monorepo is at `op-contracts/v1.5.1`. Various tech debt and code is cleaned up in contracts, but no features are added, so at most, contracts bumped the minor version. The next monorepo release will be `op-contracts/v1.6.0`.
  - Example 3: The monorepo is at `op-contracts/v1.5.1`. Legacy `ALL_CAPS()` getter methods are removed from a contract, causing that contract to bump the major version. The next monorepo release will be `op-contracts/v2.0.0`.
- Feature specific monorepo releases (such as a beta release of the custom gas token feature) are supported, and should follow the guidelines in the [Smart Contract Feature Development](https://github.com/ethereum-optimism/design-docs/blob/main/smart-contract-feature-development.md) design doc. Bump the overall monorepo semver as required by the above rules, and append the `-beta,n` modifier to the version number. For example, if the last release before the custom gas token feature was `op-contracts/v1.5.1`, because the custom gas token introduces breaking changes, its beta release will be `op-contracts/v2.0.0-beta.n`.
  - A subsequent release of the custom gas token feature that fixes bugs and introduces an additional breaking change would be `op-contracts/v2.0.0-beta.2`.
  - This means `+feature-name` naming is not used for monorepo releases, only for individual contracts as described below.
- A monorepo contracts release must map to an exact set of contract semvers, and this mapping must be defined in the contract release notes which are the source of truth. See [`op-contracts/v1.4.0-rc.4`](https://github.com/ethereum-optimism/optimism/releases/tag/op-contracts%2Fv1.4.0-rc.4) for an example of what release notes should look like.

## Release Process

When a release is proposed to governance, the proposal includes a commit hash, and often the
contracts from that commit hash are already deployed to mainnet with their addresses included
in the proposal.
For example, the [Fault Proofs governance proposal](https://gov.optimism.io/t/upgrade-proposal-fault-proofs/8161) provides specific addresses that will be used.

To accommodate this, once contract changes are ready for governance approval, the release flow is:

- On the `develop` branch, bump the version of all contracts to be included in this release to their respective `X.Y.Z-rc.n`. The `X.Y.Z` here refers to the contract-specific versions, so it differs per-contract. The `-rc.n` begins as `-rc.1` for all contracts.
  - Any `-beta.n` and `+feature-name` identifiers are removed at this point.
  - Contracts that are not included as part of this release are left untouched.
- Branch off of `develop` and create a branch named `proposal/op-contracts/vX.Y.Z`. Here, `X.Y.Z` is the new version of the monorepo release.
  - Using the `proposal/` prefix signals that this branch is for a governance proposal, and intentionally does not convey whether or not the proposal has passed.
- Open a PR into the `proposal/op-contracts/vX.Y.Z` branch that removes the `-rc.1` suffixes from all contracts, and merge it into the `proposal/op-contracts/vX.Y.Z` branch.
  - After merge, the new commit on the `proposal/op-contracts/vX.Y.Z` branch is the commit hash that will be tagged as `op-contracts/vX.Y.Z-rc.1`, used to deploy the contracts, and proposed to governance.
  - Sometimes additional release candidates are needed before proposal—see [Additional Release Candidates](#additional-release-candidates) for more information on this flow.
- Once the governance approval is posted, any lock on contracts on `develop` is released.
- Once governance approves the proposal:
    - Create the official `op-contracts/vX.Y.Z` off of this `proposal/op-contracts/vX.Y.Z` branch. It should be at the same commit as the most recent release candidate.
    - Merge the proposal branch into `develop` and set the version of all contracts to the appropriate `X.Y.Z` after considering any changes made to `develop` since the release candidate was created.
  - See [Merging Back to Develop After Governance Approval](#merging-back-to-develop-after-governance-approval) for more information on how to choose the resulting contract versions when merging back into `develop`.

### Additional Release Candidates

Sometimes additional release candidate versions are needed.
The process for this is:

- Make the fixes on `develop`. Increment the `-rc.n` qualifier for the changed contracts only.
- Open a PR into the `proposal/op-contracts/vX.Y.Z` branch that incorporates the changes from `develop`.
- Open another PR to remove the new `-rc.2` version identifiers from the changed contracts. Tag the resulting commit on the proposal branch as `op-contracts/vX.Y.Z-rc.2`.
- This flow (1) ensures develop stays up to date during the release process, (2) mitigates the risk of forgetting to merge the release back into the develop branch, and (3) mitigates the risk of the merge into develop removing the required `-rc.n` version that is needed until the release is approved.

### Merging Back to Develop After Governance Approval

A release will change a set of contracts, and those contracts may have changed on `develop` since the release candidate was created.

If there have been no changes to a contract since the release candidate, the version of that contract stays at `X.Y.Z` and just has the `-rc.n` removed.
For example, if the release candidate is `1.2.3-rc.1`, the resulting version on `develop` will be `1.2.3`.

If there have been changes to a contract, the `X.Y.Z` will stay the same as whatever is the latest version on `develop`, with the `-beta.n` qualifier incremented.

For example, given that ContractA is `1.2.3-rc.1` on develop, then the initial sequence of events is:

- We create the release branch, and on that branch remove the `-rc.1`, giving a final ContractA version on that branch of `1.2.3`
- Governance proposal is posted, pointing to the corresponding monorepo tag.
- Governance approves the release.
- Open a PR to merge the final versions of the contracts (ContractA) back into develop.

Now there are two scenarios for the PR that merges the release branch back into develop:

1. On develop, no changes have been made to ContractA. The PR therefore changes ContractA's version on develop from `1.2.3-rc.1` to `1.2.3`, and no other changes to ContractA occur.
2. On develop, breaking changes have been made to ContractA for a new feature, and it's currently versioned as `2.0.0-beta.3`. The PR should bump the version to `2.0.0-beta.4` if it changes the source code of ContractA.
    - In practice, this one unlikely to occur when using inheritance for feature development, as specified in [Smart Contract Feature Development](https://github.com/ethereum-optimism/design-docs/blob/main/smart-contract-feature-development.md) architecture. It's more likely that (1) is the case, and we merge the version change into the base contract.

This flow also provides a dedicated branch for each release, making it easy to deploy a patch or bug fix, regardless of other changes that may have occurred on develop since the release.
