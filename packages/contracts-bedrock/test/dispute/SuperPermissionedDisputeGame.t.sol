// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Testing
import { DisputeGameFactory_Init } from "test/dispute/DisputeGameFactory.t.sol";
import { AlphabetVM } from "test/mocks/AlphabetVM.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import "src/dispute/lib/Types.sol";
import "src/dispute/lib/Errors.sol";

// Interfaces
import { IPreimageOracle } from "interfaces/dispute/IBigStepper.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";

contract SuperPermissionedDisputeGame_Init is DisputeGameFactory_Init {
    /// @dev The type of the game being tested.
    GameType internal constant GAME_TYPE = GameType.wrap(1);
    /// @dev Mock proposer key
    address internal constant PROPOSER = address(0xfacade9);
    /// @dev Mock challenger key
    address internal constant CHALLENGER = address(0xfacadec);

    /// @dev The implementation of the game.
    IPermissionedDisputeGame internal gameImpl;
    /// @dev The `Clone` proxy of the game.
    IPermissionedDisputeGame internal gameProxy;

    /// @dev The extra data passed to the game for initialization.
    bytes internal extraData;

    event Move(uint256 indexed parentIndex, Claim indexed pivot, address indexed claimant);

    function init(Claim rootClaim, Claim absolutePrestate, uint256 l2BlockNumber) public {
        if (isForkTest()) {
            // Fund the proposer on this fork.
            vm.deal(PROPOSER, 100 ether);
        } else {
            // Set the time to a realistic date.
            vm.warp(1690906994);
        }

        // Set the extra data for the game creation
        extraData = abi.encode(l2BlockNumber);

        IPreimageOracle oracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
            })
        );
        AlphabetVM _vm = new AlphabetVM(absolutePrestate, oracle);

        // Use a 7 day delayed WETH to simulate withdrawals.
        IDelayedWETH _weth = IDelayedWETH(
            DeployUtils.create1({
                _name: "DelayedWETH",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IDelayedWETH.__constructor__, (7 days)))
            })
        );

        // Deploy an implementation of the fault game
        gameImpl = IPermissionedDisputeGame(
            DeployUtils.create1({
                _name: "SuperPermissionedDisputeGame",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IPermissionedDisputeGame.__constructor__,
                        (
                            IFaultDisputeGame.GameConstructorParams({
                                gameType: GAME_TYPE,
                                absolutePrestate: absolutePrestate,
                                maxGameDepth: 2 ** 3,
                                splitDepth: 2 ** 2,
                                clockExtension: Duration.wrap(3 hours),
                                maxClockDuration: Duration.wrap(3.5 days),
                                vm: _vm,
                                weth: _weth,
                                anchorStateRegistry: anchorStateRegistry,
                                l2ChainId: 10
                            }),
                            PROPOSER,
                            CHALLENGER
                        )
                    )
                )
            })
        );
        // Register the game implementation with the factory.
        disputeGameFactory.setImplementation(GAME_TYPE, gameImpl);

        // Create a new game.
        uint256 bondAmount = disputeGameFactory.initBonds(GAME_TYPE);
        vm.mockCall(
            address(anchorStateRegistry),
            abi.encodeCall(anchorStateRegistry.anchors, (GAME_TYPE)),
            abi.encode(rootClaim, 0)
        );
        vm.prank(PROPOSER, PROPOSER);
        gameProxy = IPermissionedDisputeGame(
            payable(address(disputeGameFactory.create{ value: bondAmount }(GAME_TYPE, rootClaim, extraData)))
        );

        // Check immutables
        assertEq(gameProxy.proposer(), PROPOSER);
        assertEq(gameProxy.challenger(), CHALLENGER);
        assertEq(gameProxy.gameType().raw(), GAME_TYPE.raw());
        assertEq(gameProxy.absolutePrestate().raw(), absolutePrestate.raw());
        assertEq(gameProxy.maxGameDepth(), 2 ** 3);
        assertEq(gameProxy.splitDepth(), 2 ** 2);
        assertEq(gameProxy.maxClockDuration().raw(), 3.5 days);
        assertEq(address(gameProxy.vm()), address(_vm));

        // Label the proxy
        vm.label(address(gameProxy), "FaultDisputeGame_Clone");
    }

    fallback() external payable { }

    receive() external payable { }
}

contract SuperPermissionedDisputeGame_Test is SuperPermissionedDisputeGame_Init {
    /// @dev The root claim of the game.
    Claim internal rootClaim;
    /// @dev An arbitrary root claim for testing.
    Claim internal arbitaryRootClaim = Claim.wrap(bytes32(uint256(123)));
    /// @dev Minimum bond value that covers all possible moves.
    uint256 internal constant MIN_BOND = 50 ether;

    /// @dev The preimage of the absolute prestate claim
    bytes internal absolutePrestateData;
    /// @dev The absolute prestate of the trace.
    Claim internal absolutePrestate;
    /// @dev A valid l2BlockNumber that comes after the current anchor root block.
    uint256 validL2BlockNumber;

    function setUp() public override {
        absolutePrestateData = abi.encode(0);
        absolutePrestate = _changeClaimStatus(Claim.wrap(keccak256(absolutePrestateData)), VMStatuses.UNFINISHED);

        super.setUp();

        // Get the actual anchor roots
        (Hash root, uint256 l2BlockNumber) = anchorStateRegistry.getAnchorRoot();
        validL2BlockNumber = l2BlockNumber + 1;
        rootClaim = Claim.wrap(Hash.unwrap(root));
        super.init({ rootClaim: rootClaim, absolutePrestate: absolutePrestate, l2BlockNumber: validL2BlockNumber });
    }

    /// @dev Tests that the game's version function returns a string.
    function test_version_works() public view {
        assertTrue(bytes(gameProxy.version()).length > 0);
    }

    /// @dev Tests that the proposer can create a permissioned dispute game.
    function test_createGame_proposer_succeeds() public {
        uint256 bondAmount = disputeGameFactory.initBonds(GAME_TYPE);
        vm.prank(PROPOSER, PROPOSER);
        disputeGameFactory.create{ value: bondAmount }(GAME_TYPE, arbitaryRootClaim, abi.encode(validL2BlockNumber));
    }

    /// @dev Tests that the permissioned game cannot be created by any address other than the proposer.
    function testFuzz_createGame_notProposer_reverts(address _p) public {
        vm.assume(_p != PROPOSER);

        uint256 bondAmount = disputeGameFactory.initBonds(GAME_TYPE);
        vm.deal(_p, bondAmount);
        vm.prank(_p, _p);
        vm.expectRevert(BadAuth.selector);
        disputeGameFactory.create{ value: bondAmount }(GAME_TYPE, arbitaryRootClaim, abi.encode(validL2BlockNumber));
    }

    /// @dev Tests that the challenger can participate in a permissioned dispute game.
    function test_participateInGame_challenger_succeeds() public {
        vm.startPrank(CHALLENGER, CHALLENGER);
        uint256 firstBond = _getRequiredBond(0);
        vm.deal(CHALLENGER, firstBond);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: firstBond }(disputed, 0, Claim.wrap(0));
        uint256 secondBond = _getRequiredBond(1);
        vm.deal(CHALLENGER, secondBond);
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.defend{ value: secondBond }(disputed, 1, Claim.wrap(0));
        uint256 thirdBond = _getRequiredBond(2);
        vm.deal(CHALLENGER, thirdBond);
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.move{ value: thirdBond }(disputed, 2, Claim.wrap(0), true);
        vm.stopPrank();
    }

    /// @dev Tests that the proposer can participate in a permissioned dispute game.
    function test_participateInGame_proposer_succeeds() public {
        vm.startPrank(PROPOSER, PROPOSER);
        uint256 firstBond = _getRequiredBond(0);
        vm.deal(PROPOSER, firstBond);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: firstBond }(disputed, 0, Claim.wrap(0));
        uint256 secondBond = _getRequiredBond(1);
        vm.deal(PROPOSER, secondBond);
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.defend{ value: secondBond }(disputed, 1, Claim.wrap(0));
        uint256 thirdBond = _getRequiredBond(2);
        vm.deal(PROPOSER, thirdBond);
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.move{ value: thirdBond }(disputed, 2, Claim.wrap(0), true);
        vm.stopPrank();
    }

    /// @dev Tests that addresses that are not the proposer or challenger cannot participate in a permissioned dispute
    ///      game.
    function test_participateInGame_notAuthorized_reverts(address _p) public {
        vm.assume(_p != PROPOSER && _p != CHALLENGER);

        vm.startPrank(_p, _p);
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        vm.expectRevert(BadAuth.selector);
        gameProxy.attack(disputed, 0, Claim.wrap(0));
        vm.expectRevert(BadAuth.selector);
        gameProxy.defend(disputed, 0, Claim.wrap(0));
        vm.expectRevert(BadAuth.selector);
        gameProxy.move(disputed, 0, Claim.wrap(0), true);
        vm.expectRevert(BadAuth.selector);
        gameProxy.step(0, true, absolutePrestateData, hex"");
        vm.stopPrank();
    }

    /// @dev Tests that step works properly.
    function test_step_succeeds() public {
        // Give the test contract some ether
        vm.deal(CHALLENGER, 1_000 ether);

        vm.startPrank(CHALLENGER, CHALLENGER);

        // Make claims all the way down the tree.
        (,,,, Claim disputed,,) = gameProxy.claimData(0);
        gameProxy.attack{ value: _getRequiredBond(0) }(disputed, 0, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(1);
        gameProxy.attack{ value: _getRequiredBond(1) }(disputed, 1, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(2);
        gameProxy.attack{ value: _getRequiredBond(2) }(disputed, 2, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(3);
        gameProxy.attack{ value: _getRequiredBond(3) }(disputed, 3, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(4);
        gameProxy.attack{ value: _getRequiredBond(4) }(disputed, 4, _changeClaimStatus(_dummyClaim(), VMStatuses.PANIC));
        (,,,, disputed,,) = gameProxy.claimData(5);
        gameProxy.attack{ value: _getRequiredBond(5) }(disputed, 5, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(6);
        gameProxy.attack{ value: _getRequiredBond(6) }(disputed, 6, _dummyClaim());
        (,,,, disputed,,) = gameProxy.claimData(7);
        gameProxy.attack{ value: _getRequiredBond(7) }(disputed, 7, _dummyClaim());

        // Verify game state before step
        assertEq(uint256(gameProxy.status()), uint256(GameStatus.IN_PROGRESS));

        gameProxy.addLocalData(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, 8, 0);
        gameProxy.step(8, true, absolutePrestateData, hex"");

        vm.warp(block.timestamp + gameProxy.maxClockDuration().raw() + 1);
        gameProxy.resolveClaim(8, 0);
        gameProxy.resolveClaim(7, 0);
        gameProxy.resolveClaim(6, 0);
        gameProxy.resolveClaim(5, 0);
        gameProxy.resolveClaim(4, 0);
        gameProxy.resolveClaim(3, 0);
        gameProxy.resolveClaim(2, 0);
        gameProxy.resolveClaim(1, 0);

        gameProxy.resolveClaim(0, 0);
        gameProxy.resolve();

        assertEq(uint256(gameProxy.status()), uint256(GameStatus.CHALLENGER_WINS));
        assertEq(gameProxy.resolvedAt().raw(), block.timestamp);
        (, address counteredBy,,,,,) = gameProxy.claimData(0);
        assertEq(counteredBy, CHALLENGER);
    }

    /// @dev Helper to return a pseudo-random claim
    function _dummyClaim() internal view returns (Claim) {
        return Claim.wrap(keccak256(abi.encode(gasleft())));
    }

    /// @dev Helper to get the required bond for the given claim index.
    function _getRequiredBond(uint256 _claimIndex) internal view returns (uint256 bond_) {
        (,,,,, Position parent,) = gameProxy.claimData(_claimIndex);
        Position pos = parent.move(true);
        bond_ = gameProxy.getRequiredBond(pos);
    }
}

/// @dev Helper to change the VM status byte of a claim.
function _changeClaimStatus(Claim _claim, VMStatus _status) pure returns (Claim out_) {
    assembly {
        out_ := or(and(not(shl(248, 0xFF)), _claim), shl(248, _status))
    }
}
