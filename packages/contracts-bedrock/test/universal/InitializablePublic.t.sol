// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { Test } from "forge-std/Test.sol";
import { IOptimismSuperchainERC20 } from "interfaces/L2/IOptimismSuperchainERC20.sol";
import { InitializablePublic } from "src/universal/InitializablePublic.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Constants } from "src/libraries/Constants.sol";

/// @title InitializablePublic_Test
/// @dev Ensures that the the getter to check the versionreturns the correct value.
///      Tests the contracts inheriting from `InitializablePublic`
contract InitializablePublic_Test is Test {
    /// @notice Contains the address of an `InitializablePublic` contract and the calldata
    ///         used to initialize it.
    struct InitializablePublicContract {
        address target;
        bytes initCalldata;
    }

    /// @notice Contains the addresses of the contracts to test as well as the calldata
    ///         used to initialize them.
    InitializablePublicContract[] private contracts;

    function setUp() public {
        // Initialize the `contracts` array with the addresses of the contracts to test and the
        // calldata used to initialize them

        // OptimismSuperchainERC20
        contracts.push(
            InitializablePublicContract({
                target: address(
                    DeployUtils.create1({
                        _name: "OptimismSuperchainERC20",
                        _args: DeployUtils.encodeConstructor(abi.encodeCall(IOptimismSuperchainERC20.__constructor__, ()))
                    })
                ),
                initCalldata: abi.encodeCall(IOptimismSuperchainERC20.initialize, (address(0), "", "", 18))
            })
        );
    }

    function test_correctVersion_succeeds() public {
        for (uint256 i; i < contracts.length; ++i) {
            bytes32 slotVal = vm.load(contracts[i].target, Constants.INITIALIZABLE_STORAGE);
            uint64 initialized = uint64(uint256(slotVal));
            // Uint64 max means initialized and _disableInitializers called
            assertEq(initialized, type(uint64).max);
        }
    }
}
