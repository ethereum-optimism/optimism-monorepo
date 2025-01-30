#!/usr/bin/env bash

# TODO: actually test something. Right now it just gives an idea of what's
# possible.

# shellcheck disable=SC1091
source "$(dirname "$0")/boilerplate.sh"

echo "DEVNET: $DEVNET"
echo "ENVIRONMENT:"
cat "$ENVIRONMENT"

l1_name=$(cat "$ENVIRONMENT" | jq -r '.l1.name')
echo "L1 NAME: $l1_name"

cast --version