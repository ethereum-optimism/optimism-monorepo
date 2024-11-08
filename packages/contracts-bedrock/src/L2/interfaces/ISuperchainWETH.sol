// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IWETH98 } from "src/universal/interfaces/IWETH98.sol";
import { IERC7802 } from "src/L2/interfaces/IERC7802.sol";
import { ISemver } from "src/universal/interfaces/ISemver.sol";

interface ISuperchainWETH is IWETH98, IERC7802, ISemver {
    error Unauthorized();
    error NotCustomGasToken();

    function balanceOf(address src) external view returns (uint256);
    function withdraw(uint256 _amount) external;
    function supportsInterface(bytes4 _interfaceId) external view returns (bool);

    function __constructor__() external;
}
