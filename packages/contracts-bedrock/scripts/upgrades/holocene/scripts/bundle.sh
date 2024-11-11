#!/usr/bin/env bash
set -euo pipefail

# Grab the script directory
SCRIPT_DIR=$(dirname "$0")

# Load common.sh
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"

# Check the env
reqenv "ETH_RPC_URL"
reqenv "OUTPUT_FOLDER_PATH"
reqenv "SYSTEM_CONFIG_IMPL"
reqenv "MIPS_IMPL"
reqenv "FDG_IMPL"
reqenv "PDG_IMPL"
reqenv "PROXY_ADMIN_ADDR"
reqenv "SYSTEM_CONFIG_PROXY_ADDR"
reqenv "DISPUTE_GAME_FACTORY_PROXY_ADDR"
reqenv "USE_PERMISSIONLESS_FAULT_PROOFS"

# Local environment
BUNDLE_PATH="$OUTPUT_FOLDER_PATH/bundle.json"
L1_CHAIN_ID=$(cast chain-id)

# Copy the bundle template
cp ./templates/bundle_template.json "$BUNDLE_PATH"

# Tx 1: Upgrade SystemConfigProxy implementation
TX_1_PAYLOAD=$(cast calldata "upgrade(address,address)" "$SYSTEM_CONFIG_PROXY_ADDR" "$SYSTEM_CONFIG_IMPL")

# Tx 2: Upgrade PermissionedDisputeGame implementation
TX_2_PAYLOAD=$(cast calldata "setImplementation(uint32,address)" 1 "$PDG_IMPL")

# Tx 3: Upgrade FaultDisputeGame implementation
TX_3_PAYLOAD=$(cast calldata "setImplementation(uint32,address)" 0 "$FDG_IMPL")

# Replace variables
sed -i '' "s/\$L1_CHAIN_ID/$L1_CHAIN_ID/g" "$BUNDLE_PATH"
sed -i '' "s/\$PROXY_ADMIN_ADDR/$PROXY_ADMIN_ADDR/g" "$BUNDLE_PATH"
sed -i '' "s/\$SYSTEM_CONFIG_PROXY_ADDR/$SYSTEM_CONFIG_PROXY_ADDR/g" "$BUNDLE_PATH"
sed -i '' "s/\$SYSTEM_CONFIG_IMPL/$SYSTEM_CONFIG_IMPL/g" "$BUNDLE_PATH"
sed -i '' "s/\$PDG_IMPL/$PDG_IMPL/g" "$BUNDLE_PATH"
sed -i '' "s/\$TX_1_PAYLOAD/$TX_1_PAYLOAD/g" "$BUNDLE_PATH"
sed -i '' "s/\$TX_2_PAYLOAD/$TX_2_PAYLOAD/g" "$BUNDLE_PATH"

# Conditionally, if the FDG is being deployed, append the bundle extension
if [ "$USE_PERMISSIONLESS_FAULT_PROOFS" == true ]; then
  echo "✨ USE_PERMISSIONLESS_FAULT_PROOFS=true | Adding FDG deployment to upgrade bundle."
  jq --argjson fdg_extension "$(cat ./templates/fdg_bundle_extension.json)" \
    '.transactions += [$fdg_extension]' \
    "$BUNDLE_PATH" > "$BUNDLE_PATH.tmp"
  mv "$BUNDLE_PATH.tmp" "$BUNDLE_PATH"

  # Replace variables
  sed -i '' "s/\$FDG_IMPL/$FDG_IMPL/g" "$BUNDLE_PATH"
  sed -i '' "s/\$TX_3_PAYLOAD/$TX_3_PAYLOAD/g" "$BUNDLE_PATH"
fi

sed -i '' "s/\$DISPUTE_GAME_FACTORY_PROXY_ADDR/$DISPUTE_GAME_FACTORY_PROXY_ADDR/g" "$BUNDLE_PATH"

echo "✨ Generated upgrade bundle at \"$BUNDLE_PATH\""
