// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { IOptimismSuperchainERC20 } from "src/L2/interfaces/IOptimismSuperchainERC20.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { SuperchainERC20 } from "src/L2/SuperchainERC20.sol";
import { Initializable } from "@openzeppelin/contracts-v5/proxy/utils/Initializable.sol";
import { ZeroAddress, Unauthorized } from "src/libraries/errors/CommonErrors.sol";

/// @custom:proxied true
/// @title OptimismSuperchainERC20
/// @notice OptimismSuperchainERC20 is a standard extension of the base ERC20 token contract that unifies ERC20 token
///         bridging to make it fungible across the Superchain. This construction allows the L2StandardBridge to burn
///         and mint tokens. This makes it possible to convert a valid OptimismMintableERC20 token to a
///         OptimismSuperchainERC20 token, turning it fungible and interoperable across the superchain. Likewise, it
///         also enables the inverse conversion path.
///         Moreover, it builds on top of the L2ToL2CrossDomainMessenger for both replay protection and domain binding.
contract OptimismSuperchainERC20 is SuperchainERC20, Initializable {
    /// @notice Emitted whenever tokens are minted for an account.
    /// @param to Address of the account tokens are being minted for.
    /// @param amount  Amount of tokens minted.
    event Mint(address indexed to, uint256 amount);

    /// @notice Emitted whenever tokens are burned from an account.
    /// @param from Address of the account tokens are being burned from.
    /// @param amount  Amount of tokens burned.
    event Burn(address indexed from, uint256 amount);

    /// @notice Storage slot that the OptimismSuperchainERC20Metadata struct is stored at.
    /// keccak256(abi.encode(uint256(keccak256("optimismSuperchainERC20.metadata")) - 1)) & ~bytes32(uint256(0xff));
    bytes32 internal constant OPTIMISM_SUPERCHAIN_ERC20_METADATA_SLOT =
        0x07f04e84143df95a6373fcf376312ae41da81a193a3089073a54f47a74d8fb00;

    /// @notice Storage struct for the OptimismSuperchainERC20 metadata.
    /// @custom:storage-location erc7201:optimismSuperchainERC20.metadata
    struct OptimismSuperchainERC20Metadata {
        /// @notice Address of the corresponding version of this token on the remote chain.
        address remoteToken;
        /// @notice Name of the token
        string name;
        /// @notice Symbol of the token
        string symbol;
        /// @notice Decimals of the token
        uint8 decimals;
    }

    /// @notice Returns the storage for the OptimismSuperchainERC20Metadata.
    function _getStorage() private pure returns (OptimismSuperchainERC20Metadata storage storage_) {
        assembly {
            storage_.slot := OPTIMISM_SUPERCHAIN_ERC20_METADATA_SLOT
        }
    }

    /// @notice A modifier that only allows the L2StandardBridge to call
    modifier onlyL2StandardBridge() {
        if (msg.sender != Predeploys.L2_STANDARD_BRIDGE) revert Unauthorized();
        _;
    }

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.9
    string public constant override version = "1.0.0-beta.9";

    /// @notice Constructs the OptimismSuperchainERC20 contract.
    constructor() {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _remoteToken    Address of the corresponding remote token.
    /// @param _name           ERC20 name.
    /// @param _symbol         ERC20 symbol.
    /// @param _decimals       ERC20 decimals.
    function initialize(
        address _remoteToken,
        string memory _name,
        string memory _symbol,
        uint8 _decimals
    )
        external
        initializer
    {
        OptimismSuperchainERC20Metadata storage _storage = _getStorage();
        _storage.remoteToken = _remoteToken;
        _storage.name = _name;
        _storage.symbol = _symbol;
        _storage.decimals = _decimals;
    }

    /// @notice Allows the L2StandardBridge to mint tokens.
    /// @param _to     Address to mint tokens to.
    /// @param _amount Amount of tokens to mint.
    function mint(address _to, uint256 _amount) external virtual onlyL2StandardBridge {
        if (_to == address(0)) revert ZeroAddress();

        _mint(_to, _amount);

        emit Mint(_to, _amount);
    }

    /// @notice Allows the L2StandardBridge to burn tokens.
    /// @param _from   Address to burn tokens from.
    /// @param _amount Amount of tokens to burn.
    function burn(address _from, uint256 _amount) external virtual onlyL2StandardBridge {
        if (_from == address(0)) revert ZeroAddress();

        _burn(_from, _amount);

        emit Burn(_from, _amount);
    }

    /// @notice Returns the address of the corresponding version of this token on the remote chain.
    function remoteToken() public view returns (address) {
        return _getStorage().remoteToken;
    }

    /// @notice Returns the name of the token.
    function name() public view virtual override returns (string memory) {
        return _getStorage().name;
    }

    /// @notice Returns the symbol of the token.
    function symbol() public view virtual override returns (string memory) {
        return _getStorage().symbol;
    }

    /// @notice Returns the number of decimals used to get its user representation.
    /// For example, if `decimals` equals `2`, a balance of `505` tokens should
    /// be displayed to a user as `5.05` (`505 / 10 ** 2`).
    /// NOTE: This information is only used for _display_ purposes: it in
    /// no way affects any of the arithmetic of the contract, including
    /// {IERC20-balanceOf} and {IERC20-transfer}.
    function decimals() public view override returns (uint8) {
        return _getStorage().decimals;
    }

    /// @notice ERC165 interface check function.
    /// @param _interfaceId Interface ID to check.
    /// @return Whether or not the interface is supported by this contract.
    function supportsInterface(bytes4 _interfaceId) public view virtual override returns (bool) {
        return _interfaceId == type(IOptimismSuperchainERC20).interfaceId || super.supportsInterface(_interfaceId);
    }

    /// @notice Sets Permit2 contract's allowance at infinity.
    function _givePermit2InfiniteAllowance() internal view virtual override returns (bool) {
        return true;
    }
}
