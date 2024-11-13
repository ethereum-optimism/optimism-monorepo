#!/usr/bin/env bash
set -euo pipefail

# Grab the directory of the contracts-bedrock package.
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# Load semver-utils.
# shellcheck source=/dev/null
source "$SCRIPT_DIR/utils/semver-utils.sh"

# Path to semver-lock.json.
SEMVER_LOCK="snapshots/semver-lock.json"

# Create a temporary directory.
temp_dir=$(mktemp -d)
trap 'rm -rf "$temp_dir"' EXIT

# Exit early if semver-lock.json has not changed.
if ! { git diff origin/develop...HEAD --name-only; git diff --name-only; git diff --cached --name-only; } | grep -q "$SEMVER_LOCK"; then
    echo "No changes detected in semver-lock.json"
    exit 0
fi

# Get the upstream semver-lock.json.
if ! git show origin/develop:packages/contracts-bedrock/snapshots/semver-lock.json > "$temp_dir/upstream_semver_lock.json" 2>/dev/null; then
      echo "❌ Error: Could not find semver-lock.json in the snapshots/ directory of develop branch"
      exit 1
fi

# Copy the local semver-lock.json.
cp "$SEMVER_LOCK" "$temp_dir/local_semver_lock.json"

# Get the changed contracts.
changed_contracts=$(jq -r '
    def changes:
        to_entries as $local
        | input as $upstream
        | $local | map(
            select(
                .key as $key
                | .value != $upstream[$key]
            )
        ) | map(.key);
    changes[]
' "$temp_dir/local_semver_lock.json" "$temp_dir/upstream_semver_lock.json")

# Flag to track if any errors are detected.
has_errors=false

# Check each changed contract for a semver version change.
for contract in $changed_contracts; do
    # Check if the contract file exists.
    if [ ! -f "$contract" ]; then
        echo "❌ Error: Contract file $contract not found"
        has_errors=true
        continue
    fi

    # Extract the old and new source files.
    old_source_file="$temp_dir/old_${contract##*/}"
    new_source_file="$temp_dir/new_${contract##*/}"
    git show origin/develop:packages/contracts-bedrock/"$contract" > "$old_source_file" 2>/dev/null || true
    cp "$contract" "$new_source_file"

    # Extract the old and new versions.
    old_version=$(extract_version "$old_source_file" 2>/dev/null || echo "N/A")
    new_version=$(extract_version "$new_source_file" 2>/dev/null || echo "N/A")

    # Check if the versions were extracted successfully.
    if [ "$old_version" = "N/A" ] || [ "$new_version" = "N/A" ]; then
        echo "❌ Error: unable to extract version for $contract"
        echo "          this is probably a bug in check-semver-diff.sh"
        echo "          please report or fix the issue if possible"
        has_errors=true
    fi

    # TODO: Use an existing semver comparison function since this will only
    # check if the version has changed at all and not that the version has
    # increased properly.
    # Check if the version changed.
    if [ "$old_version" = "$new_version" ]; then
        echo "❌ Error: $contract has changes in semver-lock.json but no version change"
        echo "   Old version: $old_version"
        echo "   New version: $new_version"
        has_errors=true
    else
        echo "✅ $contract: version changed from $old_version to $new_version"
    fi
done

# Exit with error if any issues were found.
if [ "$has_errors" = true ]; then
    exit 1
fi
