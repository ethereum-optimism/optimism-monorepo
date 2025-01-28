#!/bin/bash

# Array of paths that should not change
FROZEN_PATHS=(
  "packages/contracts-bedrock/src/L1/"
  "packages/contracts-bedrock/src/dispute/"
)

changed_paths=()
# Check each frozen path
for path in "${FROZEN_PATHS[@]}"; do
  # Get all changes from working directory, staged files, and branch diff
  changes=$({
    git diff origin/develop...HEAD --name-only
    git diff --name-only
    git diff --cached --name-only
  })

  # Check if any changes match this frozen path
  if echo "$changes" | grep -q "$path"; then
    # Extract the specific changed files in this path
    changed_files=$(echo "$changes" | grep "$path")
    changed_paths+=("$changed_files")
  fi
done

if [ ${#changed_paths[@]} -gt 0 ]; then
  echo "These path(s) should not be modified:"
fi
for path in "${changed_paths[@]}"; do
  echo "$path"
done
if [ ${#changed_paths[@]} -gt 0 ]; then
  exit 1
fi

echo "No changes detected in frozen paths"
exit 0
