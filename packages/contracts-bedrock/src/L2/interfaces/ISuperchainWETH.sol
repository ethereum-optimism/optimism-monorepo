// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IWETH98 } from "src/universal/interfaces/IWETH98.sol";
import { ICrosschainERC20 } from "src/L2/interfaces/ICrosschainERC20.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";

interface ISuperchainWETH is IWETH98, ICrosschainERC20, ISemver {
    error Unauthorized();
    error NotCustomGasToken();

    function balanceOf(address src) external view returns (uint256);
    function withdraw(uint256 _amount) external;

    function __constructor__() external;
}
