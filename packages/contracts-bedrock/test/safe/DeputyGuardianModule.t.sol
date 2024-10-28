// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";
import { ForgeArtifacts, Abi } from "scripts/libraries/ForgeArtifacts.sol";
import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import "test/safe-tools/SafeTestTools.sol";

// Contracts
import { IDeputyGuardianModule } from "src/safe/interfaces/IDeputyGuardianModule.sol";

// Libraries
import "src/dispute/lib/Types.sol";

// Interfaces
import { IDisputeGame } from "src/dispute/interfaces/IDisputeGame.sol";
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract DeputyGuardianModule_TestInit is CommonTest, SafeTestTools {
    using SafeTestLib for SafeInstance;

    error Unauthorized();
    error ExecutionFailed(string);

    event ExecutionFromModuleSuccess(address indexed);

    IDeputyGuardianModule deputyGuardianModule;
    SafeInstance safeInstance;
    address deputyGuardian;

    /// @dev Sets up the test environment
    function setUp() public virtual override {
        super.setUp();

        // Create a Safe with 10 owners
        (, uint256[] memory keys) = SafeTestLib.makeAddrsAndKeys("moduleTest", 10);
        safeInstance = _setupSafe(keys, 10);

        // Set the Safe as the Guardian of the SuperchainConfig
        vm.store(
            address(superchainConfig),
            superchainConfig.GUARDIAN_SLOT(),
            bytes32(uint256(uint160(address(safeInstance.safe))))
        );

        deputyGuardian = makeAddr("deputyGuardian");

        deputyGuardianModule = IDeputyGuardianModule(
            DeployUtils.create1({
                _name: "DeputyGuardianModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyGuardianModule.__constructor__, (safeInstance.safe, superchainConfig, deputyGuardian)
                    )
                )
            })
        );
        safeInstance.enableModule(address(deputyGuardianModule));
    }
}

contract DeputyGuardianModule_Getters_Test is DeputyGuardianModule_TestInit {
    /// @dev Tests that the constructor sets the correct values
    function test_getters_works() external view {
        assertEq(address(deputyGuardianModule.safe()), address(safeInstance.safe));
        assertEq(address(deputyGuardianModule.deputyGuardian()), address(deputyGuardian));
        assertEq(address(deputyGuardianModule.superchainConfig()), address(superchainConfig));
    }
}

contract DeputyGuardianModule_Pause_Test is DeputyGuardianModule_TestInit {
    /// @dev Tests that `pause` successfully pauses when called by the deputy guardian.
    function test_pause_succeeds() external {
        vm.expectEmit(address(superchainConfig));
        emit Paused("Deputy Guardian");

        vm.expectEmit(address(safeInstance.safe));
        emit ExecutionFromModuleSuccess(address(deputyGuardianModule));

        vm.expectEmit(address(deputyGuardianModule));
        emit Paused("Deputy Guardian");

        vm.prank(address(deputyGuardian));
        deputyGuardianModule.pause();
        assertEq(superchainConfig.paused(), true);
    }
}

contract DeputyGuardianModule_Pause_TestFail is DeputyGuardianModule_TestInit {
    /// @dev Tests that `pause` reverts when called by a non deputy guardian.
    function test_pause_notDeputyGuardian_reverts() external {
        vm.expectRevert(abi.encodeWithSelector(Unauthorized.selector));
        deputyGuardianModule.pause();
    }

    /// @dev Tests that when the call from the Safe reverts, the error message is returned.
    function test_pause_targetReverts_reverts() external {
        vm.mockCallRevert(
            address(superchainConfig),
            abi.encodePacked(superchainConfig.pause.selector),
            "SuperchainConfig: pause() reverted"
        );

        vm.prank(address(deputyGuardian));
        vm.expectRevert(abi.encodeWithSelector(ExecutionFailed.selector, "SuperchainConfig: pause() reverted"));
        deputyGuardianModule.pause();
    }
}

contract DeputyGuardianModule_Unpause_Test is DeputyGuardianModule_TestInit {
    /// @dev Sets up the test environment with the SuperchainConfig paused
    function setUp() public override {
        super.setUp();
        vm.prank(address(deputyGuardian));
        deputyGuardianModule.pause();
        assertTrue(superchainConfig.paused());
    }

    /// @dev Tests that `unpause` successfully unpauses when called by the deputy guardian.
    function test_unpause_succeeds() external {
        vm.expectEmit(address(superchainConfig));
        emit Unpaused();

        vm.expectEmit(address(safeInstance.safe));
        emit ExecutionFromModuleSuccess(address(deputyGuardianModule));

        vm.expectEmit(address(deputyGuardianModule));
        emit Unpaused();

        vm.prank(address(deputyGuardian));
        deputyGuardianModule.unpause();
        assertFalse(superchainConfig.paused());
    }
}

/// @dev Note that this contract inherits from DeputyGuardianModule_Unpause_Test to ensure that the SuperchainConfig is
///      paused before the tests are run.
contract DeputyGuardianModule_Unpause_TestFail is DeputyGuardianModule_Unpause_Test {
    /// @dev Tests that `unpause` reverts when called by a non deputy guardian.
    function test_unpause_notDeputyGuardian_reverts() external {
        vm.expectRevert(abi.encodeWithSelector(Unauthorized.selector));
        deputyGuardianModule.unpause();
        assertTrue(superchainConfig.paused());
    }

    /// @dev Tests that when the call from the Safe reverts, the error message is returned.
    function test_unpause_targetReverts_reverts() external {
        vm.mockCallRevert(
            address(superchainConfig),
            abi.encodePacked(superchainConfig.unpause.selector),
            "SuperchainConfig: unpause reverted"
        );

        vm.prank(address(deputyGuardian));
        vm.expectRevert(abi.encodeWithSelector(ExecutionFailed.selector, "SuperchainConfig: unpause reverted"));
        deputyGuardianModule.unpause();
    }
}

contract DeputyGuardianModule_SetAnchorState_TestFail is DeputyGuardianModule_TestInit {
    function test_setAnchorState_notDeputyGuardian_reverts() external {
        IAnchorStateRegistry asr = IAnchorStateRegistry(makeAddr("asr"));
        vm.expectRevert(abi.encodeWithSelector(Unauthorized.selector));
        deputyGuardianModule.setAnchorState(asr, IFaultDisputeGame(address(0)));
    }

    function test_setAnchorState_targetReverts_reverts() external {
        IAnchorStateRegistry asr = IAnchorStateRegistry(makeAddr("asr"));
        vm.mockCallRevert(
            address(asr), abi.encodePacked(asr.setAnchorState.selector), "AnchorStateRegistry: setAnchorState reverted"
        );
        vm.prank(address(deputyGuardian));
        vm.expectRevert(
            abi.encodeWithSelector(ExecutionFailed.selector, "AnchorStateRegistry: setAnchorState reverted")
        );
        deputyGuardianModule.setAnchorState(asr, IFaultDisputeGame(address(0)));
    }
}

contract DeputyGuardianModule_SetAnchorState_Test is DeputyGuardianModule_TestInit {
    function test_setAnchorState_succeeds() external {
        IAnchorStateRegistry asr = IAnchorStateRegistry(makeAddr("asr"));
        vm.mockCall(
            address(asr), abi.encodeCall(IAnchorStateRegistry.setAnchorState, (IFaultDisputeGame(address(0)))), ""
        );
        vm.expectEmit(address(safeInstance.safe));
        emit ExecutionFromModuleSuccess(address(deputyGuardianModule));
        vm.prank(address(deputyGuardian));
        deputyGuardianModule.setAnchorState(asr, IFaultDisputeGame(address(0)));
    }
}

contract DeputyGuardianModule_BlacklistDisputeGame_Test is DeputyGuardianModule_TestInit {
    /// @dev Tests that `blacklistDisputeGame` successfully blacklists a dispute game when called by the deputy
    /// guardian.
    function test_blacklistDisputeGame_succeeds() external {
        IDisputeGame game = IDisputeGame(makeAddr("game"));

        vm.expectEmit(address(safeInstance.safe));
        emit ExecutionFromModuleSuccess(address(deputyGuardianModule));

        vm.expectEmit(address(deputyGuardianModule));
        emit DisputeGameBlacklisted(game);

        vm.prank(address(deputyGuardian));
        deputyGuardianModule.blacklistDisputeGame(optimismPortal2, game);
        assertTrue(optimismPortal2.disputeGameBlacklist(game));
    }
}

contract DeputyGuardianModule_BlacklistDisputeGame_TestFail is DeputyGuardianModule_TestInit {
    /// @dev Tests that `blacklistDisputeGame` reverts when called by a non deputy guardian.
    function test_blacklistDisputeGame_notDeputyGuardian_reverts() external {
        IDisputeGame game = IDisputeGame(makeAddr("game"));
        vm.expectRevert(abi.encodeWithSelector(Unauthorized.selector));
        deputyGuardianModule.blacklistDisputeGame(optimismPortal2, game);
        assertFalse(optimismPortal2.disputeGameBlacklist(game));
    }

    /// @dev Tests that when the call from the Safe reverts, the error message is returned.
    function test_blacklistDisputeGame_targetReverts_reverts() external {
        vm.mockCallRevert(
            address(optimismPortal2),
            abi.encodePacked(optimismPortal2.blacklistDisputeGame.selector),
            "OptimismPortal2: blacklistDisputeGame reverted"
        );

        IDisputeGame game = IDisputeGame(makeAddr("game"));
        vm.prank(address(deputyGuardian));
        vm.expectRevert(
            abi.encodeWithSelector(ExecutionFailed.selector, "OptimismPortal2: blacklistDisputeGame reverted")
        );
        deputyGuardianModule.blacklistDisputeGame(optimismPortal2, game);
    }
}

contract DeputyGuardianModule_setRespectedGameType_Test is DeputyGuardianModule_TestInit {
    /// @dev Tests that `setRespectedGameType` successfully updates the respected game type when called by the deputy
    /// guardian.
    function testFuzz_setRespectedGameType_succeeds(GameType _gameType) external {
        vm.expectEmit(address(safeInstance.safe));
        emit ExecutionFromModuleSuccess(address(deputyGuardianModule));

        vm.expectEmit(address(deputyGuardianModule));
        emit RespectedGameTypeSet(_gameType, Timestamp.wrap(uint64(block.timestamp)));

        vm.prank(address(deputyGuardian));
        deputyGuardianModule.setRespectedGameType(optimismPortal2, _gameType);
        assertEq(GameType.unwrap(optimismPortal2.respectedGameType()), GameType.unwrap(_gameType));
        assertEq(optimismPortal2.respectedGameTypeUpdatedAt(), uint64(block.timestamp));
    }
}

contract DeputyGuardianModule_setRespectedGameType_TestFail is DeputyGuardianModule_TestInit {
    /// @dev Tests that `setRespectedGameType` when called by a non deputy guardian.
    function testFuzz_setRespectedGameType_notDeputyGuardian_reverts(GameType _gameType) external {
        vm.assume(GameType.unwrap(optimismPortal2.respectedGameType()) != GameType.unwrap(_gameType));
        vm.expectRevert(abi.encodeWithSelector(Unauthorized.selector));
        deputyGuardianModule.setRespectedGameType(optimismPortal2, _gameType);
        assertNotEq(GameType.unwrap(optimismPortal2.respectedGameType()), GameType.unwrap(_gameType));
    }

    /// @dev Tests that when the call from the Safe reverts, the error message is returned.
    function test_setRespectedGameType_targetReverts_reverts() external {
        vm.mockCallRevert(
            address(optimismPortal2),
            abi.encodePacked(optimismPortal2.setRespectedGameType.selector),
            "OptimismPortal2: setRespectedGameType reverted"
        );

        GameType gameType = GameType.wrap(1);
        vm.prank(address(deputyGuardian));
        vm.expectRevert(
            abi.encodeWithSelector(ExecutionFailed.selector, "OptimismPortal2: setRespectedGameType reverted")
        );
        deputyGuardianModule.setRespectedGameType(optimismPortal2, gameType);
    }
}

contract DeputyGuardianModule_NoPortalCollisions_Test is DeputyGuardianModule_TestInit {
    /// @dev tests that no function selectors in the L1 contracts collide with the OptimismPortal2 functions called by
    ///      the DeputyGuardianModule.
    function test_noPortalCollisions_succeeds() external {
        string[] memory excludes = new string[](5);
        excludes[0] = "src/dispute/lib/*";
        excludes[1] = "src/L1/OptimismPortal2.sol";
        excludes[2] = "src/L1/OptimismPortalInterop.sol";
        excludes[3] = "src/L1/interfaces/IOptimismPortal2.sol";
        excludes[4] = "src/L1/interfaces/IOptimismPortalInterop.sol";
        Abi[] memory abis = ForgeArtifacts.getContractFunctionAbis("src/{L1,dispute,universal}", excludes);
        for (uint256 i; i < abis.length; i++) {
            for (uint256 j; j < abis[i].entries.length; j++) {
                bytes4 sel = abis[i].entries[j].sel;
                assertNotEq(sel, optimismPortal2.blacklistDisputeGame.selector);
                assertNotEq(sel, optimismPortal2.setRespectedGameType.selector);
            }
        }
    }
}
