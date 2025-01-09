// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { WETH98 } from "src/universal/WETH98.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

/// @custom:proxied true
/// @title ManagedWETH
contract ManagedWETH is OwnableUpgradeable, WETH98, ISemver {
    /// @notice Emitted when an unwrap is started.
    /// @param src The address that started the unwrap.
    /// @param wad The amount of WETH that was unwrapped.
    event Unwrap(address indexed src, uint256 wad);

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.1
    string public constant version = "1.0.0-beta.1";

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public config;

    constructor() {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _owner The address of the owner.
    /// @param _config Address of the SuperchainConfig contract.
    function initialize(address _owner, ISuperchainConfig _config) external initializer {
        __Ownable_init();
        _transferOwnership(_owner);
        config = _config;
    }

    /// @notice Withdraws an amount of ETH.
    /// @param _wad The amount of ETH to withdraw.
    function withdraw(uint256 _wad) public override {
        require(!config.paused(), "DelayedWETH: contract is paused");
        super.withdraw(_wad);
    }

    /// @notice Allows the owner to recover from error cases by pulling ETH out of the contract.
    /// @param _wad The amount of WETH to recover.
    function recover(uint256 _wad) external {
        require(msg.sender == owner(), "DelayedWETH: not owner");
        uint256 amount = _wad < address(this).balance ? _wad : address(this).balance;
        (bool success,) = payable(msg.sender).call{ value: amount }(hex"");
        require(success, "DelayedWETH: recover failed");
    }

    /// @notice Allows the owner to recover from error cases by pulling all WETH from a specific owner.
    /// @param _guy The address to recover the WETH from.
    function hold(address _guy) external {
        return hold(_guy, balanceOf(_guy));
    }

    /// @notice Allows the owner to recover from error cases by pulling a specific amount of WETH from a specific owner.
    /// @param _guy The address to recover the WETH from.
    /// @param _wad The amount of WETH to recover.
    function hold(address _guy, uint256 _wad) public {
        require(msg.sender == owner(), "DelayedWETH: not owner");
        _allowance[_guy][msg.sender] = _wad;
        emit Approval(_guy, msg.sender, _wad);
        transferFrom(_guy, msg.sender, _wad);
    }
}
