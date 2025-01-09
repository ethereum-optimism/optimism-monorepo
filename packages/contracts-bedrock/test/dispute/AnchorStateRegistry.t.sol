// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Testing
import { FaultDisputeGame_Init, _changeClaimStatus } from "test/dispute/FaultDisputeGame.t.sol";

// Libraries
import "src/dispute/lib/Types.sol";
import "src/dispute/lib/Errors.sol";

contract AnchorStateRegistry_Init is FaultDisputeGame_Init {
    function setUp() public virtual override {
        // Duplicating the initialization/setup logic of FaultDisputeGame_Test.
        // See that test for more information, actual values here not really important.
        Claim rootClaim = Claim.wrap(bytes32((uint256(1) << 248) | uint256(10)));
        bytes memory absolutePrestateData = abi.encode(0);
        Claim absolutePrestate = _changeClaimStatus(Claim.wrap(keccak256(absolutePrestateData)), VMStatuses.UNFINISHED);

        super.setUp();
        super.init({ rootClaim: rootClaim, absolutePrestate: absolutePrestate, l2BlockNumber: 0x10 });
    }
}

contract AnchorStateRegistry_Initialize_Test is AnchorStateRegistry_Init {
    /// @dev Tests that initialization is successful.
    function test_initialize_succeeds() public view {
        // TODO: Fixme
    }
}
