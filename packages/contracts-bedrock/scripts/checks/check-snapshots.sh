#!/usr/bin/env bash
set -euo pipefail

# Check for the --no-build flag
# Generate snapshots
if [ "${1:-}" == "--no-build" ]; then
    just snapshots-no-build
else
    just snapshots
fi

# Check if the generated `snapshots` files are different from the committed versions
if git diff --exit-code snapshots > /dev/null; then
    [ -z "$(git ls-files --others --exclude-standard snapshots)" ] || exit 1
else
    exit 1
fi
