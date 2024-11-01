#!/bin/bash

set -eu

# Run this with workdir set as root of the repo
if [ -f "../versions.json" ]; then
    echo "Running create-chains script."
else
    echo "Cannot run create-chains script, must be in interop-devnet dir, but currently in:"
    pwd
    exit 1
fi

# Navigate to repository root
cd ..

# Check if already created
if [ -d ".devnet-interop" ]; then
    echo "Already created chains."
    exit 1
else
    echo "Creating new interop devnet chain configs"
fi

export OP_INTEROP_MNEMONIC="test test test test test test test test test test test junk"

go run ./op-node/cmd interop dev-setup \
  --artifacts-dir=packages/contracts-bedrock/forge-artifacts \
  --foundry-dir=packages/contracts-bedrock \
  --l1.chainid=900100 \
  --l2.chainids=900200,900201 \
  --out-dir=".devnet-interop" \
  --log.format=logfmt \
  --log.level=info

# create L1 CL genesis
eth2-testnet-genesis deneb \
  --config=./ops-bedrock/beacon-data/config.yaml \
  --preset-phase0=minimal \
  --preset-altair=minimal \
  --preset-bellatrix=minimal \
  --preset-capella=minimal \
  --preset-deneb=minimal \
  --eth1-config=.devnet-interop/genesis/l1/genesis.json \
  --state-output=.devnet-interop/genesis/l1/beaconstate.ssz \
  --tranches-dir=.devnet-interop/genesis/l1/tranches \
  --mnemonics=./ops-bedrock/mnemonics.yaml \
  --eth1-withdrawal-address=0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  --eth1-match-genesis-time

echo "Writing env files now..."

# write env files for each L2 service

chain_env=".devnet-interop/env/l2/900200"
mkdir -p "$chain_env"
key_cmd="go run ./op-node/cmd interop devkey secret --domain=chain-operator --chainid=900200"
# op-node
echo "OP_NODE_P2P_SEQUENCER_KEY=$($key_cmd --name=sequencer-p2p)" >> "$chain_env/op-node.env"
# proposer
echo "OP_PROPOSER_PRIVATE_KEY=$($key_cmd --name=proposer)" >> "$chain_env/op-proposer.env"
echo "OP_PROPOSER_GAME_FACTORY_ADDRESS=$(jq -r .DisputeGameFactoryProxy .devnet-interop/deployments/l2/900200/addresses.json)" >> "$chain_env/op-proposer.env"
# batcher
echo "OP_BATCHER_PRIVATE_KEY=$($key_cmd --name=batcher)" >> "$chain_env/op-batcher.env"

chain_env=".devnet-interop/env/l2/900201"
mkdir -p "$chain_env"
key_cmd="go run ./op-node/cmd interop devkey secret --domain=chain-operator --chainid=900201"
# op-node
echo "OP_NODE_P2P_SEQUENCER_KEY=$($key_cmd --name=sequencer-p2p)" >> "$chain_env/op-node.env"
# proposer
echo "OP_PROPOSER_PRIVATE_KEY=$($key_cmd --name=proposer)" >> "$chain_env/op-proposer.env"
echo "OP_PROPOSER_GAME_FACTORY_ADDRESS=$(jq -r .DisputeGameFactoryProxy .devnet-interop/deployments/l2/900201/addresses.json)" >> "$chain_env/op-proposer.env"
# batcher
echo "OP_BATCHER_PRIVATE_KEY=$($key_cmd --name=batcher)" >> "$chain_env/op-batcher.env"

echo "Interop devnet setup is complete!"
