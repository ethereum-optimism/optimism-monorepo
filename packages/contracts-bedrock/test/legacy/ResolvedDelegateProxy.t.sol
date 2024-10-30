// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { Test } from "forge-std/Test.sol";

// Target contract dependencies
import { IAddressManager } from "src/legacy/interfaces/IAddressManager.sol";

// Target contract
import { IResolvedDelegateProxy } from "src/legacy/interfaces/IResolvedDelegateProxy.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract ResolvedDelegateProxy_Test is Test {
    IAddressManager internal addressManager;
    SimpleImplementation internal impl;
    SimpleImplementation internal proxy;

    /// @dev Sets up the test suite.
    function setUp() public {
        // Set up the address manager.
        addressManager = IAddressManager(
            DeployUtils.create1({
                _name: "AddressManager",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IAddressManager.__constructor__, ()))
            })
        );
        impl = new SimpleImplementation();
        addressManager.setAddress("SimpleImplementation", address(impl));

        // Set up the proxy.
        proxy = SimpleImplementation(
            address(
                DeployUtils.create1({
                    _name: "ResolvedDelegateProxy",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(IResolvedDelegateProxy.__constructor__, (addressManager, "SimpleImplementation"))
                    )
                })
            )
        );
    }

    /// @dev Tests that the proxy properly bubbles up returndata when the delegatecall succeeds.
    function testFuzz_fallback_delegateCallFoo_succeeds(uint256 x) public {
        vm.expectCall(address(impl), abi.encodeCall(impl.foo, (x)));
        assertEq(proxy.foo(x), x);
    }

    /// @dev Tests that the proxy properly bubbles up returndata when the delegatecall reverts.
    function test_fallback_delegateCallBar_reverts() public {
        vm.expectRevert("SimpleImplementation: revert");
        vm.expectCall(address(impl), abi.encodeCall(impl.bar, ()));
        proxy.bar();
    }

    /// @dev Tests that the proxy fallback reverts as expected if the implementation within the
    ///      address manager is not set.
    function test_fallback_addressManagerNotSet_reverts() public {
        IAddressManager am = IAddressManager(
            DeployUtils.create1({
                _name: "AddressManager",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IAddressManager.__constructor__, ()))
            })
        );
        SimpleImplementation p = SimpleImplementation(
            address(
                DeployUtils.create1({
                    _name: "ResolvedDelegateProxy",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(IResolvedDelegateProxy.__constructor__, (am, "SimpleImplementation"))
                    )
                })
            )
        );

        vm.expectRevert("ResolvedDelegateProxy: target address must be initialized");
        p.foo(0);
    }
}

contract SimpleImplementation {
    function foo(uint256 _x) public pure returns (uint256) {
        return _x;
    }

    function bar() public pure {
        revert("SimpleImplementation: revert");
    }
}
