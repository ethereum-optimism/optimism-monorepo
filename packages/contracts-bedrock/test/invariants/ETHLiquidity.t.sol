// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { StdUtils } from "forge-std/Test.sol";
import { Vm } from "forge-std/Vm.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { IETHLiquidity } from "interfaces/L2/IETHLiquidity.sol";

/// @title ETHLiquidity_User
/// @notice Actor contract that interacts with the ETHLiquidity contract. Always pretends to be the
///         SuperchainWETH contract since it's the only contract that can use ETHLiquidity.
contract ETHLiquidity_User is StdUtils {
    /// @notice Flag to indicate if the test has failed.
    bool public failed = false;

    /// @notice The Vm contract.
    Vm internal vm;

    /// @notice The ETHLiquidity contract.
    IETHLiquidity internal liquidity;

    /// @param _vm The Vm contract.
    /// @param _liquidity The ETHLiquidity contract.
    /// @param _balance The initial balance of the contract.
    constructor(Vm _vm, IETHLiquidity _liquidity, uint256 _balance) {
        vm = _vm;
        liquidity = _liquidity;
        vm.deal(Predeploys.SUPERCHAIN_WETH, _balance);
    }

    /// @notice Mint ETH liquidity.
    /// @param _amount The amount of ETH to mint.
    function mint(uint256 _amount) public {
        vm.prank(Predeploys.SUPERCHAIN_WETH);
        liquidity.mint(_amount);
    }

    /// @notice Burn ETH liquidity.
    /// @param _amount The amount of ETH to burn.
    function burn(uint256 _amount) public {
        vm.prank(Predeploys.SUPERCHAIN_WETH);
        liquidity.burn{ value: _amount }();
    }
}

/// @title ETHLiquidity_MintBurn_Invariant
/// @notice Invariant that checks that minting/burning ETH liquidity does not cause the actor's
///         balance to magically increase beyond the starting balance.
contract ETHLiquidity_MintBurn_Invariant is CommonTest {
    /// @notice Starting balance of the contract.
    uint256 internal constant STARTING_BALANCE = type(uint248).max;

    /// @notice The ETHLiquidity_User actor.
    ETHLiquidity_User internal actor;

    /// @notice Test setup.
    function setUp() public override {
        super.enableInterop();
        super.setUp();

        // Create a new ETHLiquidity_User actor.
        actor = new ETHLiquidity_User(vm, ethLiquidity, STARTING_BALANCE);

        // Set the target contract.
        targetContract(address(actor));

        // Set the target selectors.
        bytes4[] memory selectors = new bytes4[](2);
        selectors[0] = actor.mint.selector;
        selectors[1] = actor.burn.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(actor), selectors: selectors });
        targetSelector(selector);
    }

    /// @notice Invariant that checks that repeatedly minting/burning does not cause the actor's
    ///         balance to increase beyond the starting balance.
    /// @custom:invariant Calls to mint/burn repeatedly should never cause the actor's balance to
    ///                   increase beyond the starting balance.
    function invariant_mintburn_maintainsBalance() public view {
        // Assert that the actor's balance has not somehow increased.
        assertLe(address(actor).balance, type(uint248).max);
    }
}
