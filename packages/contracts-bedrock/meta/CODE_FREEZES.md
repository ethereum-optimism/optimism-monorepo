# Smart Contract Code Freeze Process

The Smart Contract Freeze Process is used to protect specific files from accidental changes during sensitive periods.

## Code Freeze

Code freezes are implemented by comparison of the bytecode and source code hashes of the local file against the upstream files.

To enable a code freeze, follow these steps:

1. Create a PR.
2. The `semver-lock.json` file should already be up to date, but run anyway `just semver-lock` to be sure.
3. Comment out the path and filename of the file/s you want to freeze in check-frozen-files.sh.

To disable a code freeze, comment out the path and filename of the file/s you want to unfreeze in check-frozen-files.sh.
1. Create a PR.
2. Uncomment the path and filename of all files in check-frozen-files.sh.

## Exceptions

To bypass the freeze you can apply the "M-exempt-frozen-files" label on affected PRs. This should be done upon agreement with the code owner. Expected uses of this exception are to fix issues found on audits or to add comments to frozen files.

