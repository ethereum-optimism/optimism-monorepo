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
reqenv "DEPLOYMENTS_JSON_PATH"
reqenv "NETWORK"
reqenv "IMPL_SALT"
reqenv "SYSTEM_OWNER_SAFE"

# Load addresses from deployments json
PROXY_ADMIN=$(load_local_address "$DEPLOYMENTS_JSON_PATH" "ProxyAdmin")

# Fetch addresses from standard address toml
DISPUTE_GAME_FACTORY_IMPL=$(fetch_standard_address "$NETWORK" "$CONTRACTS_VERSION" "dispute_game_factory")
DELAYED_WETH_IMPL=$(fetch_standard_address "$NETWORK" "$CONTRACTS_VERSION" "delayed_weth")
PREIMAGE_ORACLE_IMPL=$(fetch_standard_address "$NETWORK" "$CONTRACTS_VERSION" "preimage_oracle")
MIPS_IMPL=$(fetch_standard_address "$NETWORK" "$CONTRACTS_VERSION" "mips")
OPTIMISM_PORTAL_2_IMPL=$(fetch_standard_address "$NETWORK" "$CONTRACTS_VERSION" "optimism_portal")

case "${NETWORK}" in
  mainnet)
    ANCHOR_STATE_REGISTRY_BLUEPRINT="0xbA7Be2bEE016568274a4D1E6c852Bb9a99FaAB8B"
    ;;
  sepolia)
    ANCHOR_STATE_REGISTRY_BLUEPRINT="0xB98095199437883b7661E0D58256060f3bc730a4"
    ;;
  *)
    echo "No AnchorStateRegistry blueprint for $NETWORK"
    exit 1
    ;;
esac

# Fetch the SuperchainConfigProxy address
SUPERCHAIN_CONFIG_PROXY=$(fetch_superchain_config_address "$NETWORK")

# Run the upgrade script
forge script DeployUpgrade.s.sol \
  --rpc-url "$ETH_RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  --etherscan-api-key "$ETHERSCAN_API_KEY" \
  --sig "deploy(address,address,address,address,address,address,address,address,address)" \
  "$PROXY_ADMIN" \
  "$SYSTEM_OWNER_SAFE" \
  "$SUPERCHAIN_CONFIG_PROXY" \
  "$DISPUTE_GAME_FACTORY_IMPL" \
  "$DELAYED_WETH_IMPL" \
  "$PREIMAGE_ORACLE_IMPL" \
  "$MIPS_IMPL" \
  "$OPTIMISM_PORTAL_2_IMPL" \
  "$ANCHOR_STATE_REGISTRY_BLUEPRINT" \
  --broadcast \
  --slow \
  --verify
