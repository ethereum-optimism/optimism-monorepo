#!/usr/bin/env bash

set -euo pipefail

DOCKER_REPO=$1
GIT_TAG=$2
GIT_SHA=$3

IMAGE_NAME=$(echo "$GIT_TAG" | grep -Eow '^(ci-builder(-rust)?|da-server|proofs-tools|holocene-deployer|cannon|ufm-[a-z0-9\-]*|op-[a-z0-9\-]*)' || true)
if [ -z "$IMAGE_NAME" ]; then
  echo "image name could not be parsed from git tag '$GIT_TAG'"
  exit 1
fi
IMAGE_TAG=$(echo "$GIT_TAG" | grep -Eow 'v.*' || true)
if [ -z "$IMAGE_TAG" ]; then
  echo "image tag could not be parsed from git tag '$GIT_TAG'"
  exit 1
fi

SOURCE_IMAGE_TAG="$DOCKER_REPO/$IMAGE_NAME:$GIT_SHA"
TARGET_IMAGE_TAG="$DOCKER_REPO/$IMAGE_NAME:$IMAGE_TAG"
TARGET_IMAGE_TAG_LATEST="$DOCKER_REPO/$IMAGE_NAME:latest"

echo "Checking if docker images exist for '$IMAGE_NAME'"
echo ""
tags=$(gcloud container images list-tags "$DOCKER_REPO/$IMAGE_NAME" --limit 1 --format json)
if [ "$tags" = "[]" ]; then
  echo "No existing docker images were found for '$IMAGE_NAME'. The code tagged with '$GIT_TAG' may not have an associated dockerfile or docker build job."
  echo "If this service has a dockerfile, add a docker-publish job for it in the circleci config."
  echo ""
  echo "Exiting"
  exit 0
fi

echo "Tagging $SOURCE_IMAGE_TAG with '$IMAGE_TAG'"
gcloud container images add-tag -q "$SOURCE_IMAGE_TAG" "$TARGET_IMAGE_TAG"

# Do not tag with latest if the release is a release candidate.
if [[ "$IMAGE_TAG" == *"rc"* ]]; then
  echo "Not tagging with 'latest' because the release is a release candidate."
  exit 0
fi

echo "Tagging $SOURCE_IMAGE_TAG with 'latest'"
gcloud container images add-tag -q "$SOURCE_IMAGE_TAG" "$TARGET_IMAGE_TAG_LATEST"
