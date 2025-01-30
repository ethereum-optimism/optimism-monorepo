#!/usr/bin/env bash
set -euo pipefail

# Grab the script directory
SCRIPT_DIR=$(dirname "$0")

# Load common.sh
source "$SCRIPT_DIR/common.sh"

# Check required environment variables
reqenv "ETH_RPC_URL"
reqenv "PRIVATE_KEY"
reqenv "ETHERSCAN_API_KEY"
reqenv "DEPLOY_CONFIG_PATH"
reqenv "DEPLOYMENTS_JSON_PATH"
reqenv "NETWORK"
reqenv "IMPL_SALT"
reqenv "SYSTEM_OWNER_SAFE"
reqenv "ASR_BLUEPRINT"

# Set the release version
RELEASE_VERSION="1.8.0-rc.4"

# Load addresses from deployments json
PROXY_ADMIN=$(load_local_address $DEPLOYMENTS_JSON_PATH "ProxyAdmin")

# Fetch addresses from standard address toml
DISPUTE_GAME_FACTORY_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "dispute_game_factory")
DELAYED_WETH_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "delayed_weth")
PREIMAGE_ORACLE_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "preimage_oracle")
MIPS_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "mips")
OPTIMISM_PORTAL_2_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "optimism_portal")

# Fetch the SuperchainConfigProxy address
SUPERCHAIN_CONFIG_PROXY=$(fetch_superchain_config_address $NETWORK)

# Run the upgrade script
forge script DeployUpgrade.s.sol:DeployUpgrade \
  --rpc-url $ETH_RPC_URL \
  --private-key $PRIVATE_KEY \
  --etherscan-api-key $ETHERSCAN_API_KEY \
  --sig "deploy(address,address,address,address,address,address,address,address)" \
  $PROXY_ADMIN \
  $SYSTEM_OWNER_SAFE \
  $SUPERCHAIN_CONFIG_PROXY \
  $DISPUTE_GAME_FACTORY_IMPL \
  $DELAYED_WETH_IMPL \
  $PREIMAGE_ORACLE_IMPL \
  $MIPS_IMPL \
  $OPTIMISM_PORTAL_2_IMPL \
  --broadcast \
  --slow \
  --verify \
  --non-interactive
