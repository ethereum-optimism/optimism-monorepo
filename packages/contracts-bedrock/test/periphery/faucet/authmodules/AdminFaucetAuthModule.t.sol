// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { Test } from "forge-std/Test.sol";
import { AdminFaucetAuthModule } from "src/periphery/faucet/authmodules/AdminFaucetAuthModule.sol";
import { Faucet } from "src/periphery/faucet/Faucet.sol";
import { FaucetHelper } from "test/mocks/FaucetHelper.sol";

/// @title  AdminFaucetAuthModuleTest
/// @notice Tests the AdminFaucetAuthModule contract.
contract AdminFaucetAuthModuleTest is Test {
    /// @notice The admin of the `AdminFaucetAuthModule` contract.
    address internal admin;

    /// @notice Private key of the `admin`.
    uint256 internal adminKey;

    /// @notice Not an admin of the `AdminFaucetAuthModule` contract.
    address internal nonAdmin;

    /// @notice Private key of the `nonAdmin`.
    uint256 internal nonAdminKey;

    /// @notice An instance of the `AdminFaucetAuthModule` contract.
    AdminFaucetAuthModule internal adminFam;

    /// @notice An instance of the `FaucetHelper` contract.
    FaucetHelper internal faucetHelper;

    string internal adminFamName = "AdminFAM";

    string internal adminFamVersion = "1";

    /// @notice Deploy the `AdminFaucetAuthModule` contract.
    function setUp() external {
        adminKey = 0xB0B0B0B0;
        admin = vm.addr(adminKey);

        nonAdminKey = 0xC0C0C0C0;
        nonAdmin = vm.addr(nonAdminKey);

        adminFam = new AdminFaucetAuthModule(admin, adminFamName, adminFamVersion);

        faucetHelper = new FaucetHelper();
    }

    /// @notice Get signature as a bytes blob.
    function _getSignature(uint256 _signingPrivateKey, bytes32 _digest) internal pure returns (bytes memory) {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(_signingPrivateKey, _digest);

        bytes memory signature = abi.encodePacked(r, s, v);
        return signature;
    }

    /// @notice Signs a proof with the given private key and returns the signature using
    ///         the given EIP712 domain separator. This assumes that the issuer's address is the
    ///        corresponding public key to _issuerPrivateKey.
    function issueProofWithEIP712Domain(
        uint256 _issuerPrivateKey,
        bytes memory _eip712Name,
        bytes memory _contractVersion,
        uint256 _eip712Chainid,
        address _eip712VerifyingContract,
        address recipient,
        bytes32 id,
        bytes32 nonce
    )
        internal
        view
        returns (bytes memory)
    {
        AdminFaucetAuthModule.Proof memory proof = AdminFaucetAuthModule.Proof(recipient, nonce, id);
        return _getSignature(
            _issuerPrivateKey,
            faucetHelper.getDigestWithEIP712Domain(
                proof, _eip712Name, _contractVersion, _eip712Chainid, _eip712VerifyingContract
            )
        );
    }

    /// @notice Assert that verify returns true for valid proofs signed by admins.
    function test_adminProof_verify_succeeds() external {
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        address fundsReceiver = makeAddr("fundsReceiver");
        bytes memory proof = issueProofWithEIP712Domain(
            adminKey,
            bytes(adminFamName),
            bytes(adminFamVersion),
            block.chainid,
            address(adminFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        vm.prank(nonAdmin);
        assertEq(
            adminFam.verify(
                Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
                keccak256(abi.encodePacked(fundsReceiver)),
                proof
            ),
            true
        );
    }

    /// @notice Assert that verify returns false for proofs signed by nonadmins.
    function test_nonAdminProof_verify_succeeds() external {
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        address fundsReceiver = makeAddr("fundsReceiver");
        bytes memory proof = issueProofWithEIP712Domain(
            nonAdminKey,
            bytes(adminFamName),
            bytes(adminFamVersion),
            block.chainid,
            address(adminFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        vm.prank(admin);
        assertEq(
            adminFam.verify(
                Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
                keccak256(abi.encodePacked(fundsReceiver)),
                proof
            ),
            false
        );
    }

    /// @notice Assert that verify returns false for proofs where the id in the proof is different
    ///         than the id in the call to verify.
    function test_proofWithWrongId_verify_succeeds() external {
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        address fundsReceiver = makeAddr("fundsReceiver");
        address randomAddress = makeAddr("randomAddress");
        bytes memory proof = issueProofWithEIP712Domain(
            adminKey,
            bytes(adminFamName),
            bytes(adminFamVersion),
            block.chainid,
            address(adminFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        vm.prank(admin);
        assertEq(
            adminFam.verify(
                Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
                keccak256(abi.encodePacked(randomAddress)),
                proof
            ),
            false
        );
    }
}
