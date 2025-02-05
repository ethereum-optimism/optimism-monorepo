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

# List of files that are allowed to be modified.
# In order to prevent a file from being modified, comment it out. Do not delete it.
# All files in semver-lock.json should be in this list.
ALLOWED_FILES=(
  "src/L1/DataAvailabilityChallenge.sol"
  # "src/L1/L1CrossDomainMessenger.sol"
  # "src/L1/L1ERC721Bridge.sol"
  # "src/L1/L1StandardBridge.sol"
  # "src/L1/OPContractsManager.sol"
  # "src/L1/OPContractsManagerInterop.sol"
  # "src/L1/OptimismPortal2.sol"
  # "src/L1/OptimismPortalInterop.sol"
  # "src/L1/ProtocolVersions.sol"
  # "src/L1/SuperchainConfig.sol"
  # "src/L1/SystemConfig.sol"
  # "src/L1/SystemConfigInterop.sol"
  "src/L2/BaseFeeVault.sol"
  "src/L2/CrossL2Inbox.sol"
  "src/L2/ETHLiquidity.sol"
  "src/L2/GasPriceOracle.sol"
  "src/L2/L1Block.sol"
  "src/L2/L1BlockInterop.sol"
  "src/L2/L1FeeVault.sol"
  "src/L2/L2CrossDomainMessenger.sol"
  "src/L2/L2ERC721Bridge.sol"
  "src/L2/L2StandardBridge.sol"
  "src/L2/L2StandardBridgeInterop.sol"
  "src/L2/L2ToL1MessagePasser.sol"
  "src/L2/L2ToL2CrossDomainMessenger.sol"
  "src/L2/OptimismMintableERC721.sol"
  "src/L2/OptimismMintableERC721Factory.sol"
  "src/L2/OptimismSuperchainERC20.sol"
  "src/L2/OptimismSuperchainERC20Beacon.sol"
  "src/L2/OptimismSuperchainERC20Factory.sol"
  "src/L2/SequencerFeeVault.sol"
  "src/L2/SuperchainERC20.sol"
  "src/L2/SuperchainTokenBridge.sol"
  "src/L2/SuperchainWETH.sol"
  "src/L2/WETH.sol"
  "src/cannon/MIPS.sol"
  "src/cannon/MIPS2.sol"
  # TODO(#14116): Disallow MIPS64 back when development is finished
  "src/cannon/MIPS64.sol"
  "src/cannon/PreimageOracle.sol"
  # "src/dispute/AnchorStateRegistry.sol"
  # "src/dispute/DelayedWETH.sol"
  # "src/dispute/DisputeGameFactory.sol"
  # "src/dispute/FaultDisputeGame.sol"
  # "src/dispute/PermissionedDisputeGame.sol"
  "src/legacy/DeployerWhitelist.sol"
  "src/legacy/L1BlockNumber.sol"
  "src/legacy/LegacyMessagePasser.sol"
  "src/safe/DeputyGuardianModule.sol"
  "src/safe/DeputyPauseModule.sol"
  "src/safe/LivenessGuard.sol"
  "src/safe/LivenessModule.sol"
  "src/universal/OptimismMintableERC20.sol"
  "src/universal/OptimismMintableERC20Factory.sol"
  "src/universal/StorageSetter.sol"
  "src/vendor/asterisc/RISCV.sol"
  "src/vendor/eas/EAS.sol"
  "src/vendor/eas/SchemaRegistry.sol"
)

MATCHED_FILES=()
# Check each changed contract against allowed patterns
for contract in $changed_contracts; do
    is_allowed=false
    for allowed_file in "${ALLOWED_FILES[@]}"; do
        if [[ "$contract" == "$allowed_file" ]]; then
            is_allowed=true
            break
        fi
    done
    if [[ "$is_allowed" == "false" ]]; then
        MATCHED_FILES+=("$contract")
    fi
done

if [ ${#MATCHED_FILES[@]} -gt 0 ]; then
    echo "❌ Error: Changes detected in files that are not allowed to be modified."
    echo "The following files were modified but are not in the allowed list:"
    printf '  - %s\n' "${MATCHED_FILES[@]}"
    echo "Only the following files can be modified:"
    printf '  - %s\n' "${ALLOWED_FILES[@]}"
    echo "The code freeze is expected to be lifted no later than 2025-02-20."
    exit 1
fi

echo "✅ All changes are in allowed files"
exit 0
