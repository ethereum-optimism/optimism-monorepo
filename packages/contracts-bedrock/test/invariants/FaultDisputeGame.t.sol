// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Vm } from "forge-std/Vm.sol";
import { StdUtils } from "forge-std/StdUtils.sol";
import { FaultDisputeGame_Init } from "test/dispute/FaultDisputeGame.t.sol";

// Libraries
import "src/dispute/lib/Types.sol";
import "src/dispute/lib/Errors.sol";

// Interfaces
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";

contract FaultDisputeGame_Solvency_Invariant is FaultDisputeGame_Init {
    Claim internal constant ROOT_CLAIM = Claim.wrap(bytes32(uint256(10)));
    Claim internal constant ABSOLUTE_PRESTATE = Claim.wrap(bytes32((uint256(3) << 248) | uint256(0)));

    RandomClaimActor internal actor;
    uint256 internal defaultSenderBalance;

    function setUp() public override {
        super.setUp();
        super.init({ rootClaim: ROOT_CLAIM, absolutePrestate: ABSOLUTE_PRESTATE, l2BlockNumber: 0x10 });

        actor = new RandomClaimActor(gameProxy, vm);

        targetContract(address(actor));
        vm.startPrank(address(actor));
    }

    /// @custom:invariant FaultDisputeGame always returns all ETH on total resolution
    ///
    /// The FaultDisputeGame contract should always return all ETH in the contract to the correct recipients upon
    /// resolution of all outstanding claims. There may never be any ETH left in the contract after a full resolution.
    function invariant_faultDisputeGame_solvency() public {
        vm.warp(block.timestamp + 7 days + 1 seconds);

        (,,, uint256 rootBond,,,) = gameProxy.claimData(0);

        for (uint256 i = gameProxy.claimDataLen(); i > 0; i--) {
            (bool success,) = address(gameProxy).call(abi.encodeCall(gameProxy.resolveClaim, (i - 1, 0)));
            assertTrue(success);
        }
        gameProxy.resolve();

        // Wait for the withdrawal delay.
        vm.warp(block.timestamp + 7 days + 1 seconds);

        if (gameProxy.credit(address(this)) == 0) {
            vm.expectRevert(NoCreditToClaim.selector);
            gameProxy.claimCredit(address(this));
        } else {
            gameProxy.claimCredit(address(this));
        }

        if (gameProxy.credit(address(actor)) == 0) {
            vm.expectRevert(NoCreditToClaim.selector);
            gameProxy.claimCredit(address(actor));
        } else {
            gameProxy.claimCredit(address(actor));
        }

        if (gameProxy.status() == GameStatus.DEFENDER_WINS) {
            assertEq(address(this).balance, type(uint96).max);
            assertEq(address(actor).balance, actor.totalBonded() - rootBond);
        } else if (gameProxy.status() == GameStatus.CHALLENGER_WINS) {
            assertEq(DEFAULT_SENDER.balance, type(uint96).max - rootBond);
            assertEq(address(actor).balance, actor.totalBonded() + rootBond);
        } else {
            revert("FaultDisputeGame_Solvency_Invariant: unreachable");
        }

        assertEq(address(gameProxy).balance, 0);
    }
}

contract RandomClaimActor is StdUtils {
    IFaultDisputeGame internal immutable GAME;
    Vm internal immutable VM;

    uint256 public totalBonded;

    constructor(IFaultDisputeGame _gameProxy, Vm _vm) {
        GAME = _gameProxy;
        VM = _vm;
    }

    function move(bool _isAttack, uint256 _parentIndex, Claim _claim, uint64 _bondAmount) public {
        _parentIndex = bound(_parentIndex, 0, GAME.claimDataLen());
        VM.deal(address(this), _bondAmount);

        totalBonded += _bondAmount;

        (,,,, Claim disputed,,) = GAME.claimData(_parentIndex);
        GAME.move{ value: _bondAmount }(disputed, _parentIndex, _claim, _isAttack);
    }

    fallback() external payable { }

    receive() external payable { }
}
