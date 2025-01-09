// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { GameType, Hash, OutputRoot } from "src/dispute/lib/Types.sol";

interface IAnchorStateRegistry {
    error AnchorStateRegistry_AnchorGameBlacklisted(IDisputeGame game);
    error AnchorStateRegistry_AnchorGameIsNewer();
    error AnchorStateRegistry_CandidateGameNotValid(string reason);
    error AnchorStateRegistry_OnlyGuardian();

    event AnchorGameSet(IDisputeGame newAnchorGame);
    event DisputeGameBlacklisted(IDisputeGame game);
    event GameRetirementTimestampSet(uint64 timestamp);
    event Initialized(uint8 version);
    event RespectedGameTypeSet(GameType gameType);

    function disputeGameFactory() external view returns (IDisputeGameFactory);
    function disputeGameFinalityDelaySeconds() external view returns (uint256);
    function gameRetirementTimestamp() external view returns (uint64);
    function getAnchorRoot() external view returns (Hash, uint256);
    function initialize(
        ISuperchainConfig _superchainConfig,
        IDisputeGameFactory _disputeGameFactory,
        OutputRoot calldata _initialAnchorRoot,
        GameType _initialRespectedGameType
    ) external;
    function isClaimValid(IDisputeGame _game) external view returns (bool, string memory);
    function isGameBlacklisted(IDisputeGame) external view returns (bool);
    function isGameRetired(IDisputeGame _game) external view returns (bool);
    function isGameFinalized(IDisputeGame _game) external view returns (bool, string memory);
    function isProperGame(IDisputeGame _game) external view returns (bool, string memory);
    function respectedGameType() external view returns (GameType);
    function retireAllExistingGames() external;
    function setAnchorGame(IDisputeGame _game) external;
    function setGameBlacklisted(IDisputeGame _game) external;
    function setRespectedGameType(GameType _gameType) external;
    function superchainConfig() external view returns (ISuperchainConfig);
    function version() external view returns (string memory);

    function __constructor__(uint256 _disputeGameFinalityDelaySeconds) external;
}
