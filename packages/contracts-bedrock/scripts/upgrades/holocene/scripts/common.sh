#!/usr/bin/env bash
set -euo pipefail

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
        [[ "$0" = "$BASH_SOURCE" ]] && exit 1 || return 1
        exit 1
    fi
}
