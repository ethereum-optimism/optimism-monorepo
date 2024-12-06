// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { GameTypes, OutputRoot, Hash } from "src/dispute/lib/Types.sol";

/// @title Constants
/// @notice Constants is a library for storing constants. Simple! Don't put everything in here, just
///         the stuff used in multiple contracts. Constants that only apply to a single contract
///         should be defined in that contract instead.
library Constants {
    /// @notice Returns the default starting anchor roots value to be used in a new dispute game.
    function DEFAULT_OUTPUT_ROOT() internal pure returns (OutputRoot memory) {
        return OutputRoot({ root: Hash.wrap(bytes32(hex"dead")), l2BlockNumber: 0 });
    }

    function DEFAULT_STARTING_ANCHOR_ROOTS() internal pure returns (IAnchorStateRegistry.StartingAnchorRoot[] memory) {
        IAnchorStateRegistry.StartingAnchorRoot[] memory defaultStartingAnchorRoots =
            new IAnchorStateRegistry.StartingAnchorRoot[](1);
        defaultStartingAnchorRoots[0] = IAnchorStateRegistry.StartingAnchorRoot({
            gameType: GameTypes.PERMISSIONED_CANNON,
            outputRoot: DEFAULT_OUTPUT_ROOT()
        });
        return defaultStartingAnchorRoots;
    }
}
