**Table of Contents**

<!--TOC-->

- [Policy](#policy)
  - [Contributing](#contributing)
  - [Versioning Policy](#versioning-policy)
  - [Code Freeze Policy](#code-freeze-policy)
  - [Upgrade Policy](#upgrade-policy)
  - [Style Guide](#style-guide)
  - [Revert Data](#revert-data)

<!--TOC-->

# Policy

This document outlines upgrade policies regarding the OP Stack codebase.

## Contributing

For any policies on contributing, please see [CONTRIBUTING](./CONTRIBUTING.md)

## Versioning Policy

For our versioning policy, please see our policy on [VERSIONING](./VERSIONING.md)

## Code Freeze Policy

For our code freeze policy, please see our doc on [CODE FREEZES](./CODE_FREEZES.md)

## Upgrade Policy

For the solidity upgrade policy, please see our doc on [SOLIDITY UPGRADES](./SOLIDITY_UPGRADES.md)

## Style Guide

For an indepth review of the code style used in the OP Stack contracts, please see our [STYLE GUIDE](./STYLE_GUIDE.md)

## Revert Data

Revert data may be changed in the future, and is not a reliable interface for external consumers. Contracts should not depend on specific revert data returned by OP Stack contracts, which can be changed during any future OP Stack contract upgrades. Revert data includes both custom errors returned by contracts, as a well as revert strings.
