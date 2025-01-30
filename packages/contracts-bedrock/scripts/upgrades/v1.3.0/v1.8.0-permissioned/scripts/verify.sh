#!/usr/bin/env bash
set -euo pipefail

# Grab the script directory
SCRIPT_DIR=$(dirname "$0")

# Load common.sh
source "$SCRIPT_DIR/common.sh"

# Check required environment variables
reqenv "NETWORK"
reqenv "ETH_RPC_URL"
reqenv "DISPUTE_GAME_FACTORY_PROXY"
reqenv "DEPLOYMENTS_JSON_PATH"
reqenv "SYSTEM_OWNER_SAFE"

# Set the release version
RELEASE_VERSION="1.8.0-rc.4"

# Load addresses from deployments json
L1_STANDARD_BRIDGE_PROXY=$(load_local_address $DEPLOYMENTS_JSON_PATH "L1StandardBridgeProxy" "Proxy__OVM_L1StandardBridge")
L1_CROSS_DOMAIN_MESSENGER_PROXY=$(load_local_address $DEPLOYMENTS_JSON_PATH "L1CrossDomainMessengerProxy" "Proxy__OVM_L1CrossDomainMessenger")
L1_ERC721_BRIDGE_PROXY=$(load_local_address $DEPLOYMENTS_JSON_PATH "L1ERC721BridgeProxy")
OPTIMISM_PORTAL_PROXY=$(load_local_address $DEPLOYMENTS_JSON_PATH "OptimismPortalProxy")
SYSTEM_CONFIG_PROXY=$(load_local_address $DEPLOYMENTS_JSON_PATH "SystemConfigProxy")
OPTIMISM_MINTABLE_ERC20_FACTORY_PROXY=$(load_local_address $DEPLOYMENTS_JSON_PATH "OptimismMintableERC20FactoryProxy")
ADDRESS_MANAGER=$(load_local_address $DEPLOYMENTS_JSON_PATH "AddressManager")

# Fetch addresses from standard address toml
L1_STANDARD_BRIDGE_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "l1_standard_bridge")
L1_CROSS_DOMAIN_MESSENGER_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "l1_cross_domain_messenger")
L1_ERC721_BRIDGE_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "l1_erc721_bridge")
OPTIMISM_PORTAL_2_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "optimism_portal")
SYSTEM_CONFIG_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "system_config")
OPTIMISM_MINTABLE_ERC20_FACTORY_IMPL=$(fetch_standard_address $NETWORK $RELEASE_VERSION "optimism_mintable_erc20_factory")

# Fetch SuperchainConfigProxy address
SUPERCHAIN_CONFIG_PROXY=$(fetch_superchain_config_address $NETWORK)

# Generate verification text
cat << EOF
# Validation

This document can be used to validate the state diff resulting from the execution of the upgrade
transaction. For each contract listed in the state diff, please verify that no contracts or state
changes shown in the Tenderly diff are missing from this document.

Additionally, please verify that for each contract:

- The following state changes (and none others) are made to that contract. This validates that no
  unexpected state changes occur.
- All addresses (in section headers and storage values) match the provided name, using the
  Etherscan and Superchain Registry links provided. This validates the bytecode deployed at the
  addresses contains the correct logic.
- All key values match the semantic meaning provided, which can be validated using the storage
  layout links provided.

## State Overrides

### SystemOwnerSafe ($SYSTEM_OWNER_SAFE)

- Key: 0x0000000000000000000000000000000000000000000000000000000000000004
  - Value: 0x0000000000000000000000000000000000000000000000000000000000000001
  - Description: Enables the simulation by setting signing threshold to 1

## State Changes

### L1StandardBridgeProxy ($L1_STANDARD_BRIDGE_PROXY)

- Key: 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc
  - Value: $(pad_to_n_bytes $L1_STANDARD_BRIDGE_IMPL 32)
  - Description: Implementation address changed to $L1_STANDARD_BRIDGE_IMPL
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address
- Key: 0x0000000000000000000000000000000000000000000000000000000000000032
  - Value: $(pad_to_n_bytes $SUPERCHAIN_CONFIG_PROXY 32)
  - Description: SuperchainConfig address variable set to shared contract address
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address

### AddressManager ($ADDRESS_MANAGER)

- Key: 0x515216935740e67dfdda5cf8e248ea32b3277787818ab59153061ac875c9385e
  - Value: $(pad_to_n_bytes $L1_CROSS_DOMAIN_MESSENGER_IMPL 32)
  - Description: L1CrossDomainMessenger address changed to $L1_CROSS_DOMAIN_MESSENGER_IMPL
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address

### L1CrossDomainMessengerProxy ($L1_CROSS_DOMAIN_MESSENGER_PROXY)

- Key: 0x00000000000000000000000000000000000000000000000000000000000000FB
  - Value: $(pad_to_n_bytes $SUPERCHAIN_CONFIG_PROXY 32)
  - Description: SuperchainConfig address variable set to shared contract address
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address

### L1ERC721BridgeProxy ($L1_ERC721_BRIDGE_PROXY)

- Key: 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc
  - Value: $(pad_to_n_bytes $L1_ERC721_BRIDGE_IMPL 32)
  - Description: Implementation address changed to $L1_ERC721_BRIDGE_IMPL
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address
- Key: 0x0000000000000000000000000000000000000000000000000000000000000032
  - Value: $(pad_to_n_bytes $SUPERCHAIN_CONFIG_PROXY 32)
  - Description: SuperchainConfig address variable set to shared contract address
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address

### SystemConfigProxy ($SYSTEM_CONFIG_PROXY)

- Key: 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc
  - Value: $(pad_to_n_bytes $SYSTEM_CONFIG_IMPL 32)
  - Description: Implementation address changed to $SYSTEM_CONFIG_IMPL
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address
- Key: 0x52322a25d9f59ea17656545543306b7aef62bc0cc53a0e65ccfa0c75b97aa906
  - Value: $(pad_to_n_bytes $DISPUTE_GAME_FACTORY_PROXY 32)
  - Description: Slot at keccak(systemconfig.disputegamefactory)-1 set to address of DisputeGameFactoryProxy deployed via upgrade script
- Key: 0xe52a667f71ec761b9b381c7b76ca9b852adf7e8905da0e0ad49986a0a6871815
  - Value: 0x0000000000000000000000000000000000000000000000000000000000000000
  - Description: Slot at keccak(systemconfig.l2outputoracle)-1 deleted

### OptimismPortalProxy ($OPTIMISM_PORTAL_PROXY)

- Key: 0x0000000000000000000000000000000000000000000000000000000000000035
  - Value: $(pad_to_n_bytes $SUPERCHAIN_CONFIG_PROXY 31)00
  - Description: SuperchainConfig address variable set to shared contract address
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address
- Key: 0x0000000000000000000000000000000000000000000000000000000000000038
  - Value: $(pad_to_n_bytes $DISPUTE_GAME_FACTORY_PROXY 32)
  - Description: DisputeGameFactory address variable set to the address deployed in upgrade script
- Key: 0x000000000000000000000000000000000000000000000000000000000000003b
  - Value: 0x00000000000000000000000000000000000000000000000TIMESTAMP00000001
  - Description: Sets the respectedGameType to 1 (permissioned game) and sets the respectedGameTypeUpdatedAt timestamp to the time when the upgrade transaction was executed (will be a dynamic value)
- Key: 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc
  - Value: $(pad_to_n_bytes $OPTIMISM_PORTAL_2_IMPL 32)
  - Description: Implementation address changed to $OPTIMISM_PORTAL_2_IMPL

### SystemOwnerSafe ($SYSTEM_OWNER_SAFE)

- Key: 0x0000000000000000000000000000000000000000000000000000000000000005
  - Value: increment
  - Description: Nonce bumped by 1 in the SystemOwnerSafe

### OptimismMintableERC20FactoryProxy ($OPTIMISM_MINTABLE_ERC20_FACTORY_PROXY)

- Key: 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc
  - Value: $(pad_to_n_bytes $OPTIMISM_MINTABLE_ERC20_FACTORY_IMPL 32)
  - Description: Implementation address changed to $OPTIMISM_MINTABLE_ERC20_FACTORY_IMPL
  - WARNING: ⚠️ You MAY not see this change if you are already using the correct address
EOF
