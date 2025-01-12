// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { SystemConfig } from "src/L1/SystemConfig.sol";

/// @custom:proxied true
/// @title SystemConfigJovian
/// @notice The SystemConfig contract is used to manage configuration of an Optimism network.
///         All configuration is stored on L1 and picked up by L2 as part of the derviation of
///         the L2 chain.
contract SystemConfigJovian is SystemConfig {
    /// @notice Version identifier, used for upgrades.
    uint256 public constant VERSION_1 = 1;

    /// @notice The storage slot that holds the deposit nonce.
    /// @dev `bytes32(uint256(keccak256('systemconfig.configupdatenonce')) - 1)`
    bytes32 public constant CONFIG_UPDATE_NONCE_SLOT =
        0x93fcce48e210616d14f0f2849f0028a91b366cdf4152de896d874c59cb47c5ee;

    /// @custom:semver +jovian-beta.1
    function version() public pure virtual override returns (string memory) {
        return string.concat(super.version(), "+jovian-beta.1");
    }

    /// @notice Nonce incremented for each ConfigUpdate event
    function configUpdateNonce() public view returns (uint64 nonce_) {
        assembly {
            nonce_ := sload(CONFIG_UPDATE_NONCE_SLOT)
        }
    }

    function _configUpdateNonceAndVersion() internal virtual override returns (uint256) {
        uint64 nonce = configUpdateNonce() + 1;
        assembly {
            sstore(CONFIG_UPDATE_NONCE_SLOT, nonce)
        }
        return uint256(nonce) << 128 | VERSION_1;
    }
}
