#!/usr/bin/env bash
set -e

SUPERCHAIN="mainnet"

echo "building bundle for op $SUPERCHAIN..."
(PROXY_ADMIN_ADDR=0x543bA4AADBAb8f9025686Bd03993043599c6fB04 SYSTEM_CONFIG_PROXY_ADDR=0x229047fed2591dbec1eF1118d64F7aF3dB9EB290 just sys-cfg-bundle "$PWD"/op)

echo "building bundle for mode $SUPERCHAIN..."
(PROXY_ADMIN_ADDR=0x470d87b1dae09a454A43D1fD772A561a03276aB7 SYSTEM_CONFIG_PROXY_ADDR=0x5e6432F18Bc5d497B1Ab2288a025Fbf9D69E2221 just sys-cfg-bundle "$PWD"/mode)

echo "building bundle for metal $SUPERCHAIN..."
(PROXY_ADMIN_ADDR=0x37Ff0ae34dadA1A95A4251d10ef7Caa868c7AC99 SYSTEM_CONFIG_PROXY_ADDR=0x7BD909970B0EEdcF078De6Aeff23ce571663b8aA just sys-cfg-bundle "$PWD"/metal)

echo "building bundle for zora $SUPERCHAIN..."
(PROXY_ADMIN_ADDR=0xD4ef175B9e72cAEe9f1fe7660a6Ec19009903b49 SYSTEM_CONFIG_PROXY_ADDR=0xA3cAB0126d5F504B071b81a3e8A2BBBF17930d86 just sys-cfg-bundle "$PWD"/zora)

echo "building bundle for arena-z $SUPERCHAIN..."
(PROXY_ADMIN_ADDR=0xEEFD1782D70824CBcacf9438afab7f353F1797F0 SYSTEM_CONFIG_PROXY_ADDR=0x34A564BbD863C4bf73Eca711Cf38a77C4Ccbdd6A just sys-cfg-bundle "$PWD"/arena-z)


echo "Combining bundles into a super bundle..."

cat <<EOF > superbundle.json
{
  "chainId": 1,
  "metadata": {
    "name": "Holocene Hardfork - Multichain SystemConfig Upgrade",
    "description": "Upgrades the 'SystemConfig' contract for Holocene for {op,mode,metal,zora,arena-z}-$SUPERCHAIN"
  },
  "transactions": []
}
EOF

CONCATENATED_TXS=$(jq -s '.[].transactions' ./op/sys_cfg_bundle.json ./mode/sys_cfg_bundle.json ./metal/sys_cfg_bundle.json ./zora/sys_cfg_bundle.json ./arena-z/sys_cfg_bundle.json | jq -s 'add')
jq --argjson transactions "$CONCATENATED_TXS" '.transactions = $transactions' superbundle.json | jq '.' > temp.json && mv temp.json superbundle.json

echo "wrote concatenated transaction bundle to superbundle.json"

rm -r "$PWD"/op
rm -r "$PWD"/mode
rm -r "$PWD"/metal
rm -r "$PWD"/zora
rm -r "$PWD"/arena-z
