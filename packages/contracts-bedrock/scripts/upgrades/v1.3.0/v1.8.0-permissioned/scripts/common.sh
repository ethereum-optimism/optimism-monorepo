#!/usr/bin/env bash
set -euo pipefail

# Associative array to store cached TOML content for different URLs
# Used by fetch_standard_address and fetch_superchain_config_address
declare -A CACHED_TOML_CONTENT

# error_handler
#
# Basic error handler
error_handler() {
  echo "Error occurred in ${BASH_SOURCE[1]} at line: ${BASH_LINENO[0]}"
  echo "Error message: $BASH_COMMAND"
  exit 1
}

# Register the error handler
trap error_handler ERR

# reqenv
#
# Checks if a specified environment variable is set.
#
# Arguments:
#   $1 - The name of the environment variable to check
#
# Exits with status 1 if:
#   - The specified environment variable is not set
reqenv() {
    if [ -z "${!1}" ]; then
        echo "Error: $1 is not set"
        exit 1
    fi
}

# load_local_address
#
# Loads an address from a deployments JSON file.
#
# Arguments:
#   $1 - Path to the deployments JSON file
#   $2 - Name of the address to load from the JSON file
#   $3 - Alternative name of the address to load from the JSON file (optional)
#
# Returns:
#   The address associated with the specified name
#
# Exits with status 1 if:
#   - The deployments JSON file is not found
#   - The specified address name is not found in the JSON file
load_local_address() {
    local deployments_json_path="$1"
    local address_name="$2"
    local alt_name="${3:-}"

    if [ ! -f "$deployments_json_path" ]; then
        echo "Error: Deployments JSON file not found: $deployments_json_path"
        exit 1
    fi

    local address=$(jq -r ".$address_name" "$deployments_json_path")

    if [ -z "$address" ] || [ "$address" == "null" ]; then
        if [ -n "$alt_name" ]; then
            address=$(jq -r ".$alt_name" "$deployments_json_path")
        fi

        if [ -z "$address" ] || [ "$address" == "null" ]; then
            echo "Error: $address_name not found in deployments JSON"
            exit 1
        fi
    fi

    echo "$address"
}

# fetch_standard_address
#
# Fetches the implementation address for a given contract from a TOML file.
# The TOML file is downloaded from a URL specified in ADDRESSES_TOML_URL
# environment variable. Results are cached to avoid repeated downloads.
#
# Arguments:
#   $1 - Network name
#   $2 - The release version
#   $3 - The name of the contract to look up
#
# Returns:
#   The implementation address of the specified contract
#
# Exits with status 1 if:
#   - Failed to fetch the TOML file
#   - The release version is not found in the TOML file
#   - The implementation address for the specified contract is not found
fetch_standard_address() {
    local network_name="$1"
    local release_version="$2"
    local contract_name="$3"

    # Determine the correct toml url
    local toml_url="https://raw.githubusercontent.com/ethereum-optimism/superchain-registry/2a82b7922a7d719e4243846c679e4a9946d04b8c/validation/standard/standard-versions"
    if [ "$network_name" = "mainnet" ]; then
        toml_url="$toml_url-mainnet.toml"
    elif [ "$network_name" = "sepolia" ]; then
        toml_url="$toml_url-sepolia.toml"
    else
        echo "Error: NETWORK must be set to 'mainnet' or 'sepolia'"
        exit 1
    fi

    # Fetch the TOML file content from the URL if not already cached for this URL
    if [ -z "${CACHED_TOML_CONTENT[$toml_url]:-}" ]; then
        CACHED_TOML_CONTENT[$toml_url]=$(curl -s "$toml_url")
        if [ $? -ne 0 ]; then
            echo "Error: Failed to fetch TOML file from $toml_url"
            exit 1
        fi
    fi

    # Use the cached content for the current URL
    local toml_content="${CACHED_TOML_CONTENT[$toml_url]}"

    # Find the section for the target release
    local section_content=$(echo "$toml_content" | awk -v version="$release_version" '
        $0 ~ "^\\[\"op-contracts/v" version "\"\\]" {
            flag=1;
            next
        }
        flag && /^\[/ {
            exit
        }
        flag {
            print
        }
    ')
    if [ -z "$section_content" ]; then
        echo "Error: v$release_version release section not found in addresses TOML"
        exit 1
    fi

    # Extract the implementation address for the specified contract
    local address=$(echo "$section_content" | grep -oP "${contract_name} *= *\{.*(address|implementation_address) *= *\"\K[^\"]+") || true

    # Error if not found
    if [ -z "$address" ]; then
        echo "Error: Implementation address for $contract_name not found in v$release_version release"
        exit 1
    fi

    # Return the address
    echo "$address"
}

# fetch_superchain_config_address
#
# Fetches the superchain config address from a TOML file.
# The TOML file is downloaded from a URL based on the network name.
# Results are cached to avoid repeated downloads.
#
# Arguments:
#   $1 - Network name
#
# Returns:
#   The superchain config address
#
# Exits with status 1 if:
#   - Failed to fetch the TOML file
#   - The superchain_config_addr is not found in the TOML file
fetch_superchain_config_address() {
    local network_name="$1"

    # Determine the correct toml url
    local toml_url="https://raw.githubusercontent.com/ethereum-optimism/superchain-registry/2a82b7922a7d719e4243846c679e4a9946d04b8c/superchain/configs/$network_name/superchain.toml"

    # Fetch the TOML file content from the URL if not already cached for this URL
    if [ -z "${CACHED_TOML_CONTENT[$toml_url]:-}" ]; then
        CACHED_TOML_CONTENT[$toml_url]=$(curl -s "$toml_url")
        if [ $? -ne 0 ]; then
            echo "Error: Failed to fetch TOML file from $toml_url"
            exit 1
        fi
    fi

    # Extract the superchain_config_addr from the TOML content
    local superchain_config_addr=$(echo "${CACHED_TOML_CONTENT[$toml_url]}" | grep "superchain_config_addr" | awk -F '"' '{print $2}')

    # Error if not found
    if [ -z "$superchain_config_addr" ]; then
        echo "Error: superchain_config_addr not found in the TOML file"
        exit 1
    fi

    # Return the address
    echo "$superchain_config_addr"
}

# pad_to_n_bytes
#
# Pads an input string to n bytes, removing the '0x' prefix if it exists.
#
# Arguments:
#   $1 - The input string (e.g., an Ethereum address)
#   $2 - The number of bytes to pad to
#
# Returns:
#   The input string left-padded with zeros to the specified number of bytes
pad_to_n_bytes() {
    local input_string="$1"
    local num_bytes="$2"

    # Remove '0x' prefix if it exists
    if [[ "$input_string" == 0x* ]]; then
        input_string="${input_string:2}"
    fi

    # Calculate the total length in hex characters (2 hex characters per byte)
    local total_length=$((num_bytes * 2))

    # Left-pad the input string with zeros to the specified length
    local padded_string=$(printf "%0${total_length}s" "$input_string" | tr ' ' '0')

    # Add '0x' prefix to the padded string
    echo "0x$padded_string"
}
