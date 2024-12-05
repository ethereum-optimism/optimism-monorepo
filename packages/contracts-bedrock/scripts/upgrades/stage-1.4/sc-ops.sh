#!/bin/bash

set -euo pipefail

# shellcheck disable=SC1091
source "./common.sh"
# shellcheck disable=SC2046
export $(grep -v '^#' .env | xargs)

reqenv "ETH_RPC_URL"
reqenv "DEPLOY_CONFIG_PATH"
reqenv "ASTERISC_KONA_GAME_TYPE"

# ----------------
# Check Arguments
# ----------------

if [ $# -ne 1 ]; then
    echo "Usage: $0 <deployments-json-path>"
    exit 1
fi

# ----------------
# Constants
# ----------------

STORAGE_SETTER_ADDR="0xd81f43eDBCAcb4c29a9bA38a13Ee5d79278270cC"

# ----------------
# Task Generation
# ----------------

# 1. Read `deployments.json` file
DEPLOYMENTS_JSON_PATH="$1"
L2_CHAIN_ID=$(jq -r '.l2ChainID' "$DEPLOYMENTS_JSON_PATH")
RISCV_IMPL=$(jq -r '.contracts.RISCV' "$DEPLOYMENTS_JSON_PATH")
FDG_IMPL=$(jq -r '.contracts.FaultDisputeGame' "$DEPLOYMENTS_JSON_PATH")
DELAYED_WETH_PROXY=$(jq -r '.contracts.DelayedWETHProxy' "$DEPLOYMENTS_JSON_PATH")

# 2. Fetch the superchain-registry addresses
REGISTRY_ADDRESSES="$(curl -LsS https://raw.githubusercontent.com/ethereum-optimism/superchain-registry/refs/heads/main/superchain/extra/addresses/addresses.json)"
PROXY_ADMIN_ADDR="$(jq -r ".[\"$L2_CHAIN_ID\"].ProxyAdmin" <<< "$REGISTRY_ADDRESSES")"
ANCHOR_STATE_REGISTRY_PROXY_ADDR="$(jq -r ".[\"$L2_CHAIN_ID\"].AnchorStateRegistryProxy" <<< "$REGISTRY_ADDRESSES")"
ANCHOR_STATE_REGISTRY_IMPL_ADDR="$(cast admin "$ANCHOR_STATE_REGISTRY_PROXY_ADDR")"
SUPERCHAIN_CONFIG_PROXY_ADDR="$(cast call "$ANCHOR_STATE_REGISTRY_PROXY_ADDR" "superchainConfig()(address)")"
DISPUTE_GAME_FACTORY_PROXY_ADDR="$(jq -r ".[\"$L2_CHAIN_ID\"].DisputeGameFactoryProxy" <<< "$REGISTRY_ADDRESSES")"

echo "✨ Fetched addresses for L2 Chain \"$L2_CHAIN_ID\" from the superchain-registry"
echo "  -> ProxyAdmin: $PROXY_ADMIN_ADDR"
echo "  -> SuperchainConfigProxy: $SUPERCHAIN_CONFIG_PROXY_ADDR"
echo "  -> AnchorStateRegistryProxy: $ANCHOR_STATE_REGISTRY_PROXY_ADDR"
echo "  -> AnchorStateRegistryImpl: $ANCHOR_STATE_REGISTRY_IMPL_ADDR"
echo "  -> DisputeGameFactoryProxy: $DISPUTE_GAME_FACTORY_PROXY_ADDR"

# 3. Create the task folder from the template
TASK_DIR="./upgrade-task"
cp -R ./superchain-ops-template $TASK_DIR

# 4. Fill in the `input.json` template

## Tx #1 - Prepare ASR for re-initialization
TX_1_DATA_INNER="$(cast calldata "setUint(bytes32,uint256)" "$(cast 2b 0)" 0)"
TX_1_DATA="$(cast calldata "upgradeAndCall(address,address,bytes)" "$ANCHOR_STATE_REGISTRY_PROXY_ADDR" "$STORAGE_SETTER_ADDR" "$TX_1_DATA_INNER")"

replace_in_place "$TASK_DIR/input.json" '%%PROXY_ADMIN_ADDR%%' "$PROXY_ADMIN_ADDR"
replace_in_place "$TASK_DIR/input.json" '%%TX_1_DATA%%' "$TX_1_DATA"
replace_in_place "$TASK_DIR/input.json" '%%TX_1_DATA_INNER%%' "$TX_1_DATA_INNER"

## Tx #2 - Upgrade ASR, re-initialize with new implementation
STARTING_ANCHOR="$(cast call "$ANCHOR_STATE_REGISTRY_PROXY_ADDR" "anchors(uint32)(bytes32)" 0)"
TX_2_DATA_INNER="$(cast calldata "initialize((uint32,bytes32)[],address)" "[($ASTERISC_KONA_GAME_TYPE, $STARTING_ANCHOR)]" "$SUPERCHAIN_CONFIG_PROXY_ADDR")"
TX_2_DATA="$(cast calldata "upgradeAndCall(address,address,bytes)" "$ANCHOR_STATE_REGISTRY_PROXY_ADDR" "$ANCHOR_STATE_REGISTRY_IMPL_ADDR" "$TX_2_DATA_INNER")"

replace_in_place "$TASK_DIR/input.json" '%%ANCHOR_STATE_REGISTRY_PROXY_ADDR%%' "$ANCHOR_STATE_REGISTRY_PROXY_ADDR"
replace_in_place "$TASK_DIR/input.json" '%%ANCHOR_STATE_REGISTRY_IMPL_ADDR%%' "$ANCHOR_STATE_REGISTRY_IMPL_ADDR"
replace_in_place "$TASK_DIR/input.json" '%%TX_2_DATA%%' "$TX_2_DATA"
replace_in_place "$TASK_DIR/input.json" '%%TX_2_DATA_INNER%%' "$TX_2_DATA_INNER"

## Tx #3 - Upgrade DisputeGameFactory
TX_3_DATA="$(cast calldata "setImplementation(uint32,address)" "$ASTERISC_KONA_GAME_TYPE" "$FDG_IMPL")"

replace_in_place "$TASK_DIR/input.json" '%%DISPUTE_GAME_FACTORY_PROXY_ADDR%%' "$DISPUTE_GAME_FACTORY_PROXY_ADDR"
replace_in_place "$TASK_DIR/input.json" '%%TX_3_DATA%%' "$TX_3_DATA"
replace_in_place "$TASK_DIR/input.json" '%%FDG_IMPL%%' "$FDG_IMPL"

# 5. Fill in the 'README.md' template
replace_in_place "$TASK_DIR/README.md" '%%RISCV_IMPL%%' "$RISCV_IMPL"
replace_in_place "$TASK_DIR/README.md" '%%DELAYED_WETH_PROXY%%' "$DELAYED_WETH_PROXY"
replace_in_place "$TASK_DIR/README.md" '%%FDG_IMPL%%' "$FDG_IMPL"

# 6. Fill in the 'VALIDATION.md' template
replace_in_place "$TASK_DIR/VALIDATION.md" '%%DISPUTE_GAME_FACTORY_PROXY_ADDR%%' "$DISPUTE_GAME_FACTORY_PROXY_ADDR"
replace_in_place "$TASK_DIR/VALIDATION.md" '%%FDG_IMPL_B32%%' "$(cast 2u "$FDG_IMPL")"
replace_in_place "$TASK_DIR/VALIDATION.md" '%%ANCHOR_STATE_REGISTRY_PROXY_ADDR%%' "$ANCHOR_STATE_REGISTRY_PROXY_ADDR"
replace_in_place "$TASK_DIR/VALIDATION.md" '%%STARTING_ANCHOR%%' "$STARTING_ANCHOR"

# 7. Finalize
echo "✨ Created upgrade task in $TASK_DIR"
