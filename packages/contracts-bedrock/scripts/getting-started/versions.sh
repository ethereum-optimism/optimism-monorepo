#!/usr/bin/env bash

# This script prints out the versions of the various tools used in the Getting
# Started quickstart guide on the docs site. Simplifies things for users so
# they can easily see if they're using the right versions of everything.

version() {
  local string=$1
  local version_regex='([0-9]+(\.[0-9]+)+)'
  if [[ $string =~ $version_regex ]]; then
    echo "${BASH_REMATCH[1]}"
  else
    echo "No version found."
  fi
}

versionFoundry() {
  local string="$1"
  local version_regex='forge ([0-9]+\.[0-9]+\.[0-9]+)'
  local commit_hash_regex='\(([a-fA-F0-9]+)'
  local full_regex="${version_regex} ${commit_hash_regex}"

  if [[ $string =~ $full_regex ]]; then
    echo "${BASH_REMATCH[1]} (${BASH_REMATCH[2]})"
  else
    echo "No version, commit hash, and timestamp found."
  fi
}


# Grab versions
ver_git=$(version "$(git --version)")
ver_go=$(version "$(go version)")
ver_node=$(version "$(node --version)")
ver_foundry=$(versionFoundry "$(forge --version)")
ver_make=$(version "$(make --version)")
ver_jq=$(version "$(jq --version)")
ver_direnv=$(version "$(direnv --version)")
ver_just=$(version "$(just --version)")

# Print versions
echo "Dependency                               | Minimum         | Actual"
echo "git (https://git-scm.com)                  2                $ver_git"
echo "go (https://go.dev)                        1.21             $ver_go"
echo "node (https://nodejs.org)                  20               $ver_node"
echo "foundry (https://getfoundry.sh)            0.2.0 (a5efe4f)  $ver_foundry"
echo "make (https://www.gnu.org/software/make)   3                $ver_make"
echo "jq (https://jqlang.github.io/jq)           1.6              $ver_jq"
echo "direnv (https://direnv.net/)               2                $ver_direnv"
echo "just (https://github.com/casey/just)       1.34.0           $ver_just"
