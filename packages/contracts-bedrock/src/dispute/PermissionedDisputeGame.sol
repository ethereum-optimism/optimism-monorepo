// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { FaultDisputeGame } from "src/dispute/FaultDisputeGame.sol";

// Libraries
import { Claim } from "src/dispute/lib/Types.sol";
import { BadAuth } from "src/dispute/lib/Errors.sol";

/// @title PermissionedDisputeGame
/// @notice PermissionedDisputeGame is a contract that inherits from `FaultDisputeGame`, and contains two roles:
///         - The `challenger` role, which is allowed to challenge a dispute.
///         - The `proposer` role, which is allowed to create proposals and participate in their game.
///         This contract exists as a way for networks to support the fault proof iteration of the OptimismPortal
///         contract without needing to support a fully permissionless system. Permissionless systems can introduce
///         costs that certain networks may not wish to support. This contract can also be used as a fallback mechanism
///         in case of a failure in the permissionless fault proof system in the stage one release.
contract PermissionedDisputeGame is FaultDisputeGame {
    /// @notice The proposer role is allowed to create proposals and participate in the dispute game.
    address internal immutable PROPOSER;

    /// @notice The challenger role is allowed to participate in the dispute game.
    address internal immutable CHALLENGER;

    /// @notice Modifier that gates access to the `challenger` and `proposer` roles.
    modifier onlyAuthorized() {
        if (!(msg.sender == PROPOSER || msg.sender == CHALLENGER)) {
            revert BadAuth();
        }
        _;
    }

    /// @notice Semantic version.
    /// @custom:semver 1.4.1
    function version() public pure override returns (string memory) {
        return "1.4.1";
    }

    /// @param _params Parameters for creating a new FaultDisputeGame.
    /// @param _proposer Address that is allowed to create instances of this contract.
    /// @param _challenger Address that is allowed to challenge instances of this contract.
    constructor(
        GameConstructorParams memory _params,
        address _proposer,
        address _challenger
    )
        FaultDisputeGame(_params)
    {
        PROPOSER = _proposer;
        CHALLENGER = _challenger;
    }

    /// @inheritdoc FaultDisputeGame
    function step(
        uint256 _claimIndex,
        bool _isAttack,
        bytes calldata _stateData,
        bytes calldata _proof
    )
        public
        override
        onlyAuthorized
    {
        super.step(_claimIndex, _isAttack, _stateData, _proof);
    }

    /// @notice Generic move function, used for both `attack` and `defend` moves.
    /// @notice _disputed The disputed `Claim`.
    /// @param _challengeIndex The index of the claim being moved against. This must match the `_disputed` claim.
    /// @param _claim The claim at the next logical position in the game.
    /// @param _isAttack Whether or not the move is an attack or defense.
    function move(
        Claim _disputed,
        uint256 _challengeIndex,
        Claim _claim,
        bool _isAttack
    )
        public
        payable
        override
        onlyAuthorized
    {
        super.move(_disputed, _challengeIndex, _claim, _isAttack);
    }

    /// @notice Initializes the contract.
    function initialize() public payable override {
        // The creator of the dispute game must be the proposer EOA.
        if (tx.origin != PROPOSER) revert BadAuth();

        // Fallthrough initialization.
        super.initialize();
    }

    ////////////////////////////////////////////////////////////////
    //                     IMMUTABLE GETTERS                      //
    ////////////////////////////////////////////////////////////////

    /// @notice Returns the proposer address.
    function proposer() external view returns (address proposer_) {
        proposer_ = PROPOSER;
    }

    /// @notice Returns the challenger address.
    function challenger() external view returns (address challenger_) {
        challenger_ = CHALLENGER;
    }
}
