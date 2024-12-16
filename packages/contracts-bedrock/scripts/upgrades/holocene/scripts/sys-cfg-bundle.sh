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

# We need to re-generate the SystemConfig initialization call
# We want to use the exact same values that the SystemConfig is already using, apart from baseFeeScalar and blobBaseFeeScalar.
# Start with values we can just read off:
OWNER=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "owner()")
SCALAR=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "scalar()")
BATCHER_HASH=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "batcherHash()")
GAS_LIMIT=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "gasLimit()")
UNSAFE_BLOCK_SIGNER=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "unsafeBlockSigner()")
RESOURCE_CONFIG=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "resourceConfig()")
BATCH_INBOX=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "batchInbox()")
GAS_PAYING_TOKEN=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "gasPayingToken()(address)")
L1_CROSS_DOMAIN_MESSENGER_PROXY=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "l1CrossDomainMessenger()(address)")
L1_STANDARD_BRIDGE_PROXY=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "l1StandardBridge()(address)")
L1_ERC721_BRIDGE_PROXY=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "l1ERC721Bridge()(address)")
DISPUTE_GAME_FACTORY_PROXY=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "disputeGameFactory()(address)")
OPTIMISM_PORTAL_PROXY=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "optimismPortal()(address)")
OPTIMISM_MINTABLE_ERC20_FACTORY_PROXY=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "optimismMintableERC20Factory()(address)")


# Decode base fee scalar and blob base fee scalar from scalar value:
BASE_FEE_SCALAR=$(go run github.com/ethereum-optimism/optimism/op-chain-ops/cmd/ecotone-scalar --decode="$SCALAR" | awk '/^# base fee scalar[[:space:]]*:/{print $NF}')
BLOB_BASE_FEE_SCALAR=$(go run github.com/ethereum-optimism/optimism/op-chain-ops/cmd/ecotone-scalar --decode="$SCALAR" | awk '/^# blob base fee scalar[[:space:]]*:/{print $NF}')

# Now we generate the initialization calldata
SYSTEM_CONFIG_INITIALIZE_CALLDATA=$(cast calldata \
  "initialize(address,uint32,uint32,bytes32,uint64,address,(uint32,uint8,uint8,uint32,uint32,uint128),address,(address,address,address,address,address,address,address))" \
  "$(cast parse-bytes32-address "$OWNER")" \
  "$BASE_FEE_SCALAR" \
  "$BLOB_BASE_FEE_SCALAR" \
  "$BATCHER_HASH" \
  "$GAS_LIMIT" \
  "$(cast parse-bytes32-address "$UNSAFE_BLOCK_SIGNER")" \
  "($(cast abi-decode "null()(uint32,uint8,uint8,uint32,uint32,uint128)" "$RESOURCE_CONFIG" --json | jq -r 'join(",")'))" \
  "$(cast parse-bytes32-address "$BATCH_INBOX")" \
  "($L1_CROSS_DOMAIN_MESSENGER_PROXY,$L1_ERC721_BRIDGE_PROXY,$L1_STANDARD_BRIDGE_PROXY,$DISPUTE_GAME_FACTORY_PROXY,$OPTIMISM_PORTAL_PROXY,$OPTIMISM_MINTABLE_ERC20_FACTORY_PROXY,$GAS_PAYING_TOKEN)"
)

# Replace variables
sed -i "s/\$L1_CHAIN_ID/$L1_CHAIN_ID/g" "$BUNDLE_PATH"
sed -i "s/\$PROXY_ADMIN_ADDR/$PROXY_ADMIN_ADDR/g" "$BUNDLE_PATH"
sed -i "s/\$SYSTEM_CONFIG_PROXY_ADDR/$SYSTEM_CONFIG_PROXY_ADDR/g" "$BUNDLE_PATH"
sed -i "s/\$SYSTEM_CONFIG_IMPL/$SYSTEM_CONFIG_IMPL/g" "$BUNDLE_PATH"
sed -i "s/\$SYSTEM_CONFIG_INITIALIZE_CALLDATA/$SYSTEM_CONFIG_INITIALIZE_CALLDATA/g" "$BUNDLE_PATH"

echo "✨ Generated SystemConfig upgrade bundle at \"$BUNDLE_PATH\""
