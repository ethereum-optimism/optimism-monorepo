#!/usr/bin/env bash
set -euo pipefail

# Cache dir to store fetched TOML files
declare CACHE_DIR

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

  if [[ "$network_name" != "mainnet" && "$network_name" != "sepolia" ]]; then
    echo "Error: NETWORK must be set to 'mainnet' or 'sepolia'"
    exit 1
  fi

  # Ensure cache dir exists
  CACHE_DIR="${CACHE_DIR:-$(mktemp -d)}"

  local toml_path="${CACHE_DIR}/standard-versions-$network_name.toml"
  if [[ ! -f "$toml_path" ]]; then
    local toml_url="https://raw.githubusercontent.com/ethereum-optimism/superchain-registry/refs/heads/main/validation/standard/standard-versions-$network_name.toml"
    if ! curl -s "$toml_url" -o "$toml_path"; then
      echo "Error: Failed to fetch TOML file from $toml_url"
      exit 1
    fi
  fi

  local contract_path=".\"op-contracts/${release_version}\".$contract_name"
  local contract_address
  contract_address=$(yq "${contract_path}.address // ${contract_path}.implementation_address // \"\"" "${toml_path}")
  if [[ -z "$contract_address" ]]; then
    echo "Error: Implementation address for $contract_name not found in $release_version release"
    exit 1
  fi
  echo "${contract_address}"
}
