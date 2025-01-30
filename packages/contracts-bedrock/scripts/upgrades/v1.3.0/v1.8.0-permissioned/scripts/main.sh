#!/usr/bin/env bash
set -euo pipefail

# Grab the script directory
SCRIPT_DIR=$(dirname "$0")

# Load common.sh
source "$SCRIPT_DIR/common.sh"

# Check if both input files are provided
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <deploy_config_path> <deployments_json_path>"
    exit 1
fi

# Set variables from environment or generate them
export NETWORK=${NETWORK:?NETWORK must be set}
export ETHERSCAN_API_KEY=${ETHERSCAN_API_KEY:?ETHERSCAN_API_KEY must be set}
export ETH_RPC_URL=${ETH_RPC_URL:?ETH_RPC_URL must be set}
export PRIVATE_KEY=${PRIVATE_KEY:?PRIVATE_KEY must be set}
export BASE_FEE_SCALAR=${BASE_FEE_SCALAR:?BASE_FEE_SCALAR must be set}
export BLOB_BASE_FEE_SCALAR=${BLOB_BASE_FEE_SCALAR:?BLOB_BASE_FEE_SCALAR must be set}

# Set IMPL_SALT to a random value
export IMPL_SALT=$(openssl rand -hex 16)

# Set the release version
RELEASE_VERSION="1.8.0-rc.4"

# Check that network is either "mainnet" or "sepolia"
if [ "$NETWORK" == "mainnet" ]; then
  export CHAIN_ID="1"
  export ASR_BLUEPRINT="0xbA7Be2bEE016568274a4D1E6c852Bb9a99FaAB8B"
elif [ "$NETWORK" == "sepolia" ]; then
  export CHAIN_ID="11155111"
  export ASR_BLUEPRINT="0xB98095199437883b7661E0D58256060f3bc730a4"
else
  echo "Error: NETWORK must be either 'mainnet' or 'sepolia'"
  exit 1
fi

# Find the contracts-bedrock directory
CONTRACTS_BEDROCK_DIR=$(pwd)
while [[ "$CONTRACTS_BEDROCK_DIR" != "/" && "${CONTRACTS_BEDROCK_DIR##*/}" != "contracts-bedrock" ]]; do
    CONTRACTS_BEDROCK_DIR=$(dirname "$CONTRACTS_BEDROCK_DIR")
done

# Error out if we couldn't find it for some reason
if [[ "$CONTRACTS_BEDROCK_DIR" == "/" ]]; then
    echo "Error: 'contracts-bedrock' directory not found"
    exit 1
fi

# Set file paths from command-line arguments
export DEPLOY_CONFIG_PATH="$CONTRACTS_BEDROCK_DIR/deploy-config/deploy-config.json"
export DEPLOYMENTS_JSON_PATH="$CONTRACTS_BEDROCK_DIR/deployments/deployments.json"

# Copy the files into the paths so that the script can actually access it
cp $1 $DEPLOY_CONFIG_PATH
cp $2 $DEPLOYMENTS_JSON_PATH

# Set the StorageSetter address
export STORAGE_SETTER=0xd81f43edbcacb4c29a9ba38a13ee5d79278270cc

# Get the SystemOwnerSafe address from the ProxyAdmin
export PROXY_ADMIN=$(load_local_address $DEPLOYMENTS_JSON_PATH "ProxyAdmin")
export SYSTEM_OWNER_SAFE=$(cast call $PROXY_ADMIN "owner()" | cast parse-bytes32-address)

# Run deploy.sh
if ! "$SCRIPT_DIR/deploy.sh" | tee deploy.log; then
    echo "Error: deploy.sh failed"
    exit 1
fi

# Extract the address of the DisputeGameFactoryProxy from the deploy.log
export DISPUTE_GAME_FACTORY_PROXY=$(grep "0. DisputeGameFactoryProxy:" deploy.log | awk '{print $3}')
export ANCHOR_STATE_REGISTRY_PROXY=$(grep "1. AnchorStateRegistryProxy:" deploy.log | awk '{print $3}')
export ANCHOR_STATE_REGISTRY_IMPL=$(grep "2. AnchorStateRegistryImpl:" deploy.log | awk '{print $3}')
export PERMISSIONED_DELAYED_WETH_PROXY=$(grep "3. PermissionedDelayedWETHProxy:" deploy.log | awk '{print $3}')
export PERMISSIONED_DISPUTE_GAME=$(grep "4. PermissionedDisputeGame:" deploy.log | awk '{print $3}')

# Make sure everything was extracted properly
reqenv "DISPUTE_GAME_FACTORY_PROXY"
reqenv "ANCHOR_STATE_REGISTRY_PROXY"
reqenv "ANCHOR_STATE_REGISTRY_IMPL"
reqenv "PERMISSIONED_DELAYED_WETH_PROXY"
reqenv "PERMISSIONED_DISPUTE_GAME"

# Generate deployments.json with extracted addresses
cat << EOF > "deployments.json"
{
  "DisputeGameFactoryProxy": "$DISPUTE_GAME_FACTORY_PROXY",
  "AnchorStateRegistryProxy": "$ANCHOR_STATE_REGISTRY_PROXY",
  "AnchorStateRegistryImpl": "$ANCHOR_STATE_REGISTRY_IMPL",
  "PermissionedDelayedWETHProxy": "$PERMISSIONED_DELAYED_WETH_PROXY",
  "PermissionedDisputeGame": "$PERMISSIONED_DISPUTE_GAME"
}
EOF

# Extract the path to the transactions json file
export TRANSACTIONS_JSON_PATH=$(grep "Transactions saved to:" deploy.log | awk -F': ' '{print $2}')

# Verify that the path was extracted successfully
reqenv "TRANSACTIONS_JSON_PATH"

# Run bundle.sh
if ! "$SCRIPT_DIR/bundle.sh" > bundle.json; then
    echo "Error: bundle.sh failed"
    exit 1
fi

# Run verify.sh
if ! "$SCRIPT_DIR/verify.sh" > validation.txt; then
    echo "Error: verify.sh failed"
    exit 1
fi

# Grab the various standard implementation addresses
SYSTEM_CONFIG_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "system_config")
OPTIMISM_PORTAL_2_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "optimism_portal")
L1_CROSS_DOMAIN_MESSENGER_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "l1_cross_domain_messenger")
L1_STANDARD_BRIDGE_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "l1_standard_bridge")
L1_ERC721_BRIDGE_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "l1_erc721_bridge")
OPTIMISM_MINTABLE_ERC20_FACTORY_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "optimism_mintable_erc20_factory")

# Generate standard-addresses.json
cat << EOF > "standard-addresses.json"
{
  "SystemConfigImpl": "$SYSTEM_CONFIG_IMPL",
  "OptimismPortal2Impl": "$OPTIMISM_PORTAL_2_IMPL",
  "L1CrossDomainMessengerImpl": "$L1_CROSS_DOMAIN_MESSENGER_IMPL",
  "L1StandardBridgeImpl": "$L1_STANDARD_BRIDGE_IMPL",
  "L1ERC721BridgeImpl": "$L1_ERC721_BRIDGE_IMPL",
  "OptimismMintableERC20FactoryImpl": "$OPTIMISM_MINTABLE_ERC20_FACTORY_IMPL"
}
EOF

# Copy results into output directory
cp deploy.log /outputs/deploy.log
cp bundle.json /outputs/bundle.json
cp validation.txt /outputs/validation.txt
cp deployments.json /outputs/deployments.json
cp standard-addresses.json /outputs/standard-addresses.json
cp $TRANSACTIONS_JSON_PATH /outputs/transactions.json
