#!/usr/bin/env bash
set -euo pipefail

# Grab the script directory
SCRIPT_DIR=$(dirname "$0")

# Load common.sh
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"

echo "
⠄⢀⠀⠀⡐⠠⠀⢂⣠⡤⣤⣖⣤⣤⣄⢢⣤⣤⣤⡀⠄⠰⠀⠆⠀⠀⠠⠀⠆⠠⢀⠢⠐⠆⡀
⠔⢀⠀⠄⡀⠔⢠⣾⣿⣿⣷⣿⣿⣿⣿⣷⣟⣛⣻⣿⣤⡠⢀⠆⢄⠂⠠⢠⠀⠄⠀⢠⠰⢄⠄
⣈⠀⠐⡀⠀⠈⣾⣿⣿⣿⣿⡿⡿⣿⣿⣿⣿⡿⢿⣿⣿⣿⣦⠀⠂⠀⢈⠀⠁⠂⡀⢀⠠⠈⡀
⠆⠈⠀⠄⣃⣼⣿⣿⣿⠿⡿⣿⣿⣷⣾⣷⣾⡾⣿⣿⣿⢿⡟⡇⠠⠁⠨⠐⠀⠃⠱⠊⠀⠀⠄
⠐⣠⣶⣿⣿⣿⣿⣿⣿⣿⣾⣮⣛⢿⣿⡿⠘⣇⢯⣹⣶⣷⣿⣿⡄⠆⢐⠀⢄⠢⡒⠐⠠⢀⠂
⢴⣿⣿⣿⣿⣿⣿⡿⠿⢿⣿⣿⣟⢿⣶⣾⣿⣧⢻⣿⢿⣿⣿⣿⠂⠈⢀⠈⡀⡁⢀⠉⢀⠀⠀
⡜⣿⣿⣿⣿⣿⣿⣦⡻⠀⢨⣽⣿⣿⣿⣿⣿⣿⣦⡛⣾⣭⣃⣀⣦⠀⠨⠐⠀⠄⠃⠈⠀⠀⠁
⣳⡘⣿⣿⣿⣿⣿⣿⣿⣿⣿⢛⣭⣿⣿⣿⣿⣿⣿⣿⢢⣿⣿⣿⣿⡇⢀⠲⠂⠄⢀⡐⠠⠐⠂
⢣⠵⢹⣿⣿⣿⣿⣿⣿⣿⣿⣧⣛⣻⣿⣭⣿⣿⠻⢥⣿⣿⣿⣿⣿⣿⡄⠀⠁⡀⢀⢈⠀⢀⠁
⡼⣹⡘⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠿⣟⣻⠿⠿⣿⣿⣿⣿⣿⣿⣿⣿⠇⠀⠀⠁⠉⠐⠀⠂⠈
⡷⣡⠧⢹⣿⣿⣿⣿⣿⣿⣗⣶⣾⣿⣿⣿⣿⣿⠮⢿⣿⣿⡿⠿⣿⣿⢀⣀⠂⡁⠐⢌⠐⡠⠁
⢷⠡⠏⢧⢌⢻⣿⣿⣿⣟⣿⣿⣻⣿⡿⠿⣛⣵⢟⣭⡭⠥⠮⠕⣒⣒⣚⣮⣰⢤⣤⣄⣀⡠⠄
⠁⠠⠀⠄⣏⢦⡙⣿⣿⣽⣟⣿⡟⣬⣾⣿⣿⣿⣾⣆⠭⠽⠶⢶⢶⣖⣶⣹⣖⡿⣿⣿⣿⣿⡆
⠀⡁⠂⡟⡜⣦⢫⣌⠻⣷⣿⢏⣾⣿⣿⣿⢿⣿⢿⣿⣿⣿⣿⣿⠿⣛⣛⣛⠛⣭⣿⣿⢻⣽⡇
⣏⠖⣮⢱⣋⢖⣣⢎⡳⢤⡅⣾⣿⢯⣷⡏⡖⣰⣝⣚⣙⣫⢍⡶⠷⣽⢭⣛⡇⠀⢰⣶⣾⣿⠀
⡮⡝⢦⣓⢎⡳⡜⠎⠡⠁⢸⣿⣿⣿⣿⣁⢠⣿⡿⣿⡟⣎⣯⣽⣋⡷⣾⣹⡃⠀⢸⣿⣿⢿⠀
⣳⠁⠀⡝⢮⣱⢹⠀⠂⠈⣿⣿⣿⣿⡻⣈⣸⣿⣙⢾⢿⣹⡶⣿⣼⣗⣻⡞⣡⠁⣼⣿⣿⣿⡀
⡔⢦⢥⡛⣜⠦⣏⡄⡈⣸⢿⣿⡿⣽⢃⠇⣿⣧⡝⣟⡳⢾⣹⣟⡻⣾⣹⢣⠞⣄⣯⣿⣷⣿⡆
   -*~ [ Grug Deployer mk2 ] ~*-
       ~*- [ Holocene ] -*~
"

# Set variables from environment or error.
export ETHERSCAN_API_KEY=${ETHERSCAN_API_KEY:?ETHERSCAN_API_KEY must be set}
export ETH_RPC_URL=${ETH_RPC_URL:?ETH_RPC_URL must be set}
export PRIVATE_KEY=${PRIVATE_KEY:?PRIVATE_KEY must be set}
export BASE_DEPLOY_CONFIG_PATH=${DEPLOY_CONFIG_PATH:?DEPLOY_CONFIG_PATH must be set}
export OUTPUT_FOLDER_PATH=${OUTPUT_FOLDER_PATH:?OUTPUT_FOLDER_PATH must be set}
export SYSTEM_CONFIG_IMPL_ADDR=${SYSTEM_CONFIG_IMPL_ADDR:-$(cast az)}
export MIPS_IMPL_ADDR=${MIPS_IMPL_ADDR:-$(cast az)}
export PREIMAGE_ORACLE_ADDR=${PREIMAGE_ORACLE_ADDR:?PREIMAGE_ORACLE_ADDR must be set}
export ANCHOR_STATE_REGISTRY_PROXY_ADDR=${ANCHOR_STATE_REGISTRY_PROXY_ADDR:?ANCHOR_STATE_REGISTRY_PROXY_ADDR must be set}
export DELAYED_WETH_PROXY_ADDR=${DELAYED_WETH_PROXY_ADDR:?DELAYED_WETH_PROXY_ADDR must be set}
export PROXY_ADMIN_ADDR=${PROXY_ADMIN_ADDR:?PROXY_ADMIN_ADDR must be set}
export SYSTEM_CONFIG_PROXY_ADDR=${SYSTEM_CONFIG_PROXY_ADDR:?SYSTEM_CONFIG_PROXY_ADDR must be set}
export DISPUTE_GAME_FACTORY_PROXY_ADDR=${DISPUTE_GAME_FACTORY_PROXY_ADDR:?DISPUTE_GAME_FACTORY_PROXY_ADDR must be set}

# Make the output folder, if it doesn't exist
mkdir -p "$OUTPUT_FOLDER_PATH"

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

# Copy the files into the paths so that the script can actually access it
cp "$BASE_DEPLOY_CONFIG_PATH" "$DEPLOY_CONFIG_PATH"

# Run deploy.sh
DEPLOY_LOG_PATH="$OUTPUT_FOLDER_PATH/deploy.log"
if ! "$SCRIPT_DIR/deploy.sh" | tee "$DEPLOY_LOG_PATH"; then
    echo "Error: deploy.sh failed"
    exit 1
fi

# Extract the addresses from the deployment logs
# shellcheck disable=2155
export SYSTEM_CONFIG_IMPL=$(grep "1. SystemConfig:" "$DEPLOY_LOG_PATH" | awk '{print $3}')
# shellcheck disable=2155
export MIPS_IMPL=$(grep "2. MIPS:" "$DEPLOY_LOG_PATH" | awk '{print $3}')
# shellcheck disable=2155
export FDG_IMPL=$(grep "3. FaultDisputeGame:" "$DEPLOY_LOG_PATH" | awk '{print $3}')
# shellcheck disable=2155
export PDG_IMPL=$(grep "4. PermissionedDisputeGame:" "$DEPLOY_LOG_PATH" | awk '{print $3}')

# Ensure that the addresses were extracted properly
reqenv "SYSTEM_CONFIG_IMPL"
reqenv "MIPS_IMPL"
reqenv "FDG_IMPL"
reqenv "PDG_IMPL"

# Generate deployments.json with extracted addresses
DEPLOYMENTS_JSON_PATH="$OUTPUT_FOLDER_PATH/deployments.json"
cat << EOF > "$DEPLOYMENTS_JSON_PATH"
{
  "SystemConfig": "$SYSTEM_CONFIG_IMPL",
  "MIPS": "$MIPS_IMPL",
  "FaultDisputeGame": "$FDG_IMPL",
  "PermissionedDisputeGame": "$PDG_IMPL"
}
EOF

echo "✨ Deployed contracts and saved addresses to \"$DEPLOYMENTS_JSON_PATH\""

# Print a message when the script exits
trap 'echo "✨ Done. Artifacts are available in \"$OUTPUT_FOLDER_PATH\""' EXIT

prompt "Generate safe upgrade bundle?"

# Generate the upgrade bundle
if ! "$SCRIPT_DIR/bundle.sh"; then
    echo "Error: bundle.sh failed"
    exit 1
fi

prompt "Generate superchain-ops upgrade task?"

# Generate the superchain-ops upgrade task
if ! "$SCRIPT_DIR/sc-ops.sh"; then
    echo "Error: sc-ops.sh failed"
    exit 1
fi
