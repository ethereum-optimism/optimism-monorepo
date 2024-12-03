# Validation

This document can be used to validate the state diff resulting from the execution of the upgrade
transaction.

For each contract listed in the state diff, please verify that no contracts or state changes shown in the Tenderly diff are missing from this document. Additionally, please verify that for each contract:

- The following state changes (and none others) are made to that contract. This validates that no unexpected state changes occur.
- All addresses (in section headers and storage values) match the provided name, using the Etherscan and Superchain Registry links provided. This validates the bytecode deployed at the addresses contains the correct logic.
- All key values match the semantic meaning provided, which can be validated using the storage layout links provided.

## State Changes

### `0x05F9613aDB30026FFd634f38e5C4dFd30a197Fa1` (`DisputeGameFactoryProxy`)

- **Key**: `0xf08cf23b9096e47c93681aae499ab9bfe983e27d836cc8ef3d90a528deceea0c` <br/>
  **Before**: `0x0000000000000000000000000000000000000000000000000000000000000000` <br/>
  **After**: `0x00000000000000000000000091ee75c3c0f238652d88b62e29bebddd5a92ed7f` <br/>
  **Meaning**: Updates the ASTERISC_KONA game type implementation. Verify that the new implementation is set using `cast call 0x05F9613aDB30026FFd634f38e5C4dFd30a197Fa1 "gameImpls(uint32)(address)" 3`. Where `3` is the `ASTERISC_KONA` game type.

### `0x218CD9489199F321E1177b56385d333c5B598629` (`AnchorStateRegistryProxy`)

- **Key**: `0x7dfe757ecd65cbd7922a9c0161e935dd7fdbcc0e999689c7d31633896b1fc60b` <br/>
  **Before**: `0x0000000000000000000000000000000000000000000000000000000000000000` <br/>
  **After**: `0xbdf43aeb76d543940d6d9a86af7c0735eb425d6758a41c4a47c2aa476c994ee5` <br/>
  **Meaning**: Sets the initial anchor for the new ASTERISC_KONA game type.
