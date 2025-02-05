// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Libraries
import { PreimageKeyLib } from "src/cannon/PreimageKeyLib.sol";
import "src/dispute/lib/Types.sol";

// Interfaces
import { IBigStepper, IPreimageOracle } from "interfaces/dispute/IBigStepper.sol";

/// @title AlphabetVM
/// @dev A mock VM for the purpose of testing the dispute game infrastructure. Note that this only works
///      for games with an execution trace subgame max depth of 3 (8 instructions per subgame).
contract AlphabetVM is IBigStepper {
    Claim internal immutable ABSOLUTE_PRESTATE;
    IPreimageOracle public oracle;

    constructor(Claim _absolutePrestate, IPreimageOracle _oracle) {
        ABSOLUTE_PRESTATE = _absolutePrestate;
        oracle = _oracle;
    }

    /// @inheritdoc IBigStepper
    function step(
        bytes calldata _stateData,
        bytes calldata,
        bytes32 _localContext
    )
        external
        view
        returns (bytes32 postState_)
    {
        uint256 traceIndex;
        uint256 claim;
        if ((keccak256(_stateData) << 8) == (Claim.unwrap(ABSOLUTE_PRESTATE) << 8)) {
            // If the state data is empty, then the absolute prestate is the claim.
            (bytes32 dat,) = oracle.readPreimage(
                PreimageKeyLib.localizeIdent(LocalPreimageKey.DISPUTED_L2_BLOCK_NUMBER, _localContext), 0
            );
            uint256 startingL2BlockNumber = ((uint256(dat) >> 128) & 0xFFFFFFFF) - 1;
            traceIndex = startingL2BlockNumber << 4;
            (uint256 absolutePrestateClaim) = abi.decode(_stateData, (uint256));
            claim = absolutePrestateClaim + traceIndex;
        } else {
            // Otherwise, decode the state data.
            (traceIndex, claim) = abi.decode(_stateData, (uint256, uint256));
            traceIndex++;
            claim++;
        }

        // STF: n -> n + 1
        postState_ = keccak256(abi.encode(traceIndex, claim));
        assembly {
            postState_ := or(and(postState_, not(shl(248, 0xFF))), shl(248, 1))
        }
    }
}
