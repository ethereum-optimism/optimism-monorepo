// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Interfaces
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISystemConfigJovian } from "interfaces/L1/ISystemConfigJovian.sol";

contract SystemConfig_Init is CommonTest {
    event ConfigUpdate(uint256 indexed nonceAndVersion, ISystemConfig.UpdateType indexed updateType, bytes data);
}

contract SystemConfigJovian_Test is SystemConfig_Init {
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function setUp() public virtual override {
        super.enableJovian();
        super.setUp();
        systemConfig.version();
    }

    function test_nonce_increment_works() external {
        bytes32 newBatcherHash = bytes32(uint256(1234));
        for (uint256 i = 4; i <= 10; i++) {
            vm.expectEmit(address(systemConfig));
            emit ConfigUpdate(i << 128 | 1, ISystemConfig.UpdateType.BATCHER, abi.encode(newBatcherHash));
            vm.prank(systemConfig.owner());
            systemConfig.setBatcherHash(newBatcherHash);
        }
    }

    function test_configUpdateNonce_works() external {
        // genesis emits 3 logs, so nonce starts at 3
        uint64 nonce = ISystemConfigJovian(payable(address(systemConfig))).configUpdateNonce();
        assertEq(3, nonce);

        for (uint64 i = 1; i <= 2; i++) {
            vm.prank(systemConfig.owner());
            systemConfig.setBatcherHash(bytes32(0));
        }

        nonce = ISystemConfigJovian(payable(address(systemConfig))).configUpdateNonce();
        assertEq(5, nonce);
    }
}

contract SystemConfigJovian_Setters_Test is SystemConfig_Init {
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function setUp() public virtual override {
        super.enableJovian();
        super.setUp();
    }

    /// @dev Tests that `setBatcherHash` updates the batcher hash successfully.
    function testFuzz_setBatcherHash_succeeds(bytes32 newBatcherHash) external {
        vm.expectEmit(address(systemConfig));
        emit ConfigUpdate(4 << 128 | 1, ISystemConfig.UpdateType.BATCHER, abi.encode(newBatcherHash));

        vm.prank(systemConfig.owner());
        systemConfig.setBatcherHash(newBatcherHash);
        assertEq(systemConfig.batcherHash(), newBatcherHash);
    }

    /// @dev Tests that `setGasConfig` updates the overhead and scalar successfully.
    function testFuzz_setGasConfig_succeeds(uint256 newOverhead, uint256 newScalar) external {
        // always zero out most significant byte
        newScalar = (newScalar << 16) >> 16;
        vm.expectEmit(address(systemConfig));
        emit ConfigUpdate(4 << 128 | 1, ISystemConfig.UpdateType.FEE_SCALARS, abi.encode(newOverhead, newScalar));

        vm.prank(systemConfig.owner());
        systemConfig.setGasConfig(newOverhead, newScalar);
        assertEq(systemConfig.overhead(), newOverhead);
        assertEq(systemConfig.scalar(), newScalar);
    }

    function testFuzz_setGasConfigEcotone_succeeds(uint32 _basefeeScalar, uint32 _blobbasefeeScalar) external {
        // TODO(opcm upgrades): remove skip once upgrade is implemented
        skipIfForkTest("SystemConfig_Setters_TestFail: 'setGasConfigEcotone' method DNE on op mainnet");
        bytes32 encoded =
            ffi.encodeScalarEcotone({ _basefeeScalar: _basefeeScalar, _blobbasefeeScalar: _blobbasefeeScalar });

        vm.expectEmit(address(systemConfig));
        emit ConfigUpdate(
            4 << 128 | 1, ISystemConfig.UpdateType.FEE_SCALARS, abi.encode(systemConfig.overhead(), encoded)
        );

        vm.prank(systemConfig.owner());
        systemConfig.setGasConfigEcotone({ _basefeeScalar: _basefeeScalar, _blobbasefeeScalar: _blobbasefeeScalar });
        assertEq(systemConfig.basefeeScalar(), _basefeeScalar);
        assertEq(systemConfig.blobbasefeeScalar(), _blobbasefeeScalar);
        assertEq(systemConfig.scalar(), uint256(encoded));

        (uint32 basefeeScalar, uint32 blobbbasefeeScalar) = ffi.decodeScalarEcotone(encoded);
        assertEq(uint256(basefeeScalar), uint256(_basefeeScalar));
        assertEq(uint256(blobbbasefeeScalar), uint256(_blobbasefeeScalar));
    }

    /// @dev Tests that `setGasLimit` updates the gas limit successfully.
    function testFuzz_setGasLimit_succeeds(uint64 newGasLimit) external {
        uint64 minimumGasLimit = systemConfig.minimumGasLimit();
        uint64 maximumGasLimit = systemConfig.maximumGasLimit();
        newGasLimit = uint64(bound(uint256(newGasLimit), uint256(minimumGasLimit), uint256(maximumGasLimit)));

        vm.expectEmit(address(systemConfig));
        emit ConfigUpdate(4 << 128 | 1, ISystemConfig.UpdateType.GAS_LIMIT, abi.encode(newGasLimit));

        vm.prank(systemConfig.owner());
        systemConfig.setGasLimit(newGasLimit);
        assertEq(systemConfig.gasLimit(), newGasLimit);
    }

    /// @dev Tests that `setUnsafeBlockSigner` updates the block signer successfully.
    function testFuzz_setUnsafeBlockSigner_succeeds(address newUnsafeSigner) external {
        vm.expectEmit(address(systemConfig));
        emit ConfigUpdate(4 << 128 | 1, ISystemConfig.UpdateType.UNSAFE_BLOCK_SIGNER, abi.encode(newUnsafeSigner));

        vm.prank(systemConfig.owner());
        systemConfig.setUnsafeBlockSigner(newUnsafeSigner);
        assertEq(systemConfig.unsafeBlockSigner(), newUnsafeSigner);
    }

    /// @dev Tests that `setEIP1559Params` updates the EIP1559 parameters successfully.
    function testFuzz_setEIP1559Params_succeeds(uint32 _denominator, uint32 _elasticity) external {
        // TODO(opcm upgrades): remove skip once upgrade is implemented
        skipIfForkTest("SystemConfig_Setters_TestFail: 'setEIP1559Params' method DNE on op mainnet");
        _denominator = uint32(bound(_denominator, 2, type(uint32).max));
        _elasticity = uint32(bound(_elasticity, 2, type(uint32).max));

        vm.expectEmit(address(systemConfig));
        emit ConfigUpdate(
            4 << 128 | 1,
            ISystemConfig.UpdateType.EIP_1559_PARAMS,
            abi.encode(uint256(_denominator) << 32 | uint64(_elasticity))
        );

        vm.prank(systemConfig.owner());
        systemConfig.setEIP1559Params(_denominator, _elasticity);
        assertEq(systemConfig.eip1559Denominator(), _denominator);
        assertEq(systemConfig.eip1559Elasticity(), _elasticity);
    }
}
