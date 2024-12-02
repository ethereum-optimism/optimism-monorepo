// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { StdUtils } from "forge-std/StdUtils.sol";

/// @notice helper for tracking actors, taking advantage of the fuzzer already using several `msg.sender`s
contract Actors is StdUtils {
    mapping(address => bool) private _isActor;
    address[] private _actors;
    address private _currentActor;

    /// @notice register an actor if it's not already registered
    /// usually called with msg.sender as a parameter, to track the actors
    /// already provided by the fuzzer
    modifier withActor(address who) {
        addActor(who);
        _currentActor = who;
        _;
    }

    function addActor(address who) internal {
        if (!_isActor[who]) {
            _isActor[who] = true;
            _actors.push(who);
        }
    }

    /// @notice get the currently configured actor, should equal msg.sender
    function currentActor() internal view returns (address) {
        return _currentActor;
    }

    /// @notice get one of the actors by index, useful to get another random
    /// actor than the one set as currentActor, to perform operations between them
    function getActorByRawIndex(uint256 rawIndex) internal view returns (address) {
        return _actors[bound(rawIndex, 0, _actors.length - 1)];
    }
}
