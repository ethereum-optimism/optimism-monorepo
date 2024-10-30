// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { Test } from "forge-std/Test.sol";
import { IOptimismSuperchainERC20 } from "src/L2/interfaces/IOptimismSuperchainERC20.sol";
import { Initializable } from "@openzeppelin/contracts-v5/proxy/utils/Initializable.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
/// @title InitializerOZv5_Test
/// @dev Ensures that the `initialize()` function on contracts cannot be called more than
///      once. Tests the contracts inheriting from `Initializable` from OpenZeppelin Contracts v5.

contract InitializerOZv5_Test is Test {
    /// @notice The storage slot of the `initialized` flag in the `Initializable` contract from OZ v5.
    /// keccak256(abi.encode(uint256(keccak256("openzeppelin.storage.Initializable")) - 1)) & ~bytes32(uint256(0xff))
    bytes32 private constant INITIALIZABLE_STORAGE = 0xf0c57e16840df040f15088dc2f81fe391c3923bec73e23a9662efc9c229c6a00;

    /// @notice Contains the address of an `Initializable` contract and the calldata
    ///         used to initialize it.
    struct InitializeableContract {
        address target;
        bytes initCalldata;
    }

    /// @notice Contains the addresses of the contracts to test as well as the calldata
    ///         used to initialize them.
    InitializeableContract[] contracts;

    function setUp() public {
        // Initialize the `contracts` array with the addresses of the contracts to test and the
        // calldata used to initialize them

        // OptimismSuperchainERC20
        contracts.push(
            InitializeableContract({
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

    /// @notice Tests that:
    ///         1. The `initialized` flag of each contract is properly set to `type(uint64).max`,
    ///            signifying that the contracts are initialized.
    ///         2. The `initialize()` function of each contract cannot be called more than once.
    ///         3. Returns the correct error when attempting to re-initialize a contract.
    function test_cannotReinitialize_succeeds() public {
        // Attempt to re-initialize all contracts within the `contracts` array.
        for (uint256 i; i < contracts.length; i++) {
            InitializeableContract memory _contract = contracts[i];
            uint256 size;
            address target = _contract.target;
            assembly {
                size := extcodesize(target)
            }

            // Assert that the contract is already initialized.
            bytes32 slotVal = vm.load(_contract.target, INITIALIZABLE_STORAGE);
            uint64 initialized = uint64(uint256(slotVal));
            assertEq(initialized, type(uint64).max);

            // Then, attempt to re-initialize the contract. This should fail.
            (bool success, bytes memory returnData) = _contract.target.call(_contract.initCalldata);
            assertFalse(success);
            assertEq(bytes4(returnData), Initializable.InvalidInitialization.selector);
        }
    }
}
