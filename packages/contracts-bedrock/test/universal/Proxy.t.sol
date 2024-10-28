// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { Bytes32AddressLib } from "@rari-capital/solmate/src/utils/Bytes32AddressLib.sol";
import { IProxy } from "src/universal/interfaces/IProxy.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract SimpleStorage {
    mapping(uint256 => uint256) internal store;

    function get(uint256 key) external payable returns (uint256) {
        return store[key];
    }

    function set(uint256 key, uint256 value) external payable {
        store[key] = value;
    }
}

contract Clasher {
    function upgradeTo(address) external pure {
        revert("Clasher: upgradeTo");
    }
}

contract Proxy_Test is Test {
    event Upgraded(address indexed implementation);
    event AdminChanged(address previousAdmin, address newAdmin);

    address alice = address(64);

    bytes32 internal constant IMPLEMENTATION_KEY = bytes32(uint256(keccak256("eip1967.proxy.implementation")) - 1);

    bytes32 internal constant OWNER_KEY = bytes32(uint256(keccak256("eip1967.proxy.admin")) - 1);

    IProxy proxy;
    SimpleStorage simpleStorage;

    function setUp() external {
        // Deploy a proxy and simple storage contract as
        // the implementation
        proxy = IProxy(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (alice)))
            })
        );
        simpleStorage = new SimpleStorage();

        vm.prank(alice);
        proxy.upgradeTo(address(simpleStorage));
    }

    function test_implementationKey_succeeds() external {
        // The hardcoded implementation key should be correct
        vm.prank(alice);
        proxy.upgradeTo(address(6));

        bytes32 key = vm.load(address(proxy), IMPLEMENTATION_KEY);
        assertEq(address(6), Bytes32AddressLib.fromLast20Bytes(key));

        vm.prank(alice);
        address impl = proxy.implementation();
        assertEq(impl, address(6));
    }

    function test_ownerKey_succeeds() external {
        // The hardcoded owner key should be correct
        vm.prank(alice);
        proxy.changeAdmin(address(6));

        bytes32 key = vm.load(address(proxy), OWNER_KEY);
        assertEq(address(6), Bytes32AddressLib.fromLast20Bytes(key));

        vm.prank(address(6));
        address owner = proxy.admin();
        assertEq(owner, address(6));
    }

    function test_proxyCallToImp_notAdmin_succeeds() external {
        // The implementation does not have a `upgradeTo`
        // method, calling `upgradeTo` not as the owner
        // should revert.
        vm.expectRevert(bytes(""));
        proxy.upgradeTo(address(64));

        // Call `upgradeTo` as the owner, it should succeed
        // and emit the `Upgraded` event.
        vm.expectEmit(true, true, true, true);
        emit Upgraded(address(64));
        vm.prank(alice);
        proxy.upgradeTo(address(64));

        // Get the implementation as the owner
        vm.prank(alice);
        address impl = proxy.implementation();
        assertEq(impl, address(64));
    }

    function test_ownerProxyCall_notAdmin_succeeds() external {
        // Calling `changeAdmin` not as the owner should revert
        // as the implementation does not have a `changeAdmin` method.
        vm.expectRevert(bytes(""));
        proxy.changeAdmin(address(1));

        // Call `changeAdmin` as the owner, it should succeed
        // and emit the `AdminChanged` event.
        vm.expectEmit(true, true, true, true);
        emit AdminChanged(alice, address(1));
        vm.prank(alice);
        proxy.changeAdmin(address(1));

        // Calling `admin` not as the owner should
        // revert as the implementation does not have
        // a `admin` method.
        vm.expectRevert(bytes(""));
        proxy.admin();

        // Calling `admin` as the owner should work.
        vm.prank(address(1));
        address owner = proxy.admin();
        assertEq(owner, address(1));
    }

    function test_delegatesToImpl_succeeds() external {
        // Call the storage setter on the proxy
        SimpleStorage(address(proxy)).set(1, 1);

        // The key should not be set in the implementation
        uint256 result = simpleStorage.get(1);
        assertEq(result, 0);
        {
            // The key should be set in the proxy
            uint256 expect = SimpleStorage(address(proxy)).get(1);
            assertEq(expect, 1);
        }

        {
            // The owner should be able to call through the proxy
            // when there is not a function selector crash
            vm.prank(alice);
            uint256 expect = SimpleStorage(address(proxy)).get(1);
            assertEq(expect, 1);
        }
    }

    function test_upgradeToAndCall_succeeds() external {
        {
            // There should be nothing in the current proxy storage
            uint256 expect = SimpleStorage(address(proxy)).get(1);
            assertEq(expect, 0);
        }

        // Deploy a new SimpleStorage
        simpleStorage = new SimpleStorage();

        // Set the new SimpleStorage as the implementation
        // and call.
        vm.expectEmit(true, true, true, true);
        emit Upgraded(address(simpleStorage));
        vm.prank(alice);
        proxy.upgradeToAndCall(address(simpleStorage), abi.encodeCall(SimpleStorage.set, (1, 1)));

        // The call should have impacted the state
        uint256 result = SimpleStorage(address(proxy)).get(1);
        assertEq(result, 1);
    }

    function test_upgradeToAndCall_functionDoesNotExist_reverts() external {
        // Get the current implementation address
        vm.prank(alice);
        address impl = proxy.implementation();
        assertEq(impl, address(simpleStorage));

        // Deploy a new SimpleStorage
        simpleStorage = new SimpleStorage();

        // Set the new SimpleStorage as the implementation
        // and call. This reverts because the calldata doesn't
        // match a function on the implementation.
        vm.expectRevert("Proxy: delegatecall to new implementation contract failed");
        vm.prank(alice);
        proxy.upgradeToAndCall(address(simpleStorage), hex"");

        // The implementation address should have not
        // updated because the call to `upgradeToAndCall`
        // reverted.
        vm.prank(alice);
        address postImpl = proxy.implementation();
        assertEq(impl, postImpl);

        // The attempt to `upgradeToAndCall`
        // should revert when it is not called by the owner.
        vm.expectRevert(bytes(""));
        proxy.upgradeToAndCall(address(simpleStorage), abi.encodeCall(simpleStorage.set, (1, 1)));
    }

    function test_upgradeToAndCall_isPayable_succeeds() external {
        // Give alice some funds
        vm.deal(alice, 1 ether);
        // Set the implementation and call and send
        // value.
        vm.prank(alice);
        proxy.upgradeToAndCall{ value: 1 ether }(address(simpleStorage), abi.encodeCall(simpleStorage.set, (1, 1)));

        // The implementation address should be correct
        vm.prank(alice);
        address impl = proxy.implementation();
        assertEq(impl, address(simpleStorage));

        // The proxy should have a balance
        assertEq(address(proxy).balance, 1 ether);
    }

    function test_upgradeTo_clashingFunctionSignatures_succeeds() external {
        // Clasher has a clashing function with the proxy.
        Clasher clasher = new Clasher();

        // Set the clasher as the implementation.
        vm.prank(alice);
        proxy.upgradeTo(address(clasher));

        {
            // Assert that the implementation was set properly.
            vm.prank(alice);
            address impl = proxy.implementation();
            assertEq(impl, address(clasher));
        }

        // Call the clashing function on the proxy
        // not as the owner so that the call passes through.
        // The implementation will revert so we can be
        // sure that the call passed through.
        vm.expectRevert(bytes("Clasher: upgradeTo"));
        proxy.upgradeTo(address(0));

        {
            // Now call the clashing function as the owner
            // and be sure that it doesn't pass through to
            // the implementation.
            vm.prank(alice);
            proxy.upgradeTo(address(0));
            vm.prank(alice);
            address impl = proxy.implementation();
            assertEq(impl, address(0));
        }
    }

    // Allow for `eth_call` to call proxy methods
    // by setting "from" to `address(0)`.
    function test_implementation_zeroAddressCaller_succeeds() external {
        vm.prank(address(0));
        address impl = proxy.implementation();
        assertEq(impl, address(simpleStorage));
    }

    function test_implementation_isZeroAddress_reverts() external {
        // Set `address(0)` as the implementation.
        vm.prank(alice);
        proxy.upgradeTo(address(0));

        (bool success, bytes memory returndata) = address(proxy).call(hex"");
        assertEq(success, false);

        bytes memory err = abi.encodeWithSignature("Error(string)", "Proxy: implementation not initialized"); // nosemgrep:
            // sol-style-use-abi-encodecall

        assertEq(returndata, err);
    }
}
