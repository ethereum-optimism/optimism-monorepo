// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IGasPriceOracle {
    function DECIMALS() external view returns (uint256);
    function baseFee() external view returns (uint256);
    function baseFeeScalar() external view returns (uint32);
    function blobBaseFee() external view returns (uint256);
    function blobBaseFeeScalar() external view returns (uint32);
    function decimals() external pure returns (uint256);
    function gasPrice() external view returns (uint256);
    function getL1Fee(bytes memory _data) external view returns (uint256);
    function getL1FeeUpperBound(uint256 _unsignedTxSize) external view returns (uint256);
    function getL1GasUsed(bytes memory _data) external view returns (uint256);
    function isEcotone() external view returns (bool);
    function isFjord() external view returns (bool);
    function l1BaseFee() external view returns (uint256);
    function overhead() external view returns (uint256);
    function scalar() external view returns (uint256);
    function setEcotone() external;
    function setFjord() external;
    function version() external view returns (string memory);

    function __constructor__() external;
}
