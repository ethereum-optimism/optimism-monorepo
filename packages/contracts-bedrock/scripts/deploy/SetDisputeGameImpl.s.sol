// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";

import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { GameType } from "src/dispute/lib/Types.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";

contract SetDisputeGameImplInput is BaseDeployIO {
    IDisputeGameFactory internal _factory;
    IAnchorStateRegistry internal _anchorStateRegistry;
    IFaultDisputeGame internal _impl;
    uint32 internal _gameType;

    // Setter for address type
    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "SetDisputeGameImplInput: cannot set zero address");

        if (_sel == this.factory.selector) _factory = IDisputeGameFactory(_addr);
        else if (_sel == this.anchorStateRegistry.selector) _anchorStateRegistry = IAnchorStateRegistry(_addr);
        else if (_sel == this.impl.selector) _impl = IFaultDisputeGame(_addr);
        else revert("SetDisputeGameImplInput: unknown selector");
    }

    // Setter for GameType
    function set(bytes4 _sel, uint32 _type) public {
        if (_sel == this.gameType.selector) _gameType = _type;
        else revert("SetDisputeGameImplInput: unknown selector");
    }

    function factory() public view returns (IDisputeGameFactory) {
        require(address(_factory) != address(0), "SetDisputeGameImplInput: not set");
        return _factory;
    }

    function anchorStateRegistry() public view returns (IAnchorStateRegistry) {
        return _anchorStateRegistry;
    }

    function impl() public view returns (IFaultDisputeGame) {
        require(address(_impl) != address(0), "SetDisputeGameImplInput: not set");
        return _impl;
    }

    function gameType() public view returns (uint32) {
        return _gameType;
    }
}

contract SetDisputeGameImpl is Script {
    function run(SetDisputeGameImplInput _input) public {
        IDisputeGameFactory factory = _input.factory();
        GameType gameType = GameType.wrap(_input.gameType());
        require(address(factory.gameImpls(gameType)) == address(0), "SDGI-10");

        IFaultDisputeGame impl = _input.impl();
        IAnchorStateRegistry anchorStateRegistry = _input.anchorStateRegistry();

        vm.broadcast(msg.sender);
        factory.setImplementation(gameType, impl);

        if (address(anchorStateRegistry) != address(0)) {
            require(address(anchorStateRegistry.disputeGameFactory()) == address(factory), "SDGI-20");
            vm.broadcast(msg.sender);
            anchorStateRegistry.setRespectedGameType(gameType);
        }

        assertValid(_input);
    }

    function assertValid(SetDisputeGameImplInput _input) public view {
        GameType gameType = GameType.wrap(_input.gameType());
        require(address(_input.factory().gameImpls(gameType)) == address(_input.impl()), "SDGI-30");

        if (address(_input.anchorStateRegistry()) != address(0)) {
            require(address(_input.anchorStateRegistry().disputeGameFactory()) == address(_input.factory()), "SDGI-40");
            require(
                GameType.unwrap(_input.anchorStateRegistry().respectedGameType()) == GameType.unwrap(gameType),
                "SDGI-50"
            );
        }
    }
}
