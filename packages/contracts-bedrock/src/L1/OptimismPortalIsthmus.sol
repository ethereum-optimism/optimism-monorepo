// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { OptimismPortal2 } from "src/L1/OptimismPortal2.sol";

/// @custom:proxied true
/// @title OptimismPortalIsthmus
/// @notice The OptimismPortal is a low-level contract responsible for passing messages between L1
///         and L2. Messages sent directly to the OptimismPortal have no form of replayability.
///         Users are encouraged to use the L1CrossDomainMessenger for a higher-level interface.
contract OptimismPortalIsthmus is OptimismPortal2 {
    /// @notice Version of the deposit event.
    uint256 internal constant DEPOSIT_VERSION_1 = 1;

    /// @notice The storage slot that holds the deposit nonce.
    /// @dev `bytes32(uint256(keccak256('optimismportal.depositnonce')) - 1)`
    bytes32 public constant DEPOSIT_NONCE_SLOT = 0xfbdb6804978a124792ffaccc985bc1ad9ad7a5b3ff3fc4eb6936a7c373b67089;

    constructor(
        uint256 _proofMaturityDelaySeconds,
        uint256 _disputeGameFinalityDelaySeconds
    )
        OptimismPortal2(_proofMaturityDelaySeconds, _disputeGameFinalityDelaySeconds)
    { }

    /// @custom:semver +isthmus-beta.1
    function version() public pure virtual override returns (string memory) {
        return string.concat(super.version(), "+isthmus-beta.1");
    }

    /// @notice Nonce incremented for each TransactionDeposited event
    function depositNonce() public view returns (uint64 nonce_) {
        assembly {
            nonce_ := sload(DEPOSIT_NONCE_SLOT)
        }
    }

    function _transactionDepositedNonceAndVersion() internal virtual override returns (uint256) {
        uint64 nonce = depositNonce() + 1;
        assembly {
            sstore(DEPOSIT_NONCE_SLOT, nonce)
        }
        return uint256(nonce) << 128 | DEPOSIT_VERSION_1;
    }
}
