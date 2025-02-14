# Pull Request Guidelines and Best Practices

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Overview](#overview)
- [PR Lifecycle Best Practices](#pr-lifecycle-best-practices)
  - [Before Starting PRs](#before-starting-prs)
  - [Opening PRs](#opening-prs)
  - [Reviewing PRs](#reviewing-prs)
  - [Merging PRs](#merging-prs)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Overview

This document contains guidelines and best practices in PRs that should be enforced as much as possible. The motivations and goals behind these best practices are:

- **Ensure thorough reviews**: By the time the PR is merged, at least one other person—because there is always at least one reviewer—should understand the PR’s changes just as well as the PR author. This helps improve security by reducing bugs and single points of failure (i.e. there should never be only one person who understands certain code).
- **Reduce PR churn**: PRs should be quickly reviewable and mergeable without much churn (both in terms of code rewrites and comment cycles). This saves time by reducing the need for rebases due to conflicts. Similarly, too many review cycles are a burden for both PR authors and reviewers, and results in “review fatigue” where reviews become less careful and thorough, increasing the likelihood of bugs.
- **Traceability**: We should be able to look back at issues and PRs to understand why a certain decision was made or why a given approach was taken.

## PR Lifecycle Best Practices

This is organized by current state of PR, so it can be easily referenced frequently to help internalize the guidelines.

### Before Starting PRs

- **Keep PRs Focused**: Each PR should be a single, narrow, well-defined scope.

### Opening PRs

- **Review Your Own Code**: Reviewing the diff yourself *in a different context*, can be very useful for discovering issues, typos, and bugs before opening the PR. For example, write code in your IDE, then review it in the GitHub diff view. The perspective change forces you to slow down and helps reveal issues you may have missed.
- **Explain Decisions/Tradeoffs**: Explain rationale for any design/architecture decisions and implementation details in the PR description. If it closes an issue, remember to mention the issue it closes, e.g. `Closes <issueUrl>`. Otherwise, just link to the issue. If there is no issue, whatever details would have been in the issue should be in the PR description.
- **Guide PR reviewers:** Let them know about areas of concern, under-tested areas, or vague requirements that should be ironed out.

### Triggering CI on PRs from external forks
If the PR is from an external fork, our CI suite will not automatically run on the PR. A reviewer with sufficient permissions (e.g. the automatically assigened reviewer) needs to comment on the PR wih

> /ci authorize COMMITHASH

or

> /ci authorize https://github.com/ethereum-optimism/optimism/pull/PR_NUMBER/commits/COMMITHASH

to trigger the CI suite to run. CI is a precondition for merging the PR and should be done before review is conducted, because it will reveal any failing tests or other problems such as linting errors.

> [!NOTE]
> COMMITHASH and PR_NUMBER have their usual meanings but you must use the **full** commit hash and not a shortened version. Otherwise CI will not be triggered.


### Reviewing PRs

- **Verify Requirements are Met**: If the PR claims to fix or close an issue, check that all the requirements in the issue are actually met. Otherwise the issue may be in a good place to merge, but just shouldn’t close the issue.
- **Focus on Tests**: The tests are the spec and therefore should be the focus of reviews. If tests are thorough and passing, the rest is an implementation detail (to an extent—don’t skip source code reviews) that can be fixed in a future optimization/cleanup PR. Make sure edge case behaviors are defined and handled.
- **Think like an Auditor:** What edge cases were ignored? How can the code break? When might it behave incorrectly and unexpectedly? What code should have been changed that isn’t in the diff? What implicit assumptions are made that might be invalid?
- **Ensure Comment Significance is Clear**: Indicate which comments are nits/optionals that the PR author can resolve, compared to which you want to follow up on.
    - Prefix non-blocking comments with `[nit]` or `[non-blocking]`.
- **Consider Reviewing in Your IDE**: For example, GitHub has [this VSCode extension](https://marketplace.visualstudio.com/items?itemName=GitHub.vscode-pull-request-github) to review PRs. This provides more code context and enables review to benefit from your standard lints and IDE features, whereas GitHub’s diff shows none of that.

### Merging PRs

- **Resolve all Comments**: Comments can be resolved by (1) the PR author for nits/optionals, (2) the author or reviewer after discussions, or (3) extracting the comment into an issue to address in a future PR. For (3), ensure the new issue links to the specific comment thread. This is currently enforced by GitHub's merge requirements.
- **Other Standard Merge Requirements**: The PR must be approved by the appropriate reviewers, CI must pass, and other standard merge requirements apply.
