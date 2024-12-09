// SPDX-License-Identifier: UNLICENSED
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";
import { VmSafe } from "forge-std/Vm.sol";

// Target contract
import { IL1ChugSplashProxy } from "interfaces/legacy/IL1ChugSplashProxy.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract Owner {
    bool public isUpgrading;

    function setIsUpgrading(bool _isUpgrading) public {
        isUpgrading = _isUpgrading;
    }
}

contract Implementation {
    function setCode(bytes memory) public pure returns (uint256) {
        return 1;
    }

    function setStorage(bytes32, bytes32) public pure returns (uint256) {
        return 2;
    }

    function setOwner(address) public pure returns (uint256) {
        return 3;
    }

    function getOwner() public pure returns (uint256) {
        return 4;
    }

    function getImplementation() public pure returns (uint256) {
        return 5;
    }
}

contract L1ChugSplashProxy_Test is Test {
    IL1ChugSplashProxy proxy;
    address impl;
    address owner = makeAddr("owner");
    address alice = makeAddr("alice");

    function setUp() public {
        proxy = IL1ChugSplashProxy(
            DeployUtils.create1({
                _name: "L1ChugSplashProxy",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IL1ChugSplashProxy.__constructor__, (owner)))
            })
        );
        vm.prank(owner);
        assertEq(proxy.getOwner(), owner);

        vm.prank(owner);
        proxy.setCode(type(Implementation).runtimeCode);

        vm.prank(owner);
        impl = proxy.getImplementation();
    }

    /// @notice Tests that the owner can deploy a new implementation with a given runtime code
    function test_setCode_whenOwner_succeeds() public {
        vm.prank(owner);
        proxy.setCode(hex"604260005260206000f3");

        vm.prank(owner);
        assertNotEq(proxy.getImplementation(), impl);
    }

    /// @notice Tests that when not the owner, `setCode` delegatecalls the implementation
    function test_setCode_whenNotOwner_works() public view {
        uint256 ret = Implementation(address(proxy)).setCode(hex"604260005260206000f3");
        assertEq(ret, 1);
    }

    /// @notice Tests that when the owner deploys the same bytecode as the existing implementation,
    ///         it does not deploy a new implementation
    function test_setCode_whenOwnerSameBytecode_works() public {
        vm.prank(owner);
        proxy.setCode(type(Implementation).runtimeCode);

        // does not deploy new implementation
        vm.prank(owner);
        assertEq(proxy.getImplementation(), impl);
    }

    /// @notice Tests that when the owner calls `setCode` with insufficient gas to complete the implementation
    ///         contract's deployment, it reverts.
    /// @dev    If this solc version/settings change and modifying this proves time consuming, we can just remove it.
    function test_setCode_whenOwnerAndDeployOutOfGas_reverts() public {
        // The values below are best gotten by removing the gas limit parameter from the call and running the test with
        // a
        // verbosity of `-vvvv` then setting the value to a few thousand gas lower than the gas used by the call.
        // A faster way to do this for forge coverage cases, is to comment out the optimizer and optimizer runs in
        // the foundry.toml file and then run forge test. This is faster because forge test only compiles modified
        // contracts unlike forge coverage.
        uint256 gasLimit;

        // Because forge coverage always runs with the optimizer disabled,
        // if forge coverage is run before testing this with forge test or forge snapshot, forge clean should be
        // run first so that it recompiles the contracts using the foundry.toml optimizer settings.
        if (vm.isContext(VmSafe.ForgeContext.Coverage)) {
            gasLimit = 95_000;
        } else if (vm.isContext(VmSafe.ForgeContext.Test) || vm.isContext(VmSafe.ForgeContext.Snapshot)) {
            gasLimit = 65_000;
        } else {
            revert("SafeCall_Test: unknown context");
        }

        vm.prank(owner);
        vm.expectRevert(bytes("L1ChugSplashProxy: code was not correctly deployed")); // Ran out of gas
        proxy.setCode{ gas: gasLimit }(
            hex"fefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefe"
        );
    }

    /// @notice Tests that when the caller is not the owner and the implementation is not set, all calls reverts
    function test_calls_whenNotOwnerNoImplementation_reverts() public {
        proxy = IL1ChugSplashProxy(
            DeployUtils.create1({
                _name: "L1ChugSplashProxy",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IL1ChugSplashProxy.__constructor__, (owner)))
            })
        );

        vm.expectRevert(bytes("L1ChugSplashProxy: implementation is not set yet"));
        Implementation(address(proxy)).setCode(hex"604260005260206000f3");
    }

    /// @notice Tests that when the caller is not the owner but the owner has marked `isUpgrading` as true, the call
    ///         reverts
    function test_calls_whenUpgrading_reverts() public {
        Owner ownerContract = new Owner();
        vm.prank(owner);
        proxy.setOwner(address(ownerContract));

        ownerContract.setIsUpgrading(true);

        vm.expectRevert(bytes("L1ChugSplashProxy: system is currently being upgraded"));
        Implementation(address(proxy)).setCode(hex"604260005260206000f3");
    }

    /// @notice Tests that the owner can set storage of the proxy
    function test_setStorage_whenOwner_works() public {
        vm.prank(owner);
        proxy.setStorage(bytes32(0), bytes32(uint256(42)));
        assertEq(vm.load(address(proxy), bytes32(0)), bytes32(uint256(42)));
    }

    /// @notice Tests that when not the owner, `setStorage` delegatecalls the implementation
    function test_setStorage_whenNotOwner_works() public view {
        uint256 ret = Implementation(address(proxy)).setStorage(bytes32(0), bytes32(uint256(42)));
        assertEq(ret, 2);
        assertEq(vm.load(address(proxy), bytes32(0)), bytes32(uint256(0)));
    }

    /// @notice Tests that the owner can set the owner of the proxy
    function test_setOwner_whenOwner_works() public {
        vm.prank(owner);
        proxy.setOwner(alice);

        vm.prank(alice);
        assertEq(proxy.getOwner(), alice);
    }

    /// @notice Tests that when not the owner, `setOwner` delegatecalls the implementation
    function test_setOwner_whenNotOwner_works() public {
        uint256 ret = Implementation(address(proxy)).setOwner(alice);
        assertEq(ret, 3);

        vm.prank(owner);
        assertEq(proxy.getOwner(), owner);
    }

    /// @notice Tests that the owner can get the owner of the proxy
    function test_getOwner_whenOwner_works() public {
        vm.prank(owner);
        assertEq(proxy.getOwner(), owner);
    }

    /// @notice Tests that when not the owner, `getOwner` delegatecalls the implementation
    function test_getOwner_whenNotOwner_works() public view {
        uint256 ret = Implementation(address(proxy)).getOwner();
        assertEq(ret, 4);
    }

    /// @notice Tests that the owner can get the implementation of the proxy
    function test_getImplementation_whenOwner_works() public {
        vm.prank(owner);
        assertEq(proxy.getImplementation(), impl);
    }

    /// @notice Tests that when not the owner, `getImplementation` delegatecalls the implementation
    function test_getImplementation_whenNotOwner_works() public view {
        uint256 ret = Implementation(address(proxy)).getImplementation();
        assertEq(ret, 5);
    }
}
