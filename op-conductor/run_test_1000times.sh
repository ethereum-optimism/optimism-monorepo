#!/bin/bash

set -e

for i in {1..1000}; do
  echo "======================="
  echo "Running iteration $i"

  if ! go test -v ./conductor/... -race -count=1; then
    echo "Test failed"
    exit 1
  fi
done
