// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { ILegacyMintableERC20 } from "src/universal/OptimismMintableERC20.sol";

/// @title LegacyMintableERC20
/// @notice The legacy implementation of the OptimismMintableERC20. This
///         contract is deprecated and should no longer be used.
contract LegacyMintableERC20 is ERC20 {
    /// @notice Emitted when the token is minted by the bridge.
    event Mint(address indexed _account, uint256 _amount);

    /// @notice Emitted when a token is burned by the bridge.
    event Burn(address indexed _account, uint256 _amount);

    /// @notice The token on the remote domain.
    address public l1Token;

    /// @notice The local bridge.
    address public l2Bridge;

    /// @param _l2Bridge Address of the L2 standard bridge.
    /// @param _l1Token Address of the corresponding L1 token.
    /// @param _name ERC20 name.
    /// @param _symbol ERC20 symbol.
    constructor(
        address _l2Bridge,
        address _l1Token,
        string memory _name,
        string memory _symbol
    )
        ERC20(_name, _symbol)
    {
        l1Token = _l1Token;
        l2Bridge = _l2Bridge;
    }

    /// @notice Modifier that requires the contract was called by the bridge.
    modifier onlyL2Bridge() {
        require(msg.sender == l2Bridge, "Only L2 Bridge can mint and burn");
        _;
    }

    /// @notice EIP165 implementation.
    function supportsInterface(bytes4 _interfaceId) public pure returns (bool) {
        bytes4 firstSupportedInterface = bytes4(keccak256("supportsInterface(bytes4)")); // ERC165
        bytes4 secondSupportedInterface = ILegacyMintableERC20.l1Token.selector ^ ILegacyMintableERC20.mint.selector
            ^ ILegacyMintableERC20.burn.selector;
        return _interfaceId == firstSupportedInterface || _interfaceId == secondSupportedInterface;
    }

    /// @notice Only the bridge can mint tokens.
    /// @param _to     The account receiving tokens.
    /// @param _amount The amount of tokens to receive.
    function mint(address _to, uint256 _amount) public virtual onlyL2Bridge {
        _mint(_to, _amount);

        emit Mint(_to, _amount);
    }

    /// @notice Only the bridge can burn tokens.
    /// @param _from   The account having tokens burnt.
    /// @param _amount The amount of tokens being burnt.
    function burn(address _from, uint256 _amount) public virtual onlyL2Bridge {
        _burn(_from, _amount);

        emit Burn(_from, _amount);
    }
}
