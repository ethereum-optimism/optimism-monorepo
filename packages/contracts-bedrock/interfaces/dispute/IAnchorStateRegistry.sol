// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { GameType, Hash, OutputRoot } from "src/dispute/lib/Types.sol";

interface IAnchorStateRegistry {
    struct StartingAnchorRoot {
        GameType gameType;
        OutputRoot outputRoot;
    }

    error InvalidGameStatus();
    error Unauthorized();
    error UnregisteredGame();

    event Initialized(uint8 version);

    function anchors(GameType) external view returns (Hash root, uint256 l2BlockNumber); // nosemgrep
    function disputeGameFactory() external view returns (IDisputeGameFactory);
    function initialize(
        IDisputeGameFactory _disputeGameFactory,
        uint256 _disputeGameFinalityDelaySeconds,
        IFaultDisputeGame _authorizedGame,
        ISuperchainConfig _superchainConfig,
        uint256 _tryUpdateAnchorStateGas
    )
        external;
    function isGameMaybeValid(IFaultDisputeGame _game) external view returns (bool);
    function isGameValid(IFaultDisputeGame _game) external view returns (bool);
    function pokeAnchorState(uint256 _candidateGameIndex) external;
    function respectedGameType() external view returns (GameType);
    function retireAllExistingGames() external;
    function setAnchorState(IFaultDisputeGame _game) external;
    function setGameBlacklisted(IFaultDisputeGame _disputeGame) external;
    function setRespectedGameType(GameType _gameType) external;
    function superchainConfig() external view returns (ISuperchainConfig);
    function tryUpdateAnchorState() external;
    function version() external view returns (string memory);

    function __constructor__(IDisputeGameFactory _disputeGameFactory) external;
}
