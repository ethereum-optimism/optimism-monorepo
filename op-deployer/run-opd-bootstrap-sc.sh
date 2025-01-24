#!/bin/bash

./op-deployer bootstrap superchain \
  --artifacts-locator tag://op-contracts/v1.8.0-rc.4 \
  --l1-rpc-url $SEP_RPC_URL \
  --private-key 0x3bb7422a9707cc8b76ce10ae266135578474ffbd403df2a9bd258dcc83f6efa1 \
  --recommended-protocol-version 0x0000000000000000000000000000000000000000000000000000000000000000 \
  --required-protocol-version 0x0000000000000000000000000000000000000000000000000000000000000000 \
  --superchain-proxy-admin-owner 0xA6aFc9612b504202E0A5F6cf3C8E89C49EA06037 \
  --protocol-versions-owner 0xA6aFc9612b504202E0A5F6cf3C8E89C49EA06037 \
  --guardian 0xA6aFc9612b504202E0A5F6cf3C8E89C49EA06037

# {
#   "SuperchainProxyAdmin": "0x567732d483a5307535a77a12e1c3a16b7e4e20e7",
#   "SuperchainConfigImpl": "0x6579cabfcf54327c3195f3b4a1fa2f15e18251b9",
#   "SuperchainConfigProxy": "0x553eb72a9f1fc85dcffeb2ebce652726d06e0bbf",
#   "ProtocolVersionsImpl": "0xbbed762e39bc32b12dac0d9e2f0fc5ffa9c054ef",
#   "ProtocolVersionsProxy": "0xe14770dbc8b19f165d49aed9f1eefccfeb8980e6"
# }
