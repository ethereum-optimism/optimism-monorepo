# Solidity Versioning Policy

This document outlines the process for proposing and implementing Solidity version updates in the
OP Stack codebase.

## Unified Solidity Version

The OP Stack codebase maintains a single, unified Solidity version across all contracts and
components. This ensures consistency, simplifies maintenance, and reduces the risk of
version-related issues.

**Important**: New Solidity versions must not be introduced to any part of the codebase without
going through the formal version update proposal process outlined in this document.

## Update Process

1. **Minimum Delay Period**: A new Solidity version must be at least 6 months old before it can be
  considered for adoption.
2. **Proposal Submission**: Before any Solidity version upgrades are made, a detailed proposal must
  be submitted as a pull request to the [`ethereum-optimism/design-docs`][1] repository in the
  `solidity/` subfolder, following the standardized format outlined below. This applies to the
  entire codebase; individual components or contracts cannot be upgraded separately.
3. **Review and Approval**: A dedicated review panel will assess the proposal based on the
  following criteria:
    - Is the Solidity version at least 6 months old?
    - Does the proposed upgrade provide clear value to the codebase?
    - Do any new features or bug fixes pose an unnecessary risk to the codebase?
4. **Implementation**: If the proposal receives unanimous approval from the review panel, the
  Solidity version upgrade will be implemented across the entire OP Stack codebase.

## Proposal Submission Guidelines

To submit a Solidity version upgrade proposal, create a new pull request to the
[`ethereum-optimism/design-docs`][1] repository, adding a new file in the [`solidity/`][2]
subfolder. Please use the dedicated [Solidity update proposal format][3]. Ensure that all sections
are filled out comprehensively. Incomplete proposals may be delayed or rejected.

## Review Process

The review panel will evaluate each proposal based on the criteria mentioned in the "Review and Approval" section above. They may request additional information or clarifications if needed.

## Implementation

If approved, the Solidity version upgrade will be implemented across the entire OP Stack codebase. This process will be managed by the development team to ensure consistency and minimize potential issues. The upgrade will apply to all contracts and components simultaneously.

<!-- References -->
[1]: https://github.com/ethereum-optimism/design-docs
[2]: https://github.com/ethereum-optimism/design-docs/tree/main/solidity
[3]: https://github.com/ethereum-optimism/design-docs/tree/main/assets/solc-update-template.md
