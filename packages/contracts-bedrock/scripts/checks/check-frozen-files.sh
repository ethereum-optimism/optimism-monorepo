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

FROZEN_FILES=(
  "src/L1/DataAvailabilityChallenge.sol"
  "src/L1/L1CrossDomainMessenger.sol"
  "src/L1/L1ERC721Bridge.sol"
  "src/L1/L1StandardBridge.sol"
  "src/L1/OptimismPortal2.sol"
  "src/L1/ProtocolVersions.sol"
  "src/L1/SuperchainConfig.sol"
  "src/L1/SystemConfig.sol"
  "src/dispute/AnchorStateRegistry.sol"
  "src/dispute/DelayedWETH.sol"
  "src/dispute/DisputeGameFactory.sol"
  "src/dispute/FaultDisputeGame.sol"
  "src/dispute/PermissionedDisputeGame.sol"
  "src/cannon/MIPS.sol"
  "src/cannon/MIPS2.sol"
# TODO(#14116): Add MIPS64 back when development is finished
#  "src/cannon/MIPS64.sol"
  "src/cannon/PreimageOracle.sol"
)

MATCHED_FILES=()
# Check each changed contract against protected patterns
for contract in $changed_contracts; do
    for frozen_file in "${FROZEN_FILES[@]}"; do
        if [[ "$contract" == "$frozen_file" ]]; then
            MATCHED_FILES+=("$contract")
        fi
    done
done


if [ ${#MATCHED_FILES[@]} -gt 0 ]; then
    echo "❌ Error: The following files should not be modified:"
    printf '  - %s\n' "${MATCHED_FILES[@]}"
    echo "In order to make changes to these contracts, they must be removed from the FROZEN_FILES array in check-frozen-files.sh"
    echo "The code freeze is expected to be lifted no later than 2025-02-20."
    exit 1
fi

echo "✅ No changes detected in frozen files"
exit 0
