// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";

import { DeployAltDAInput, DeployAltDAOutput, DeployAltDA } from "scripts/deploy/DeployAltDA.s.sol";
import { IDataAvailabilityChallenge } from "src/L1/interfaces/IDataAvailabilityChallenge.sol";
import { IProxyAdmin } from "src/universal/interfaces/IProxyAdmin.sol";
import { IProxy } from "src/universal/interfaces/IProxy.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract DeployAltDAInput_Test is Test {
    DeployAltDAInput dai;

    // Define defaults
    bytes32 salt = bytes32(uint256(1));
    address proxyAdminAddr = makeAddr("proxyAdmin");
    address challengeContractOwner = makeAddr("challengeContractOwner");
    uint256 challengeWindow = 100;
    uint256 resolveWindow = 200;
    uint256 bondSize = 1 ether;
    uint256 resolverRefundPercentage = 10;

    function setUp() public {
        dai = new DeployAltDAInput();
    }

    function test_set_succeeds() public {
        dai.set(dai.salt.selector, salt);
        dai.set(dai.proxyAdmin.selector, proxyAdminAddr);
        dai.set(dai.challengeContractOwner.selector, challengeContractOwner);
        dai.set(dai.challengeWindow.selector, challengeWindow);
        dai.set(dai.resolveWindow.selector, resolveWindow);
        dai.set(dai.bondSize.selector, bondSize);
        dai.set(dai.resolverRefundPercentage.selector, resolverRefundPercentage);

        // Compare the default inputs to the getter methods
        assertEq(salt, dai.salt(), "100");
        assertEq(proxyAdminAddr, address(dai.proxyAdmin()), "200");
        assertEq(challengeContractOwner, dai.challengeContractOwner(), "300");
        assertEq(challengeWindow, dai.challengeWindow(), "400");
        assertEq(resolveWindow, dai.resolveWindow(), "500");
        assertEq(bondSize, dai.bondSize(), "600");
        assertEq(resolverRefundPercentage, dai.resolverRefundPercentage(), "700");
    }

    function test_getters_whenNotSet_reverts() public {
        bytes memory expectedErr = "DeployAltDAInput: ";

        vm.expectRevert(abi.encodePacked(expectedErr, "salt not set"));
        dai.salt();

        vm.expectRevert(abi.encodePacked(expectedErr, "proxyAdmin not set"));
        dai.proxyAdmin();

        vm.expectRevert(abi.encodePacked(expectedErr, "challengeContractOwner not set"));
        dai.challengeContractOwner();

        vm.expectRevert(abi.encodePacked(expectedErr, "challengeWindow not set"));
        dai.challengeWindow();

        vm.expectRevert(abi.encodePacked(expectedErr, "resolveWindow not set"));
        dai.resolveWindow();

        vm.expectRevert(abi.encodePacked(expectedErr, "bondSize not set"));
        dai.bondSize();

        vm.expectRevert(abi.encodePacked(expectedErr, "resolverRefundPercentage not set"));
        dai.resolverRefundPercentage();
    }

    function test_set_zeroAddress_reverts() public {
        vm.expectRevert("DeployAltDAInput: cannot set zero address");
        dai.set(dai.proxyAdmin.selector, address(0));

        vm.expectRevert("DeployAltDAInput: cannot set zero address");
        dai.set(dai.challengeContractOwner.selector, address(0));
    }

    function test_set_unknownSelector_reverts() public {
        bytes4 unknownSelector = bytes4(keccak256("unknown()"));

        vm.expectRevert("DeployAltDAInput: unknown selector");
        dai.set(unknownSelector, bytes32(0));

        vm.expectRevert("DeployAltDAInput: unknown selector");
        dai.set(unknownSelector, address(1));

        vm.expectRevert("DeployAltDAInput: unknown selector");
        dai.set(unknownSelector, uint256(1));
    }
}

contract DeployAltDAOutput_Test is Test {
    DeployAltDAOutput dao;

    // Store contract references to avoid stack too deep
    IDataAvailabilityChallenge internal dataAvailabilityChallengeImpl;

    function setUp() public {
        dao = new DeployAltDAOutput();
        dataAvailabilityChallengeImpl = IDataAvailabilityChallenge(payable(makeAddr("dataAvailabilityChallengeImpl")));
    }

    function test_set_succeeds() public {
        // Build the implementation with some bytecode
        vm.etch(address(dataAvailabilityChallengeImpl), hex"01");

        // Build proxy with implementation
        (IProxy dataAvailabilityChallengeProxy) =
            DeployUtils.buildERC1967ProxyWithImpl("dataAvailabilityChallengeProxy");

        // Set the addresses
        dao.set(dao.dataAvailabilityChallengeProxy.selector, address(dataAvailabilityChallengeProxy));
        dao.set(dao.dataAvailabilityChallengeImpl.selector, address(dataAvailabilityChallengeImpl));

        // Verify the addresses were set correctly
        assertEq(address(dataAvailabilityChallengeProxy), address(dao.dataAvailabilityChallengeProxy()), "100");
        assertEq(address(dataAvailabilityChallengeImpl), address(dao.dataAvailabilityChallengeImpl()), "200");
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployUtils: zero address");
        dao.dataAvailabilityChallengeProxy();

        vm.expectRevert("DeployUtils: zero address");
        dao.dataAvailabilityChallengeImpl();
    }

    function test_getters_whenAddrHasNoCode_reverts() public {
        address emptyAddr = makeAddr("emptyAddr");
        bytes memory expectedErr = bytes(string.concat("DeployUtils: no code at ", vm.toString(emptyAddr)));

        dao.set(dao.dataAvailabilityChallengeProxy.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dao.dataAvailabilityChallengeProxy();

        dao.set(dao.dataAvailabilityChallengeImpl.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        dao.dataAvailabilityChallengeImpl();
    }

    function test_set_zeroAddress_reverts() public {
        vm.expectRevert("DeployAltDAOutput: cannot set zero address");
        dao.set(dao.dataAvailabilityChallengeProxy.selector, address(0));

        vm.expectRevert("DeployAltDAOutput: cannot set zero address");
        dao.set(dao.dataAvailabilityChallengeImpl.selector, address(0));
    }

    function test_set_unknownSelector_reverts() public {
        bytes4 unknownSelector = bytes4(keccak256("unknown()"));
        vm.expectRevert("DeployAltDAOutput: unknown selector");
        dao.set(unknownSelector, address(1));
    }
}

contract DeployAltDA_Test is Test {
    DeployAltDA deployer;
    DeployAltDAInput dai;
    DeployAltDAOutput dao;

    // Define defaults
    bytes32 salt = bytes32(uint256(1));
    IProxyAdmin proxyAdmin;
    address challengeContractOwner = makeAddr("challengeContractOwner");
    uint256 challengeWindow = 100;
    uint256 resolveWindow = 200;
    uint256 bondSize = 1 ether;
    uint256 resolverRefundPercentage = 10;

    function setUp() public {
        // Deploy the main contract and get input/output contracts
        deployer = new DeployAltDA();
        (dai, dao) = _setupIOContracts();

        // Setup proxyAdmin
        proxyAdmin = IProxyAdmin(
            DeployUtils.create1({
                _name: "ProxyAdmin",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxyAdmin.__constructor__, (msg.sender)))
            })
        );

        // Set the default values
        dai.set(dai.salt.selector, salt);
        dai.set(dai.proxyAdmin.selector, address(proxyAdmin));
        dai.set(dai.challengeContractOwner.selector, challengeContractOwner);
        dai.set(dai.challengeWindow.selector, challengeWindow);
        dai.set(dai.resolveWindow.selector, resolveWindow);
        dai.set(dai.bondSize.selector, bondSize);
        dai.set(dai.resolverRefundPercentage.selector, resolverRefundPercentage);
    }

    function _setupIOContracts() internal returns (DeployAltDAInput, DeployAltDAOutput) {
        DeployAltDAInput _dai = new DeployAltDAInput();
        DeployAltDAOutput _dao = new DeployAltDAOutput();
        return (_dai, _dao);
    }

    function test_run_succeeds() public {
        deployer.run(dai, dao);

        // Verify everything is set up correctly
        IDataAvailabilityChallenge dac = dao.dataAvailabilityChallengeProxy();
        assertTrue(address(dac).code.length > 0, "100");
        assertTrue(address(dao.dataAvailabilityChallengeImpl()).code.length > 0, "200");

        // Check all initialization parameters
        assertEq(dac.owner(), challengeContractOwner, "300");
        assertEq(dac.challengeWindow(), challengeWindow, "400");
        assertEq(dac.resolveWindow(), resolveWindow, "500");
        assertEq(dac.bondSize(), bondSize, "600");
        assertEq(dac.resolverRefundPercentage(), resolverRefundPercentage, "700");
        // Make sure the proxy admin is set correctly.
        vm.prank(address(0));
        assertEq(IProxy(payable(address(dac))).admin(), address(proxyAdmin), "800");
    }

    function test_checkOutput_whenNotInitialized_reverts() public {
        vm.expectRevert("DeployUtils: zero address");
        deployer.checkOutput(dai, dao);
    }

    function test_checkOutput_whenProxyNotInitialized_reverts() public {
        // Deploy but don't initialize
        deployer.deployDataAvailabilityChallengeProxy(dai, dao);
        deployer.deployDataAvailabilityChallengeImpl(dai, dao);

        vm.expectRevert("DeployUtils: zero address");
        deployer.checkOutput(dai, dao);
    }

    function testFuzz_run_withDifferentParameters_works(
        uint256 _challengeWindow,
        uint256 _resolveWindow,
        uint256 _bondSize,
        uint256 _resolverRefundPercentage
    )
        public
    {
        // Bound the values to reasonable ranges
        _challengeWindow = bound(_challengeWindow, 1, 365 days);
        _resolveWindow = bound(_resolveWindow, 1, 365 days);
        _bondSize = bound(_bondSize, 0.1 ether, 100 ether);
        _resolverRefundPercentage = bound(_resolverRefundPercentage, 1, 100);

        // Set the new values
        dai.set(dai.salt.selector, salt);
        dai.set(dai.proxyAdmin.selector, address(proxyAdmin));
        dai.set(dai.challengeWindow.selector, _challengeWindow);
        dai.set(dai.resolveWindow.selector, _resolveWindow);
        dai.set(dai.bondSize.selector, _bondSize);
        dai.set(dai.resolverRefundPercentage.selector, _resolverRefundPercentage);

        // Run deployment
        deployer.run(dai, dao);

        // Verify values
        IDataAvailabilityChallenge dac = dao.dataAvailabilityChallengeProxy();
        assertEq(dac.challengeWindow(), _challengeWindow, "100");
        assertEq(dac.resolveWindow(), _resolveWindow, "200");
        assertEq(dac.bondSize(), _bondSize, "300");
        assertEq(dac.resolverRefundPercentage(), _resolverRefundPercentage, "400");
    }
}
