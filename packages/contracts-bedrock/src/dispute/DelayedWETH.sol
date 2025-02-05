// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { WETH98 } from "src/universal/WETH98.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

/// @custom:proxied true
/// @title DelayedWETH
/// @notice DelayedWETH is an extension to WETH9 that allows for delayed withdrawals. Accounts must trigger an unlock
///         function before they can withdraw WETH. Accounts must trigger unlock by specifying a sub-account and an
///         amount of WETH to unlock. Accounts can trigger the unlock function at any time, but must wait a delay
///         period before they can withdraw after the unlock function is triggered. DelayedWETH is designed to be used
///         by the DisputeGame contracts where unlock will only be triggered after a dispute is resolved. DelayedWETH
///         is meant to sit behind a proxy contract and has an owner address that can pull WETH from any account and
///         can recover ETH from the contract itself. Variable and function naming vaguely follows the vibe of WETH9.
///         Not the prettiest contract in the world, but it gets the job done.
contract DelayedWETH is OwnableUpgradeable, WETH98, ISemver {
    /// @notice Represents a withdrawal request.
    struct WithdrawalRequest {
        uint256 amount;
        uint256 timestamp;
    }

    /// @notice Semantic version.
    /// @custom:semver 1.3.0
    string public constant version = "1.3.0";

    /// @notice Returns a withdrawal request for the given address.
    mapping(address => mapping(address => WithdrawalRequest)) public withdrawals;

    /// @notice Withdrawal delay in seconds.
    uint256 internal immutable DELAY_SECONDS;

    /// @notice Address of the SuperchainConfig contract.
    ISuperchainConfig public config;

    /// @param _delay The delay for withdrawals in seconds.
    constructor(uint256 _delay) {
        DELAY_SECONDS = _delay;
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

    /// @notice Returns the withdrawal delay in seconds.
    /// @return The withdrawal delay in seconds.
    function delay() external view returns (uint256) {
        return DELAY_SECONDS;
    }

    /// @notice Unlocks withdrawals for the sender's account, after a time delay.
    /// @param _guy Sub-account to unlock.
    /// @param _wad The amount of WETH to unlock.
    function unlock(address _guy, uint256 _wad) external {
        // Note that the unlock function can be called by any address, but the actual unlocking capability still only
        // gives the msg.sender the ability to withdraw from the account. As long as the unlock and withdraw functions
        // are called with the proper recipient addresses, this will be safe. Could be made safer by having external
        // accounts execute withdrawals themselves but that would have added extra complexity and made DelayedWETH a
        // leaky abstraction, so we chose this instead.
        WithdrawalRequest storage wd = withdrawals[msg.sender][_guy];
        wd.timestamp = block.timestamp;
        wd.amount += _wad;
    }

    /// @notice Withdraws an amount of ETH.
    /// @param _wad The amount of ETH to withdraw.
    function withdraw(uint256 _wad) public override {
        withdraw(msg.sender, _wad);
    }

    /// @notice Extension to withdrawal, must provide a sub-account to withdraw from.
    /// @param _guy Sub-account to withdraw from.
    /// @param _wad The amount of WETH to withdraw.
    function withdraw(address _guy, uint256 _wad) public {
        require(!config.paused(), "DelayedWETH: contract is paused");
        WithdrawalRequest storage wd = withdrawals[msg.sender][_guy];
        require(wd.amount >= _wad, "DelayedWETH: insufficient unlocked withdrawal");
        require(wd.timestamp > 0, "DelayedWETH: withdrawal not unlocked");
        require(wd.timestamp + DELAY_SECONDS <= block.timestamp, "DelayedWETH: withdrawal delay not met");
        wd.amount -= _wad;
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
        hold(_guy, balanceOf(_guy));
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
