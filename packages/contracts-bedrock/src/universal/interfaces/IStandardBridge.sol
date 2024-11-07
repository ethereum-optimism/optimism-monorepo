// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";

interface IStandardBridge {
    event ERC20BridgeFinalized(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 amount,
        bytes extraData
    );
    event ERC20BridgeInitiated(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 amount,
        bytes extraData
    );
    event ETHBridgeFinalized(address indexed from, address indexed to, uint256 amount, bytes extraData);
    event ETHBridgeInitiated(address indexed from, address indexed to, uint256 amount, bytes extraData);
    event Initialized(uint8 version);

    receive() external payable;

    function MESSENGER() external view returns (ICrossDomainMessenger);
    function OTHER_BRIDGE() external view returns (IStandardBridge);
    function bridgeERC20(
        address _localToken,
        address _remoteToken,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes memory _extraData
    )
        external;
    function bridgeERC20To(
        address _localToken,
        address _remoteToken,
        address _to,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes memory _extraData
    )
        external;
    function bridgeETH(uint32 _minGasLimit, bytes memory _extraData) external payable;
    function bridgeETHTo(address _to, uint32 _minGasLimit, bytes memory _extraData) external payable;
    function deposits(address, address) external view returns (uint256);
    function finalizeBridgeERC20(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _amount,
        bytes memory _extraData
    )
        external;
    function finalizeBridgeETH(address _from, address _to, uint256 _amount, bytes memory _extraData) external payable;
    function messenger() external view returns (ICrossDomainMessenger);
    function otherBridge() external view returns (IStandardBridge);
    function paused() external view returns (bool);

    function __constructor__() external;
}
