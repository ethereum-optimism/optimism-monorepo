#!/usr/bin/env bash

# TODO: actually test something. Right now it just gives an idea of what's
# possible.

# shellcheck disable=SC1091
source "$(dirname "$0")/boilerplate.sh"

# we require 2+ L2s
if [ "$(getEnvData '.l2 | length')" -lt 2 ]; then
    echo "Error: we require at least 2 L2s"
    exit 1
fi

# First chain helpers
L2_1_RPC="http://$(getSvcHostPort '.l2[0].nodes[0].services.el' 'rpc')"
L2_1_CHAINID=$(getEnvData '.l2[0].id')

function cast_l2_1() {
    cast "$@" --rpc-url "$L2_1_RPC"
}

# Second chain helpers
L2_2_RPC="http://$(getSvcHostPort '.l2[1].nodes[0].services.el' 'rpc')"
L2_2_CHAINID=$(getEnvData '.l2[1].id')

function cast_l2_2() {
    cast "$@" --rpc-url "$L2_2_RPC"
}

# Globally useful variables
MNEMONIC='test test test test test test test test test test test junk'
USER_ADDRESS=0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266

# Contract adresses
L2ToL2CrossDomainMessenger=0x4200000000000000000000000000000000000023
SuperchainWETH=0x4200000000000000000000000000000000000024
SuperchainTokenBridge=0x4200000000000000000000000000000000000028

# Actual test
cd "$TMPDIR" || exit 1


####
step "Wrap ETH into SuperchainWETH"
cast_l2_1 send "$SuperchainWETH" \
    --value "10ether" \
    --mnemonic "$MNEMONIC"


####
step "Check SuperchainWETH balance"
MIN_BALANCE=1000000000000000000
BALANCE=$(cast_l2_1 call "$SuperchainWETH" \
    "balanceOf(address)(uint256)" "$USER_ADDRESS" | cut -d' ' -f 1)
echo
echo "SuperchainWETH balance: $BALANCE"
(( $(bc <<< "$BALANCE > $MIN_BALANCE") )) && true
echo "Balance is sufficient (greater than $MIN_BALANCE)"


####
step "Send SuperchainWETH through the SuperchainTokenBridge"
DUMP_FILE=send_superchainweth_dump.txt
cast_l2_1 send "$SuperchainTokenBridge" \
    "sendERC20(address,address,uint256,uint256)" \
    "$SuperchainWETH" \
    "$USER_ADDRESS" \
    "$MIN_BALANCE" \
    "$L2_2_CHAINID" \
    --mnemonic "$MNEMONIC" | tee "$DUMP_FILE"


####
step "Build Identifier and payload"
LOG_FILE=send_superchainweth_logs.json
extract_cast_logs "$DUMP_FILE" > "$LOG_FILE"

LOG_ENTRY=L2ToL2CrossDomainMessenger_log.json
get_log_entry "$LOG_FILE" "$L2ToL2CrossDomainMessenger" > "$LOG_ENTRY"

BLOCK_NUMBER=$(jq -r '.blockNumber' "$LOG_ENTRY")
LOG_INDEX=$(jq -r '.logIndex' "$LOG_ENTRY")

DEC_BLOCK_NUMBER=$(cast to-dec "$BLOCK_NUMBER")
TIMESTAMP=$(cast_l2_1 block "$DEC_BLOCK_NUMBER" --field timestamp)

# build the payload by joining the topics and data (without the 0x prefixes)
TOPICS=$(jq -r '(.topics | join("") | gsub("0x"; ""))' "$LOG_ENTRY")
DATA=$(jq -r '(.data | gsub("0x"; ""))' "$LOG_ENTRY")
EVENT_PAYLOAD="0x$TOPICS$DATA"
echo "Event payload: $EVENT_PAYLOAD"


####
step "Relay SuperchainWETH through the L2toL2CrossDomainMessenger"
function relay_message() {
    cast_l2_2 send "$L2ToL2CrossDomainMessenger" \
        "relayMessage((address,uint256,uint256,uint256,uint256),bytes)" \
        "($L2ToL2CrossDomainMessenger,$BLOCK_NUMBER,$LOG_INDEX,$TIMESTAMP,$L2_1_CHAINID)" \
        "$EVENT_PAYLOAD" \
        --mnemonic "$MNEMONIC"
}
relay_message


####
step "Retry relay SuperchainWETH through the L2toL2CrossDomainMessenger, should be reverted"
expect_error_message "execution reverted" "$(! relay_message 2>&1)"
