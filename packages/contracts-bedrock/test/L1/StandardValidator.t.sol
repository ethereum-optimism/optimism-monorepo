// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test } from "forge-std/Test.sol";

// Target contract
import { StandardValidatorV180 } from "src/L1/StandardValidator.sol";

// Libraries
import { GameType, GameTypes, Hash } from "src/dispute/lib/Types.sol";
import { Duration } from "src/dispute/lib/LibUDT.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { ICrossDomainMessenger } from "interfaces/universal/ICrossDomainMessenger.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IOptimismMintableERC20Factory } from "interfaces/universal/IOptimismMintableERC20Factory.sol";
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IERC721Bridge } from "interfaces/universal/IERC721Bridge.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IMIPS } from "interfaces/cannon/IMIPS.sol";
import { IL1StandardBridge } from "interfaces/L1/IL1StandardBridge.sol";
import { IStandardBridge } from "interfaces/universal/IStandardBridge.sol";

contract StandardValidatorV180_Test is Test {
    StandardValidatorV180 validator;
    ISuperchainConfig superchainConfig;
    address l1PAOMultisig;
    address mips;
    address guardian;
    address challenger;

    // Mock contracts for validation
    IProxyAdmin proxyAdmin;
    ISystemConfig systemConfig;
    bytes32 absolutePrestate;
    uint256 l2ChainID;

    // Mock addresses for dependencies
    address disputeGameFactory;
    address permissionedDisputeGame;
    address permissionlessDisputeGame;
    address permissionedASR;
    address permissionlessASR;
    address optimismPortal;
    address l1CrossDomainMessenger;
    address l1StandardBridge;
    address l1ERC721Bridge;
    address optimismMintableERC20Factory;
    address permissionedDelayedWETH;
    address permissionlessDelayedWETH;
    address preimageOracle;

    function setUp() public {
        // Setup test addresses
        l1PAOMultisig = makeAddr("l1PAOMultisig");
        mips = makeAddr("mips");
        guardian = makeAddr("guardian");
        challenger = makeAddr("challenger");
        superchainConfig = ISuperchainConfig(makeAddr("superchainConfig"));

        // Mock superchainConfig calls needed in setup
        vm.mockCall(address(superchainConfig), abi.encodeCall(ISuperchainConfig.guardian, ()), abi.encode(guardian));
        vm.mockCall(address(superchainConfig), abi.encodeCall(ISuperchainConfig.paused, ()), abi.encode(false));

        // Deploy validator with all required constructor args
        validator = new StandardValidatorV180(
            StandardValidatorV180.Implementations({
                systemConfigImpl: makeAddr("systemConfigImpl"),
                optimismPortalImpl: makeAddr("optimismPortalImpl"),
                l1CrossDomainMessengerImpl: makeAddr("l1CrossDomainMessengerImpl"),
                l1StandardBridgeImpl: makeAddr("l1StandardBridgeImpl"),
                l1ERC721BridgeImpl: makeAddr("l1ERC721BridgeImpl"),
                optimismMintableERC20FactoryImpl: makeAddr("optimismMintableERC20FactoryImpl"),
                disputeGameFactoryImpl: makeAddr("disputeGameFactoryImpl"),
                mipsImpl: makeAddr("mipsImpl"),
                anchorStateRegistryImpl: makeAddr("anchorStateRegistryImpl"),
                delayedWETHImpl: makeAddr("delayedWETHImpl")
            }),
            superchainConfig,
            l1PAOMultisig,
            mips,
            challenger
        );

        // Setup mock contracts for validation
        proxyAdmin = IProxyAdmin(makeAddr("proxyAdmin"));
        systemConfig = ISystemConfig(makeAddr("systemConfig"));
        absolutePrestate = bytes32(uint256(0xdead));
        l2ChainID = 10;

        // Setup mock dependency addresses
        disputeGameFactory = makeAddr("disputeGameFactory");
        permissionedDisputeGame = makeAddr("permissionedDisputeGame");
        permissionlessDisputeGame = makeAddr("permissionlessDisputeGame");
        permissionedASR = makeAddr("anchorStateRegistry");
        permissionlessASR = makeAddr("permissionlessAnchorStateRegistry");
        optimismPortal = makeAddr("optimismPortal");
        l1CrossDomainMessenger = makeAddr("l1CrossDomainMessenger");
        l1StandardBridge = makeAddr("l1StandardBridge");
        l1ERC721Bridge = makeAddr("l1ERC721Bridge");
        optimismMintableERC20Factory = makeAddr("optimismMintableERC20Factory");
        permissionedDelayedWETH = makeAddr("delayedWETH");
        permissionlessDelayedWETH = makeAddr("permissionlessDelayedWETH");
        preimageOracle = makeAddr("preimageOracle");

        // Mock proxyAdmin owner
        vm.mockCall(address(proxyAdmin), abi.encodeCall(IProxyAdmin.owner, ()), abi.encode(l1PAOMultisig));

        preimageOracle = makeAddr("preimageOracle");
    }

    function test_validate_opMainnet_succeeds() public {
        string memory rpcUrl = vm.envOr(string("MAINNET_RPC_URL"), string(""));
        if (bytes(rpcUrl).length == 0) {
            return;
        }

        vm.createSelectFork(rpcUrl);

        StandardValidatorV180 mainnetValidator = new StandardValidatorV180(
            StandardValidatorV180.Implementations({
                systemConfigImpl: address(0xAB9d6cB7A427c0765163A7f45BB91cAfe5f2D375),
                optimismPortalImpl: address(0xe2F826324b2faf99E513D16D266c3F80aE87832B),
                l1CrossDomainMessengerImpl: address(0xD3494713A5cfaD3F5359379DfA074E2Ac8C6Fd65),
                l1StandardBridgeImpl: address(0x64B5a5Ed26DCb17370Ff4d33a8D503f0fbD06CfF),
                l1ERC721BridgeImpl: address(0xAE2AF01232a6c4a4d3012C5eC5b1b35059caF10d),
                optimismMintableERC20FactoryImpl: address(0xE01efbeb1089D1d1dB9c6c8b135C934C0734c846),
                disputeGameFactoryImpl: address(0xc641A33cab81C559F2bd4b21EA34C290E2440C2B),
                mipsImpl: address(0x5fE03a12C1236F9C22Cb6479778DDAa4bce6299C),
                anchorStateRegistryImpl: address(0x1B5CC028A4276597C607907F24E1AC05d3852cFC),
                delayedWETHImpl: address(0x71e966Ae981d1ce531a7b6d23DC0f27B38409087)
            }),
            ISuperchainConfig(address(0x95703e0982140D16f8ebA6d158FccEde42f04a4C)),
            address(0x5a0Aae59D09fccBdDb6C6CcEB07B7279367C3d2A), // l1PAOMultisig
            address(0x5fE03a12C1236F9C22Cb6479778DDAa4bce6299C), // mips
            address(0x9BA6e03D8B90dE867373Db8cF1A58d2F7F006b3A) // challenger
        );

        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: IProxyAdmin(address(0x543bA4AADBAb8f9025686Bd03993043599c6fB04)),
            sysCfg: ISystemConfig(address(0x229047fed2591dbec1eF1118d64F7aF3dB9EB290)),
            absolutePrestate: bytes32(0x03f89406817db1ed7fd8b31e13300444652cdb0b9c509a674de43483b2f83568),
            l2ChainID: 10
        });

        // OP Mainnet has a different expected root than the default one, so we expect to see ANCHORP-40.
        string memory errors = mainnetValidator.validate(input, true);
        assertEq(errors, "PDDG-ANCHORP-40,PLDG-ANCHORP-40");
    }

    /// @notice Tests that validation succeeds with valid inputs and mocked dependencies
    function test_validate_allowFailureTrue_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Mock all necessary calls for validation
        _mockValidationCalls();

        // Validate with allowFailure = true
        string memory errors = validator.validate(input, true);
        assertEq(errors, "");
    }

    /// @notice Tests that validation reverts with error message when allowFailure is false
    function test_validate_allowFailureFalse_reverts() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        _mockValidationCalls();

        // Mock null implementation for permissioned dispute game
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(IDisputeGameFactory.gameImpls, (GameTypes.PERMISSIONED_CANNON)),
            abi.encode(address(0))
        );

        // Expect revert with PDDG-10 error message
        vm.expectRevert("StandardValidatorV180: PDDG-10");
        validator.validate(input, false);
    }

    /// @notice Tests that validation fails with invalid proxy admin owner
    function test_validate_proxyAdmin_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        _mockValidationCalls();

        vm.mockCall(address(proxyAdmin), abi.encodeCall(IProxyAdmin.owner, ()), abi.encode(address(0xbad)));

        // Mocking the proxy admin like this will also break ownership checks
        // for the DGF, DelayedWETH, and other contracts.
        assertErrorCode(input, "PROXYA-10,DF-30");
    }

    /// @notice Tests validation of SystemConfig
    function test_validate_systemConfig_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Test invalid version
        _mockValidationCalls();
        vm.mockCall(address(systemConfig), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, "SYSCON-10");

        // Test invalid gas limit
        _mockValidationCalls();
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.gasLimit, ()), abi.encode(uint64(1_000_000)));
        assertErrorCode(input, "SYSCON-20");

        // Test invalid scalar
        _mockValidationCalls();
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.scalar, ()), abi.encode(uint256(2) << 248));
        assertErrorCode(input, "SYSCON-30");

        // Test invalid proxy implementation
        _mockValidationCalls();
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(systemConfig))),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "SYSCON-40");

        // Test invalid resource config - maxResourceLimit
        _mockValidationCalls();
        IResourceMetering.ResourceConfig memory badConfig = IResourceMetering.ResourceConfig({
            maxResourceLimit: 1_000_000,
            elasticityMultiplier: 10,
            baseFeeMaxChangeDenominator: 8,
            systemTxMaxGas: 1_000_000,
            minimumBaseFee: 1 gwei,
            maximumBaseFee: type(uint128).max
        });
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.resourceConfig, ()), abi.encode(badConfig));
        assertErrorCode(input, "SYSCON-50");

        // Test invalid resource config - elasticityMultiplier
        _mockValidationCalls();
        badConfig.maxResourceLimit = 20_000_000;
        badConfig.elasticityMultiplier = 5;
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.resourceConfig, ()), abi.encode(badConfig));
        assertErrorCode(input, "SYSCON-60");

        // Test invalid resource config - baseFeeMaxChangeDenominator
        _mockValidationCalls();
        badConfig.elasticityMultiplier = 10;
        badConfig.baseFeeMaxChangeDenominator = 4;
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.resourceConfig, ()), abi.encode(badConfig));
        assertErrorCode(input, "SYSCON-70");

        // Test invalid resource config - systemTxMaxGas
        _mockValidationCalls();
        badConfig.baseFeeMaxChangeDenominator = 8;
        badConfig.systemTxMaxGas = 500_000;
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.resourceConfig, ()), abi.encode(badConfig));
        assertErrorCode(input, "SYSCON-80");

        // Test invalid resource config - minimumBaseFee
        _mockValidationCalls();
        badConfig.systemTxMaxGas = 1_000_000;
        badConfig.minimumBaseFee = 2 gwei;
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.resourceConfig, ()), abi.encode(badConfig));
        assertErrorCode(input, "SYSCON-90");

        // Test invalid resource config - maximumBaseFee
        _mockValidationCalls();
        badConfig.minimumBaseFee = 1 gwei;
        badConfig.maximumBaseFee = 1_000_000;
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.resourceConfig, ()), abi.encode(badConfig));
        assertErrorCode(input, "SYSCON-100");
    }

    /// @notice Tests validation of L1CrossDomainMessenger
    function test_validate_l1CrossDomainMessenger_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Test invalid version
        _mockValidationCalls();
        vm.mockCall(address(l1CrossDomainMessenger), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, "L1xDM-10");

        // Test invalid OTHER_MESSENGER
        _mockValidationCalls();
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.OTHER_MESSENGER, ()),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1xDM-30");

        // Test invalid otherMessenger
        _mockValidationCalls();
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.otherMessenger, ()),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1xDM-40");

        // Test invalid PORTAL
        _mockValidationCalls();
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(IL1CrossDomainMessenger.PORTAL, ()),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1xDM-50");

        // Test invalid portal
        _mockValidationCalls();
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(IL1CrossDomainMessenger.portal, ()),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1xDM-60");

        // Test invalid superchainConfig
        _mockValidationCalls();
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(IL1CrossDomainMessenger.superchainConfig, ()),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1xDM-70");
    }

    /// @notice Tests validation of OptimismMintableERC20Factory
    function test_validate_optimismMintableERC20Factory_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Test invalid version
        _mockValidationCalls();
        vm.mockCall(address(optimismMintableERC20Factory), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, "MERC20F-10");

        // Test invalid BRIDGE
        _mockValidationCalls();
        vm.mockCall(
            address(optimismMintableERC20Factory),
            abi.encodeCall(IOptimismMintableERC20Factory.BRIDGE, ()),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "MERC20F-30");

        // Test invalid bridge
        _mockValidationCalls();
        vm.mockCall(
            address(optimismMintableERC20Factory),
            abi.encodeCall(IOptimismMintableERC20Factory.bridge, ()),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "MERC20F-40");
    }

    /// @notice Tests validation of L1ERC721Bridge
    function test_validate_l1ERC721Bridge_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Test invalid version
        _mockValidationCalls();
        vm.mockCall(address(l1ERC721Bridge), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, "L721B-10");

        // Test invalid OTHER_BRIDGE
        _mockValidationCalls();
        vm.mockCall(address(l1ERC721Bridge), abi.encodeCall(IERC721Bridge.OTHER_BRIDGE, ()), abi.encode(address(0xbad)));
        assertErrorCode(input, "L721B-30");

        // Test invalid otherBridge
        _mockValidationCalls();
        vm.mockCall(address(l1ERC721Bridge), abi.encodeCall(IERC721Bridge.otherBridge, ()), abi.encode(address(0xbad)));
        assertErrorCode(input, "L721B-40");

        // Test invalid MESSENGER
        _mockValidationCalls();
        vm.mockCall(address(l1ERC721Bridge), abi.encodeCall(IERC721Bridge.MESSENGER, ()), abi.encode(address(0xbad)));
        assertErrorCode(input, "L721B-50");

        // Test invalid messenger
        _mockValidationCalls();
        vm.mockCall(address(l1ERC721Bridge), abi.encodeCall(IERC721Bridge.messenger, ()), abi.encode(address(0xbad)));
        assertErrorCode(input, "L721B-60");

        // Test invalid superchainConfig
        _mockValidationCalls();
        vm.mockCall(
            address(l1ERC721Bridge), abi.encodeCall(IL1ERC721Bridge.superchainConfig, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L721B-70");
    }

    /// @notice Tests validation of OptimismPortal
    function test_validate_optimismPortal_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Test invalid version
        _mockValidationCalls();
        vm.mockCall(address(optimismPortal), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, "PORTAL-10");

        // Test invalid disputeGameFactory
        _mockValidationCalls();
        vm.mockCall(
            address(optimismPortal), abi.encodeCall(IOptimismPortal2.disputeGameFactory, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "PORTAL-30");

        // Test invalid systemConfig
        _mockValidationCalls();
        vm.mockCall(
            address(optimismPortal), abi.encodeCall(IOptimismPortal2.systemConfig, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "PORTAL-40");

        // Test invalid superchainConfig
        _mockValidationCalls();
        vm.mockCall(
            address(optimismPortal), abi.encodeCall(IOptimismPortal2.superchainConfig, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "PORTAL-50");

        // Test invalid guardian
        _mockValidationCalls();
        vm.mockCall(address(optimismPortal), abi.encodeCall(IOptimismPortal2.guardian, ()), abi.encode(address(0xbad)));
        assertErrorCode(input, "PORTAL-60");

        // Test invalid paused
        _mockValidationCalls();
        vm.mockCall(address(optimismPortal), abi.encodeCall(IOptimismPortal2.paused, ()), abi.encode(true));
        assertErrorCode(input, "PORTAL-70");

        // Test invalid l2Sender
        _mockValidationCalls();
        vm.mockCall(address(optimismPortal), abi.encodeCall(IOptimismPortal2.l2Sender, ()), abi.encode(address(0xbad)));
        assertErrorCode(input, "PORTAL-80");
    }

    /// @notice Tests validation of DisputeGameFactory
    function test_validate_disputeGameFactory_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Test invalid version
        _mockValidationCalls();
        vm.mockCall(address(disputeGameFactory), abi.encodeCall(ISemver.version, ()), abi.encode("0.9.0"));
        assertErrorCode(input, "DF-10");

        // Test invalid implementation
        _mockValidationCalls();
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(disputeGameFactory))),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "DF-20");

        // Test invalid owner
        _mockValidationCalls();
        vm.mockCall(
            address(disputeGameFactory), abi.encodeCall(IDisputeGameFactory.owner, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "DF-30");
    }

    /// @notice Tests validation of PermissionedDisputeGame. The ASR, PreimageOracle, and DelayedWETH are
    /// validated for each PDG and so are included here.
    function test_validate_permissionedDisputeGame_succeeds() public {
        _testDisputeGame(
            permissionedDisputeGame, permissionedASR, permissionedDelayedWETH, GameTypes.PERMISSIONED_CANNON
        );
    }

    function test_validate_permissionlessDisputeGame_succeeds() public {
        _testDisputeGame(permissionlessDisputeGame, permissionlessASR, permissionlessDelayedWETH, GameTypes.CANNON);
    }

    /// @notice Tests validation of L1StandardBridge
    function test_validate_l1StandardBridge_succeeds() public {
        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Test invalid version
        _mockValidationCalls();
        vm.mockCall(address(l1StandardBridge), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, "L1SB-10");

        // Test invalid MESSENGER
        _mockValidationCalls();
        vm.mockCall(
            address(l1StandardBridge), abi.encodeCall(IStandardBridge.MESSENGER, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1SB-30");

        // Test invalid messenger
        _mockValidationCalls();
        vm.mockCall(
            address(l1StandardBridge), abi.encodeCall(IStandardBridge.messenger, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1SB-40");

        // Test invalid OTHER_BRIDGE
        _mockValidationCalls();
        vm.mockCall(
            address(l1StandardBridge), abi.encodeCall(IStandardBridge.OTHER_BRIDGE, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1SB-50");

        // Test invalid otherBridge
        _mockValidationCalls();
        vm.mockCall(
            address(l1StandardBridge), abi.encodeCall(IStandardBridge.otherBridge, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1SB-60");

        // Test invalid superchainConfig
        _mockValidationCalls();
        vm.mockCall(
            address(l1StandardBridge),
            abi.encodeCall(IL1StandardBridge.superchainConfig, ()),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, "L1SB-70");
    }

    /// @notice Helper function to assert error codes in validation
    function assertErrorCode(StandardValidatorV180.Input memory _input, string memory _errCode) internal view {
        string memory errCodes = validator.validate(_input, true);
        assertEq(errCodes, _errCode);
    }

    /// @notice Helper function to mock all necessary calls for successful validation
    function _mockValidationCalls() internal {
        // Mock SystemConfig dependencies
        vm.mockCall(
            address(systemConfig), abi.encodeCall(ISystemConfig.disputeGameFactory, ()), abi.encode(disputeGameFactory)
        );
        vm.mockCall(address(systemConfig), abi.encodeCall(ISemver.version, ()), abi.encode("2.3.0"));
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.gasLimit, ()), abi.encode(uint64(60_000_000)));
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.scalar, ()), abi.encode(uint256(1) << 248));
        vm.mockCall(
            address(systemConfig),
            abi.encodeCall(ISystemConfig.l1CrossDomainMessenger, ()),
            abi.encode(l1CrossDomainMessenger)
        );
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.optimismPortal, ()), abi.encode(optimismPortal));
        vm.mockCall(
            address(systemConfig), abi.encodeCall(ISystemConfig.l1StandardBridge, ()), abi.encode(l1StandardBridge)
        );
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.l1ERC721Bridge, ()), abi.encode(l1ERC721Bridge));
        vm.mockCall(
            address(systemConfig),
            abi.encodeCall(ISystemConfig.optimismMintableERC20Factory, ()),
            abi.encode(optimismMintableERC20Factory)
        );

        // Mock proxy implementations
        StandardValidatorV180.Implementations memory impls = validator.implementations();
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(systemConfig))),
            abi.encode(impls.systemConfigImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(optimismPortal))),
            abi.encode(impls.optimismPortalImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(l1CrossDomainMessenger))),
            abi.encode(impls.l1CrossDomainMessengerImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(l1StandardBridge))),
            abi.encode(impls.l1StandardBridgeImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(l1ERC721Bridge))),
            abi.encode(impls.l1ERC721BridgeImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(optimismMintableERC20Factory))),
            abi.encode(impls.optimismMintableERC20FactoryImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(disputeGameFactory))),
            abi.encode(impls.disputeGameFactoryImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(mips))),
            abi.encode(impls.mipsImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(permissionedASR))),
            abi.encode(impls.anchorStateRegistryImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(permissionedDelayedWETH))),
            abi.encode(impls.delayedWETHImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(permissionlessDelayedWETH))),
            abi.encode(impls.delayedWETHImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(permissionedASR))),
            abi.encode(impls.anchorStateRegistryImpl)
        );
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(permissionlessASR))),
            abi.encode(impls.anchorStateRegistryImpl)
        );

        // Mock AnchorStateRegistry
        _mockAnchorStateRegistry(
            permissionedASR, disputeGameFactory, address(superchainConfig), GameTypes.PERMISSIONED_CANNON
        );
        _mockAnchorStateRegistry(permissionlessASR, disputeGameFactory, address(superchainConfig), GameTypes.CANNON);

        // Mock resource config
        IResourceMetering.ResourceConfig memory config = IResourceMetering.ResourceConfig({
            maxResourceLimit: 20_000_000,
            elasticityMultiplier: 10,
            baseFeeMaxChangeDenominator: 8,
            systemTxMaxGas: 1_000_000,
            minimumBaseFee: 1e9,
            maximumBaseFee: type(uint128).max
        });
        vm.mockCall(address(systemConfig), abi.encodeCall(ISystemConfig.resourceConfig, ()), abi.encode(config));

        // Mock DisputeGameFactory
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(IDisputeGameFactory.gameImpls, (GameTypes.PERMISSIONED_CANNON)),
            abi.encode(permissionedDisputeGame)
        );
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(IDisputeGameFactory.gameImpls, (GameTypes.CANNON)),
            abi.encode(permissionlessDisputeGame)
        );
        vm.mockCall(address(disputeGameFactory), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        vm.mockCall(
            address(disputeGameFactory), abi.encodeCall(IDisputeGameFactory.owner, ()), abi.encode(l1PAOMultisig)
        );

        _mockDisputeGame(
            permissionlessDisputeGame, permissionlessASR, permissionlessDelayedWETH, absolutePrestate, GameTypes.CANNON
        );
        _mockDisputeGame(
            permissionedDisputeGame,
            permissionedASR,
            permissionedDelayedWETH,
            absolutePrestate,
            GameTypes.PERMISSIONED_CANNON
        );
        vm.mockCall(
            address(permissionedDisputeGame),
            abi.encodeCall(IPermissionedDisputeGame.challenger, ()),
            abi.encode(challenger)
        );

        // Mock MIPS
        vm.mockCall(address(mips), abi.encodeCall(IMIPS.oracle, ()), abi.encode(preimageOracle));

        // Mock PreimageOracle
        vm.mockCall(address(preimageOracle), abi.encodeCall(ISemver.version, ()), abi.encode("1.1.2"));
        vm.mockCall(address(preimageOracle), abi.encodeCall(IPreimageOracle.challengePeriod, ()), abi.encode(86400));
        vm.mockCall(address(preimageOracle), abi.encodeCall(IPreimageOracle.minProposalSize, ()), abi.encode(126000));

        // Mock L1CrossDomainMessenger
        vm.mockCall(address(l1CrossDomainMessenger), abi.encodeCall(ISemver.version, ()), abi.encode("2.3.0"));
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.OTHER_MESSENGER, ()),
            abi.encode(ICrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER))
        );
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.otherMessenger, ()),
            abi.encode(ICrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER))
        );
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(IL1CrossDomainMessenger.PORTAL, ()),
            abi.encode(optimismPortal)
        );
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(IL1CrossDomainMessenger.portal, ()),
            abi.encode(optimismPortal)
        );
        vm.mockCall(
            address(l1CrossDomainMessenger),
            abi.encodeCall(IL1CrossDomainMessenger.superchainConfig, ()),
            abi.encode(superchainConfig)
        );

        // Mock OptimismPortal
        vm.mockCall(address(optimismPortal), abi.encodeCall(ISemver.version, ()), abi.encode("3.10.0"));
        vm.mockCall(
            address(optimismPortal),
            abi.encodeCall(IOptimismPortal2.disputeGameFactory, ()),
            abi.encode(disputeGameFactory)
        );
        vm.mockCall(
            address(optimismPortal), abi.encodeCall(IOptimismPortal2.systemConfig, ()), abi.encode(systemConfig)
        );
        vm.mockCall(
            address(optimismPortal), abi.encodeCall(IOptimismPortal2.superchainConfig, ()), abi.encode(superchainConfig)
        );
        vm.mockCall(
            address(optimismPortal),
            abi.encodeCall(IOptimismPortal2.guardian, ()),
            abi.encode(superchainConfig.guardian())
        );
        vm.mockCall(
            address(optimismPortal), abi.encodeCall(IOptimismPortal2.paused, ()), abi.encode(superchainConfig.paused())
        );
        vm.mockCall(
            address(optimismPortal),
            abi.encodeCall(IOptimismPortal2.l2Sender, ()),
            abi.encode(address(0x000000000000000000000000000000000000dEaD))
        );

        // Mock SuperchainConfig
        vm.mockCall(
            address(superchainConfig), abi.encodeCall(ISuperchainConfig.guardian, ()), abi.encode(makeAddr("guardian"))
        );
        vm.mockCall(address(superchainConfig), abi.encodeCall(ISuperchainConfig.paused, ()), abi.encode(false));

        // Mock L1StandardBridge
        vm.mockCall(address(l1StandardBridge), abi.encodeCall(ISemver.version, ()), abi.encode("2.1.0"));
        vm.mockCall(
            address(l1StandardBridge), abi.encodeCall(IStandardBridge.MESSENGER, ()), abi.encode(l1CrossDomainMessenger)
        );
        vm.mockCall(
            address(l1StandardBridge), abi.encodeCall(IStandardBridge.messenger, ()), abi.encode(l1CrossDomainMessenger)
        );
        vm.mockCall(
            address(l1StandardBridge),
            abi.encodeCall(IStandardBridge.OTHER_BRIDGE, ()),
            abi.encode(Predeploys.L2_STANDARD_BRIDGE)
        );
        vm.mockCall(
            address(l1StandardBridge),
            abi.encodeCall(IStandardBridge.otherBridge, ()),
            abi.encode(Predeploys.L2_STANDARD_BRIDGE)
        );
        vm.mockCall(
            address(l1StandardBridge),
            abi.encodeCall(IL1StandardBridge.superchainConfig, ()),
            abi.encode(superchainConfig)
        );

        // Mock L1ERC721Bridge
        vm.mockCall(address(l1ERC721Bridge), abi.encodeCall(ISemver.version, ()), abi.encode("2.1.0"));
        vm.mockCall(
            address(l1ERC721Bridge),
            abi.encodeCall(IERC721Bridge.OTHER_BRIDGE, ()),
            abi.encode(Predeploys.L2_ERC721_BRIDGE)
        );
        vm.mockCall(
            address(l1ERC721Bridge),
            abi.encodeCall(IERC721Bridge.otherBridge, ()),
            abi.encode(Predeploys.L2_ERC721_BRIDGE)
        );
        vm.mockCall(
            address(l1ERC721Bridge), abi.encodeCall(IERC721Bridge.MESSENGER, ()), abi.encode(l1CrossDomainMessenger)
        );
        vm.mockCall(
            address(l1ERC721Bridge), abi.encodeCall(IERC721Bridge.messenger, ()), abi.encode(l1CrossDomainMessenger)
        );
        vm.mockCall(
            address(l1ERC721Bridge), abi.encodeCall(IL1ERC721Bridge.superchainConfig, ()), abi.encode(superchainConfig)
        );

        // Mock OptimismMintableERC20Factory
        vm.mockCall(address(optimismMintableERC20Factory), abi.encodeCall(ISemver.version, ()), abi.encode("1.9.0"));
        vm.mockCall(
            address(optimismMintableERC20Factory),
            abi.encodeCall(IOptimismMintableERC20Factory.BRIDGE, ()),
            abi.encode(l1StandardBridge)
        );
        vm.mockCall(
            address(optimismMintableERC20Factory),
            abi.encodeCall(IOptimismMintableERC20Factory.bridge, ()),
            abi.encode(l1StandardBridge)
        );

        _mockDelayedWETH(permissionedDelayedWETH);
        _mockDelayedWETH(permissionlessDelayedWETH);
    }

    function _mockAnchorStateRegistry(
        address _asr,
        address _disputeGameFactory,
        address _superchainConfig,
        GameType _gameType
    )
        internal
    {
        vm.mockCall(address(_asr), abi.encodeCall(ISemver.version, ()), abi.encode("2.0.0"));
        vm.mockCall(
            address(_asr), abi.encodeCall(IAnchorStateRegistry.disputeGameFactory, ()), abi.encode(_disputeGameFactory)
        );
        vm.mockCall(
            address(_asr),
            abi.encodeCall(IAnchorStateRegistry.anchors, (_gameType)),
            abi.encode(Hash.wrap(0xdead000000000000000000000000000000000000000000000000000000000000), 0)
        );
        vm.mockCall(
            address(_asr), abi.encodeCall(IAnchorStateRegistry.superchainConfig, ()), abi.encode(_superchainConfig)
        );
    }

    function _mockDisputeGame(
        address _disputeGame,
        address _asr,
        address _weth,
        bytes32 _absolutePrestate,
        GameType _gameType
    )
        internal
    {
        vm.mockCall(address(_disputeGame), abi.encodeCall(ISemver.version, ()), abi.encode("1.3.1"));
        vm.mockCall(address(_disputeGame), abi.encodeCall(IDisputeGame.gameType, ()), abi.encode(_gameType));
        vm.mockCall(
            address(_disputeGame),
            abi.encodeCall(IPermissionedDisputeGame.absolutePrestate, ()),
            abi.encode(_absolutePrestate)
        );
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.vm, ()), abi.encode(mips));
        vm.mockCall(
            address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.anchorStateRegistry, ()), abi.encode(_asr)
        );
        vm.mockCall(
            address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.l2ChainId, ()), abi.encode(l2ChainID)
        );
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.l2BlockNumber, ()), abi.encode(0));
        vm.mockCall(
            address(_disputeGame),
            abi.encodeCall(IPermissionedDisputeGame.clockExtension, ()),
            abi.encode(Duration.wrap(10800))
        );
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.splitDepth, ()), abi.encode(30));
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.maxGameDepth, ()), abi.encode(73));
        vm.mockCall(
            address(_disputeGame),
            abi.encodeCall(IPermissionedDisputeGame.maxClockDuration, ()),
            abi.encode(Duration.wrap(302400))
        );
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.weth, ()), abi.encode(_weth));
    }

    function _mockDelayedWETH(address _weth) public {
        vm.mockCall(address(_weth), abi.encodeCall(ISemver.version, ()), abi.encode("1.1.0"));
        vm.mockCall(address(_weth), abi.encodeCall(IDelayedWETH.owner, ()), abi.encode(challenger));
        vm.mockCall(address(_weth), abi.encodeCall(IDelayedWETH.delay, ()), abi.encode(1 weeks));
    }

    function _testDisputeGame(address _disputeGame, address _asr, address _weth, GameType _gameType) public {
        string memory errorPrefix;
        if (_gameType.raw() == GameTypes.PERMISSIONED_CANNON.raw()) {
            errorPrefix = string.concat(errorPrefix, "PDDG");
        } else {
            errorPrefix = string.concat(errorPrefix, "PLDG");
        }

        StandardValidatorV180.Input memory input = StandardValidatorV180.Input({
            proxyAdmin: proxyAdmin,
            sysCfg: systemConfig,
            absolutePrestate: absolutePrestate,
            l2ChainID: l2ChainID
        });

        // Test null implementation
        _mockValidationCalls();
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(IDisputeGameFactory.gameImpls, (_gameType)),
            abi.encode(address(0))
        );
        assertErrorCode(input, string.concat(errorPrefix, "-10"));

        // Test invalid version
        _mockValidationCalls();
        vm.mockCall(address(_disputeGame), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, string.concat(errorPrefix, "-20"));

        // Test invalid game type
        _mockValidationCalls();
        vm.mockCall(address(_disputeGame), abi.encodeCall(IDisputeGame.gameType, ()), abi.encode(GameType.wrap(123)));
        assertErrorCode(input, string.concat(errorPrefix, "-30"));

        // Test invalid absolute prestate
        _mockValidationCalls();
        vm.mockCall(
            address(_disputeGame),
            abi.encodeCall(IPermissionedDisputeGame.absolutePrestate, ()),
            abi.encode(bytes32(uint256(0xbad)))
        );
        assertErrorCode(input, string.concat(errorPrefix, "-40"));

        // Test invalid vm
        _mockValidationCalls();
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.vm, ()), abi.encode(address(0xbad)));
        assertErrorCode(input, string.concat(errorPrefix, "-50"));

        // Test invalid l2ChainId
        _mockValidationCalls();
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.l2ChainId, ()), abi.encode(123));
        assertErrorCode(input, string.concat(errorPrefix, "-60"));

        // Test invalid l2BlockNumber
        _mockValidationCalls();
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.l2BlockNumber, ()), abi.encode(1));
        assertErrorCode(input, string.concat(errorPrefix, "-70"));

        // Test invalid clockExtension
        _mockValidationCalls();
        vm.mockCall(
            address(_disputeGame),
            abi.encodeCall(IPermissionedDisputeGame.clockExtension, ()),
            abi.encode(Duration.wrap(1000))
        );
        assertErrorCode(input, string.concat(errorPrefix, "-80"));

        // Test invalid splitDepth
        _mockValidationCalls();
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.splitDepth, ()), abi.encode(20));
        assertErrorCode(input, string.concat(errorPrefix, "-90"));

        // Test invalid maxGameDepth
        _mockValidationCalls();
        vm.mockCall(address(_disputeGame), abi.encodeCall(IPermissionedDisputeGame.maxGameDepth, ()), abi.encode(50));
        assertErrorCode(input, string.concat(errorPrefix, "-100"));

        // Test invalid maxClockDuration
        _mockValidationCalls();
        vm.mockCall(
            address(_disputeGame),
            abi.encodeCall(IPermissionedDisputeGame.maxClockDuration, ()),
            abi.encode(Duration.wrap(1000))
        );
        assertErrorCode(input, string.concat(errorPrefix, "-110"));

        if (_gameType.raw() == GameTypes.PERMISSIONED_CANNON.raw()) {
            _mockValidationCalls();
            vm.mockCall(
                address(_disputeGame),
                abi.encodeCall(IPermissionedDisputeGame.challenger, ()),
                abi.encode(address(0xbad))
            );
            assertErrorCode(input, string.concat(errorPrefix, "-120"));
        }

        // Test invalid anchor state registry version
        _mockValidationCalls();
        vm.mockCall(address(_asr), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, string.concat(errorPrefix, "-ANCHORP-10"));

        // Test invalid anchor state registry implementation
        _mockValidationCalls();
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(_asr))),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, string.concat(errorPrefix, "-ANCHORP-20"));

        // Test invalid anchor state registry factory
        _mockValidationCalls();
        vm.mockCall(
            address(_asr), abi.encodeCall(IAnchorStateRegistry.disputeGameFactory, ()), abi.encode(address(0xbad))
        );
        assertErrorCode(input, string.concat(errorPrefix, "-ANCHORP-30"));

        // Test invalid anchor state registry root
        _mockValidationCalls();
        vm.mockCall(
            address(_asr),
            abi.encodeCall(IAnchorStateRegistry.anchors, (_gameType)),
            abi.encode(Hash.wrap(bytes32(uint256(0xbad))), 0)
        );
        assertErrorCode(input, string.concat(errorPrefix, "-ANCHORP-40"));

        // Test invalid DelayedWETH version
        _mockValidationCalls();
        vm.mockCall(address(_weth), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, string.concat(errorPrefix, "-DWETH-10"));

        // Test invalid DelayedWETH implementation for permissioned game
        _mockValidationCalls();
        vm.mockCall(
            address(proxyAdmin),
            abi.encodeCall(IProxyAdmin.getProxyImplementation, (address(_weth))),
            abi.encode(address(0xbad))
        );
        assertErrorCode(input, string.concat(errorPrefix, "-DWETH-20"));

        // Test invalid DelayedWETH owner
        _mockValidationCalls();
        vm.mockCall(address(_weth), abi.encodeCall(IDelayedWETH.owner, ()), abi.encode(address(0xbad)));
        assertErrorCode(input, string.concat(errorPrefix, "-DWETH-30"));

        // Test invalid DelayedWETH delay
        _mockValidationCalls();
        vm.mockCall(address(_weth), abi.encodeCall(IDelayedWETH.delay, ()), abi.encode(2));
        assertErrorCode(input, string.concat(errorPrefix, "-DWETH-40"));

        // Since the preimage oracle is shared, the errors need to include both
        // the permissioned and permissionless game type.

        // Test invalid PreimageOracle version
        _mockValidationCalls();
        vm.mockCall(address(preimageOracle), abi.encodeCall(ISemver.version, ()), abi.encode("1.0.0"));
        assertErrorCode(input, "PDDG-PIMGO-10,PLDG-PIMGO-10");

        // Test invalid PreimageOracle challenge period
        _mockValidationCalls();
        vm.mockCall(address(preimageOracle), abi.encodeCall(IPreimageOracle.challengePeriod, ()), abi.encode(1000));
        assertErrorCode(input, "PDDG-PIMGO-20,PLDG-PIMGO-20");

        // Test invalid PreimageOracle min proposal size for permissioned game
        _mockValidationCalls();
        vm.mockCall(address(preimageOracle), abi.encodeCall(IPreimageOracle.minProposalSize, ()), abi.encode(1000));
        assertErrorCode(input, "PDDG-PIMGO-30,PLDG-PIMGO-30");
    }
}
