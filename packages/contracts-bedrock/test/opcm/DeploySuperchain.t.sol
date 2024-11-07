// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test, stdStorage, StdStorage } from "forge-std/Test.sol";
import { stdToml } from "forge-std/StdToml.sol";

import { ProxyAdmin } from "src/universal/ProxyAdmin.sol";
import { Proxy } from "src/universal/Proxy.sol";
import { SuperchainConfig } from "src/L1/SuperchainConfig.sol";
import { IProtocolVersions, ProtocolVersion } from "src/L1/interfaces/IProtocolVersions.sol";
import { DeploySuperchainInput, DeploySuperchain, DeploySuperchainOutput } from "scripts/deploy/DeploySuperchain.s.sol";

contract DeploySuperchainInput_Test is Test {
    DeploySuperchainInput dsi;

    address superchainProxyAdminOwner = makeAddr("superchainProxyAdminOwner");
    address protocolVersionsOwner = makeAddr("defaultProtocolVersionsOwner");
    address guardian = makeAddr("defaultGuardian");
    bool paused = false;
    ProtocolVersion requiredProtocolVersion = ProtocolVersion.wrap(1);
    ProtocolVersion recommendedProtocolVersion = ProtocolVersion.wrap(2);

    function setUp() public {
        dsi = new DeploySuperchainInput();
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeploySuperchainInput: superchainProxyAdminOwner not set");
        dsi.superchainProxyAdminOwner();

        vm.expectRevert("DeploySuperchainInput: protocolVersionsOwner not set");
        dsi.protocolVersionsOwner();

        vm.expectRevert("DeploySuperchainInput: guardian not set");
        dsi.guardian();

        vm.expectRevert("DeploySuperchainInput: requiredProtocolVersion not set");
        dsi.requiredProtocolVersion();

        vm.expectRevert("DeploySuperchainInput: recommendedProtocolVersion not set");
        dsi.recommendedProtocolVersion();
    }
}

contract DeploySuperchainOutput_Test is Test {
    using stdToml for string;

    DeploySuperchainOutput dso;

    function setUp() public {
        dso = new DeploySuperchainOutput();
    }

    function test_set_succeeds() public {
        // We don't fuzz, because we need code at the address, and we can't etch code if the fuzzer
        // provides precompiles, so we end up with a lot of boilerplate logic to get valid inputs.
        // Hardcoding a concrete set of valid addresses is simpler and still sufficient.
        ProxyAdmin superchainProxyAdmin = ProxyAdmin(makeAddr("superchainProxyAdmin"));
        SuperchainConfig superchainConfigImpl = SuperchainConfig(makeAddr("superchainConfigImpl"));
        SuperchainConfig superchainConfigProxy = SuperchainConfig(makeAddr("superchainConfigProxy"));
        IProtocolVersions protocolVersionsImpl = IProtocolVersions(makeAddr("protocolVersionsImpl"));
        IProtocolVersions protocolVersionsProxy = IProtocolVersions(makeAddr("protocolVersionsProxy"));

        // Ensure each address has code, since these are expected to be contracts.
        vm.etch(address(superchainProxyAdmin), hex"01");
        vm.etch(address(superchainConfigImpl), hex"01");
        vm.etch(address(superchainConfigProxy), hex"01");
        vm.etch(address(protocolVersionsImpl), hex"01");
        vm.etch(address(protocolVersionsProxy), hex"01");

        // Set the output data.
        dso.set(dso.superchainProxyAdmin.selector, address(superchainProxyAdmin));
        dso.set(dso.superchainConfigImpl.selector, address(superchainConfigImpl));
        dso.set(dso.superchainConfigProxy.selector, address(superchainConfigProxy));
        dso.set(dso.protocolVersionsImpl.selector, address(protocolVersionsImpl));
        dso.set(dso.protocolVersionsProxy.selector, address(protocolVersionsProxy));

        // Compare the test data to the getter methods.
        assertEq(address(superchainProxyAdmin), address(dso.superchainProxyAdmin()), "100");
        assertEq(address(superchainConfigImpl), address(dso.superchainConfigImpl()), "200");
        assertEq(address(superchainConfigProxy), address(dso.superchainConfigProxy()), "300");
        assertEq(address(protocolVersionsImpl), address(dso.protocolVersionsImpl()), "400");
        assertEq(address(protocolVersionsProxy), address(dso.protocolVersionsProxy()), "500");
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployUtils: zero address");
        dso.superchainConfigImpl();

        vm.expectRevert("DeployUtils: zero address");
        dso.superchainConfigProxy();

        vm.expectRevert("DeployUtils: zero address");
        dso.protocolVersionsImpl();

        vm.expectRevert("DeployUtils: zero address");
        dso.protocolVersionsProxy();
    }

    function test_getters_whenAddrHasNoCode_reverts() public {
        address emptyAddr = makeAddr("emptyAddr");
        bytes memory expectedErr = bytes(string.concat("DeployUtils: no code at ", vm.toString(emptyAddr)));

        dso.set(dso.superchainConfigImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dso.superchainConfigImpl();

        dso.set(dso.superchainConfigProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dso.superchainConfigProxy();

        dso.set(dso.protocolVersionsImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dso.protocolVersionsImpl();

        dso.set(dso.protocolVersionsProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dso.protocolVersionsProxy();
    }
}

contract DeploySuperchain_Test is Test {
    using stdStorage for StdStorage;

    DeploySuperchain deploySuperchain;
    DeploySuperchainInput dsi;
    DeploySuperchainOutput dso;

    // Define default input variables for testing.
    address defaultProxyAdminOwner = makeAddr("defaultProxyAdminOwner");
    address defaultProtocolVersionsOwner = makeAddr("defaultProtocolVersionsOwner");
    address defaultGuardian = makeAddr("defaultGuardian");
    bool defaultPaused = false;
    ProtocolVersion defaultRequiredProtocolVersion = ProtocolVersion.wrap(1);
    ProtocolVersion defaultRecommendedProtocolVersion = ProtocolVersion.wrap(2);

    function setUp() public {
        deploySuperchain = new DeploySuperchain();
        (dsi, dso) = deploySuperchain.etchIOContracts();
    }

    function unwrap(ProtocolVersion _pv) internal pure returns (uint256) {
        return ProtocolVersion.unwrap(_pv);
    }

    function hash(bytes32 _seed, uint256 _i) internal pure returns (bytes32) {
        return keccak256(abi.encode(_seed, _i));
    }

    function testFuzz_run_memory_succeeds(bytes32 _seed) public {
        // Generate random input values from the seed. This doesn't give us the benefit of the forge
        // fuzzer's dictionary, but that's ok because we are just testing that values are set and
        // passed correctly.
        address superchainProxyAdminOwner = address(uint160(uint256(hash(_seed, 0))));
        address protocolVersionsOwner = address(uint160(uint256(hash(_seed, 1))));
        address guardian = address(uint160(uint256(hash(_seed, 2))));
        bool paused = bool(uint8(uint256(hash(_seed, 3))) % 2 == 0);
        ProtocolVersion requiredProtocolVersion = ProtocolVersion.wrap(uint256(hash(_seed, 4)));
        ProtocolVersion recommendedProtocolVersion = ProtocolVersion.wrap(uint256(hash(_seed, 5)));

        // Set the input values on the input contract.
        dsi.set(dsi.superchainProxyAdminOwner.selector, superchainProxyAdminOwner);
        dsi.set(dsi.protocolVersionsOwner.selector, protocolVersionsOwner);
        dsi.set(dsi.guardian.selector, guardian);
        dsi.set(dsi.paused.selector, paused);
        dsi.set(dsi.requiredProtocolVersion.selector, requiredProtocolVersion);
        dsi.set(dsi.recommendedProtocolVersion.selector, recommendedProtocolVersion);

        // Run the deployment script.
        deploySuperchain.run(dsi, dso);

        // Assert inputs were properly passed through to the contract initializers.
        assertEq(address(dso.superchainProxyAdmin().owner()), superchainProxyAdminOwner, "100");
        assertEq(address(dso.protocolVersionsProxy().owner()), protocolVersionsOwner, "200");
        assertEq(address(dso.superchainConfigProxy().guardian()), guardian, "300");
        assertEq(dso.superchainConfigProxy().paused(), paused, "400");
        assertEq(unwrap(dso.protocolVersionsProxy().required()), unwrap(requiredProtocolVersion), "500");
        assertEq(unwrap(dso.protocolVersionsProxy().recommended()), unwrap(recommendedProtocolVersion), "600");

        // Architecture assertions.
        // We prank as the zero address due to the Proxy's `proxyCallIfNotAdmin` modifier.
        Proxy superchainConfigProxy = Proxy(payable(address(dso.superchainConfigProxy())));
        Proxy protocolVersionsProxy = Proxy(payable(address(dso.protocolVersionsProxy())));

        vm.startPrank(address(0));
        assertEq(superchainConfigProxy.implementation(), address(dso.superchainConfigImpl()), "700");
        assertEq(protocolVersionsProxy.implementation(), address(dso.protocolVersionsImpl()), "800");
        assertEq(superchainConfigProxy.admin(), protocolVersionsProxy.admin(), "900");
        assertEq(superchainConfigProxy.admin(), address(dso.superchainProxyAdmin()), "1000");
        vm.stopPrank();

        // Ensure that `checkOutput` passes. This is called by the `run` function during execution,
        // so this just acts as a sanity check. It reverts on failure.
        dso.checkOutput(dsi);
    }

    function test_run_nullInput_reverts() public {
        // Set default values for all inputs.
        dsi.set(dsi.superchainProxyAdminOwner.selector, defaultProxyAdminOwner);
        dsi.set(dsi.protocolVersionsOwner.selector, defaultProtocolVersionsOwner);
        dsi.set(dsi.guardian.selector, defaultGuardian);
        dsi.set(dsi.paused.selector, defaultPaused);
        dsi.set(dsi.requiredProtocolVersion.selector, defaultRequiredProtocolVersion);
        dsi.set(dsi.recommendedProtocolVersion.selector, defaultRecommendedProtocolVersion);

        // Assert over each role being set to the zero address. We aren't allowed to use the setter
        // methods to set the zero address, so we use StdStorage. We can't use the `checked_write`
        // method, because it does a final call to test that the value was set correctly, but for us
        // that would revert. Therefore we use StdStorage to find the slot, then we write to it.
        uint256 slot = zeroOutSlotForSelector(dsi.superchainProxyAdminOwner.selector);
        vm.expectRevert("DeploySuperchainInput: superchainProxyAdminOwner not set");
        deploySuperchain.run(dsi, dso);
        // Restore the value we just tested.
        vm.store(address(dsi), bytes32(slot), bytes32(uint256(uint160(defaultProxyAdminOwner))));

        slot = zeroOutSlotForSelector(dsi.protocolVersionsOwner.selector);
        vm.expectRevert("DeploySuperchainInput: protocolVersionsOwner not set");
        deploySuperchain.run(dsi, dso);
        vm.store(address(dsi), bytes32(slot), bytes32(uint256(uint160(defaultProtocolVersionsOwner))));

        slot = zeroOutSlotForSelector(dsi.guardian.selector);
        vm.expectRevert("DeploySuperchainInput: guardian not set");
        deploySuperchain.run(dsi, dso);
        vm.store(address(dsi), bytes32(slot), bytes32(uint256(uint160(defaultGuardian))));

        slot = zeroOutSlotForSelector(dsi.requiredProtocolVersion.selector);
        vm.expectRevert("DeploySuperchainInput: requiredProtocolVersion not set");
        deploySuperchain.run(dsi, dso);
        vm.store(address(dsi), bytes32(slot), bytes32(unwrap(defaultRequiredProtocolVersion)));

        slot = zeroOutSlotForSelector(dsi.recommendedProtocolVersion.selector);
        vm.expectRevert("DeploySuperchainInput: recommendedProtocolVersion not set");
        deploySuperchain.run(dsi, dso);
        vm.store(address(dsi), bytes32(slot), bytes32(unwrap(defaultRecommendedProtocolVersion)));
    }

    function zeroOutSlotForSelector(bytes4 _selector) internal returns (uint256 slot_) {
        slot_ = stdstore.enable_packed_slots().target(address(dsi)).sig(_selector).find();
        vm.store(address(dsi), bytes32(slot_), bytes32(0));
    }
}
