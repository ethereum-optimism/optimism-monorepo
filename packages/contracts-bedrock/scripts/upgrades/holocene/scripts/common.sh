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
    if [ -z "$1" ]; then
        echo "Error: $1 is not set"
        exit 1
    fi
}

# prompt
#
# Prompts the user for a yes/no response.
#
# Arguments:
#   $1 - The prompt message
#
# Exits with status 1 if:
#   - The user does not respond with 'y'
#   - The process is interrupted
prompt() {
    read -p "$1 [Y/n] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        [[ "$0" = "${BASH_SOURCE[0]}" ]] && exit 1 || return 1
        exit 1
    fi
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
    local toml_url="https://raw.githubusercontent.com/ethereum-optimism/superchain-registry/refs/heads/main/validation/standard/standard-versions"
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
        # shellcheck disable=SC2181
        if [ $? -ne 0 ]; then
            echo "Error: Failed to fetch TOML file from $toml_url"
            exit 1
        fi
    fi

    # Use the cached content for the current URL
    local toml_content="${CACHED_TOML_CONTENT[$toml_url]}"

    # Find the section for v1.6.0 release
    # shellcheck disable=SC2155
    local section_content=$(echo "$toml_content" | awk -v version="$release_version" '
        $0 ~ "^\\[releases.\"op-contracts/v" version "\"\\]" {
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
    local regex="(address|implementation_address) = \"(0x[a-fA-F0-9]{40})\""
    # shellcheck disable=SC2155
    local data=$(echo "$section_content" | grep "${contract_name}")
    if [[ $data =~ $regex ]]; then
        echo "${BASH_REMATCH[2]}"
    else
        echo "Error: Implementation address for $contract_name not found in v$release_version release"
        exit 1
    fi
}
