// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { StdUtils } from "forge-std/Test.sol";
import { Vm } from "forge-std/Vm.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Interfaces
import { ISuperchainWETH } from "src/L2/interfaces/ISuperchainWETH.sol";

/// @title SuperchainWETH_User
/// @notice Actor contract that interacts with the SuperchainWETH contract.
contract SuperchainWETH_User is StdUtils {
    /// @notice Flag to indicate if the test has failed.
    bool public failed = false;

    /// @notice The Vm contract.
    Vm internal vm;

    /// @notice The SuperchainWETH contract.
    ISuperchainWETH internal weth;

    /// @param _vm The Vm contract.
    /// @param _weth The SuperchainWETH contract.
    /// @param _balance The initial balance of the contract.
    constructor(Vm _vm, ISuperchainWETH _weth, uint256 _balance) {
        vm = _vm;
        weth = _weth;
        vm.deal(address(this), _balance);
    }

    /// @notice Allow the contract to receive ETH.
    receive() external payable { }

    /// @notice Deposit ETH into the contract.
    /// @param _amount The amount of ETH to deposit.
    function deposit(uint256 _amount) public {
        // Bound deposit amount to our ETH balance.
        _amount = bound(_amount, 0, address(this).balance);

        // Deposit the amount.
        try weth.deposit{ value: _amount }() {
            // Success.
        } catch {
            failed = true;
        }
    }

    /// @notice Withdraw ETH from the contract.
    /// @param _amount The amount of ETH to withdraw.
    function withdraw(uint256 _amount) public {
        // Bound withdraw amount to our WETH balance.
        _amount = bound(_amount, 0, weth.balanceOf(address(this)));

        // Withdraw the amount.
        try weth.withdraw(_amount) {
            // Success.
        } catch {
            failed = true;
        }
    }
}

/// @title SuperchainWETH_SendSucceeds_Invariant
/// @notice Invariant test that checks that sending WETH always succeeds if the actor has a
///         sufficient balance to do so and that the actor's balance does not increase out of
///         nowhere.
contract SuperchainWETH_SendSucceeds_Invariant is CommonTest {
    /// @notice Starting balance of the contract.
    uint256 internal constant STARTING_BALANCE = type(uint248).max;

    /// @notice The SuperchainWETH_User actor.
    SuperchainWETH_User internal actor;

    /// @notice Test setup.
    function setUp() public override {
        super.enableInterop();
        super.setUp();

        // Create a new SuperchainWETH_User actor.
        actor = new SuperchainWETH_User(vm, superchainWeth, STARTING_BALANCE);

        // Set the target contract.
        targetContract(address(actor));

        // Set the target selectors.
        bytes4[] memory selectors = new bytes4[](2);
        selectors[0] = actor.deposit.selector;
        selectors[1] = actor.withdraw.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @notice Invariant that checks that sending WETH always succeeds.
    /// @custom:invariant Calls to sendERC20 should always succeed as long as the actor has less
    ///                   than uint248 wei which is much greater than the total ETH supply. Actor's
    ///                   balance should also not increase out of nowhere.
    function invariant_sendERC20_succeeds() public view {
        // Assert that the actor has not failed to send WETH.
        assertEq(actor.failed(), false);

        // Assert that the actor's balance has not somehow increased.
        assertLe(address(actor).balance, STARTING_BALANCE);
    }
}
