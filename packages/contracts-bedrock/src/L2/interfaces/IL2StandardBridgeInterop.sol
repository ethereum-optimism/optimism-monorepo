// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IStandardBridge } from "src/universal/interfaces/IStandardBridge.sol";
import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";

interface IL2StandardBridgeInterop is IStandardBridge {
    error InvalidDecimals();
    error InvalidLegacyERC20Address();
    error InvalidSuperchainERC20Address();
    error InvalidTokenPair();

    event Converted(address indexed from, address indexed to, address indexed caller, uint256 amount);

    receive() external payable;

    event DepositFinalized(
        address indexed l1Token,
        address indexed l2Token,
        address indexed from,
        address to,
        uint256 amount,
        bytes extraData
    );
    event WithdrawalInitiated(
        address indexed l1Token,
        address indexed l2Token,
        address indexed from,
        address to,
        uint256 amount,
        bytes extraData
    );

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

    function initialize(IStandardBridge _otherBridge) external;
    function l1TokenBridge() external view returns (address);
    function withdraw(
        address _l2Token,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes memory _extraData
    )
        external
        payable;
    function withdrawTo(
        address _l2Token,
        address _to,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes memory _extraData
    )
        external
        payable;

    function convert(address _from, address _to, uint256 _amount) external;
    function version() external pure returns (string memory);

    function __constructor__() external;
}
