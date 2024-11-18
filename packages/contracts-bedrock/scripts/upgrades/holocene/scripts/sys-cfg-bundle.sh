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
reqenv "PROXY_ADMIN_ADDR"
reqenv "SYSTEM_CONFIG_PROXY_ADDR"
reqenv "SYSTEM_CONFIG_IMPL"

# Local environment
BUNDLE_PATH="$OUTPUT_FOLDER_PATH/sys_cfg_bundle.json"
L1_CHAIN_ID=$(cast chain-id)

# Copy the bundle template
cp ./templates/sys_cfg_upgrade_bundle_template.json "$BUNDLE_PATH"

# Tx 1: Upgrade SystemConfigProxy implementation
TX_1_PAYLOAD=$(cast calldata "upgrade(address,address)" "$SYSTEM_CONFIG_PROXY_ADDR" "$SYSTEM_CONFIG_IMPL")

# Replace variables
sed -i '' "s/\$L1_CHAIN_ID/$L1_CHAIN_ID/g" "$BUNDLE_PATH"
sed -i '' "s/\$PROXY_ADMIN_ADDR/$PROXY_ADMIN_ADDR/g" "$BUNDLE_PATH"
sed -i '' "s/\$SYSTEM_CONFIG_PROXY_ADDR/$SYSTEM_CONFIG_PROXY_ADDR/g" "$BUNDLE_PATH"
sed -i '' "s/\$SYSTEM_CONFIG_IMPL/$SYSTEM_CONFIG_IMPL/g" "$BUNDLE_PATH"
sed -i '' "s/\$TX_1_PAYLOAD/$TX_1_PAYLOAD/g" "$BUNDLE_PATH"

echo "✨ Generated SystemConfig upgrade bundle at \"$BUNDLE_PATH\""
