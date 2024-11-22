// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { Faucet } from "src/periphery/faucet/Faucet.sol";
import { AdminFaucetAuthModule } from "src/periphery/faucet/authmodules/AdminFaucetAuthModule.sol";
import { FaucetHelper } from "test/mocks/FaucetHelper.sol";

contract Faucet_Initializer is Test {
    event Drip(string indexed authModule, bytes32 indexed userId, uint256 amount, address indexed recipient);

    address internal faucetContractAdmin;
    address internal faucetAuthAdmin;
    address internal nonAdmin;
    address internal fundsReceiver;
    uint256 internal faucetAuthAdminKey;
    uint256 internal nonAdminKey;
    uint256 internal startingTimestamp = 1000;

    Faucet faucet;
    AdminFaucetAuthModule optimistNftFam;
    string internal optimistNftFamName = "OptimistNftFam";
    string internal optimistNftFamVersion = "1";
    AdminFaucetAuthModule githubFam;
    string internal githubFamName = "GithubFam";
    string internal githubFamVersion = "1";

    FaucetHelper faucetHelper;

    function setUp() public {
        vm.warp(startingTimestamp);
        faucetContractAdmin = makeAddr("faucetContractAdmin");
        fundsReceiver = makeAddr("fundsReceiver");

        faucetAuthAdminKey = 0xB0B0B0B0;
        faucetAuthAdmin = vm.addr(faucetAuthAdminKey);

        nonAdminKey = 0xC0C0C0C0;
        nonAdmin = vm.addr(nonAdminKey);

        _initializeContracts();
    }

    /// @notice Instantiates a Faucet.
    function _initializeContracts() internal {
        faucet = new Faucet(faucetContractAdmin);

        // Fill faucet with ether.
        vm.deal(address(faucet), 10 ether);
        vm.deal(address(faucetContractAdmin), 5 ether);
        vm.deal(address(nonAdmin), 5 ether);

        optimistNftFam = new AdminFaucetAuthModule(faucetAuthAdmin, optimistNftFamName, optimistNftFamVersion);
        githubFam = new AdminFaucetAuthModule(faucetAuthAdmin, githubFamName, githubFamVersion);

        faucetHelper = new FaucetHelper();
    }

    function _enableFaucetAuthModules() internal {
        vm.startPrank(faucetContractAdmin);
        faucet.configure(optimistNftFam, Faucet.ModuleConfig("OptimistNftModule", true, 1 days, 1 ether));
        faucet.configure(githubFam, Faucet.ModuleConfig("GithubModule", true, 1 days, 0.05 ether));
        vm.stopPrank();
    }

    /// @notice Get signature as a bytes blob.
    function _getSignature(uint256 _signingPrivateKey, bytes32 _digest) internal pure returns (bytes memory) {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(_signingPrivateKey, _digest);

        bytes memory signature = abi.encodePacked(r, s, v);
        return signature;
    }

    /// @notice Signs a proof with the given private key and returns the signature using
    ///         the given EIP712 domain separator. This assumes that the issuer's address is the
    ///         corresponding public key to _issuerPrivateKey.
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
}

contract FaucetTest is Faucet_Initializer {
    function test_initialize_succeeds() external view {
        assertEq(faucet.ADMIN(), faucetContractAdmin);
    }

    function test_configure_whenAdmin_succeeds() external {
        vm.startPrank(faucetContractAdmin);
        faucet.configure(optimistNftFam, Faucet.ModuleConfig("OptimistNftModule", true, 1 days, 1 ether));

        (string memory name, bool enabled, uint256 ttl, uint256 amount) = faucet.modules(optimistNftFam);
        assertEq(name, "OptimistNftModule");
        assertEq(enabled, true);
        assertEq(ttl, 1 days);
        assertEq(amount, 1 ether);

        assertTrue(faucet.isModuleEnabled(optimistNftFam));
    }

    function test_configure_whenNotAdmin_reverts() external {
        vm.expectRevert("Faucet: function can only be called by admin");
        faucet.configure(optimistNftFam, Faucet.ModuleConfig("OptimistNftModule", true, 1 days, 1 ether));
    }

    function test_authAdmin_drip_succeeds() external {
        _enableFaucetAuthModules();
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(optimistNftFamName),
            bytes(optimistNftFamVersion),
            block.chainid,
            address(optimistNftFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        vm.prank(nonAdmin);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(optimistNftFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );
    }

    function test_nonAdmin_drip_fails() external {
        _enableFaucetAuthModules();
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature = issueProofWithEIP712Domain(
            nonAdminKey,
            bytes(optimistNftFamName),
            bytes(optimistNftFamVersion),
            block.chainid,
            address(optimistNftFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        vm.prank(nonAdmin);
        vm.expectRevert("Faucet: drip parameters could not be verified by security module");
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(optimistNftFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );
    }

    function test_drip_optimistNftSendsCorrectAmount_succeeds() external {
        _enableFaucetAuthModules();
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(optimistNftFamName),
            bytes(optimistNftFamVersion),
            block.chainid,
            address(optimistNftFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        uint256 recipientBalanceBefore = address(fundsReceiver).balance;
        vm.prank(nonAdmin);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(optimistNftFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );
        uint256 recipientBalanceAfter = address(fundsReceiver).balance;
        assertEq(recipientBalanceAfter - recipientBalanceBefore, 1 ether, "expect increase of 1 ether");
    }

    function test_drip_githubSendsCorrectAmount_succeeds() external {
        _enableFaucetAuthModules();
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(githubFamName),
            bytes(githubFamVersion),
            block.chainid,
            address(githubFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        uint256 recipientBalanceBefore = address(fundsReceiver).balance;
        vm.prank(nonAdmin);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );
        uint256 recipientBalanceAfter = address(fundsReceiver).balance;
        assertEq(recipientBalanceAfter - recipientBalanceBefore, 0.05 ether, "expect increase of .05 ether");
    }

    function test_drip_emitsEvent_succeeds() external {
        _enableFaucetAuthModules();
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(githubFamName),
            bytes(githubFamVersion),
            block.chainid,
            address(githubFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        vm.expectEmit(true, true, true, true, address(faucet));
        emit Drip("GithubModule", keccak256(abi.encodePacked(fundsReceiver)), 0.05 ether, fundsReceiver);

        vm.prank(nonAdmin);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );
    }

    function test_drip_disabledModule_reverts() external {
        _enableFaucetAuthModules();
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(githubFamName),
            bytes(githubFamVersion),
            block.chainid,
            address(githubFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        vm.startPrank(faucetContractAdmin);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );

        faucet.configure(githubFam, Faucet.ModuleConfig("GithubModule", false, 1 days, 0.05 ether));

        vm.expectRevert("Faucet: provided auth module is not supported by this faucet");
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );
        vm.stopPrank();
    }

    function test_drip_preventsReplayAttacks_succeeds() external {
        _enableFaucetAuthModules();
        bytes32 nonce = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(githubFamName),
            bytes(githubFamVersion),
            block.chainid,
            address(githubFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce
        );

        vm.startPrank(faucetContractAdmin);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );

        vm.expectRevert("Faucet: nonce has already been used");
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature)
        );
        vm.stopPrank();
    }

    function test_drip_beforeTimeout_reverts() external {
        _enableFaucetAuthModules();
        bytes32 nonce0 = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature0 = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(githubFamName),
            bytes(githubFamVersion),
            block.chainid,
            address(githubFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce0
        );

        vm.startPrank(faucetContractAdmin);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce0, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature0)
        );

        bytes32 nonce1 = faucetHelper.consumeNonce();
        bytes memory signature1 = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(githubFamName),
            bytes(githubFamVersion),
            block.chainid,
            address(githubFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce1
        );

        vm.expectRevert("Faucet: auth cannot be used yet because timeout has not elapsed");
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce1, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature1)
        );
        vm.stopPrank();
    }

    function test_drip_afterTimeout_succeeds() external {
        _enableFaucetAuthModules();
        bytes32 nonce0 = faucetHelper.consumeNonce();
        bytes memory data = "0x";
        uint32 gasLimit = 200000;
        bytes memory signature0 = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(githubFamName),
            bytes(githubFamVersion),
            block.chainid,
            address(githubFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce0
        );

        vm.startPrank(faucetContractAdmin);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce0, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature0)
        );

        bytes32 nonce1 = faucetHelper.consumeNonce();
        bytes memory signature1 = issueProofWithEIP712Domain(
            faucetAuthAdminKey,
            bytes(githubFamName),
            bytes(githubFamVersion),
            block.chainid,
            address(githubFam),
            fundsReceiver,
            keccak256(abi.encodePacked(fundsReceiver)),
            nonce1
        );

        vm.warp(startingTimestamp + 1 days + 1 seconds);
        faucet.drip(
            Faucet.DripParameters(payable(fundsReceiver), data, nonce1, gasLimit),
            Faucet.AuthParameters(githubFam, keccak256(abi.encodePacked(fundsReceiver)), signature1)
        );
        vm.stopPrank();
    }

    function test_withdraw_succeeds() external {
        vm.startPrank(faucetContractAdmin);
        uint256 recipientBalanceBefore = address(fundsReceiver).balance;

        faucet.withdraw(payable(fundsReceiver), 2 ether);

        uint256 recipientBalanceAfter = address(fundsReceiver).balance;
        assertEq(recipientBalanceAfter - recipientBalanceBefore, 2 ether, "expect increase of 2 ether");
        vm.stopPrank();
    }

    function test_withdraw_nonAdmin_reverts() external {
        vm.prank(nonAdmin);
        vm.expectRevert("Faucet: function can only be called by admin");
        faucet.withdraw(payable(fundsReceiver), 2 ether);
    }

    function test_receive_succeeds() external {
        uint256 faucetBalanceBefore = address(faucet).balance;

        vm.prank(nonAdmin);
        (bool success,) = address(faucet).call{ value: 1 ether }("");
        assertTrue(success);

        uint256 faucetBalanceAfter = address(faucet).balance;
        assertEq(faucetBalanceAfter - faucetBalanceBefore, 1 ether, "expect increase of 1 ether");
    }
}
