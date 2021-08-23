#!/bin/sh

# FIXME: Cannot use set -e since bash is not installed in Dockerfile
# set -e

RETRIES=${RETRIES:-40}
VERBOSITY=${VERBOSITY:-6}

if [[ ! -z "$URL" ]]; then
    # get the addrs from the URL provided
    ADDRESSES=$(curl --fail --show-error --silent --retry-connrefused --retry $RETRIES --retry-delay 5 $URL)

    function envSet() {
        VAR=$1
        export $VAR=$(echo $ADDRESSES | jq -r ".$2")
    }

    # set all the necessary env vars
    envSet ETH1_ADDRESS_RESOLVER_ADDRESS  AddressManager
    envSet ETH1_L1_CROSS_DOMAIN_MESSENGER_ADDRESS Proxy__OVM_L1CrossDomainMessenger

    # set the address to the proxy gateway if possible
    envSet ETH1_L1_STANDARD_BRIDGE_ADDRESS Proxy__OVM_L1StandardBridge
    if [ $ETH1_L1_STANDARD_BRIDGE_ADDRESS == null ]; then
        envSet ETH1_L1_STANDARD_BRIDGE_ADDRESS OVM_L1StandardBridge
    fi
fi

# wait for the dtl to be up, else geth will crash if it cannot connect
curl \
    --fail \
    --show-error \
    --silent \
    --output /dev/null \
    --retry-connrefused \
    --retry $RETRIES \
    --retry-delay 1 \
    $ROLLUP_CLIENT_HTTP

exec geth --verbosity="$VERBOSITY" "$@"
