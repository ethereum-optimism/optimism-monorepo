// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";

import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { GameType } from "src/dispute/lib/Types.sol";

contract SetDisputeGameImplInput is BaseDeployIO {
    IDisputeGameFactory internal _factory;
    IDisputeGame internal _impl;
    uint32 internal _gameType;

    // Setter for address type
    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "SetDisputeGameImplInput: cannot set zero address");

        if (_sel == this.factory.selector) _factory = IDisputeGameFactory(_addr);
        else if (_sel == this.impl.selector) _impl = IDisputeGame(_addr);
        else revert("SetDisputeGameImplInput: unknown selector");
    }

    // Setter for GameType
    function set(bytes4 _sel, uint32 _type) public {
        if (_sel == this.gameType.selector) _gameType = _type;
        else revert("SetDisputeGameImplInput: unknown selector");
    }

    // Getters
    function factory() public view returns (IDisputeGameFactory) {
        require(address(_factory) != address(0), "SetDisputeGameImplInput: not set");
        return _factory;
    }

    function impl() public view returns (IDisputeGame) {
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

        IDisputeGame impl = _input.impl();
        vm.broadcast(msg.sender);
        factory.setImplementation(gameType, impl);
        assertValid(_input);
    }

    function assertValid(SetDisputeGameImplInput _input) public view {
        GameType gameType = GameType.wrap(_input.gameType());
        require(address(_input.factory().gameImpls(gameType)) == address(_input.impl()), "SDGI-20");
    }
}
