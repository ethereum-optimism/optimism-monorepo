#!/usr/bin/env bash

set -euo pipefail

# Default values
ENVIRONMENT=""
PERSPECTIVE="external"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --environment)
      ENVIRONMENT=$(realpath "$2")
      shift 2
      ;;
    --perspective)
      PERSPECTIVE="$2"
      shift 2
      ;;
    *)
      echo "Invalid option: $1" >&2
      exit 1
      ;;
  esac
done

# Validate required arguments
if [ -z "$ENVIRONMENT" ]; then
  echo "Error: --environment argument is required" >&2
  exit 1
fi

if [ "$PERSPECTIVE" != "external" ] && [ "$PERSPECTIVE" != "internal" ]; then
  echo "Error: --perspective argument must be either 'external' or 'internal'" >&2
  exit 1
fi

# Some helper functions

# Stepping through a test
blue=$(tput setaf 4 2>/dev/null || echo '')
normal=$(tput sgr0 2>/dev/null || echo '')

function step() {
    echo
    echo "${blue}TEST STEP: $1${normal}"
    echo
}

function expect_error_message() {
  MSG="$1"
  OUTPUT="$2"
  if echo "$OUTPUT" | grep -q "$MSG"; then
    echo "Expected error message found: $MSG"
    true
  else
    echo "Expected error message not found: $MSG"
    exit 1
  fi
}

# Getting data from the environment file
function getEnvData() {
    jq -r "$1" "$ENVIRONMENT"
}

# TODO: this is a chack. We need to adjust the environment to provide this info.
# Then we can remove this function.
function resolvePort() {
  TYPE="$1"
  case "$TYPE" in
    rpc)
      echo "8545"
      ;;
    *)
      echo "Invalid type: $TYPE"
      exit 1
      ;;
  esac
}
function getSvcHostPort() {
  SVC="$1"
  TYPE="$2"
  if [ "$PERSPECTIVE" == "external" ]; then
    HOST=$(getEnvData "$SVC.endpoints.$TYPE.host")
    PORT=$(getEnvData "$SVC.endpoints.$TYPE.port")
  else
    HOST=$(getEnvData "$SVC.name")
    PORT=$(resolvePort "$TYPE")
  fi
  echo "$HOST:$PORT"
}

function extract_cast_logs() {
    input="$1"
    # make a json file out of the logs field in the input file
    grep '^logs\b' "$input" | sed -e 's/^logs/\{"logs":/' -e 's/$/\}/' | jq .
}

function get_log_entry() {
    logfile="$1"
    address="$2"
    jq -r --arg addr "$address" \
        '.logs[] | select(.address == $addr)' \
        "$logfile"
}

# Temporary directory for storing logs and other files
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
