// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { Test } from "forge-std/Test.sol";
import { CheckGelatoLow, IGelatoTreasury } from "src/periphery/drippie/dripchecks/CheckGelatoLow.sol";

/// @title  MockGelatoTreasury
/// @notice Mocks the Gelato treasury for testing purposes. Allows arbitrary setting of balances.
contract MockGelatoTreasury is IGelatoTreasury {
    mapping(address => mapping(address => uint256)) private totalDeposited;
    mapping(address => mapping(address => uint256)) private totalWithdrawn;

    function totalDepositedAmount(address _user, address _token) external view override returns (uint256) {
        return totalDeposited[_token][_user];
    }

    function totalWithdrawnAmount(address _user, address _token) external view override returns (uint256) {
        return totalWithdrawn[_token][_user];
    }

    function setTotalDepositedAmount(address _user, address _token, uint256 _amount) external {
        totalDeposited[_token][_user] = _amount;
    }
}

/// @title  CheckGelatoLowTest
/// @notice Tests the CheckGelatoLow contract via fuzzing both the success and failure cases.
contract CheckGelatoLowTest is Test {
    /// @notice An instance of the CheckGelatoLow contract.
    CheckGelatoLow c;

    /// @notice An instance of the MockGelatoTreasury contract.
    MockGelatoTreasury gelato;

    /// @notice The account Gelato uses to represent ether
    address internal constant eth = 0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE;

    /// @notice Deploy the `CheckGelatoLow` and `MockGelatoTreasury` contracts.
    function setUp() external {
        c = new CheckGelatoLow();
        gelato = new MockGelatoTreasury();
    }

    /// @notice Test that the `name` function returns the correct value.
    function test_name_succeeds() external view {
        assertEq(c.name(), "CheckGelatoLow");
    }

    /// @notice Fuzz the `check` function and assert that it always returns true
    ///         when the user's balance in the treasury is less than the threshold.
    function testFuzz_check_succeeds(uint256 _threshold, address _recipient) external view {
        CheckGelatoLow.Params memory p =
            CheckGelatoLow.Params({ treasury: address(gelato), threshold: _threshold, recipient: _recipient });

        vm.assume(
            gelato.totalDepositedAmount(_recipient, eth) - gelato.totalWithdrawnAmount(_recipient, eth) < _threshold
        );

        assertEq(c.check(abi.encode(p)), true);
    }

    /// @notice Fuzz the `check` function and assert that it always returns false
    ///         when the user's balance in the treasury is greater than or equal
    ///         to the threshold.
    function testFuzz_check_highBalance_fails(uint256 _threshold, address _recipient) external {
        CheckGelatoLow.Params memory p =
            CheckGelatoLow.Params({ treasury: address(gelato), threshold: _threshold, recipient: _recipient });

        gelato.setTotalDepositedAmount(_recipient, eth, _threshold);

        assertEq(c.check(abi.encode(p)), false);
    }
}
