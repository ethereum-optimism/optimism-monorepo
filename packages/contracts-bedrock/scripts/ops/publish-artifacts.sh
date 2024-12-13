#!/usr/bin/env bash

set -euo pipefail

echoerr() {
  echo "$@" 1>&2
}

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
CONTRACTS_DIR="$SCRIPT_DIR/../.."
DEPLOY_BUCKET="oplabs-contract-artifacts"

cd "$CONTRACTS_DIR"

# ensure that artifacts exists and is non-empty
if [ ! -d "forge-artifacts" ] || [ -z "$(ls -A forge-artifacts)" ]; then
  echoerr "> No forge-artifacts directory found."
  exit 1
fi

if [ ! -d "artifacts" ] || [ -z "$(ls -A artifacts)" ]; then
  echoerr "> No artifacts directory found."
  exit 1
fi

checksum=$(bash scripts/ops/calculate-checksum.sh)

echoerr "> Checksum: $checksum"
echoerr "> Checking for existing artifacts..."
exists=$(curl -s -o /dev/null --fail -LI "https://storage.googleapis.com/$DEPLOY_BUCKET/artifacts-v1-$checksum.tar.gz" || echo "fail")

if [ "$exists" != "fail" ]; then
  echoerr "> Existing artifacts found, nothing to do."
  exit 0
fi

echoerr "> Archiving artifacts..."
archive_name="artifacts-v1-$checksum.tar.gz"

# use gtar on darwin
if [[ "$OSTYPE" == "darwin"* ]]; then
  tar="gtar"
else
  tar="tar"
fi

rm -f COMMIT
commit=$(git rev-parse HEAD)
echo "$commit" > COMMIT

"$tar" -czf "$archive_name" artifacts forge-artifacts cache COMMIT
du -sh "$archive_name" | awk '{$1=$1};1' # trim leading whitespace
echoerr "> Done."

echoerr "> Uploading artifacts to GCS..."
gcloud storage cp "$archive_name" "gs://$DEPLOY_BUCKET/$archive_name"
echoerr "> Done."

rm "$archive_name"
rm COMMIT