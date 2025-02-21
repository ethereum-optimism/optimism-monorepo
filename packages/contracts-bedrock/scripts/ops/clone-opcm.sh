#!/bin/bash

set -euo pipefail

CREATE2_DEPLOYER="0x4e59b44847b379578588920ca78fbf26c0b4956c"

# Ensure environment variables are set
if [ -z "$SOURCE_RPC" ]; then
    echo "Error: SOURCE_RPC is not set."
    exit 1
fi

if [ -z "$TARGET_RPC" ]; then
    echo "Error: TARGET_RPC is not set."
    exit 1
fi

if [ -z "$OPCM_ADDRESS" ]; then
    echo "Error: OPCM_ADDRESS is not set."
    exit 1
fi

if [ -z "$PRIVATE_KEY" ]; then
    echo "Error: PRIVATE_KEY is not set."
    exit 1
fi

if [ -z "$ETHERSCAN_API_KEY" ]; then
    echo "Error: ETHERSCAN_API_KEY is not set."
    exit 1
fi

if [ -z "$SUPERCHAIN_CONFIG" ]; then
    echo "Error: SUPERCHAIN_CONFIG is not set."
    exit 1
fi

if [ -z "$PROTOCOL_VERSIONS" ]; then
    echo "Error: PROTOCOL_VERSIONS is not set."
    exit 1
fi

get_contract_creation_tx() {
  local contract_address=$1

  curl -s -X GET "https://api.etherscan.io/api?module=contract&action=getcontractcreation&contractaddresses=$contract_address&apikey=$ETHERSCAN_API_KEY" | jq -r '.result[0].txHash'
}

get_contract_creation_info() {
  local tx_hash=$1

  cast tx "$tx_hash" --rpc-url "$SOURCE_RPC" --json
}

# Get implementation addresses from OPCM
echo "Fetching implementation addresses..."
IMPLEMENTATIONS=$(cast call "$OPCM_ADDRESS" 'implementations()(address,address,address,address,address,address,address,address,address)' --rpc-url "$SOURCE_RPC")

# Parse the implementation addresses into individual variables
L1_ERC721_BRIDGE_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '1p')
OPTIMISM_PORTAL_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '2p')
SYSTEM_CONFIG_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '3p')
OPTIMISM_MINTABLE_ERC20_FACTORY_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '4p')
L1_CROSS_DOMAIN_MESSENGER_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '5p')
L1_STANDARD_BRIDGE_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '6p')
DISPUTE_GAME_FACTORY_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '7p')
DELAYED_WETH_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '8p')
MIPS_IMPL=$(echo "$IMPLEMENTATIONS" | sed -n '9p')

# Create arrays of implementation names and addresses
IMPL_NAMES=(
    "L1ERC721Bridge"
    "OptimismPortal"
    "SystemConfig"
    "OptimismMintableERC20Factory"
    "L1CrossDomainMessenger"
    "L1StandardBridge"
    "DisputeGameFactory"
    "DelayedWETH"
    "MIPS"
)

IMPL_ADDRESSES=(
    "$L1_ERC721_BRIDGE_IMPL"
    "$OPTIMISM_PORTAL_IMPL"
    "$SYSTEM_CONFIG_IMPL"
    "$OPTIMISM_MINTABLE_ERC20_FACTORY_IMPL"
    "$L1_CROSS_DOMAIN_MESSENGER_IMPL"
    "$L1_STANDARD_BRIDGE_IMPL"
    "$DISPUTE_GAME_FACTORY_IMPL"
    "$DELAYED_WETH_IMPL"
    "$MIPS_IMPL"
)

# Arrays to store new addresses and their corresponding names
NEW_IMPL_ADDRESSES=()

# Clone each implementation
echo "Cloning implementation contracts..."
for i in "${!IMPL_NAMES[@]}"; do
    impl_name="${IMPL_NAMES[$i]}"
    impl_address="${IMPL_ADDRESSES[$i]}"
    echo "Processing $impl_name at address $impl_address..."

    # Special handling for MIPS due to immutable preimage oracle
    if [ "$impl_name" = "MIPS" ]; then
        echo "Special handling for MIPS contract..."

        # Get the preimage oracle address from the MIPS contract
        echo "Getting preimage oracle address from MIPS contract..."
        old_oracle_address=$(cast call "$impl_address" "oracle()(address)" --rpc-url "$SOURCE_RPC")
        echo "Found preimage oracle at $old_oracle_address"

        # Clone the preimage oracle first
        echo "Cloning preimage oracle..."
        oracle_tx=$(get_contract_creation_tx "$old_oracle_address" "$ETHERSCAN_API_KEY")
        oracle_info=$(get_contract_creation_info "$oracle_tx")
        oracle_input=$(echo "$oracle_info" | jq -r '.input')

        # Remove 0x prefix
        oracle_input=${oracle_input:2}

        # Extract salt and initcode from the creation input
        oracle_salt="${oracle_input:0:64}"
        oracle_code="${oracle_input:64}"

        # Precompute the oracle address
        new_oracle_address=$(cast create2 -d $CREATE2_DEPLOYER --salt "0x$oracle_salt" -i "0x$oracle_code")

        # Deploy oracle if not already deployed
        code=$(cast code "$new_oracle_address" --rpc-url "$TARGET_RPC")
        if [ "$code" = "0x" ]; then
            echo "Deploying oracle..."
            cast send --private-key "$PRIVATE_KEY" --rpc-url "$TARGET_RPC" $CREATE2_DEPLOYER "0x$oracle_input"
        fi
        echo "Preimage oracle at $new_oracle_address"

        # Now handle MIPS deployment with the new oracle address
        creation_tx=$(get_contract_creation_tx "$impl_address" "$ETHERSCAN_API_KEY")
        creation_info=$(get_contract_creation_info "$creation_tx")
        creation_input=$(echo "$creation_info" | jq -r '.input')

        # Remove 0x prefix for string manipulation
        creation_input=${creation_input:2}

        # Replace old oracle address with new one in the bytecode
        # Convert addresses to lowercase for consistent replacement
        old_oracle_lower=$(echo "$old_oracle_address" | tr '[:upper:]' '[:lower:]')
        old_oracle_lower=${old_oracle_lower:2}  # Remove 0x
        new_oracle_lower=$(echo "$new_oracle_address" | tr '[:upper:]' '[:lower:]')
        new_oracle_lower=${new_oracle_lower:2}  # Remove 0x

        # shellcheck disable=SC2001
        modified_input=$(echo "$creation_input" | sed "s/$old_oracle_lower/$new_oracle_lower/g")

        # Deploy MIPS using Create1
        echo "Deploying MIPS via Create1..."
        new_address=$(cast send --private-key "$PRIVATE_KEY" \
            --rpc-url "$TARGET_RPC" \
            --create "0x$modified_input" \
            --json | jq -r '.contractAddress')

        echo "✅ Successfully deployed MIPS to $new_address"
        NEW_IMPL_ADDRESSES+=("$new_address")
        continue
    fi

    # Regular implementation handling for non-MIPS contracts
    # Get the contract creation transaction
    echo "Fetching contract creation transaction..."
    creation_tx=$(get_contract_creation_tx "$impl_address" "$ETHERSCAN_API_KEY")

    if [ -z "$creation_tx" ]; then
        echo "Failed to get creation transaction for $impl_name"
        exit 1
    fi

    # Get the contract creation input
    echo "Fetching contract creation input..."
    creation_info=$(get_contract_creation_info "$creation_tx")
    creation_input=$(echo "$creation_info" | jq -r '.input')
    # Remove the 0x from the input since we need to slice it
    creation_input=${creation_input:2}
    creator_to=$(echo "$creation_info" | jq -r '.to')

    if [ "$creator_to" = "$CREATE2_DEPLOYER" ]; then
        echo "Original contract was deployed via Create2..."
        # Extract salt and initcode from the creation input
        # Format: salt(32) + initcode
        salt="${creation_input:0:64}"
        initcode="${creation_input:64}"

        # Precompute the create2 address
        echo "Precomputing Create2 address..."
        precomputed_address=$(cast create2 \
            -d $CREATE2_DEPLOYER \
            --salt "0x$salt" \
            -i "0x$initcode")

        echo "Precomputed address: $precomputed_address"

        echo "Checking if the new address already has code..."
        code=$(cast code "$precomputed_address" --rpc-url "$TARGET_RPC")
        if [ "$code" != "0x" ]; then
            echo "The new address already has code, skipping..."
            NEW_IMPL_ADDRESSES+=("$precomputed_address")
            continue
        fi

        # Deploy the contract to the target chain using Create2Deployer
        echo "Deploying $impl_name to target chain via Create2..."

        cast send --private-key "$PRIVATE_KEY" \
            --rpc-url "$TARGET_RPC" \
            $CREATE2_DEPLOYER \
            "0x$creation_input"

        # Validate that the new address has code
        echo "Validating that the new address has code..."
        code=$(cast code "$precomputed_address" --rpc-url "$TARGET_RPC")
        if [ "$code" == "0x" ]; then
            echo "❌ No code was deployed for $impl_name"
            continue
        fi

        echo "✅ Successfully cloned $impl_name to $precomputed_address"
        NEW_IMPL_ADDRESSES+=("$precomputed_address")
    else
        echo "Original contract was deployed via regular Create..."
        # For regular create, we just need the initialization code
        initcode="0x$creation_input"

        echo "Deploying $impl_name to target chain via Create..."
        new_address=$(cast send --private-key "$PRIVATE_KEY" \
            --rpc-url "$TARGET_RPC" \
            --create "$initcode" \
            --json | jq -r '.contractAddress')

        echo "✅ Successfully cloned $impl_name to $new_address"
        NEW_IMPL_ADDRESSES+=("$new_address")
    fi
done

echo "Fetching blueprint addresses..."
BLUEPRINT_ADDRESSES=$(cast call "$OPCM_ADDRESS" 'blueprints()(address,address,address,address,address,address,address,address)' --rpc-url "$SOURCE_RPC")

# Parse the blueprint addresses into individual variables
ADDRESS_MANAGER_BLUEPRINT=$(echo "$BLUEPRINT_ADDRESSES" | sed -n '1p')
PROXY_BLUEPRINT=$(echo "$BLUEPRINT_ADDRESSES" | sed -n '2p')
PROXY_ADMIN_BLUEPRINT=$(echo "$BLUEPRINT_ADDRESSES" | sed -n '3p')
L1_CHUG_SPLASH_PROXY_BLUEPRINT=$(echo "$BLUEPRINT_ADDRESSES" | sed -n '4p')
RESOLVED_DELEGATE_PROXY_BLUEPRINT=$(echo "$BLUEPRINT_ADDRESSES" | sed -n '5p')
ANCHOR_STATE_REGISTRY_BLUEPRINT=$(echo "$BLUEPRINT_ADDRESSES" | sed -n '6p')
PERMISSIONED_DISPUTE_GAME_1_BLUEPRINT=$(echo "$BLUEPRINT_ADDRESSES" | sed -n '7p')
PERMISSIONED_DISPUTE_GAME_2_BLUEPRINT=$(echo "$BLUEPRINT_ADDRESSES" | sed -n '8p')

# Create arrays of blueprint names and addresses
BLUEPRINT_NAMES=(
    "AddressManager"
    "Proxy"
    "ProxyAdmin"
    "L1ChugSplashProxy"
    "ResolvedDelegateProxy"
    "AnchorStateRegistry"
    "PermissionedDisputeGame1"
    "PermissionedDisputeGame2"
)

BLUEPRINT_ADDRESSES=(
    "$ADDRESS_MANAGER_BLUEPRINT"
    "$PROXY_BLUEPRINT"
    "$PROXY_ADMIN_BLUEPRINT"
    "$L1_CHUG_SPLASH_PROXY_BLUEPRINT"
    "$RESOLVED_DELEGATE_PROXY_BLUEPRINT"
    "$ANCHOR_STATE_REGISTRY_BLUEPRINT"
    "$PERMISSIONED_DISPUTE_GAME_1_BLUEPRINT"
    "$PERMISSIONED_DISPUTE_GAME_2_BLUEPRINT"
)

# Array to store new blueprint addresses
NEW_BLUEPRINT_ADDRESSES=()

# Function to create blueprint deployer bytecode
create_blueprint_deployer_bytecode() {
    local initcode=$1

    # Concatenate everything
    echo "600D380380600D6000396000f3${initcode}"
}

# Clone each blueprint
echo "Cloning blueprint contracts..."
for i in "${!BLUEPRINT_NAMES[@]}"; do
    blueprint_name="${BLUEPRINT_NAMES[$i]}"
    blueprint_address="${BLUEPRINT_ADDRESSES[$i]}"
    echo "Processing $blueprint_name at address $blueprint_address..."

    # Get the deployed bytecode from the source chain
    echo "Fetching blueprint bytecode..."
    deployed_code=$(cast code "$blueprint_address" --rpc-url "$SOURCE_RPC")
    if [ -z "$deployed_code" ] || [ "$deployed_code" = "0x" ]; then
        echo "Bad bytecode for $blueprint_name"
        continue
    fi
    # Remove 0x prefix
    deployed_code=${deployed_code:2}

    # Create the blueprint deployer bytecode
    echo "Creating blueprint deployer bytecode..."
    deployer_bytecode=$(create_blueprint_deployer_bytecode "$deployed_code")

    # Generate salt from version and blueprint name
    salt="$(cast hz)"

    # Precompute the create2 address
    echo "Precomputing Create2 address..."
    precomputed_address=$(cast create2 \
        -d $CREATE2_DEPLOYER \
        --salt "$salt" \
        -i "0x$deployer_bytecode")

    echo "Precomputed address: $precomputed_address"

    # Check if the contract is already deployed
    echo "Checking if the new address already has code..."
    code=$(cast code "$precomputed_address" --rpc-url "$TARGET_RPC")
    if [ "$code" != "0x" ]; then
        echo "The new address already has code, skipping..."
        NEW_BLUEPRINT_ADDRESSES+=("$precomputed_address")
        continue
    fi

    # Deploy the blueprint using Create2
    echo "Deploying $blueprint_name blueprint via Create2..."
    cast send --private-key "$PRIVATE_KEY" \
        --rpc-url "$TARGET_RPC" \
        $CREATE2_DEPLOYER \
        "0x${salt:2}$deployer_bytecode"

    # Validate that the new address has code
    echo "Validating that the new address has code..."
    code=$(cast code "$precomputed_address" --rpc-url "$TARGET_RPC")
    if [ "$code" == "0x" ]; then
        echo "❌ No code was deployed for $blueprint_name"
        exit 1
    fi

    echo "✅ Successfully deployed $blueprint_name blueprint to $precomputed_address"
    NEW_BLUEPRINT_ADDRESSES+=("$precomputed_address")
done

# Encode the constructor call
echo "Encoding constructor call..."
CONSTRUCTOR_ARGS=$(cast abi-encode "constructor(address,address,string,(address,address,address,address,address,address,address,address),(address,address,address,address,address,address,address,address,address))" \
    "$SUPERCHAIN_CONFIG" \
    "$PROTOCOL_VERSIONS" \
    "v1.8.0-rc.4" \
    "(${NEW_BLUEPRINT_ADDRESSES[0]},${NEW_BLUEPRINT_ADDRESSES[1]},${NEW_BLUEPRINT_ADDRESSES[2]},${NEW_BLUEPRINT_ADDRESSES[3]},${NEW_BLUEPRINT_ADDRESSES[4]},${NEW_BLUEPRINT_ADDRESSES[5]},${NEW_BLUEPRINT_ADDRESSES[6]},${NEW_BLUEPRINT_ADDRESSES[7]})" \
    "(${NEW_IMPL_ADDRESSES[0]},${NEW_IMPL_ADDRESSES[1]},${NEW_IMPL_ADDRESSES[2]},${NEW_IMPL_ADDRESSES[3]},${NEW_IMPL_ADDRESSES[4]},${NEW_IMPL_ADDRESSES[5]},${NEW_IMPL_ADDRESSES[6]},${NEW_IMPL_ADDRESSES[7]},${NEW_IMPL_ADDRESSES[8]})")

# strip 0x
CONSTRUCTOR_ARGS=${CONSTRUCTOR_ARGS:2}

# Deploy the OPCM
echo "Deploying OPCM..."

# Get the contract creation transaction
echo "Fetching OPCM creation transaction..."
opcm_creation_tx=$(get_contract_creation_tx "$OPCM_ADDRESS" "$ETHERSCAN_API_KEY")

# Get the contract creation input
echo "Fetching OPCM creation input..."
opcm_creation_info=$(get_contract_creation_info "$opcm_creation_tx")
opcm_creation_input=$(echo "$opcm_creation_info" | jq -r '.input')
# Remove 0x prefix
opcm_creation_input=${opcm_creation_input:2}

# Calculate the position to cut at (length - 1408)
input_length=${#opcm_creation_input}
cut_position=$((input_length - 1408))

# Remove the last 1408 characters (original constructor args) and append our new ones
echo "Creating new deployment transaction..."
deployment_input="0x${opcm_creation_input:0:$cut_position}${CONSTRUCTOR_ARGS}"

# Deploy OPCM to the target chain
echo "Deploying OPCM to target chain..."
new_opcm_address=$(cast send --private-key "$PRIVATE_KEY" \
    --rpc-url "$TARGET_RPC" \
    --create "$deployment_input" \
    --json | jq -r '.contractAddress')

echo "✅ Successfully deployed OPCM to $new_opcm_address"
