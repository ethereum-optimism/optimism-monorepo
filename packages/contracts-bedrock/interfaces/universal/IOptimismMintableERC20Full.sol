// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IOptimismMintableERC20Full {
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event Burn(address indexed account, uint256 amount);
    event Mint(address indexed account, uint256 amount);
    event Transfer(address indexed from, address indexed to, uint256 value);

    function BRIDGE() external view returns (address);
    function DOMAIN_SEPARATOR() external view returns (bytes32);
    function PERMIT2() external pure returns (address);
    function REMOTE_TOKEN() external view returns (address);
    function allowance(address _owner, address _spender) external view returns (uint256);
    function approve(address spender, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
    function bridge() external view returns (address);
    function burn(address _from, uint256 _amount) external;
    function decimals() external view returns (uint8);
    function decreaseAllowance(address spender, uint256 subtractedValue) external returns (bool);
    function increaseAllowance(address spender, uint256 addedValue) external returns (bool);
    function l1Token() external view returns (address);
    function l2Bridge() external view returns (address);
    function mint(address _to, uint256 _amount) external;
    function name() external view returns (string memory);
    function nonces(address owner) external view returns (uint256);
    function permit(address owner, address spender, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s)
        external;
    function remoteToken() external view returns (address);
    function supportsInterface(bytes4 _interfaceId) external pure returns (bool);
    function symbol() external view returns (string memory);
    function totalSupply() external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function version() external view returns (string memory);

    function __constructor__(address _bridge, address _remoteToken, string memory _name, string memory _symbol, uint8 _decimals) external;
}
