// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IERC20Solady {
    /// @dev The total supply has overflowed.
    error TotalSupplyOverflow();

    /// @dev The allowance has overflowed.
    error AllowanceOverflow();

    /// @dev The allowance has underflowed.
    error AllowanceUnderflow();

    /// @dev Insufficient balance.
    error InsufficientBalance();

    /// @dev Insufficient allowance.
    error InsufficientAllowance();

    /// @dev The permit is invalid.
    error InvalidPermit();

    /// @dev The permit has expired.
    error PermitExpired();

    /// @dev The allowance of Permit2 is fixed at infinity.
    error Permit2AllowanceIsFixedAtInfinity();

    /// @dev Emitted when `amount` tokens is transferred from `from` to `to`.
    event Transfer(address indexed from, address indexed to, uint256 amount);

    /// @dev Emitted when `amount` tokens is approved by `owner` to be used by `spender`.
    event Approval(address indexed owner, address indexed spender, uint256 amount);

    /// @dev Returns the name of the token.
    function name() external view returns (string memory);

    /// @dev Returns the symbol of the token.
    function symbol() external view returns (string memory);

    /// @dev Returns the decimals places of the token.
    function decimals() external view returns (uint8);

    /// @dev Returns the amount of tokens in existence.
    function totalSupply() external view returns (uint256 result);

    /// @dev Returns the amount of tokens owned by `owner`
    function balanceOf(address owner) external view returns (uint256 result);

    /// @dev Returns the amount of tokens that `spender` can spend on behalf of `owner`.
    function allowance(address owner, address spender) external view returns (uint256 result);

    /// @dev Sets `amount` as the allowance of `spender` over the caller's tokens.
    ///
    /// Emits a {Approval} event.
    function approve(address spender, uint256 amount) external returns (bool);

    /// @dev Transfer `amount` tokens from the caller to `to`.
    ///
    /// Requirements:
    /// - `from` must at least have `amount`.
    ///
    /// Emits a {Transfer} event.
    function transfer(address to, uint256 amount) external returns (bool);

    /// @dev Transfers `amount` tokens from `from` to `to`.
    ///
    /// Note: Does not update the allowance if it is the maximum uint256 value.
    ///
    /// Requirements:
    /// - `from` must at least have `amount`.
    /// - The caller must have at least `amount` of allowance to transfer the tokens of `from`.
    ///
    /// Emits a {Transfer} event.
    function transferFrom(address from, address to, uint256 amount) external returns (bool);

    /// @dev Returns the current nonce for `owner`.
    /// This value is used to compute the signature for EIP-2612 permit.
    function nonces(address owner) external view returns (uint256 result);

    /// @dev Sets `value` as the allowance of `spender` over the tokens of `owner`,
    /// authorized by a signed approval by `owner`.
    ///
    /// Emits a {Approval} event.
    function permit(
        address owner,
        address spender,
        uint256 value,
        uint256 deadline,
        uint8 v,
        bytes32 r,
        bytes32 s
    )
        external;

    /// @dev Returns the EIP-712 domain separator for the EIP-2612 permit.
    function DOMAIN_SEPARATOR() external view returns (bytes32 result);
}
