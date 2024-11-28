#!/bin/bash

set -e

# Check if kup exists, if not, install it
if ! command -v kup &> /dev/null; then
  echo "kup not found, installing..."
  yes | bash <(curl -L https://kframework.org/install)
fi

SCRIPTS_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
MONOREPO_DIR=$(cd "$SCRIPTS_DIR/../../" && pwd)

# Grab the correct kontrol version.
VERSION=$(jq -r .kontrol < "$MONOREPO_DIR"/versions.json)

kup install kontrol --version v"$VERSION"
