set -e

SUPERCHAIN="sepolia"

echo "building bundle for op $SUPERCHAIN..."
SYSTEM_CONFIG_PROXY_ADDR=0x034edD2A225f7f429A63E0f1D2084B9E0A93b538 just sys-cfg-bundle $PWD/op

echo "building bundle for mode $SUPERCHAIN..."
SYSTEM_CONFIG_PROXY_ADDR=0x15cd4f6e0CE3B4832B33cB9c6f6Fe6fc246754c2 just sys-cfg-bundle $PWD/mode

echo "building bundle for metal $SUPERCHAIN..."
SYSTEM_CONFIG_PROXY_ADDR=0x5D63A8Dc2737cE771aa4a6510D063b6Ba2c4f6F2 just sys-cfg-bundle $PWD/metal

echo "building bundle for zora $SUPERCHAIN..."
SYSTEM_CONFIG_PROXY_ADDR=0xB54c7BFC223058773CF9b739cC5bd4095184Fb08 just sys-cfg-bundle $PWD/zora

echo "Combining bundles into a super bundle..."

cat <<EOF > superbundle.json
{
  "chainId": 11155111,
  "metadata": {
    "name": "Holocene Hardfork - Multichain SystemConfig Upgrade",
    "description": "Upgrades the 'SystemConfig' contract for Holocene for {op,mode,metal,zora}-$SUPERCHAIN"
  },
  "transactions": []
}
EOF

CONCATENATED_TXS=$(jq -s '.[].transactions' ./op/sys_cfg_bundle.json ./mode/sys_cfg_bundle.json ./metal/sys_cfg_bundle.json)
CONCATENATED_TXS=$(echo "$CONCATENATED_TXS" | jq -s 'add')
jq --argjson transactions "$CONCATENATED_TXS" '.transactions = $transactions' superbundle.json | jq '.' > temp.json && mv temp.json superbundle.json

echo "wrote concatenated transaction bundle to superbundle.json"

rm -r $PWD/op
rm -r $PWD/mode
rm -r $PWD/metal
rm -r $PWD/zora
