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
reqenv "PROXY_ADMIN_ADDR"
reqenv "SUPERCHAIN_CONFIG_PROXY_ADDR"
reqenv "PREIMAGE_ORACLE_ADDR"
reqenv "ANCHOR_STATE_REGISTRY_PROXY_ADDR"
reqenv "DELAYED_WETH_IMPL_ADDR"
reqenv "SYSTEM_CONFIG_IMPL_ADDR"
reqenv "MIPS_IMPL_ADDR"
reqenv "USE_FAULT_PROOFS"
reqenv "USE_PERMISSIONLESS_FAULT_PROOFS"

# Run the upgrade script
forge script DeployUpgrade.s.sol \
  --rpc-url "$ETH_RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  --etherscan-api-key "$ETHERSCAN_API_KEY" \
  --sig "deploy(address,address,address,address,address,address,address,bool,bool)" \
  "$PROXY_ADMIN_ADDR" \
  "$SUPERCHAIN_CONFIG_PROXY_ADDR" \
  "$SYSTEM_CONFIG_IMPL_ADDR" \
  "$MIPS_IMPL_ADDR" \
  "$DELAYED_WETH_IMPL_ADDR" \
  "$PREIMAGE_ORACLE_ADDR" \
  "$ANCHOR_STATE_REGISTRY_PROXY_ADDR" \
  "$USE_FAULT_PROOFS" \
  "$USE_PERMISSIONLESS_FAULT_PROOFS" \
  --broadcast \
  --verify \
  --slow
