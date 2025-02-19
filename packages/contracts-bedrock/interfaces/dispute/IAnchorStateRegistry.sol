// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { GameType, Hash, OutputRoot } from "src/dispute/lib/Types.sol";

interface IAnchorStateRegistry {
    error AnchorStateRegistry_Unauthorized();
    error AnchorStateRegistry_InvalidAnchorGame();
    error AnchorStateRegistry_AnchorGameBlacklisted();

    event AnchorNotUpdated(IFaultDisputeGame indexed game);
    event AnchorUpdated(IFaultDisputeGame indexed game);
    event Initialized(uint8 version);

    function anchorGame() external view returns (IFaultDisputeGame);
    function anchors(GameType) external view returns (Hash, uint256);
    function getAnchorRoot() external view returns (Hash, uint256);
    function disputeGameFactory() external view returns (IDisputeGameFactory);
    function initialize(
        ISuperchainConfig _superchainConfig,
        IDisputeGameFactory _disputeGameFactory,
        IOptimismPortal2 _portal,
        OutputRoot memory _startingAnchorRoot
    )
        external;

    function isGameBlacklisted(IDisputeGame _game) external view returns (bool);
    function isGameProper(IDisputeGame _game) external view returns (bool);
    function isGameRegistered(IDisputeGame _game) external view returns (bool);
    function isGameResolved(IDisputeGame _game) external view returns (bool);
    function isGameRespected(IDisputeGame _game) external view returns (bool);
    function isGameRetired(IDisputeGame _game) external view returns (bool);
    function isGameFinalized(IDisputeGame _game) external view returns (bool);
    function isGameClaimValid(IDisputeGame _game) external view returns (bool);
    function portal() external view returns (IOptimismPortal2);
    function respectedGameType() external view returns (GameType);
    function setAnchorState(IDisputeGame _game) external;
    function superchainConfig() external view returns (ISuperchainConfig);
    function version() external view returns (string memory);

    function __constructor__() external;
}
