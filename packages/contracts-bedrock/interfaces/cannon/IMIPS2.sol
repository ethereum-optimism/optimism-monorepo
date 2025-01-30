// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "interfaces/universal/ISemver.sol";
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";

/// @title IMIPS2
/// @notice Interface for the MIPS2 contract.
interface IMIPS2 is ISemver {
    struct ThreadState {
        uint32 threadID;
        uint8 exitCode;
        bool exited;
        uint32 pc;
        uint32 nextPC;
        uint32 lo;
        uint32 hi;
        uint32[32] registers;
    }

    struct State {
        bytes32 memRoot;
        bytes32 preimageKey;
        uint32 preimageOffset;
        uint32 heap;
        uint8 llReservationStatus;
        uint32 llAddress;
        uint32 llOwnerThread;
        uint8 exitCode;
        bool exited;
        uint64 step;
        uint64 stepsSinceLastContextSwitch;
        bool traverseRight;
        bytes32 leftThreadStack;
        bytes32 rightThreadStack;
        uint32 nextThreadID;
    }

    error InvalidExitedValue();
    error InvalidMemoryProof();
    error InvalidSecondMemoryProof();
    error InvalidRMWInstruction();

    function oracle() external view returns (IPreimageOracle oracle_);
    function step(
        bytes memory _stateData,
        bytes memory _proof,
        bytes32 _localContext
    )
        external
        returns (bytes32 postState_);

    function __constructor__(IPreimageOracle _oracle) external;
}
