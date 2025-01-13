// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";
import "test/safe-tools/SafeTestTools.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { IDeputyGuardianModule } from "interfaces/safe/IDeputyGuardianModule.sol";
import { IDeputyPauseModule } from "interfaces/safe/IDeputyPauseModule.sol";

/// @title DeputyPauseModule_TestInit
/// @notice Base test setup for the DeputyPauseModule.
contract DeputyPauseModule_TestInit is CommonTest, SafeTestTools {
    using SafeTestLib for SafeInstance;

    event ExecutionFromModuleSuccess(address indexed);
    event DeputySet(address indexed);
    event DeputyGuardianModuleSet(IDeputyGuardianModule indexed);
    event PauseTriggered(address indexed deputy, bytes32 nonce);

    IDeputyPauseModule deputyPauseModule;
    IDeputyGuardianModule deputyGuardianModule;
    SafeInstance securityCouncilSafeInstance;
    SafeInstance foundationSafeInstance;
    address deputy;
    uint256 deputyKey;
    bytes deputyAuthSignature;

    bytes32 constant SOME_VALID_NONCE = keccak256("some valid nonce");
    bytes32 constant PAUSE_MESSAGE_TYPEHASH = keccak256("PauseMessage(bytes32 nonce)");
    bytes32 constant DEPUTY_AUTH_MESSAGE_TYPEHASH = keccak256("DeputyAuthMessage(address deputy)");

    /// @notice Sets up the test environment.
    function setUp() public virtual override {
        super.setUp();

        // Set up 20 keys.
        (, uint256[] memory keys) = SafeTestLib.makeAddrsAndKeys("DeputyPauseModule_test_", 20);

        // Split into two sets of 10 keys.
        uint256[] memory keys1 = new uint256[](10);
        uint256[] memory keys2 = new uint256[](10);
        for (uint256 i; i < 10; i++) {
            keys1[i] = keys[i];
            keys2[i] = keys[i + 10];
        }

        // Create a Security Council Safe with 10 owners.
        securityCouncilSafeInstance = _setupSafe(keys1, 10);

        // Create a Foundation Safe with 10 different owners.
        foundationSafeInstance = _setupSafe(keys2, 10);

        // Set the Security Council Safe as the Guardian of the SuperchainConfig.
        vm.store(
            address(superchainConfig),
            superchainConfig.GUARDIAN_SLOT(),
            bytes32(uint256(uint160(address(securityCouncilSafeInstance.safe))))
        );

        // Create a DeputyGuardianModule and set the Foundation Safe as the Deputy Guardian.
        deputyGuardianModule = IDeputyGuardianModule(
            DeployUtils.create1({
                _name: "DeputyGuardianModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyGuardianModule.__constructor__,
                        (securityCouncilSafeInstance.safe, superchainConfig, address(foundationSafeInstance.safe))
                    )
                )
            })
        );

        // Enable the DeputyGuardianModule on the Security Council Safe.
        securityCouncilSafeInstance.enableModule(address(deputyGuardianModule));

        // Create the deputy for the DeputyPauseModule.
        (deputy, deputyKey) = makeAddrAndKey("deputy");

        // Create the deputy auth signature.
        deputyAuthSignature = makeAuthSignature(getNextContract(), deputyKey, deputy);

        // Create the DeputyPauseModule.
        deputyPauseModule = IDeputyPauseModule(
            DeployUtils.create1({
                _name: "DeputyPauseModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyPauseModule.__constructor__,
                        (foundationSafeInstance.safe, deputyGuardianModule, superchainConfig, deputy, deputyAuthSignature)
                    )
                )
            })
        );

        // Enable the DeputyPauseModule on the Foundation Safe.
        foundationSafeInstance.enableModule(address(deputyPauseModule));
    }

    /// @notice Generates a signature to authenticate as the deputy.
    /// @param _verifyingContract The verifying contract.
    /// @param _privateKey The private key to use to sign the message.
    /// @param _deputy The deputy to authenticate as.
    /// @return Generated signature.
    function makeAuthSignature(
        address _verifyingContract,
        uint256 _privateKey,
        address _deputy
    )
        internal
        view
        returns (bytes memory)
    {
        return makeAuthSignature(block.chainid, _verifyingContract, _privateKey, _deputy);
    }

    /// @notice Generates a signature to authenticate as the deputy.
    /// @param _chainId Chain ID to use for the domain separator.
    /// @param _verifyingContract The verifying contract.
    /// @param _privateKey The private key to use to sign the message.
    /// @param _deputy The deputy to authenticate as.
    /// @return Generated signature.
    function makeAuthSignature(
        uint256 _chainId,
        address _verifyingContract,
        uint256 _privateKey,
        address _deputy
    )
        internal
        pure
        returns (bytes memory)
    {
        bytes32 structHash = keccak256(abi.encode(DEPUTY_AUTH_MESSAGE_TYPEHASH, _deputy));
        bytes32 digest = hashTypedData(_verifyingContract, _chainId, structHash);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(_privateKey, digest);
        return abi.encodePacked(r, s, v);
    }

    /// @notice Generates a signature to trigger a pause.
    /// @param _verifyingContract The verifying contract.
    /// @param _nonce Signature nonce.
    /// @param _privateKey The private key to use to sign the message.
    /// @return Generated signature.
    function makePauseSignature(
        address _verifyingContract,
        bytes32 _nonce,
        uint256 _privateKey
    )
        internal
        view
        returns (bytes memory)
    {
        return makePauseSignature(block.chainid, _verifyingContract, _nonce, _privateKey);
    }

    /// @notice Generates a signature to trigger a pause.
    /// @param _chainId Chain ID to use for the domain separator.
    /// @param _verifyingContract The verifying contract.
    /// @param _nonce Signature nonce.
    /// @param _privateKey The private key to use to sign the message.
    /// @return Generated signature.
    function makePauseSignature(
        uint256 _chainId,
        address _verifyingContract,
        bytes32 _nonce,
        uint256 _privateKey
    )
        internal
        pure
        returns (bytes memory)
    {
        bytes32 structHash = keccak256(abi.encode(PAUSE_MESSAGE_TYPEHASH, _nonce));
        bytes32 digest = hashTypedData(_verifyingContract, _chainId, structHash);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(_privateKey, digest);
        return abi.encodePacked(r, s, v);
    }

    /// @notice Helper function to compute EIP-712 typed data hash
    /// @param _verifyingContract The verifying contract.
    /// @param _chainId Chain ID to use for the domain separator.
    /// @param _structHash The struct hash.
    /// @return The EIP-712 typed data hash.
    function hashTypedData(
        address _verifyingContract,
        uint256 _chainId,
        bytes32 _structHash
    )
        internal
        pure
        returns (bytes32)
    {
        bytes32 DOMAIN_SEPARATOR = keccak256(
            abi.encode(
                keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
                keccak256("DeputyPauseModule"),
                keccak256("1"),
                _chainId,
                _verifyingContract
            )
        );
        return keccak256(abi.encodePacked("\x19\x01", DOMAIN_SEPARATOR, _structHash));
    }

    /// @notice Gets the next contract that will be created by this test contract.
    /// @return Address of the next contract to be created.
    function getNextContract() internal view returns (address) {
        return vm.computeCreateAddress(address(this), vm.getNonce(address(this)));
    }
}

/// @title DeputyPauseModule_Constructor_Test
/// @notice Tests that the constructor works.
contract DeputyPauseModule_Constructor_Test is DeputyPauseModule_TestInit {
    /// @notice Tests that the constructor works.
    function test_constructor_validParameters_succeeds() external {
        // Create the signature.
        address nextContract = getNextContract();
        bytes memory signature = makeAuthSignature(nextContract, deputyKey, deputy);

        // Deploy the module.
        vm.expectEmit(address(nextContract));
        emit DeputySet(deputy);
        deputyPauseModule = IDeputyPauseModule(
            DeployUtils.create1({
                _name: "DeputyPauseModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyPauseModule.__constructor__,
                        (foundationSafeInstance.safe, deputyGuardianModule, superchainConfig, deputy, signature)
                    )
                )
            })
        );
    }
}

/// @title DeputyPauseModule_Constructor_TestFail
/// @notice Tests that the constructor fails when it should.
contract DeputyPauseModule_Constructor_TestFail is DeputyPauseModule_TestInit {
    /// @notice Tests that the constructor reverts when the signature is not the deputy auth message.
    function testFuzz_constructor_signatureNotNextContract_reverts(address _nextContract) external {
        // Make sure that the next contract is not correct.
        vm.assume(_nextContract != getNextContract());

        // Create the signature.
        bytes memory signature = makeAuthSignature(_nextContract, deputyKey, deputy);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_InvalidDeputy.selector));
        IDeputyPauseModule(
            DeployUtils.create1({
                _name: "DeputyPauseModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyPauseModule.__constructor__,
                        (foundationSafeInstance.safe, deputyGuardianModule, superchainConfig, deputy, signature)
                    )
                )
            })
        );
    }

    /// @notice Tests that the constructor reverts when the signature is not the deputy auth message.
    function testFuzz_constructor_signatureNotOverDeputy_reverts(address _deputy) external {
        // Make sure that the deputy is not correct.
        vm.assume(_deputy != deputy);

        // Create the signature.
        bytes memory signature = makeAuthSignature(getNextContract(), deputyKey, _deputy);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_InvalidDeputy.selector));
        IDeputyPauseModule(
            DeployUtils.create1({
                _name: "DeputyPauseModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyPauseModule.__constructor__,
                        (foundationSafeInstance.safe, deputyGuardianModule, superchainConfig, deputy, signature)
                    )
                )
            })
        );
    }

    /// @notice Tests that the constructor reverts when the signature is not from the deputy.
    function testFuzz_constructor_signatureNotFromDeputy_reverts(uint256 _privateKey) external {
        // Make sure that the private key is not the deputy's private key.
        vm.assume(_privateKey != deputyKey);

        // Make sure that the private key is in the range of a valid secp256k1 private key.
        _privateKey = bound(_privateKey, 1, SECP256K1_ORDER - 1);

        // Create the signature.
        bytes memory signature = makeAuthSignature(getNextContract(), _privateKey, deputy);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_InvalidDeputy.selector));
        IDeputyPauseModule(
            DeployUtils.create1({
                _name: "DeputyPauseModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyPauseModule.__constructor__,
                        (foundationSafeInstance.safe, deputyGuardianModule, superchainConfig, deputy, signature)
                    )
                )
            })
        );
    }

    /// @notice Tests that the constructor reverts when the signature uses the wrong chain ID.
    function testFuzz_constructor_wrongChainId_reverts(uint256 _chainId) external {
        // Make sure that the chain ID is not the current chain ID.
        vm.assume(_chainId != block.chainid);

        // Create the signature.
        bytes memory signature = makeAuthSignature(_chainId, getNextContract(), deputyKey, deputy);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_InvalidDeputy.selector));
        IDeputyPauseModule(
            DeployUtils.create1({
                _name: "DeputyPauseModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyPauseModule.__constructor__,
                        (foundationSafeInstance.safe, deputyGuardianModule, superchainConfig, deputy, signature)
                    )
                )
            })
        );
    }

    /// @notice Tests that the constructor reverts when the signature is not the deputy auth message.
    function test_constructor_signatureNotAuthMessage_reverts() external {
        // Create the signature.
        bytes memory signature = makePauseSignature(getNextContract(), bytes32(0), deputyKey);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_InvalidDeputy.selector));
        IDeputyPauseModule(
            DeployUtils.create1({
                _name: "DeputyPauseModule",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        IDeputyPauseModule.__constructor__,
                        (foundationSafeInstance.safe, deputyGuardianModule, superchainConfig, deputy, signature)
                    )
                )
            })
        );
    }
}

/// @title DeputyPauseModule_Getters_Test
/// @notice Tests that the getters work.
contract DeputyPauseModule_Getters_Test is DeputyPauseModule_TestInit {
    /// @notice Tests that the getters work.
    function test_getters_works() external view {
        assertEq(address(deputyPauseModule.foundationSafe()), address(foundationSafeInstance.safe));
        assertEq(address(deputyPauseModule.deputyGuardianModule()), address(deputyGuardianModule));
        assertEq(address(deputyPauseModule.superchainConfig()), address(superchainConfig));
        assertEq(deputyPauseModule.deputy(), deputy);
        assertEq(deputyPauseModule.pauseMessageTypehash(), PAUSE_MESSAGE_TYPEHASH);
        assertEq(deputyPauseModule.deputyAuthMessageTypehash(), DEPUTY_AUTH_MESSAGE_TYPEHASH);
    }
}

/// @title DeputyPauseModule_Pause_Test
/// @notice Tests that the pause() function works.
contract DeputyPauseModule_Pause_Test is DeputyPauseModule_TestInit {
    /// @notice Tests that pause() successfully pauses when called by the deputy.
    /// @param _nonce Signature nonce.
    function testFuzz_pause_validParameters_succeeds(bytes32 _nonce) external {
        vm.expectEmit(address(superchainConfig));
        emit Paused("Deputy Guardian");

        vm.expectEmit(address(securityCouncilSafeInstance.safe));
        emit ExecutionFromModuleSuccess(address(deputyGuardianModule));

        vm.expectEmit(address(deputyGuardianModule));
        emit Paused("Deputy Guardian");

        vm.expectEmit(address(foundationSafeInstance.safe));
        emit ExecutionFromModuleSuccess(address(deputyPauseModule));

        vm.expectEmit(address(deputyPauseModule));
        emit PauseTriggered(deputy, _nonce);

        // State assertions before the pause.
        assertEq(deputyPauseModule.usedNonces(_nonce), false);
        assertEq(superchainConfig.paused(), false);

        // Trigger the pause.
        bytes memory signature = makePauseSignature(address(deputyPauseModule), _nonce, deputyKey);
        deputyPauseModule.pause(_nonce, signature);

        // State assertions after the pause.
        assertEq(deputyPauseModule.usedNonces(_nonce), true);
        assertEq(superchainConfig.paused(), true);
    }

    /// @notice Tests that pause() succeeds when called with two different nonces if the
    ///         SuperchainConfig contract is not paused between calls.
    /// @param _nonce1 First nonce.
    /// @param _nonce2 Second nonce.
    function testFuzz_pause_differentNonces_succeeds(bytes32 _nonce1, bytes32 _nonce2) external {
        // Make sure that the nonces are different.
        vm.assume(_nonce1 != _nonce2);

        // Pause once.
        bytes memory sig1 = makePauseSignature(address(deputyPauseModule), _nonce1, deputyKey);
        deputyPauseModule.pause(_nonce1, sig1);

        // Unpause.
        vm.prank(address(securityCouncilSafeInstance.safe));
        superchainConfig.unpause();

        // Pause again with a different nonce.
        bytes memory sig2 = makePauseSignature(address(deputyPauseModule), _nonce2, deputyKey);
        deputyPauseModule.pause(_nonce2, sig2);
    }

    /// @notice Tests that pause() succeeds when called with two different nonces after the
    ///         superchain has already been paused between calls.
    /// @param _nonce1 First nonce.
    /// @param _nonce2 Second nonce.
    function testFuzz_pause_differentNoncesAlreadyPaused_succeeds(bytes32 _nonce1, bytes32 _nonce2) external {
        // Make sure that the nonces are different.
        vm.assume(_nonce1 != _nonce2);

        // Pause once.
        bytes memory sig1 = makePauseSignature(address(deputyPauseModule), _nonce1, deputyKey);
        deputyPauseModule.pause(_nonce1, sig1);

        // Pause again with a different nonce.
        bytes memory sig2 = makePauseSignature(address(deputyPauseModule), _nonce2, deputyKey);
        deputyPauseModule.pause(_nonce2, sig2);
    }

    /// @notice Tests that pause() succeeds within 1 million gas.
    function test_pause_withinMillionGas_succeeds() external {
        bytes memory signature = makePauseSignature(address(deputyPauseModule), SOME_VALID_NONCE, deputyKey);

        uint256 gasBefore = gasleft();
        deputyPauseModule.pause(SOME_VALID_NONCE, signature);
        uint256 gasUsed = gasBefore - gasleft();

        // Ensure gas usage is within expected bounds.
        // 1m is a conservative limit that means we can trigger the pause in most blocks. It would
        // be prohibitively expensive to fill up blocks to prevent the pause from being triggered
        // even at 1m gas for any prolonged duration. Means that we can always trigger the pause
        // within a short period of time.
        assertLt(gasUsed, 1000000);
    }
}

/// @title DeputyPauseModule_Pause_TestFail
/// @notice Tests that the pause() function reverts when it should.
contract DeputyPauseModule_Pause_TestFail is DeputyPauseModule_TestInit {
    /// @notice Tests that pause() reverts when called by an address other than the deputy.
    /// @param _privateKey The private key to use to sign the message.
    function testFuzz_pause_notDeputy_reverts(uint256 _privateKey) external {
        // Make sure that the private key is in the range of a valid secp256k1 private key.
        _privateKey = bound(_privateKey, 1, SECP256K1_ORDER - 1);

        // Make sure that the private key is not the deputy's private key.
        vm.assume(_privateKey != deputyKey);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_Unauthorized.selector));
        bytes memory signature = makePauseSignature(address(deputyPauseModule), SOME_VALID_NONCE, _privateKey);
        deputyPauseModule.pause(SOME_VALID_NONCE, signature);
    }

    /// @notice Tests that pause() reverts when the nonce has already been used.
    /// @param _nonce Signature nonce.
    function testFuzz_pause_nonceAlreadyUsed_reverts(bytes32 _nonce) external {
        // Pause once.
        bytes memory signature = makePauseSignature(address(deputyPauseModule), _nonce, deputyKey);
        deputyPauseModule.pause(_nonce, signature);

        // Unpause.
        vm.prank(address(securityCouncilSafeInstance.safe));
        superchainConfig.unpause();

        // Expect that the nonce is now used.
        assertEq(deputyPauseModule.usedNonces(_nonce), true);

        // Pause again.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_NonceAlreadyUsed.selector));
        deputyPauseModule.pause(_nonce, signature);
    }

    /// @notice Tests that pause() reverts when the signature is longer than 65 bytes.
    /// @param _length The length of the malformed signature.
    function testFuzz_pause_signatureTooLong_reverts(uint256 _length) external {
        // Make sure signature is longer than 65 bytes.
        _length = bound(_length, 66, 1000);

        // Create the malformed signature.
        bytes memory signature = new bytes(_length);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.ECDSAInvalidSignatureLength.selector, _length));
        deputyPauseModule.pause(SOME_VALID_NONCE, signature);
    }

    /// @notice Tests that pause() reverts when the signature is shorter than 65 bytes.
    /// @param _length The length of the malformed signature.
    function testFuzz_pause_signatureTooShort_reverts(uint256 _length) external {
        // Make sure signature is shorter than 65 bytes.
        _length = bound(_length, 0, 64);

        // Create the malformed signature.
        bytes memory signature = new bytes(_length);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.ECDSAInvalidSignatureLength.selector, _length));
        deputyPauseModule.pause(SOME_VALID_NONCE, signature);
    }

    /// @notice Tests that pause() reverts when the chain ID is not the same as the chain ID that
    ///         the signature was created for.
    /// @param _chainId Chain ID to use for the signature.
    function testFuzz_pause_wrongChainId_reverts(uint256 _chainId) external {
        // Make sure that the chain ID is not the current chain ID.
        vm.assume(_chainId != block.chainid);

        // Signature with the wrong chain ID.
        bytes memory signature = makePauseSignature(_chainId, address(deputyPauseModule), SOME_VALID_NONCE, deputyKey);

        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_Unauthorized.selector));
        deputyPauseModule.pause(SOME_VALID_NONCE, signature);
    }

    /// @notice Tests that pause() reverts when the verifying contract is not the deputy pause module.
    /// @param _verifyingContract The verifying contract.
    function testFuzz_pause_wrongVerifyingContract_reverts(address _verifyingContract) external {
        // Make sure that the verifying contract is not the deputy pause module.
        vm.assume(_verifyingContract != address(deputyPauseModule));

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_Unauthorized.selector));
        bytes memory signature = makePauseSignature(_verifyingContract, SOME_VALID_NONCE, deputyKey);
        deputyPauseModule.pause(SOME_VALID_NONCE, signature);
    }

    /// @notice Tests that the error message is returned when the call to the safe reverts.
    function test_pause_targetReverts_reverts() external {
        // Make sure that the SuperchainConfig pause() reverts.
        vm.mockCallRevert(
            address(superchainConfig),
            abi.encodePacked(superchainConfig.pause.selector),
            "SuperchainConfig: pause() reverted"
        );

        // Note that the error here will be somewhat awkwardly double-encoded because the
        // DeputyGuardianModule will encode the revert message as an ExecutionFailed error and then
        // the DeputyPauseModule will re-encode it as another ExecutionFailed error.
        vm.expectRevert(
            abi.encodeWithSelector(
                IDeputyPauseModule.DeputyPauseModule_ExecutionFailed.selector,
                string(
                    abi.encodeWithSelector(
                        IDeputyGuardianModule.ExecutionFailed.selector, "SuperchainConfig: pause() reverted"
                    )
                )
            )
        );
        bytes memory signature = makePauseSignature(address(deputyPauseModule), SOME_VALID_NONCE, deputyKey);
        deputyPauseModule.pause(SOME_VALID_NONCE, signature);
    }

    /// @notice Tests that pause() reverts when the superchain is not in a paused state after the
    /// transaction is sent.
    function test_pause_superchainPauseFails_reverts() external {
        // Make sure that the SuperchainConfig paused() returns false.
        vm.mockCall(address(superchainConfig), abi.encodePacked(superchainConfig.paused.selector), abi.encode(false));

        // Expect a revert.
        vm.expectRevert(IDeputyPauseModule.DeputyPauseModule_SuperchainNotPaused.selector);
        deputyPauseModule.pause(
            SOME_VALID_NONCE, makePauseSignature(address(deputyPauseModule), SOME_VALID_NONCE, deputyKey)
        );
    }
}

/// @title DeputyPauseModule_SetDeputy_Test
/// @notice Tests that the setDeputy() function works.
contract DeputyPauseModule_SetDeputy_Test is DeputyPauseModule_TestInit {
    /// @notice Tests that setDeputy() succeeds when called from the safe.
    /// @param _seed Seed used to generate a private key.
    function testFuzz_setDeputy_fromSafe_succeeds(bytes32 _seed) external {
        (address newDeputy, uint256 newDeputyKey) = makeAddrAndKey(string(abi.encodePacked(_seed)));

        // Make sure the private key is not the existing deputy's private key.
        vm.assume(newDeputyKey != deputyKey);

        // Sign the message.
        bytes memory signature = makeAuthSignature(address(deputyPauseModule), newDeputyKey, newDeputy);

        // Set the deputy address.
        vm.expectEmit(address(deputyPauseModule));
        emit DeputySet(newDeputy);
        vm.prank(address(foundationSafeInstance.safe));
        deputyPauseModule.setDeputy(newDeputy, signature);

        // Assert that the deputy address has been set.
        assertEq(deputyPauseModule.deputy(), newDeputy);
    }
}

/// @title DeputyPauseModule_SetDeputy_TestFail
/// @notice Tests that the setDeputy() function reverts when it should.
contract DeputyPauseModule_SetDeputy_TestFail is DeputyPauseModule_TestInit {
    /// @notice Tests that setDeputy() reverts when called by an address other than the safe.
    function testFuzz_setDeputy_notSafe_reverts(address _sender) external {
        // Make sure that the sender is not the safe.
        vm.assume(_sender != address(foundationSafeInstance.safe));

        // Create the key.
        (address newDeputy, uint256 newDeputyKey) = makeAddrAndKey("whatever");

        // Sign the message.
        bytes memory signature = makeAuthSignature(address(deputyPauseModule), newDeputyKey, newDeputy);

        // Expect a revert.
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_NotFromSafe.selector));
        deputyPauseModule.setDeputy(newDeputy, signature);

        // Make sure deputy has not changed.
        assertEq(deputyPauseModule.deputy(), deputy);
    }
}

/// @title DeputyPauseModule_SetDeputyGuardianModule_Test
/// @notice Tests that the setDeputyGuardianModule() function works.
contract DeputyPauseModule_SetDeputyGuardianModule_Test is DeputyPauseModule_TestInit {
    /// @notice Tests that setDeputyGuardianModule() succeeds when called from the safe.
    function testFuzz_setDeputyGuardianModule_fromSafe_succeeds(address _newModule) external {
        vm.assume(_newModule != address(0));
        vm.assume(_newModule != address(deputyGuardianModule));
        vm.assume(_newModule != address(foundationSafeInstance.safe));
        assumeNotPrecompile(_newModule);

        // Write code to the module address if it has no code.
        // vm.assume would throw out too many inputs.
        if (_newModule.code.length == 0) {
            vm.etch(_newModule, hex"FF");
        }

        // Set the new DeputyGuardianModule
        vm.expectEmit(address(deputyPauseModule));
        emit DeputyGuardianModuleSet(IDeputyGuardianModule(_newModule));
        vm.prank(address(foundationSafeInstance.safe));
        deputyPauseModule.setDeputyGuardianModule(IDeputyGuardianModule(_newModule));

        // Assert that the DeputyGuardianModule has been set
        assertEq(address(deputyPauseModule.deputyGuardianModule()), _newModule);
    }
}

/// @title DeputyPauseModule_SetDeputyGuardianModule_TestFail
/// @notice Tests that the setDeputyGuardianModule() function reverts when it should.
contract DeputyPauseModule_SetDeputyGuardianModule_TestFail is DeputyPauseModule_TestInit {
    /// @notice Tests that setDeputyGuardianModule() reverts when called by an address other than the safe.
    function testFuzz_setDeputyGuardianModule_notSafe_reverts(address _sender, address _newModule) external {
        vm.assume(_sender != address(foundationSafeInstance.safe));
        vm.assume(_newModule != address(0));

        // Expect a revert when called from non-safe address
        vm.prank(_sender);
        vm.expectRevert(abi.encodeWithSelector(IDeputyPauseModule.DeputyPauseModule_NotFromSafe.selector));
        deputyPauseModule.setDeputyGuardianModule(IDeputyGuardianModule(_newModule));

        // Make sure DeputyGuardianModule has not changed
        assertEq(address(deputyPauseModule.deputyGuardianModule()), address(deputyGuardianModule));
    }

    /// @notice Tests that setDeputyGuardianModule() reverts when the DeputyGuardianModule has no code.
    /// @param _newModule The new DeputyGuardianModule.
    function testFuzz_setDeputyGuardianModule_hasNoCode_reverts(address _newModule) external {
        vm.assume(_newModule.code.length == 0);

        // Expect a revert.
        vm.expectRevert(IDeputyPauseModule.DeputyPauseModule_InvalidDeputyGuardianModule.selector);
        vm.prank(address(foundationSafeInstance.safe));
        deputyPauseModule.setDeputyGuardianModule(IDeputyGuardianModule(_newModule));
    }

    /// @notice Tests that setDeputyGuardianModule() reverts when the DeputyGuardianModule is the Foundation Safe.
    function test_setDeputyGuardianModule_isFoundationSafe_reverts() external {
        vm.expectRevert(IDeputyPauseModule.DeputyPauseModule_InvalidDeputyGuardianModule.selector);
        vm.prank(address(foundationSafeInstance.safe));
        deputyPauseModule.setDeputyGuardianModule(IDeputyGuardianModule(address(foundationSafeInstance.safe)));
    }
}
