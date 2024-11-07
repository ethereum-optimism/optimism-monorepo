#!/usr/bin/env bash
set -euo pipefail

# Grab the script directory
SCRIPT_DIR=$(dirname "$0")

# Load common.sh
source "$SCRIPT_DIR/common.sh"

# Check required environment variables
reqenv "OUTPUT_FOLDER_PATH"
reqenv "SYSTEM_CONFIG_IMPL"
reqenv "MIPS_IMPL"
reqenv "FDG_IMPL"
reqenv "PDG_IMPL"
reqenv "DISPUTE_GAME_FACTORY_PROXY_ADDR"
reqenv "SYSTEM_CONFIG_PROXY_ADDR"

# Create directory for the task
TASK_DIR="$OUTPUT_FOLDER_PATH/sc-ops-task"
mkdir -p "$TASK_DIR"

# Copy the bundle and task template
cp $OUTPUT_FOLDER_PATH/bundle.json $TASK_DIR/input.json
cp $SCRIPT_DIR/../templates/sc-ops-task/** $TASK_DIR/

# Generate the task overview
msup render -i $TASK_DIR/input.json -o $TASK_DIR/OVERVIEW.md

# Generate the README
sed -i '' "s/\$SYSTEM_CONFIG_IMPL/$SYSTEM_CONFIG_IMPL/g" $TASK_DIR/README.md
sed -i '' "s/\$MIPS_IMPL/$MIPS_IMPL/g" $TASK_DIR/README.md
sed -i '' "s/\$FDG_IMPL/$FDG_IMPL/g" $TASK_DIR/README.md
sed -i '' "s/\$PDG_IMPL/$PDG_IMPL/g" $TASK_DIR/README.md

# Generate the validation doc
OLD_SYS_CFG=$(cast impl $SYSTEM_CONFIG_PROXY_ADDR)
OLD_FDG=$(cast call $DISPUTE_GAME_FACTORY_PROXY_ADDR "gameImpls(uint32)" 0)
OLD_PDG=$(cast call $DISPUTE_GAME_FACTORY_PROXY_ADDR "gameImpls(uint32)" 1)

PADDED_OLD_SYS_CFG=$(cast 2u $OLD_SYS_CFG)
PADDED_OLD_FDG=$(cast 2u $OLD_FDG)
PADDED_OLD_PDG=$(cast 2u $OLD_PDG)
PADDED_SYS_CFG=$(cast 2u $SYSTEM_CONFIG_IMPL)
PADDED_FDG_IMPL=$(cast 2u $FDG_IMPL)
PADDED_PDG_IMPL=$(cast 2u $PDG_IMPL)

sed -i '' "s/\$SYSTEM_CONFIG_PROXY_ADDR/$SYSTEM_CONFIG_PROXY_ADDR/g" $TASK_DIR/VALIDATION.md
sed -i '' "s/\$OLD_SYS_CFG/$PADDED_OLD_SYS_CFG/g" $TASK_DIR/VALIDATION.md
sed -i '' "s/\$SYSTEM_CONFIG_IMPL/$PADDED_SYS_CFG/g" $TASK_DIR/VALIDATION.md
sed -i '' "s/\$DISPUTE_GAME_FACTORY_PROXY_ADDR/$DISPUTE_GAME_FACTORY_PROXY_ADDR/g" $TASK_DIR/VALIDATION.md
sed -i '' "s/\$OLD_FDG/$PADDED_OLD_FDG/g" $TASK_DIR/VALIDATION.md
sed -i '' "s/\$FDG_IMPL/$PADDED_FDG_IMPL/g" $TASK_DIR/VALIDATION.md
sed -i '' "s/\$PDG_IMPL/$PADDED_PDG_IMPL/g" $TASK_DIR/VALIDATION.md
sed -i '' "s/\$OLD_PDG/$PADDED_OLD_PDG/g" $TASK_DIR/VALIDATION.md
