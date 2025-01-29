#!/usr/bin/env bash
set -euo pipefail

# Function to extract version from contract source as a constant
extract_constant_version() {
    local file=$1
    grep -o 'string.*constant.*version.*=.*"[^"]*"' "$file" | sed 's/.*"\([^"]*\)".*/\1/' || echo ""
}

# Function to extract version from contract source as a function
extract_function_version() {
    local file=$1
    sed -n '/function.*version()/,/return/p' "$file" | grep -o '"[^"]*"' | sed 's/"//g' || echo ""
}

# Function to extract version from either constant or function
extract_version() {
    local file=$1
    version=$(extract_constant_version "$file")
    if [ -z "$version" ]; then
        version=$(extract_function_version "$file")
    fi
    echo "$version"
}
