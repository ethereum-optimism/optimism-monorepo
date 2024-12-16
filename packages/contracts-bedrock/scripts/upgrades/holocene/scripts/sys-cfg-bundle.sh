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
SYSTEM_CONFIG_OWNER=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "owner()")
SYSTEM_CONFIG_SCALAR=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "scalar()")
SYSTEM_CONFIG_BATCHER_HASH=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "batcherHash()")
SYSTEM_CONFIG_GAS_LIMIT=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "gasLimit()")
SYSTEM_CONFIG_UNSAFE_BLOCK_SIGNER=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "unsafeBlockSigner()")
SYSTEM_CONFIG_RESOURCE_CONFIG=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "resourceConfig()")
SYSTEM_CONFIG_BATCH_INBOX=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "batchInbox()")
SYSTEM_CONFIG_GAS_PAYING_TOKEN=$(cast call "$SYSTEM_CONFIG_PROXY_ADDR" "gasPayingToken()(address)")

# Decode base fee scalar and blob base fee scalar from scalar value:
SYSTEM_CONFIG_BASE_FEE_SCALAR=$(go run github.com/ethereum-optimism/optimism/op-chain-ops/cmd/ecotone-scalar --decode="$SYSTEM_CONFIG_SCALAR" | awk '/^# base fee scalar[[:space:]]*:/{print $NF}')
SYSTEM_CONFIG_BLOB_BASE_FEE_SCALAR=$(go run github.com/ethereum-optimism/optimism/op-chain-ops/cmd/ecotone-scalar --decode="$SYSTEM_CONFIG_SCALAR" | awk '/^# blob base fee scalar[[:space:]]*:/{print $NF}')

# Now we generate the initialization calldata
SYSTEM_CONFIG_INITIALIZE_CALLDATA=$(cast calldata \
  "initialize(address,uint32,uint32,bytes32,uint64,address,(uint32,uint8,uint8,uint32,uint32,uint128),address,(address,address,address,address,address,address,address))" \
  "$(cast parse-bytes32-address "$SYSTEM_CONFIG_OWNER")" \
  "$SYSTEM_CONFIG_BASE_FEE_SCALAR" \
  "$SYSTEM_CONFIG_BLOB_BASE_FEE_SCALAR" \
  "$SYSTEM_CONFIG_BATCHER_HASH" \
  "$SYSTEM_CONFIG_GAS_LIMIT" \
  "$(cast parse-bytes32-address "$SYSTEM_CONFIG_UNSAFE_BLOCK_SIGNER")" \
  "($(cast abi-decode "null()(uint32,uint8,uint8,uint32,uint32,uint128)" "$SYSTEM_CONFIG_RESOURCE_CONFIG" --json | jq -r 'join(",")'))" \
  "$(cast parse-bytes32-address "$SYSTEM_CONFIG_BATCH_INBOX")" \
  "($L1_CROSS_DOMAIN_MESSENGER_PROXY,$L1_ERC721_BRIDGE_PROXY,$L1_STANDARD_BRIDGE_PROXY,$DISPUTE_GAME_FACTORY_PROXY,$OPTIMISM_PORTAL_PROXY,$OPTIMISM_MINTABLE_ERC20_FACTORY_PROXY,$SYSTEM_CONFIG_GAS_PAYING_TOKEN)"
)


# Replace variables
sed -i "s/\$L1_CHAIN_ID/$L1_CHAIN_ID/g" "$BUNDLE_PATH"
sed -i "s/\$PROXY_ADMIN_ADDR/$PROXY_ADMIN_ADDR/g" "$BUNDLE_PATH"
sed -i "s/\$SYSTEM_CONFIG_PROXY_ADDR/$SYSTEM_CONFIG_PROXY_ADDR_ADDR/g" "$BUNDLE_PATH"
sed -i "s/\$SYSTEM_CONFIG_IMPL/$SYSTEM_CONFIG_IMPL/g" "$BUNDLE_PATH"
sed -i "s/\$SYSTEM_CONFIG_INITIALIZE_CALLDATA/$SYSTEM_CONFIG_INITIALIZE_CALLDATA/g" "$BUNDLE_PATH"

echo "✨ Generated SystemConfig upgrade bundle at \"$BUNDLE_PATH\""
