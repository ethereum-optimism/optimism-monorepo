// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IGovernanceToken } from "interfaces/governance/IGovernanceToken.sol";

interface IMintManager {
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    function DENOMINATOR() external view returns (uint256);
    function MINT_CAP() external view returns (uint256);
    function MINT_PERIOD() external view returns (uint256);
    function governanceToken() external view returns (IGovernanceToken);
    function mint(address _account, uint256 _amount) external;
    function mintPermittedAfter() external view returns (uint256);
    function owner() external view returns (address);
    function renounceOwnership() external;
    function transferOwnership(address newOwner) external; // nosemgrep
    function upgrade(address _newMintManager) external;

    function __constructor__(address _upgrader, address _governanceToken) external;
}
