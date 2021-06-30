// SPDX-License-Identifier: MIT
pragma solidity >0.5.0 <0.8.0;

/**
 * @title iOVM_GasPriceOracle
 */
interface iOVM_GasPriceOracle {

    /**********
     * Events *
     **********/

    event GasPriceUpdated(uint256 _oldPrice, uint256 _newPrice);

    /********************
     * Public Functions *
     ********************/

    function getGasPrice() external returns (uint256);
    function setGasPrice(uint256 _gasPrice) external;
}
