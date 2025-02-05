// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { ReentrancyGuard } from "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// Libraries
import { SafeERC20 } from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

/// @title  TransferOnion
/// @notice TransferOnion is a hash onion for distributing tokens. The shell commits
///         to an ordered list of the token transfers and can be permissionlessly
///         unwrapped in order. The SENDER must `approve` this contract as
///         `transferFrom` is used to move the token balances.
contract TransferOnion is ReentrancyGuard {
    using SafeERC20 for ERC20;

    /// @notice Struct representing a layer of the onion.
    struct Layer {
        address recipient;
        uint256 amount;
        bytes32 shell;
    }

    /// @notice Address of the token to distribute.
    ERC20 public immutable TOKEN;

    /// @notice Address of the account to distribute tokens from.
    address public immutable SENDER;

    /// @notice Current shell hash.
    bytes32 public shell;

    /// @notice Constructs a new TransferOnion.
    /// @param _token  Address of the token to distribute.
    /// @param _sender Address of the sender to distribute from.
    /// @param _shell  Initial shell of the onion.
    constructor(ERC20 _token, address _sender, bytes32 _shell) {
        TOKEN = _token;
        SENDER = _sender;
        shell = _shell;
    }

    /// @notice Peels layers from the onion and distributes tokens.
    /// @param _layers Array of onion layers to peel.
    function peel(Layer[] memory _layers) public nonReentrant {
        bytes32 tempShell = shell;
        uint256 length = _layers.length;
        for (uint256 i = 0; i < length;) {
            Layer memory layer = _layers[i];

            // Confirm that the onion layer is correct.
            require(
                keccak256(abi.encode(layer.recipient, layer.amount, layer.shell)) == tempShell,
                "TransferOnion: what are you doing in my swamp?"
            );

            // Update the onion layer.
            tempShell = layer.shell;

            // Transfer the tokens.
            // slither-disable-next-line arbitrary-send-erc20
            TOKEN.safeTransferFrom(SENDER, layer.recipient, layer.amount);

            // Unchecked increment to save some gas.
            unchecked {
                ++i;
            }
        }

        shell = tempShell;
    }
}
