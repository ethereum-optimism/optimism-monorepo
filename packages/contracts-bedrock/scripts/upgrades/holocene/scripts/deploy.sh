#!/usr/bin/env bash
set -euo pipefail

# Grab the script directory
SCRIPT_DIR=$(dirname "$0")

# Load common.sh
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"

# Check required environment variables
reqenv "ETH_RPC_URL"
reqenv "PRIVATE_KEY"
reqenv "ETHERSCAN_API_KEY"
reqenv "DEPLOY_CONFIG_PATH"
reqenv "IMPL_SALT"

# Check required address environment variables
reqenv "PREIMAGE_ORACLE_ADDR"
reqenv "ANCHOR_STATE_REGISTRY_PROXY_ADDR"
reqenv "DELAYED_WETH_PROXY_ADDR"

# Run the upgrade script
forge script DeployUpgrade.s.sol \
  --rpc-url "$ETH_RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  --etherscan-api-key "$ETHERSCAN_API_KEY" \
  --sig "deploy(address,address,address)" \
  "$PREIMAGE_ORACLE_ADDR" \
  "$ANCHOR_STATE_REGISTRY_PROXY_ADDR" \
  "$DELAYED_WETH_PROXY_ADDR" \
  --broadcast \
  --slow
