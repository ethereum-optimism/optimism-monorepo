// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

/// @title PredeploysTest
contract PredeploysBaseTest is CommonTest {
    //////////////////////////////////////////////////////
    /// Internal helpers
    //////////////////////////////////////////////////////

    /// @dev Returns true if the address is a predeploy that has a different code in the interop mode.
    function _interopCodeDiffer(address _addr) internal pure returns (bool) {
        return _addr == Predeploys.L1_BLOCK_ATTRIBUTES || _addr == Predeploys.L2_STANDARD_BRIDGE;
    }

    /// @dev Returns true if the account is not meant to be in the L2 genesis anymore.
    function _isOmitted(address _addr) internal pure returns (bool) {
        return _addr == Predeploys.L1_MESSAGE_SENDER;
    }

    /// @dev Returns true if the predeploy is initializable.
    function _isInitializable(address _addr) internal pure returns (bool) {
        return _addr == Predeploys.L2_CROSS_DOMAIN_MESSENGER || _addr == Predeploys.L2_STANDARD_BRIDGE
            || _addr == Predeploys.L2_ERC721_BRIDGE || _addr == Predeploys.OPTIMISM_MINTABLE_ERC20_FACTORY;
    }

    /// @dev Returns true if the predeploy uses immutables.
    function _usesImmutables(address _addr) internal pure returns (bool) {
        return _addr == Predeploys.OPTIMISM_MINTABLE_ERC721_FACTORY || _addr == Predeploys.SEQUENCER_FEE_WALLET
            || _addr == Predeploys.BASE_FEE_VAULT || _addr == Predeploys.L1_FEE_VAULT || _addr == Predeploys.EAS
            || _addr == Predeploys.GOVERNANCE_TOKEN;
    }

    function test_predeployToCodeNamespace_works() external pure {
        assertEq(
            address(0xc0D3C0d3C0d3C0D3c0d3C0d3c0D3C0d3c0d30000),
            Predeploys.predeployToCodeNamespace(Predeploys.LEGACY_MESSAGE_PASSER)
        );
        assertEq(
            address(0xc0d3C0d3C0d3c0D3C0D3C0d3C0d3C0D3C0D3000f),
            Predeploys.predeployToCodeNamespace(Predeploys.GAS_PRICE_ORACLE)
        );
        assertEq(
            address(0xC0d3C0d3c0d3c0d3C0d3c0D3c0D3c0D3C0d30420),
            Predeploys.predeployToCodeNamespace(address(0x4200000000000000000000000000000000000420))
        );
    }

    function _test_predeploys(bool _useInterop) internal {
        uint256 count = 2048;
        uint160 prefix = uint160(0x420) << 148;

        bytes memory proxyCode = vm.getDeployedCode("Proxy.sol:Proxy");

        for (uint256 i = 0; i < count; i++) {
            address addr = address(prefix | uint160(i));
            address implAddr = Predeploys.predeployToCodeNamespace(addr);

            if (_isOmitted(addr)) {
                assertEq(implAddr.code.length, 0, "must have no code");
                continue;
            }

            bool isPredeploy = Predeploys.isSupportedPredeploy(addr, _useInterop);

            bytes memory code = addr.code;
            if (isPredeploy) assertTrue(code.length > 0);

            bool proxied = Predeploys.notProxied(addr) == false;

            if (!isPredeploy) {
                // All of the predeploys, even if inactive, have their admin set to the proxy admin
                if (proxied) assertEq(EIP1967Helper.getAdmin(addr), Predeploys.PROXY_ADMIN, "Admin mismatch");
                continue;
            }

            string memory cname = Predeploys.getName(addr);
            assertNotEq(cname, "", "must have a name");

            bytes memory supposedCode = vm.getDeployedCode(string.concat(cname, ".sol:", cname));
            assertNotEq(supposedCode.length, 0, "must have supposed code");

            if (proxied == false) {
                // can't check bytecode if it's modified with immutables in genesis.
                if (!_usesImmutables(addr)) {
                    assertEq(code, supposedCode, "non-proxy contract should be deployed in-place");
                }
                continue;
            }

            // The code is a proxy
            assertEq(code, proxyCode);

            assertEq(
                EIP1967Helper.getImplementation(addr),
                implAddr,
                string.concat("Implementation mismatch for ", vm.toString(addr))
            );
            assertNotEq(implAddr.code.length, 0, "predeploy implementation account must have code");
            if (!_usesImmutables(addr) && !_interopCodeDiffer(addr)) {
                // can't check bytecode if it's modified with immutables in genesis.
                assertEq(implAddr.code, supposedCode, "proxy implementation contract should match contract source");
            }

            if (_isInitializable(addr)) {
                assertEq(l2Genesis.loadInitializedSlot(cname), uint8(1));
            }
        }
    }
}

contract PredeploysTest is PredeploysBaseTest {
    /// @dev Tests that the predeploy addresses are set correctly. They have code
    ///      and the proxied accounts have the correct admin.
    function test_predeploys_succeeds() external {
        _test_predeploys(false);
    }
}

contract PredeploysInteropTest is PredeploysBaseTest {
    /// @notice Test setup. Enabling interop to get all predeploys.
    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    /// @dev Tests that the predeploy addresses are set correctly. They have code
    ///      and the proxied accounts have the correct admin. Using interop.
    function test_predeploys_succeeds() external {
        _test_predeploys(true);
    }
}
