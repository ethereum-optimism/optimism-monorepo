#!/bin/bash

set -e

SCRIPTS_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
MONOREPO_DIR=$(cd "$SCRIPTS_DIR/../../" && pwd)

# Grab the foundry commit hash.
SHA=$(jq -r .foundry < "$MONOREPO_DIR"/versions.json)

# Check if there is a nightly tag corresponding to the commit hash
TAG="nightly-$SHA"

# If the foundry repository exists and a branch is checked out, we need to abort
# any changes inside ~/.foundry/foundry-rs/foundry. This is because foundryup will
# attempt to pull the latest changes from the remote repository, which will fail
# if there are any uncommitted changes.
if [ -d ~/.foundry/foundry-rs/foundry ]; then
  echo "Foundry repository exists! Aborting any changes..."
  cd ~/.foundry/foundry-rs/foundry
  git reset --hard
  git clean -fd
  cd -
fi

# Create a temporary directory
TMP_DIR=$(mktemp -d)
echo "Created tempdir @ $TMP_DIR"

# Clone the foundry repo temporarily. We do this to avoid the need for a personal access
# token to interact with the GitHub REST API, and clean it up after we're done.
git clone https://github.com/foundry-rs/foundry.git "$TMP_DIR" && cd "$TMP_DIR"

# If the nightly tag exists, we can download the pre-built binaries rather than building
# from source. Otherwise, clone the repository, check out the commit SHA, and build `forge`,
# `cast`, `anvil`, and `chisel` from source.
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Nightly tag exists! Downloading prebuilt binaries..."
  foundryup -i "$TAG"
else
  echo "Nightly tag doesn't exist! Building from source..."
  git checkout "$SHA"

  # Use native `cargo` build to avoid any rustc environment variables `foundryup` sets. We explicitly
  # ignore chisel, as it is not a part of `ci-builder`.
  cargo build --bin forge --release
  cargo build --bin cast --release
  cargo build --bin anvil --release
  mkdir -p ~/.foundry/bin
  mv target/release/forge ~/.foundry/bin
  mv target/release/cast ~/.foundry/bin
  mv target/release/anvil ~/.foundry/bin
fi

# Remove the temporary foundry repo; Used just for checking the nightly tag's existence.
rm -rf "$TMP_DIR"
echo "Removed tempdir @ $TMP_DIR"
