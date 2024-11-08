// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ISemver } from "src/universal/interfaces/ISemver.sol";

/// @title ISuperchainTokenBridge
/// @notice Interface for the SuperchainTokenBridge contract.
interface ISuperchainTokenBridge is ISemver {
    error ZeroAddress();
    error Unauthorized();
    error InvalidCrossDomainSender();
    error InvalidERC7802();

    event SendERC20(
        address indexed token, address indexed from, address indexed to, uint256 amount, uint256 destination
    );

    event RelayERC20(address indexed token, address indexed from, address indexed to, uint256 amount, uint256 source);

    function sendERC20(
        address _token,
        address _to,
        uint256 _amount,
        uint256 _chainId
    )
        external
        returns (bytes32 msgHash_);

    function relayERC20(address _token, address _from, address _to, uint256 _amount) external;

    function __constructor__() external;
}
