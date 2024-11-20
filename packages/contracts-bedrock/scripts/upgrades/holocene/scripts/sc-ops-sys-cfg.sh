#!/usr/bin/env bash
set -euo pipefail

# Grab the script directory
SCRIPT_DIR=$(dirname "$0")

# Load common.sh
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"

# Check required environment variables
reqenv "OUTPUT_FOLDER_PATH"
reqenv "SYSTEM_CONFIG_IMPL"
reqenv "SYSTEM_CONFIG_PROXY_ADDR"

# Create directory for the task
TASK_DIR="$OUTPUT_FOLDER_PATH/sys-cfg-sc-ops-task"
mkdir -p "$TASK_DIR"

# Copy the bundle and task template
cp "$OUTPUT_FOLDER_PATH/sys_cfg_bundle.json" "$TASK_DIR/input.json"
cp -R "$SCRIPT_DIR/../templates/sys-cfg-sc-ops-task/." "$TASK_DIR/"

# Generate the task overview
msup render -i "$TASK_DIR/input.json" -o "$TASK_DIR/OVERVIEW.md"

# Generate the README
sed -i '' "s/\$SYSTEM_CONFIG_IMPL/$SYSTEM_CONFIG_IMPL/g" "$TASK_DIR/README.md"

# Generate the validation doc
OLD_SYS_CFG=$(cast impl "$SYSTEM_CONFIG_PROXY_ADDR")

PADDED_OLD_SYS_CFG=$(cast 2u "$OLD_SYS_CFG")
PADDED_SYS_CFG=$(cast 2u "$SYSTEM_CONFIG_IMPL")

sed -i '' "s/\$SYSTEM_CONFIG_PROXY_ADDR/$SYSTEM_CONFIG_PROXY_ADDR/g" "$TASK_DIR/VALIDATION.md"
sed -i '' "s/\$OLD_SYS_CFG/$PADDED_OLD_SYS_CFG/g" "$TASK_DIR/VALIDATION.md"
sed -i '' "s/\$SYSTEM_CONFIG_IMPL/$PADDED_SYS_CFG/g" "$TASK_DIR/VALIDATION.md"
