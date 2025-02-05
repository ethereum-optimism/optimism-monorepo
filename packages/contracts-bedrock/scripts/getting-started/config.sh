#!/usr/bin/env bash

# This script is used to generate the getting-started.json configuration file
# used in the Getting Started quickstart guide on the docs site. Avoids the
# need to have the getting-started.json committed to the repo since it's an
# invalid JSON file when not filled in, which is annoying.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
CONTRACTS_BASE=$(dirname "$(dirname "$SCRIPT_DIR")")

reqenv() {
    if [ -z "${!1}" ]; then
        echo "Error: environment variable '$1' is undefined"
        exit 1
    fi
}

append_with_default() {
    json_key="$1"
    env_var_name="$2"
    default_value="$3"
    var_value="${!env_var_name}"

    if [ -z "$var_value" ] || [ "$var_value" == "None" ]; then
        var_value="$default_value"
    fi

    echo "  \"$json_key\": \"$var_value\"," >> tmp_config.json
}

# Check required environment variables
reqenv "GS_ADMIN_ADDRESS"
reqenv "GS_BATCHER_ADDRESS"
reqenv "GS_PROPOSER_ADDRESS"
reqenv "GS_SEQUENCER_ADDRESS"
reqenv "L1_RPC_URL"
reqenv "L1_CHAIN_ID"
reqenv "L2_CHAIN_ID"
reqenv "L1_BLOCK_TIME"
reqenv "L2_BLOCK_TIME"

# Get the latest block timestamp and hash
block=$(cast block latest --rpc-url "$L1_RPC_URL")
timestamp=$(echo "$block" | awk '/timestamp/ { print $2 }')
blockhash=$(echo "$block" | awk '/hash/ { print $2 }')

# Start generating the config file in a temporary file

cat << EOL > tmp_config.json
{

 "l1StartingBlockTag": "$blockhash",

  "l1ChainID": $L1_CHAIN_ID,
  "l2ChainID": $L2_CHAIN_ID,
  "l2BlockTime": $L2_BLOCK_TIME,
  "l1BlockTime": $L1_BLOCK_TIME,

  "maxSequencerDrift": 600,
  "sequencerWindowSize": 3600,
  "channelTimeout": 300,

  "p2pSequencerAddress": "$GS_SEQUENCER_ADDRESS",
  "batchInboxAddress": "0xff00000000000000000000000000000000042069",
  "batchSenderAddress": "$GS_BATCHER_ADDRESS",

  "l2OutputOracleSubmissionInterval": 120,
  "l2OutputOracleStartingBlockNumber": 0,
  "l2OutputOracleStartingTimestamp": $timestamp,

  "l2OutputOracleProposer": "$GS_PROPOSER_ADDRESS",
  "l2OutputOracleChallenger": "$GS_ADMIN_ADDRESS",

  "finalizationPeriodSeconds": 12,

  "proxyAdminOwner": "$GS_ADMIN_ADDRESS",
  "baseFeeVaultRecipient": "$GS_ADMIN_ADDRESS",
  "l1FeeVaultRecipient": "$GS_ADMIN_ADDRESS",
  "sequencerFeeVaultRecipient": "$GS_ADMIN_ADDRESS",
  "finalSystemOwner": "$GS_ADMIN_ADDRESS",
  "superchainConfigGuardian": "$GS_ADMIN_ADDRESS",

  "baseFeeVaultMinimumWithdrawalAmount": "0x8ac7230489e80000",
  "l1FeeVaultMinimumWithdrawalAmount": "0x8ac7230489e80000",
  "sequencerFeeVaultMinimumWithdrawalAmount": "0x8ac7230489e80000",
  "baseFeeVaultWithdrawalNetwork": 0,
  "l1FeeVaultWithdrawalNetwork": 0,
  "sequencerFeeVaultWithdrawalNetwork": 0,

  "gasPriceOracleOverhead": 0,
  "gasPriceOracleScalar": 1000000,

  "enableGovernance": true,
  "governanceTokenSymbol": "OP",
  "governanceTokenName": "Optimism",
  "governanceTokenOwner": "$GS_ADMIN_ADDRESS",

  "l2GenesisBlockGasLimit": "0x1c9c380",
  "l2GenesisBlockBaseFeePerGas": "0x3b9aca00",

  "eip1559Denominator": 50,
  "eip1559DenominatorCanyon": 250,
  "eip1559Elasticity": 6,
EOL

# Append conditional environment variables with their corresponding default values
# Activate granite fork
if [ -n "${GRANITE_TIME_OFFSET}" ]; then
    append_with_default "l2GenesisGraniteTimeOffset" "GRANITE_TIME_OFFSET" "0x0"
fi
# Activate holocene fork
if [ -n "${HOLOCENE_TIME_OFFSET}" ]; then
    append_with_default "l2GenesisHoloceneTimeOffset" "HOLOCENE_TIME_OFFSET" "0x0"
fi

# Activate the interop fork
if [ -n "${INTEROP_TIME_OFFSET}" ]; then
    append_with_default "l2GenesisInteropTimeOffset" "INTEROP_TIME_OFFSET" "0x0"
fi

# Already forked updates
append_with_default "l2GenesisFjordTimeOffset" "FJORD_TIME_OFFSET" "0x0"
append_with_default "l2GenesisRegolithTimeOffset" "REGOLITH_TIME_OFFSET" "0x0"
append_with_default "l2GenesisEcotoneTimeOffset" "ECOTONE_TIME_OFFSET" "0x0"
append_with_default "l2GenesisDeltaTimeOffset" "DELTA_TIME_OFFSET" "0x0"
append_with_default "l2GenesisCanyonTimeOffset" "CANYON_TIME_OFFSET" "0x0"

# Continue generating the config file
cat << EOL >> tmp_config.json
  "systemConfigStartBlock": 0,

  "requiredProtocolVersion": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "recommendedProtocolVersion": "0x0000000000000000000000000000000000000000000000000000000000000000",

  "faultGameAbsolutePrestate": "0x03c7ae758795765c6664a5d39bf63841c71ff191e9189522bad8ebff5d4eca98",
  "faultGameMaxDepth": 44,
  "faultGameClockExtension": 0,
  "faultGameMaxClockDuration": 1200,
  "faultGameGenesisBlock": 0,
  "faultGameGenesisOutputRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "faultGameSplitDepth": 14,
  "faultGameWithdrawalDelay": 600,

  "preimageOracleMinProposalSize": 1800000,
  "preimageOracleChallengePeriod": 300
}
EOL

# Write the final config file
mv tmp_config.json "$CONTRACTS_BASE/deploy-config/getting-started.json"
