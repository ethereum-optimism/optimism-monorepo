#!/usr/bin/env bash

set -euo pipefail

# Default values
DEVNET=""
ENVIRONMENT=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --devnet)
      DEVNET="$2"
      shift 2
      ;;
    --environment)
      ENVIRONMENT="$2"
      shift 2
      ;;
    *)
      echo "Invalid option: $1" >&2
      exit 1
      ;;
  esac
done

# Validate required arguments
if [ -z "$DEVNET" ]; then
  echo "Error: --devnet argument is required" >&2
  exit 1
fi

if [ -z "$ENVIRONMENT" ]; then
  echo "Error: --environment argument is required" >&2
  exit 1
fi
