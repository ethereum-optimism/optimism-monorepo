// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Interfaces
import { IDripCheck } from "src/periphery/drippie/IDripCheck.sol";

/// @title CheckSecrets
/// @notice DripCheck that checks if specific secrets exist (or not). Supports having a secret that
///         must exist for the check to pass as well as a second secret that must not exist. First
///         secret can be revealed to begin the drip, second secret can be revealed to stop it.
contract CheckSecrets is IDripCheck {
    struct Params {
        uint256 delay;
        bytes32 secretHashMustExist;
        bytes32 secretHashMustNotExist;
    }

    /// @notice External event used to help client-side tooling encode parameters.
    /// @param params Parameters to encode.
    event _EventToExposeStructInABI__Params(Params params);

    /// @notice Event emitted when a secret is revealed.
    event SecretRevealed(bytes32 indexed secretHash, bytes secret);

    /// @inheritdoc IDripCheck
    string public name = "CheckSecrets";

    /// @notice Keeps track of when secrets were revealed.
    mapping(bytes32 => uint256) public revealedSecrets;

    /// @inheritdoc IDripCheck
    function check(bytes memory _params) external view returns (bool execute_) {
        Params memory params = abi.decode(_params, (Params));

        // Check that the secrets have/have not been revealed.
        execute_ = (
            revealedSecrets[params.secretHashMustExist] > 0
                && block.timestamp >= revealedSecrets[params.secretHashMustExist] + params.delay
                && revealedSecrets[params.secretHashMustNotExist] == 0
        );
    }

    /// @notice Reveal a secret.
    /// @param _secret Secret to reveal.
    function reveal(bytes memory _secret) external {
        bytes32 secretHash = keccak256(_secret);
        require(revealedSecrets[secretHash] == 0, "CheckSecrets: secret already revealed");
        revealedSecrets[secretHash] = block.timestamp;
        emit SecretRevealed(secretHash, _secret);
    }
}
