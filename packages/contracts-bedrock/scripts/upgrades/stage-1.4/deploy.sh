#!/bin/bash

set -euo pipefail

# shellcheck disable=SC1091
source "./common.sh"
# shellcheck disable=SC2046
export $(grep -v '^#' .env | xargs)

reqenv "ETH_RPC_URL"
reqenv "DEPLOY_CONFIG_PATH"
reqenv "PRIVATE_KEY"
reqenv "KONA_RELEASE"
reqenv "ASTERISC_KONA_GAME_TYPE"

# ----------------
# Constants
# ----------------

MONOREPO_ROOT="$(git rev-parse --show-toplevel)"
OP_DEPLOYER="$MONOREPO_ROOT/op-deployer/bin/op-deployer"
FORGE_ARTIFACTS_PATH="$MONOREPO_ROOT/packages/contracts-bedrock/forge-artifacts"
L2_CHAIN_ID="$(jq -r '.l2ChainID' "$DEPLOY_CONFIG_PATH")"

# ----------------
# Deployment
# ----------------

# 1. Fetch the superchain-registry addresses
REGISTRY_ADDRESSES="$(curl -LsS https://raw.githubusercontent.com/ethereum-optimism/superchain-registry/refs/heads/main/superchain/extra/addresses/addresses.json)"
ANCHOR_STATE_REGISTRY_PROXY_ADDR="$(jq -r ".[\"$L2_CHAIN_ID\"].AnchorStateRegistryProxy" <<< "$REGISTRY_ADDRESSES")"
PREIMAGE_ORACLE_ADDR="$(jq -r ".[\"$L2_CHAIN_ID\"].PreimageOracle" <<< "$REGISTRY_ADDRESSES")"
echo "✨ Fetched addresses for L2 Chain \"$L2_CHAIN_ID\" from the superchain-registry"
echo "  -> AnchorStateRegistryProxy: $ANCHOR_STATE_REGISTRY_PROXY_ADDR"
echo "  -> PreimageOracle: $PREIMAGE_ORACLE_ADDR"

# 2. Build `op-deployer`
(cd "$MONOREPO_ROOT/op-deployer" && just build)

# 3. Build contracts
(cd "$MONOREPO_ROOT/packages/contracts-bedrock" && forge build)

# 4. Download Kona Release Artifacts + Extract Prestate Hash
echo "📦 Downloading Kona Release Artifacts ($KONA_RELEASE)"
TEMP_DIR="$(mktemp -d)"
curl \
  -L "https://github.com/anton-rs/kona/releases/download/$KONA_RELEASE/prestate-artifacts-asterisc-arm64.tar.zst" \
  -o "$TEMP_DIR/prestate-artifacts.tar.zst"
tar --zstd -xf "$TEMP_DIR/prestate-artifacts.tar.zst" -C "$TEMP_DIR"
KONA_PRESTATE_HASH="$(jq -r '.pre' "$TEMP_DIR/prestate-artifacts-asterisc/prestate-proof.json")"
rm -rf "$TEMP_DIR"
echo "✨ Kona Prestate Hash: $KONA_PRESTATE_HASH"

# 5. Deploy asterisc VM
ASTERISC_VM_ADDR="$(capture_output "$OP_DEPLOYER" \
  bootstrap asterisc \
  --artifacts-locator "file://$FORGE_ARTIFACTS_PATH" \
  --l1-rpc-url "$L1_RPC_URL" \
  --preimage-oracle "$PREIMAGE_ORACLE_ADDR" \
  --private-key "$PRIVATE_KEY" | sed -n '/^{/,/^}$/p' | jq -r '.AsteriscSingleton')"
echo "✨ Deployed Asterisc @ $ASTERISC_VM_ADDR"

# 6. Deploy new DelayedWETH proxy
DELAYED_WETH_PROXY_ADDR="$(capture_output "$OP_DEPLOYER" \
  bootstrap delayedweth \
  --artifacts-locator "file://$FORGE_ARTIFACTS_PATH" \
  --l1-rpc-url "$L1_RPC_URL" \
  --private-key "$PRIVATE_KEY" | sed -n '/^{/,/^}$/p' | jq -r '.DelayedWethProxy')"
echo "✨ Deployed DelayedWETHProxy @ $DELAYED_WETH_PROXY_ADDR"

# 7. Deploy Fault Dispute Game
FDG_ADDR="$(capture_output "$OP_DEPLOYER" \
  bootstrap disputegame \
  --artifacts-locator "file://$FORGE_ARTIFACTS_PATH" \
  --l1-rpc-url "$L1_RPC_URL" \
  --game-kind "FaultDisputeGame" \
  --game-type "$ASTERISC_KONA_GAME_TYPE" \
  --absolute-prestate "$KONA_PRESTATE_HASH" \
  --l2-chain-id "$L2_CHAIN_ID" \
  --max-game-depth "$(jq -r '.faultGameMaxDepth' "$DEPLOY_CONFIG_PATH")" \
  --split-depth "$(jq -r '.faultGameSplitDepth' "$DEPLOY_CONFIG_PATH")" \
  --clock-extension "$(jq -r '.faultGameClockExtension' "$DEPLOY_CONFIG_PATH")" \
  --max-clock-duration "$(jq -r '.faultGameMaxClockDuration' "$DEPLOY_CONFIG_PATH")" \
  --vm "$ASTERISC_VM_ADDR" \
  --delayed-weth-proxy "$DELAYED_WETH_PROXY_ADDR" \
  --anchor-state-registry-proxy "$ANCHOR_STATE_REGISTRY_PROXY_ADDR" \
  --private-key "$PRIVATE_KEY" | sed -n '/^{/,/^}$/p' | jq -r '.DisputeGameImpl')"
echo "✨ Deployed FaultDisputeGame @ $FDG_ADDR"

# 8. Export deployment addresses
printf "\n✅ Deployment complete\n"
DEPLOYMENTS=$(cat <<EOF
{
  "l2ChainID": "$L2_CHAIN_ID",
  "contracts": {
    "RISCV": "$ASTERISC_VM_ADDR",
    "DelayedWETHProxy": "$DELAYED_WETH_PROXY_ADDR",
    "FaultDisputeGame": "$FDG_ADDR"
  }
}
EOF
)

echo "$DEPLOYMENTS" > deployments.json
echo "📝 Wrote deployments to $(pwd)/deployments.json"
