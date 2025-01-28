#!/bin/bash

# Array of paths that should not change
FROZEN_PATHS=(
  "src/L1"
  "src/dispute"
)

# Check each frozen path
for path in "${FROZEN_PATHS[@]}"; do
  # Check if there are any changes in git for this path
  if git diff --name-only origin/develop...HEAD -- "$path" | grep -q .; then
    echo "Error: Changes detected in frozen path: $path"
    echo "These paths should not be modified."
    exit 1
  fi
done

# No changes detected in frozen paths
echo "No changes detected in frozen paths"
exit 0
