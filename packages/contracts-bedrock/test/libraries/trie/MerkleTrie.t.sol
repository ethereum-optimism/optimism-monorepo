// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { MerkleTrie } from "src/libraries/trie/MerkleTrie.sol";
import { FFIInterface } from "test/setup/FFIInterface.sol";
import "src/libraries/rlp/RLPErrors.sol";

contract MerkleTrie_get_Test is Test {
    FFIInterface constant ffi = FFIInterface(address(uint160(uint256(keccak256(abi.encode("optimism.ffi"))))));

    function setUp() public {
        vm.etch(address(ffi), vm.getDeployedCode("FFIInterface.sol:FFIInterface"));
        vm.label(address(ffi), "FFIInterface");
    }

    function test_get_validProof1_succeeds() external pure {
        bytes32 root = 0xd582f99275e227a1cf4284899e5ff06ee56da8859be71b553397c69151bc942f;
        bytes memory key = hex"6b6579326262";
        bytes memory val = hex"6176616c32";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e68416b65793a03101b4447781f1e6c51ce76c709274fc80bd064f3a58ff981b6015348a826386";
        proof[1] =
            hex"f84580a0582eed8dd051b823d13f8648cdcd08aa2d8dac239f458863c4620e8c4d605debca83206262856176616c32ca83206363856176616c3380808080808080808080808080";
        proof[2] = hex"ca83206262856176616c32";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof2_succeeds() external pure {
        bytes32 root = 0xd582f99275e227a1cf4284899e5ff06ee56da8859be71b553397c69151bc942f;
        bytes memory key = hex"6b6579316161";
        bytes memory val = hex"303132333435363738393031323334353637383930313233343536373839303132333435363738397878";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e68416b65793a03101b4447781f1e6c51ce76c709274fc80bd064f3a58ff981b6015348a826386";
        proof[1] =
            hex"f84580a0582eed8dd051b823d13f8648cdcd08aa2d8dac239f458863c4620e8c4d605debca83206262856176616c32ca83206363856176616c3380808080808080808080808080";
        proof[2] = hex"ef83206161aa303132333435363738393031323334353637383930313233343536373839303132333435363738397878";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof3_succeeds() external pure {
        bytes32 root = 0xf838216fa749aefa91e0b672a9c06d3e6e983f913d7107b5dab4af60b5f5abed;
        bytes memory key = hex"6b6579316161";
        bytes memory val = hex"303132333435363738393031323334353637383930313233343536373839303132333435363738397878";
        bytes[] memory proof = new bytes[](1);
        proof[0] =
            hex"f387206b6579316161aa303132333435363738393031323334353637383930313233343536373839303132333435363738397878";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof4_succeeds() external pure {
        bytes32 root = 0x37956bab6bba472308146808d5311ac19cb4a7daae5df7efcc0f32badc97f55e;
        bytes memory key = hex"6b6579316161";
        bytes memory val = hex"3031323334";
        bytes[] memory proof = new bytes[](1);
        proof[0] = hex"ce87206b6579316161853031323334";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof5_succeeds() external pure {
        bytes32 root = 0xcb65032e2f76c48b82b5c24b3db8f670ce73982869d38cd39a624f23d62a9e89;
        bytes memory key = hex"6b657931";
        bytes memory val =
            hex"30313233343536373839303132333435363738393031323334353637383930313233343536373839566572795f4c6f6e67";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e68416b65793a0f3f387240403976788281c0a6ee5b3fc08360d276039d635bb824ea7e6fed779";
        proof[1] =
            hex"f87180a034d14ccc7685aa2beb64f78b11ee2a335eae82047ef97c79b7dda7f0732b9f4ca05fb052b64e23d177131d9f32e9c5b942209eb7229e9a07c99a5d93245f53af18a09a137197a43a880648d5887cce656a5e6bbbe5e44ecb4f264395ccaddbe1acca80808080808080808080808080";
        proof[2] =
            hex"f862808080808080a057895fdbd71e2c67c2f9274a56811ff5cf458720a7fa713a135e3890f8cafcf8808080808080808080b130313233343536373839303132333435363738393031323334353637383930313233343536373839566572795f4c6f6e67";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof6_succeeds() external pure {
        bytes32 root = 0xcb65032e2f76c48b82b5c24b3db8f670ce73982869d38cd39a624f23d62a9e89;
        bytes memory key = hex"6b657932";
        bytes memory val = hex"73686f7274";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e68416b65793a0f3f387240403976788281c0a6ee5b3fc08360d276039d635bb824ea7e6fed779";
        proof[1] =
            hex"f87180a034d14ccc7685aa2beb64f78b11ee2a335eae82047ef97c79b7dda7f0732b9f4ca05fb052b64e23d177131d9f32e9c5b942209eb7229e9a07c99a5d93245f53af18a09a137197a43a880648d5887cce656a5e6bbbe5e44ecb4f264395ccaddbe1acca80808080808080808080808080";
        proof[2] = hex"df808080808080c9823262856176616c338080808080808080808573686f7274";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof7_succeeds() external pure {
        bytes32 root = 0xcb65032e2f76c48b82b5c24b3db8f670ce73982869d38cd39a624f23d62a9e89;
        bytes memory key = hex"6b657933";
        bytes memory val = hex"31323334353637383930313233343536373839303132333435363738393031";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e68416b65793a0f3f387240403976788281c0a6ee5b3fc08360d276039d635bb824ea7e6fed779";
        proof[1] =
            hex"f87180a034d14ccc7685aa2beb64f78b11ee2a335eae82047ef97c79b7dda7f0732b9f4ca05fb052b64e23d177131d9f32e9c5b942209eb7229e9a07c99a5d93245f53af18a09a137197a43a880648d5887cce656a5e6bbbe5e44ecb4f264395ccaddbe1acca80808080808080808080808080";
        proof[2] =
            hex"f839808080808080c9823363856176616c338080808080808080809f31323334353637383930313233343536373839303132333435363738393031";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof8_succeeds() external pure {
        bytes32 root = 0x72e6c01ad0c9a7b517d4bc68a5b323287fe80f0e68f5415b4b95ecbc8ad83978;
        bytes memory key = hex"61";
        bytes memory val = hex"61";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"d916d780c22061c22062c2206380808080808080808080808080";
        proof[1] = hex"d780c22061c22062c2206380808080808080808080808080";
        proof[2] = hex"c22061";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof9_succeeds() external pure {
        bytes32 root = 0x72e6c01ad0c9a7b517d4bc68a5b323287fe80f0e68f5415b4b95ecbc8ad83978;
        bytes memory key = hex"62";
        bytes memory val = hex"62";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"d916d780c22061c22062c2206380808080808080808080808080";
        proof[1] = hex"d780c22061c22062c2206380808080808080808080808080";
        proof[2] = hex"c22062";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_validProof10_succeeds() external pure {
        bytes32 root = 0x72e6c01ad0c9a7b517d4bc68a5b323287fe80f0e68f5415b4b95ecbc8ad83978;
        bytes memory key = hex"63";
        bytes memory val = hex"63";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"d916d780c22061c22062c2206380808080808080808080808080";
        proof[1] = hex"d780c22061c22062c2206380808080808080808080808080";
        proof[2] = hex"c22063";

        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    function test_get_nonexistentKey1_reverts() external {
        bytes32 root = 0xd582f99275e227a1cf4284899e5ff06ee56da8859be71b553397c69151bc942f;
        bytes memory key = hex"6b657932";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e68416b65793a03101b4447781f1e6c51ce76c709274fc80bd064f3a58ff981b6015348a826386";
        proof[1] =
            hex"f84580a0582eed8dd051b823d13f8648cdcd08aa2d8dac239f458863c4620e8c4d605debca83206262856176616c32ca83206363856176616c3380808080808080808080808080";
        proof[2] = hex"ca83206262856176616c32";

        vm.expectRevert("MerkleTrie: path remainder must share all nibbles with key");
        MerkleTrie.get(key, proof, root);
    }

    function test_get_nonexistentKey2_reverts() external {
        bytes32 root = 0xd582f99275e227a1cf4284899e5ff06ee56da8859be71b553397c69151bc942f;
        bytes memory key = hex"616e7972616e646f6d6b6579";
        bytes[] memory proof = new bytes[](1);
        proof[0] = hex"e68416b65793a03101b4447781f1e6c51ce76c709274fc80bd064f3a58ff981b6015348a826386";

        vm.expectRevert("MerkleTrie: path remainder must share all nibbles with key");
        MerkleTrie.get(key, proof, root);
    }

    function test_get_wrongKeyProof_reverts() external {
        bytes32 root = 0x2858eebfa9d96c8a9e6a0cae9d86ec9189127110f132d63f07d3544c2a75a696;
        bytes memory key = hex"6b6579316161";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e216a04892c039d654f1be9af20e88ae53e9ab5fa5520190e0fb2f805823e45ebad22f";
        proof[1] =
            hex"f84780d687206e6f746865728d33343938683472697568677765808080808080808080a0854405b57aa6dc458bc41899a761cbbb1f66a4998af6dd0e8601c1b845395ae38080808080";
        proof[2] = hex"d687206e6f746865728d33343938683472697568677765";

        vm.expectRevert("MerkleTrie: invalid internal node hash");
        MerkleTrie.get(key, proof, root);
    }

    function test_get_corruptedProof_reverts() external {
        bytes32 root = 0x2858eebfa9d96c8a9e6a0cae9d86ec9189127110f132d63f07d3544c2a75a696;
        bytes memory key = hex"6b6579326262";
        bytes[] memory proof = new bytes[](5);
        proof[0] = hex"2fd2ba5ee42358802ffbe0900152a55fabe953ae880ef29abef154d639c09248a016e2";
        proof[1] =
            hex"f84780d687206e6f746865728d33343938683472697568677765808080808080808080a0854405b57aa6dc458bc41899a761cbbb1f66a4998af6dd0e8601c1b845395ae38080808080";
        proof[2] = hex"e583165793a03101b4447781f1e6c51ce76c709274fc80bd064f3a58ff981b6015348a826386";
        proof[3] =
            hex"f84580a0582eed8dd051b823d13f8648cdcd08aa2d8dac239f458863c4620e8c4d605debca83206262856176616c32ca83206363856176616c3380808080808080808080808080";
        proof[4] = hex"ca83206262856176616c32";

        vm.expectRevert(UnexpectedString.selector);
        MerkleTrie.get(key, proof, root);
    }

    function test_get_invalidDataRemainder_reverts() external {
        bytes32 root = 0x278c88eb59beba4f8b94f940c41614bb0dd80c305859ebffcd6ce07c93ca3749;
        bytes memory key = hex"aa";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"d91ad780808080808080808080c32081aac32081ab8080808080";
        proof[1] = hex"d780808080808080808080c32081aac32081ab8080808080";
        proof[2] = hex"c32081aa000000000000000000000000000000";

        vm.expectRevert(InvalidDataRemainder.selector);
        MerkleTrie.get(key, proof, root);
    }

    function test_get_invalidInternalNodeHash_reverts() external {
        bytes32 root = 0xa827dff1a657bb9bb9a1c3abe9db173e2f1359f15eb06f1647ea21ac7c95d8fa;
        bytes memory key = hex"aa";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e21aa09862c6b113008c4204c13755693cbb868acc25ebaa98db11df8c89a0c0dd3157";
        proof[1] =
            hex"f380808080808080808080a0de2a9c6a46b6ea71ab9e881c8420570cf19e833c85df6026b04f085016e78f00c220118080808080";
        proof[2] = hex"de2a9c6a46b6ea71ab9e881c8420570cf19e833c85df6026b04f085016e78f";

        vm.expectRevert("MerkleTrie: invalid internal node hash");
        MerkleTrie.get(key, proof, root);
    }

    function test_get_zeroBranchValueLength_reverts() external {
        bytes32 root = 0xe04b3589eef96b237cd49ccb5dcf6e654a47682bfa0961d563ab843f7ad1e035;
        bytes memory key = hex"aa";
        bytes[] memory proof = new bytes[](2);
        proof[0] = hex"dd8200aad98080808080808080808080c43b82aabbc43c82aacc80808080";
        proof[1] = hex"d98080808080808080808080c43b82aabbc43c82aacc80808080";

        vm.expectRevert("MerkleTrie: value length must be greater than zero (branch)");
        MerkleTrie.get(key, proof, root);
    }

    function test_get_zeroLengthKey_reverts() external {
        bytes32 root = 0x54157fd62cdf2f474e7bfec2d3cd581e807bee38488c9590cb887add98936b73;
        bytes memory key = hex"";
        bytes[] memory proof = new bytes[](1);
        proof[0] = hex"c78320f00082b443";

        vm.expectRevert("MerkleTrie: empty key");
        MerkleTrie.get(key, proof, root);
    }

    function test_get_smallerPathThanKey1_reverts() external {
        bytes32 root = 0xa513ba530659356fb7588a2c831944e80fd8aedaa5a4dc36f918152be2be0605;
        bytes memory key = hex"01";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"db10d9c32081bbc582202381aa808080808080808080808080808080";
        proof[1] = hex"d9c32081bbc582202381aa808080808080808080808080808080";
        proof[2] = hex"c582202381aa";

        vm.expectRevert("MerkleTrie: path remainder must share all nibbles with key");
        MerkleTrie.get(key, proof, root);
    }

    function test_get_smallerPathThanKey2_reverts() external {
        bytes32 root = 0xa06abffaec4ebe8ccde595f4547b864b4421b21c1fc699973f94710c9bc17979;
        bytes memory key = hex"aa";
        bytes[] memory proof = new bytes[](3);
        proof[0] = hex"e21aa07ea462226a3dc0a46afb4ded39306d7a84d311ada3557dfc75a909fd25530905";
        proof[1] =
            hex"f380808080808080808080a027f11bd3af96d137b9287632f44dd00fea1ca1bd70386c30985ede8cc287476e808080c220338080";
        proof[2] = hex"e48200bba0a6911545ed01c2d3f4e15b8b27c7bfba97738bd5e6dd674dd07033428a4c53af";

        vm.expectRevert("MerkleTrie: path remainder must share all nibbles with key");
        MerkleTrie.get(key, proof, root);
    }

    function test_get_extraProofElements_reverts() external {
        bytes32 root = 0x278c88eb59beba4f8b94f940c41614bb0dd80c305859ebffcd6ce07c93ca3749;
        bytes memory key = hex"aa";
        bytes[] memory proof = new bytes[](4);
        proof[0] = hex"d91ad780808080808080808080c32081aac32081ab8080808080";
        proof[1] = hex"d780808080808080808080c32081aac32081ab8080808080";
        proof[2] = hex"c32081aa";
        proof[3] = hex"c32081aa";

        vm.expectRevert("MerkleTrie: value node must be last node in proof (leaf)");
        MerkleTrie.get(key, proof, root);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_validProofs_succeeds(bytes4) external {
        // Generate a test case with a valid proof of inclusion for the k/v pair in the trie.
        (bytes32 root, bytes memory key, bytes memory val, bytes[] memory proof) = ffi.getMerkleTrieFuzzCase("valid");

        // Assert that our expected value is equal to our actual value.
        assertEq(val, MerkleTrie.get(key, proof, root));
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_invalidRoot_reverts(bytes4) external {
        // Get a random test case with a valid trie / proof
        (bytes32 root, bytes memory key,, bytes[] memory proof) = ffi.getMerkleTrieFuzzCase("valid");

        bytes32 rootHash = keccak256(abi.encodePacked(root));
        vm.expectRevert("MerkleTrie: invalid root hash");
        MerkleTrie.get(key, proof, rootHash);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_extraProofElements_reverts(bytes4) external {
        // Generate an invalid test case with an extra proof element attached to an otherwise
        // valid proof of inclusion for the passed k/v.
        (bytes32 root, bytes memory key,, bytes[] memory proof) = ffi.getMerkleTrieFuzzCase("extra_proof_elems");

        vm.expectRevert("MerkleTrie: value node must be last node in proof (leaf)");
        MerkleTrie.get(key, proof, root);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_invalidLargeInternalHash_reverts(bytes4) external {
        // Generate an invalid test case where a long proof element is incorrect for the root.
        (bytes32 root, bytes memory key,, bytes[] memory proof) =
            ffi.getMerkleTrieFuzzCase("invalid_large_internal_hash");

        vm.expectRevert("MerkleTrie: invalid large internal hash");
        MerkleTrie.get(key, proof, root);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_invalidInternalNodeHash_reverts(bytes4) external {
        // Generate an invalid test case where a small proof element is incorrect for the root.
        (bytes32 root, bytes memory key,, bytes[] memory proof) =
            ffi.getMerkleTrieFuzzCase("invalid_internal_node_hash");

        vm.expectRevert("MerkleTrie: invalid internal node hash");
        MerkleTrie.get(key, proof, root);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_corruptedProof_reverts(bytes4) external {
        // Generate an invalid test case where the proof is malformed.
        (bytes32 root, bytes memory key,, bytes[] memory proof) = ffi.getMerkleTrieFuzzCase("corrupted_proof");

        vm.expectRevert(UnexpectedString.selector);
        MerkleTrie.get(key, proof, root);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_invalidDataRemainder_reverts(bytes4) external {
        // Generate an invalid test case where a random element of the proof has more bytes than the
        // length designates within the RLP list encoding.
        (bytes32 root, bytes memory key,, bytes[] memory proof) = ffi.getMerkleTrieFuzzCase("invalid_data_remainder");

        vm.expectRevert(InvalidDataRemainder.selector);
        MerkleTrie.get(key, proof, root);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_prefixedValidKey_reverts(bytes4) external {
        // Get a random test case with a valid trie / proof and a valid key that is prefixed
        // with random bytes
        (bytes32 root, bytes memory key,, bytes[] memory proof) = ffi.getMerkleTrieFuzzCase("prefixed_valid_key");

        // Ambiguous revert check- all that we care is that it *does* fail. This case may
        // fail within different branches.
        vm.expectRevert(); // nosemgrep: sol-safety-expectrevert-no-args
        MerkleTrie.get(key, proof, root);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_emptyKey_reverts(bytes4) external {
        // Get a random test case with a valid trie / proof and an empty key
        (bytes32 root, bytes memory key,, bytes[] memory proof) = ffi.getMerkleTrieFuzzCase("empty_key");

        vm.expectRevert("MerkleTrie: empty key");
        MerkleTrie.get(key, proof, root);
    }

    /// @notice The `bytes4` parameter is to enable parallel fuzz runs; it is ignored.
    function testFuzz_get_partialProof_reverts(bytes4) external {
        // Get a random test case with a valid trie / partially correct proof
        (bytes32 root, bytes memory key,, bytes[] memory proof) = ffi.getMerkleTrieFuzzCase("partial_proof");

        vm.expectRevert("MerkleTrie: ran out of proof elements");
        MerkleTrie.get(key, proof, root);
    }
}
