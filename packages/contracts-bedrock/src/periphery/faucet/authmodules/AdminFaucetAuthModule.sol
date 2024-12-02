// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { EIP712 } from "@openzeppelin/contracts/utils/cryptography/draft-EIP712.sol";
import { SignatureChecker } from "@openzeppelin/contracts/utils/cryptography/SignatureChecker.sol";

// Interfaces
import { IFaucetAuthModule } from "src/periphery/faucet/authmodules/IFaucetAuthModule.sol";
import { Faucet } from "src/periphery/faucet/Faucet.sol";

/// @title  AdminFaucetAuthModule
/// @notice FaucetAuthModule that allows an admin to sign off on a given faucet drip. Takes an admin
///         as the constructor argument.
contract AdminFaucetAuthModule is IFaucetAuthModule, EIP712 {
    /// @notice Admin address that can sign off on drips.
    address public immutable ADMIN;

    /// @notice EIP712 typehash for the Proof type.
    bytes32 public immutable PROOF_TYPEHASH = keccak256("Proof(address recipient,bytes32 nonce,bytes32 id)");

    /// @notice Struct that represents a proof that verifies the admin.
    /// @custom:field recipient Address that will be receiving the faucet funds.
    /// @custom:field nonce     Pseudorandom nonce to prevent replay attacks.
    /// @custom:field id        id for the user requesting the faucet funds.
    struct Proof {
        address recipient;
        bytes32 nonce;
        bytes32 id;
    }

    /// @param _admin   Admin address that can sign off on drips.
    /// @param _name    Contract name.
    /// @param _version The current major version of the signing domain.
    constructor(address _admin, string memory _name, string memory _version) EIP712(_name, _version) {
        ADMIN = _admin;
    }

    /// @inheritdoc IFaucetAuthModule
    function verify(
        Faucet.DripParameters memory _params,
        bytes32 _id,
        bytes memory _proof
    )
        external
        view
        returns (bool valid_)
    {
        // Generate a EIP712 typed data hash to compare against the proof.
        valid_ = SignatureChecker.isValidSignatureNow(
            ADMIN,
            _hashTypedDataV4(keccak256(abi.encode(PROOF_TYPEHASH, _params.recipient, _params.nonce, _id))),
            _proof
        );
    }
}
