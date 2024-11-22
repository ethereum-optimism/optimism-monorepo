// SPDX-License-Identifier: UNLICENSED
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";

// Target contract
import { L1ChugSplashProxy } from "src/legacy/L1ChugSplashProxy.sol";

contract L1ChugSplashProxyWrapper is L1ChugSplashProxy {
    constructor(address admin) L1ChugSplashProxy(admin) { }

    function getDeployCodePrefix() public pure returns (bytes13) {
        return DEPLOY_CODE_PREFIX;
    }
}

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
    L1ChugSplashProxyWrapper proxy;
    address impl;
    address owner = makeAddr("owner");
    address alice = makeAddr("alice");

    function setUp() public {
        proxy = new L1ChugSplashProxyWrapper(owner);
        vm.prank(owner);
        assertEq(proxy.getOwner(), owner);

        vm.prank(owner);
        proxy.setCode(type(Implementation).runtimeCode);

        vm.prank(owner);
        impl = proxy.getImplementation();
    }

    function test_getDeployCodePrefix_works() public view {
        assertTrue(proxy.getDeployCodePrefix() == 0x600D380380600D6000396000f3);
    }

    function test_setCode_whenOwner_succeeds() public {
        vm.prank(owner);
        proxy.setCode(hex"604260005260206000f3");

        vm.prank(owner);
        assertNotEq(proxy.getImplementation(), impl);
    }

    function test_setCode_whenNotOwner_works() public view {
        uint256 ret = Implementation(address(proxy)).setCode(hex"604260005260206000f3");
        assertEq(ret, 1);
    }

    function test_setCode_whenOwnerSameBytecode_works() public {
        vm.prank(owner);
        proxy.setCode(type(Implementation).runtimeCode);

        // does not deploy new implementation
        vm.prank(owner);
        assertEq(proxy.getImplementation(), impl);
    }

    // If this solc version/settings change and modifying this proves time consuming, we can just remove it.
    function test_setCode_whenOwnerAndDeployOutOfGas_reverts() public {
        vm.prank(owner);
        vm.expectRevert(bytes("L1ChugSplashProxy: code was not correctly deployed")); // Ran out of gas
        proxy.setCode{ gas: 65_000 }(
            hex"fefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefe"
        );
    }

    function test_calls_whenNotOwnerNoImplementation_reverts() public {
        proxy = new L1ChugSplashProxyWrapper(owner);

        vm.expectRevert(bytes("L1ChugSplashProxy: implementation is not set yet"));
        Implementation(address(proxy)).setCode(hex"604260005260206000f3");
    }

    function test_calls_whenUpgrading_reverts() public {
        Owner ownerContract = new Owner();
        vm.prank(owner);
        proxy.setOwner(address(ownerContract));

        ownerContract.setIsUpgrading(true);

        vm.expectRevert(bytes("L1ChugSplashProxy: system is currently being upgraded"));
        Implementation(address(proxy)).setCode(hex"604260005260206000f3");
    }

    function test_setStorage_whenOwner_works() public {
        vm.prank(owner);
        proxy.setStorage(bytes32(0), bytes32(uint256(42)));
        assertEq(vm.load(address(proxy), bytes32(0)), bytes32(uint256(42)));
    }

    function test_setStorage_whenNotOwner_works() public view {
        uint256 ret = Implementation(address(proxy)).setStorage(bytes32(0), bytes32(uint256(42)));
        assertEq(ret, 2);
        assertEq(vm.load(address(proxy), bytes32(0)), bytes32(uint256(0)));
    }

    function test_setOwner_whenOwner_works() public {
        vm.prank(owner);
        proxy.setOwner(alice);

        vm.prank(alice);
        assertEq(proxy.getOwner(), alice);
    }

    function test_setOwner_whenNotOwner_works() public {
        uint256 ret = Implementation(address(proxy)).setOwner(alice);
        assertEq(ret, 3);

        vm.prank(owner);
        assertEq(proxy.getOwner(), owner);
    }

    function test_getOwner_whenOwner_works() public {
        vm.prank(owner);
        assertEq(proxy.getOwner(), owner);
    }

    function test_getOwner_whenNotOwner_works() public view {
        uint256 ret = Implementation(address(proxy)).getOwner();
        assertEq(ret, 4);
    }

    function test_getImplementation_whenOwner_works() public {
        vm.prank(owner);
        assertEq(proxy.getImplementation(), impl);
    }

    function test_getImplementation_whenNotOwner_works() public view {
        uint256 ret = Implementation(address(proxy)).getImplementation();
        assertEq(ret, 5);
    }
}
