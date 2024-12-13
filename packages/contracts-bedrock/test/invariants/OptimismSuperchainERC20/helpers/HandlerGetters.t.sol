// SPDX-License-Identifier: GPL-3
pragma solidity ^0.8.24;

import { ProtocolHandler } from "../handlers/Protocol.t.sol";
import { EnumerableMap } from "@openzeppelin/contracts/utils/structs/EnumerableMap.sol";

contract HandlerGetters is ProtocolHandler {
    using EnumerableMap for EnumerableMap.Bytes32ToUintMap;

    function deploySaltsLength() external view returns (uint256 length_) {
        return ghost_totalSupplyAcrossChains.length();
    }

    function totalSupplyAcrossChainsAtIndex(uint256 _index) external view returns (bytes32 salt_, uint256 supply_) {
        return ghost_totalSupplyAcrossChains.at(_index);
    }

    function tokensInTransitForDeploySalt(bytes32 _salt) external view returns (uint256 amount_) {
        (, amount_) = ghost_tokensInTransit.tryGet(_salt);
        return amount_;
    }
}
