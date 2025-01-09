// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { GameType, Hash, OutputRoot } from "src/dispute/lib/Types.sol";

interface IAnchorStateRegistry {
    error InvalidGameStatus();
    error Unauthorized();
    error UnregisteredGame();

    event Initialized(uint8 version);

    function anchors(GameType) external view returns (Hash root, uint256 l2BlockNumber); // nosemgrep
    function disputeGameFactory() external view returns (IDisputeGameFactory);
    function getAnchorState() external view returns (Hash root, uint256 l2BlockNumber);
    function initialize(
        ISuperchainConfig _superchainConfig,
        OutputRoot calldata _startingAnchorRoot,
        IDisputeGameFactory _disputeGameFactory,
        uint256 _disputeGameFinalityDelaySeconds
    )
        external;
    function isGameBlacklisted(IDisputeGame _game) external view returns (bool);
    function isGameRetired(IDisputeGame _game) external view returns (bool);
    function isGameMaybeValid(IDisputeGame _game) external view returns (bool, string memory);
    function isGameFinalized(IDisputeGame _game) external view returns (bool, string memory);
    function isGameValid(IDisputeGame _game) external view returns (bool, string memory);
    function respectedGameType() external view returns (GameType);
    function retireAllExistingGames() external;
    function setAnchorState(IDisputeGame _game) external;
    function setGameBlacklisted(IDisputeGame _game) external;
    function setRespectedGameType(GameType _gameType) external;
    function superchainConfig() external view returns (ISuperchainConfig);
    function version() external view returns (string memory);

    function __constructor__(IDisputeGameFactory _disputeGameFactory) external;
}
