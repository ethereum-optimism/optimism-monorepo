// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Libraries
import { SignedMath } from "@openzeppelin/contracts/utils/math/SignedMath.sol";
import { FixedPointMathLib } from "@rari-capital/solmate/src/utils/FixedPointMathLib.sol";

/// @title Arithmetic
/// @notice Even more math than before.
library Arithmetic {
    /// @notice Clamps a value between a minimum and maximum.
    /// @param _value The value to clamp.
    /// @param _min   The minimum value.
    /// @param _max   The maximum value.
    /// @return The clamped value.
    function clamp(int256 _value, int256 _min, int256 _max) internal pure returns (int256) {
        return SignedMath.min(SignedMath.max(_value, _min), _max);
    }

    /// @notice (c)oefficient (d)enominator (exp)onentiation function.
    ///         Returns the result of: c * (1 - 1/d)^exp.
    /// @param _coefficient Coefficient of the function.
    /// @param _denominator Fractional denominator.
    /// @param _exponent    Power function exponent.
    /// @return Result of c * (1 - 1/d)^exp.
    function cdexp(int256 _coefficient, int256 _denominator, int256 _exponent) internal pure returns (int256) {
        return (_coefficient * (FixedPointMathLib.powWad(1e18 - (1e18 / _denominator), _exponent * 1e18))) / 1e18;
    }

    /// @notice Saturating addition.
    /// @param _x The first value.
    /// @param _y The second value.
    /// @return z_ The sum of the two values, or the maximum value if the sum overflows.
    /// @dev Returns `min(2 ** 256 - 1, x + y)`.
    /// @dev Taken from Solady
    /// https://github.com/Vectorized/solady/blob/63416d60c78aba70a12ca1b3c11125d1061caa12/src/utils/FixedPointMathLib.sol#L673
    function saturatingAdd(uint256 _x, uint256 _y) internal pure returns (uint256 z_) {
        /// @solidity memory-safe-assembly
        assembly {
            z_ := or(sub(0, lt(add(_x, _y), _x)), add(_x, _y))
        }
    }

    /// @notice Saturating multiplication.
    /// @param _x The first value.
    /// @param _y The second value.
    /// @return z_ The product of the two values, or the maximum value if the product overflows.
    /// @dev Returns `min(2 ** 256 - 1, x * y).
    /// @dev Taken from Solady
    /// https://github.com/Vectorized/solady/blob/63416d60c78aba70a12ca1b3c11125d1061caa12/src/utils/FixedPointMathLib.sol#L681
    function saturatingMul(uint256 _x, uint256 _y) internal pure returns (uint256 z_) {
        /// @solidity memory-safe-assembly
        assembly {
            z_ := or(sub(or(iszero(_x), eq(div(mul(_x, _y), _x), _y)), 1), mul(_x, _y))
        }
    }
}
