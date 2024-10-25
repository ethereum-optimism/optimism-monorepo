// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { InvalidExitedValue } from "src/cannon/libraries/CannonErrors.sol";

library MIPS64State {
    struct CpuScalars {
        uint64 pc;
        uint64 nextPC;
        uint64 lo;
        uint64 hi;
    }

    function assertExitedIsValid(uint32 _exited) internal pure {
        if (_exited > 1) {
            revert InvalidExitedValue();
        }
    }
}
