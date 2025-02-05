// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface ILegacyMintableERC20Full {
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event Burn(address indexed _account, uint256 _amount);
    event Mint(address indexed _account, uint256 _amount);
    event Transfer(address indexed from, address indexed to, uint256 value);

    function allowance(address owner, address spender) external view returns (uint256);
    function approve(address spender, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
    function burn(address _from, uint256 _amount) external;
    function decimals() external view returns (uint8);
    function decreaseAllowance(address spender, uint256 subtractedValue) external returns (bool);
    function increaseAllowance(address spender, uint256 addedValue) external returns (bool);
    function l1Token() external view returns (address);
    function l2Bridge() external view returns (address);
    function mint(address _to, uint256 _amount) external;
    function name() external view returns (string memory);
    function supportsInterface(bytes4 _interfaceId) external pure returns (bool);
    function symbol() external view returns (string memory);
    function totalSupply() external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);

    function __constructor__(address _l2Bridge, address _l1Token, string memory _name, string memory _symbol) external;
}
