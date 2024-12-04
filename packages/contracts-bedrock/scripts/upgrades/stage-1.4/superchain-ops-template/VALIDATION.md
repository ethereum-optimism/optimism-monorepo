# Validation

This document can be used to validate the state diff resulting from the execution of the upgrade
transaction.

For each contract listed in the state diff, please verify that no contracts or state changes shown in the Tenderly diff are missing from this document. Additionally, please verify that for each contract:

- The following state changes (and none others) are made to that contract. This validates that no unexpected state changes occur.
- All addresses (in section headers and storage values) match the provided name, using the Etherscan and Superchain Registry links provided. This validates the bytecode deployed at the addresses contains the correct logic.
- All key values match the semantic meaning provided, which can be validated using the storage layout links provided.

## State Changes

### `%%DISPUTE_GAME_FACTORY_PROXY_ADDR%%` (`DisputeGameFactoryProxy`)

- **Key**: `0xf08cf23b9096e47c93681aae499ab9bfe983e27d836cc8ef3d90a528deceea0c` <br/>
  **Before**: `0x0000000000000000000000000000000000000000000000000000000000000000` <br/>
  **After**: `%%FDG_IMPL_B32%%` <br/>
  **Meaning**: Updates the ASTERISC_KONA game type implementation. Verify that the new implementation is set using `cast call %%DISPUTE_GAME_FACTORY_PROXY_ADDR%% "gameImpls(uint32)(address)" 3`. Where `3` is the `ASTERISC_KONA` game type.

### `%%ANCHOR_STATE_REGISTRY_PROXY_ADDR%%` (`AnchorStateRegistryProxy`)

- **Key**: `0x7dfe757ecd65cbd7922a9c0161e935dd7fdbcc0e999689c7d31633896b1fc60b` <br/>
  **Before**: `0x0000000000000000000000000000000000000000000000000000000000000000` <br/>
  **After**: `%%STARTING_ANCHOR%%` <br/>
  **Meaning**: Sets the initial anchor for the new ASTERISC_KONA game type.
