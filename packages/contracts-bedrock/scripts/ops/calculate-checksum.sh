#!/usr/bin/env bash

set -euo pipefail

echoerr() {
  echo "$@" 1>&2
}

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
CONTRACTS_DIR="$SCRIPT_DIR/../.."

cd "$CONTRACTS_DIR"

echoerr "> Calculating contracts checksum..."

find . -type f -name '*.sol' -exec sha256sum {} + > manifest.txt
sha256sum snapshots/semver-lock.json >> manifest.txt
sha256sum foundry.toml >> manifest.txt
# need to specify the locale to ensure consistent sorting across platforms
LC_ALL=C sort -o manifest.txt manifest.txt
checksum=$(sha256sum manifest.txt | awk '{print $1}')
rm manifest.txt
echoerr "> Checksum: $checksum"
echoerr "> Done."

echo -n "$checksum"