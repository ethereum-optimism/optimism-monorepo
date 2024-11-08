// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";
import { Preinstalls } from "src/libraries/Preinstalls.sol";
import { Bytes } from "src/libraries/Bytes.sol";
import { IEIP712 } from "src/universal/interfaces/IEIP712.sol";

/// @title PreinstallsTest
contract PreinstallsTest is CommonTest {
    /// @dev The domain separator commits to the chainid of the chain
    function test_preinstall_permit2DomainSeparator_works() external view {
        bytes32 domainSeparator = IEIP712(Preinstalls.Permit2).DOMAIN_SEPARATOR();
        bytes32 typeHash =
            keccak256(abi.encodePacked("EIP712Domain(string name,uint256 chainId,address verifyingContract)"));
        bytes32 nameHash = keccak256(abi.encodePacked("Permit2"));
        uint256 chainId = block.chainid;
        bytes memory encoded = abi.encode(typeHash, nameHash, chainId, Preinstalls.Permit2);
        bytes32 expectedDomainSeparator = keccak256(encoded);
        assertEq(domainSeparator, expectedDomainSeparator, "Domain separator mismatch");
        assertEq(chainId, uint256(901)); // uses devnet config
        assertEq(domainSeparator, bytes32(0x48deb34b39fb4b41f5c195008940d5ef510cdd7853eba5807b2fa08dfd586475));
        // Warning the Permit2 domain separator as cached in the DeployPermit2.sol bytecode is incorrect.
    }

    function test_permit2_templating_works() external pure {
        bytes memory customCode = Preinstalls.getPermit2Code(1234);
        assertNotEq(customCode.length, 0, "must have code");
        assertEq(uint256(bytes32(Bytes.slice(customCode, 6945, 32))), uint256(1234), "expecting custom chain ID");
        assertEq(
            bytes32(Bytes.slice(customCode, 6983, 32)),
            bytes32(0x6cda538cafce36292a6ef27740629597f85f6716f5694d26d5c59fc1d07cfd95),
            "expecting custom domain separator"
        );

        bytes memory defaultCode = Preinstalls.getPermit2Code(1);
        assertNotEq(defaultCode.length, 0, "must have code");
        assertEq(uint256(bytes32(Bytes.slice(defaultCode, 6945, 32))), uint256(1), "expecting default chain ID");
        assertEq(
            bytes32(Bytes.slice(defaultCode, 6983, 32)),
            bytes32(0x866a5aba21966af95d6c7ab78eb2b2fc913915c28be3b9aa07cc04ff903e3f28),
            "expecting default domain separator"
        );
        assertEq(defaultCode, Preinstalls.Permit2TemplateCode, "template is using chain ID 1");
    }

    function assertPreinstall(address _addr, bytes memory _code) internal view {
        assertNotEq(_code.length, 0, "must have code");
        assertNotEq(_addr.code.length, 0, "deployed preinstall account must have code");
        assertEq(_addr.code, _code, "equal code must be deployed");
        assertEq(Preinstalls.getDeployedCode(_addr, block.chainid), _code, "deployed-code getter must match");
        assertNotEq(Preinstalls.getName(_addr), "", "must have a name");
        if (_addr != Preinstalls.DeterministicDeploymentProxy) {
            assertEq(vm.getNonce(_addr), 1, "preinstall account must have 1 nonce");
        }
    }

    function test_preinstall_multicall3_succeeds() external view {
        assertPreinstall(Preinstalls.MultiCall3, Preinstalls.MultiCall3Code);
    }

    function test_preinstall_create2Deployer_succeeds() external view {
        assertPreinstall(Preinstalls.Create2Deployer, Preinstalls.Create2DeployerCode);
    }

    function test_preinstall_safev130_succeeds() external view {
        assertPreinstall(Preinstalls.Safe_v130, Preinstalls.Safe_v130Code);
    }

    function test_preinstall_safeL2v130_succeeds() external view {
        assertPreinstall(Preinstalls.SafeL2_v130, Preinstalls.SafeL2_v130Code);
    }

    function test_preinstall_multisendCallOnlyv130_succeeds() external view {
        assertPreinstall(Preinstalls.MultiSendCallOnly_v130, Preinstalls.MultiSendCallOnly_v130Code);
    }

    function test_preinstall_safeSingletonFactory_succeeds() external view {
        assertPreinstall(Preinstalls.SafeSingletonFactory, Preinstalls.SafeSingletonFactoryCode);
    }

    function test_preinstall_deterministicDeploymentProxy_succeeds() external view {
        assertPreinstall(Preinstalls.DeterministicDeploymentProxy, Preinstalls.DeterministicDeploymentProxyCode);
    }

    function test_preinstall_multisendv130_succeeds() external view {
        assertPreinstall(Preinstalls.MultiSend_v130, Preinstalls.MultiSend_v130Code);
    }

    function test_preinstall_permit2_succeeds() external {
        uint256 pre = block.chainid;
        vm.chainId(901); // TODO legacy deployment does not use same chainID as tests run with
        assertPreinstall(Preinstalls.Permit2, Preinstalls.getPermit2Code(block.chainid));
        vm.chainId(pre);
    }

    function test_preinstall_senderCreatorv060_succeeds() external view {
        assertPreinstall(Preinstalls.SenderCreator_v060, Preinstalls.SenderCreator_v060Code);
    }

    function test_preinstall_entrypointv060_succeeds() external view {
        assertPreinstall(Preinstalls.EntryPoint_v060, Preinstalls.EntryPoint_v060Code);
    }

    function test_preinstall_senderCreatorv070_succeeds() external view {
        assertPreinstall(Preinstalls.SenderCreator_v070, Preinstalls.SenderCreator_v070Code);
    }

    function test_preinstall_entrypointv070_succeeds() external view {
        assertPreinstall(Preinstalls.EntryPoint_v070, Preinstalls.EntryPoint_v070Code);
    }

    function test_preinstall_beaconBlockRoots_succeeds() external view {
        assertPreinstall(Preinstalls.BeaconBlockRoots, Preinstalls.BeaconBlockRootsCode);
        assertEq(vm.getNonce(Preinstalls.BeaconBlockRootsSender), 1, "4788 sender must have nonce=1");
    }

    function test_preinstall_createX_succeeds() external view {
        assertPreinstall(Preinstalls.CreateX, Preinstalls.CreateXCode);
    }

    function test_createX_runtimeBytecodeHash_works() external view {
        bytes memory createXRuntimeBytecode = Preinstalls.CreateX.code;
        bytes32 createXRuntimeBytecodeHash = keccak256(createXRuntimeBytecode);

        assertEq(
            createXRuntimeBytecodeHash,
            0xbd8a7ea8cfca7b4e5f5041d7d4b17bc317c5ce42cfbc42066a00cf26b43eb53f,
            "CreateX runtime bytecode hash mismatch"
        );
    }
}
