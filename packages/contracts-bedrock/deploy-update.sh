#!/bin/bash
export DEPLOYER=0xA6aFc9612b504202E0A5F6cf3C8E89C49EA06037
export SALT=miniopcm-888
forge script DeployOPCMRunner --rpc-url $SEP_RPC_URL \
--keystore ~/.foundry/keystores/sep-tester \
--sender $DEPLOYER \
--resume --slow \
-vvvv --broadcast --verify \
--etherscan-api-key WH64STM7TTRGEDCR1E7NWB8Q9RIUEPKB1Q
