#!/usr/bin/env bash
set -euo pipefail

# Check if both arguments are provided
if [ $# -ne 2 ]; then
    echo "Usage: $0 <file_path> <expected_hash>"
    exit 1
fi

file_path="$1"
expected_hash="$2"

# Check if the file exists
if [ ! -f "$file_path" ]; then
    echo "Error: File '$file_path' does not exist."
    exit 1
fi

# Calculate the actual hash of the file
actual_hash=$(openssl dgst -sha256 -r "$file_path" | awk '{print $1}')

# Compare the hashes
if [ "$actual_hash" = "$expected_hash" ]; then
    exit 0
else
    echo "File '$file_path' has changed when it shouldn't have"
    echo "Expected hash: $expected_hash"
    echo "Actual hash:   $actual_hash"
    exit 1
fi
