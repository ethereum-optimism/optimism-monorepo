// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { MerkleTrie } from "src/libraries/trie/MerkleTrie.sol";
import { RLPReader } from "src/libraries/rlp/RLPReader.sol";
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

    /// @notice Tests that `get` reverts if a proof node has an unknown prefix
    function test_get_unknownNodePrefix_reverts(uint8 prefix) external {
        // bound it to only have prefixes where the first nibble is >= 4
        prefix = uint8(bound(prefix, 0x40, 0xff));
        // if the first nibble of the prefix is odd, make it even by adding 16
        if (((prefix / 16) % 2) == 1) {
            unchecked {
                prefix += 16;
            }
            // bound it again in case it overflowed
            prefix = uint8(bound(prefix, 0x40, 0xff));
        }

        MerkleTrieWrapper wrapper = new MerkleTrieWrapper();

        bytes memory key = abi.encodePacked(
            keccak256(abi.encodePacked(bytes32(0xa15bc60c955c405d20d9149c709e2460f1c2d9a497496a7f46004d1772c3054c)))
        );
        bytes[] memory proof = new bytes[](5);
        proof[0] =
            hex"f90211a085ed702d58e6a962ad0e785e5c9036e06d878fd065eb9669122447f6aee7957da05badb8cfd5a7493d928614730af6e14eabe2c93fbac93c853dde3270c446309da01de85a57c524ac56a5bd4bed0b0aa7d963e364ad930ea964d0a42631a77ded4da0fe3143892366faeb9fae1117b888263afe0f74e6c73555fee53a604bf7188431a0af2c79f0dddd15d6f62e3fa60d515c44d58444ad3915c7ca4bddb31c8f148d0ca08f37a2f9093a4aee39519f3a06fe4674cc670fbbbd7a5f4eb096310b7bc1fdc9a086bd12d2031d9714130c687e6822250fa24b3147824780bea96cf8a7406c8966a03e42538ba2da8adaa0eca4550ef62de4dabde8ca06b71ac1b276ff31b67a7655a04a439f7eb6a62c77ec921139925af3359f61d16e083076e0e425583289107d7da0c453a51991b5a4c6174ddff77c0b7d9cc86f05ffda6ff523e2365f19797c7a00a06f43b7b9a118264ab4b6c24d7d3de77f39071a298426bfc27180adfca57d590da0032e0db4dcf122d4bdb1d4ec3c5df5fabd3127bcefe412cb046b7f0d80d11c9fa0560c2b8c9466b8cb5ffd600f24ea0ed9838bfdab7870d505d4387c2468d3c498a0597996e939ff8c29c9e59dc47c111e760450a9c4fe2b065825762da2a1f32495a0e3411c9af104364230c49de5a4d0802c84df18beee9778673364e1747a875622a02a6928825356d8280f361a02285af30e73889f4e55ecb63ed85c8581e07061d680";
        proof[1] =
            hex"f90211a0db246208c4cef45e9aeb9f1df1baa8572675bc453f7da538165a2bb9e6a4c416a0d26d82a9ddff901d2a1f9e18506120dec0e5b3c95549c5bff0efc355061ea73fa04f1cedbb5c7df7ee5cc3210980baf5537affb29c661c6a4eeb193bc42e7fbc74a0acea389e0cf577d0e13483d98b15c82618ac18b7cc4a479981e3e672ddd16867a0ef59a06aeea1eb5ba1313bbe1fa74ff264d84d7319ab6178213734b5b5efa9c1a08f85dc6001713d77aa4e12982dfdb28fd1c7dcc290d46f2749e8a7d67ba2a694a0f6698ff794881edc50340b75311de64ce3da5f97debcfdfd4d20de57ef3ba7eba0680071ce05e9c7915f731bac8b9673332d1d77ea1f7dadab36d9b233eea32ba4a035ad3686f436232360c2aa364c9f2aa2081318b9fb33cd1050d69ee46f791d62a03b495b3d65d9ae39680a0f835c1d1378d218f7b1fb88d2b2c6ac6ef916f09172a0a808d1e8c632d9a6cfeb3c2c123a58b5b3e1998d4bd02c5fb0f7c5d4ba1338e6a0369376e9152831135ff3a902c9740cf22951d67edd51bf0541565e379d7efc25a0cc26d7fa1c326bc14950e92d9de78e4ed8372ff9727dec34602f24057b3a9b30a0278081783022e748dc70874a72377038935c00c1f0a24bbb8cd0fc208d8b68f4a06c4e83593571b94d08cb78ece0de4920b02a650a47a16583f94c8fe35f724707a0cd7eb9d730e5138fd943200b577e7bbb827d010a50d19af2f4b19ef19658156d80";
        proof[2] =
            hex"f90211a0065f58fbe63e8e3387e91047d5b7a4532e7d9de0abc47f04791aef27a140fdb5a0858beea29778551c01b0d3e542d707675856da9c3f1d065c845e55c24d77be89a0e90a410489eff6f4f8d70b0cce1fb1339117ec0f6f1db195a6cc410509a2ebaea078ba7fe504e8d01d57f6bee52c4938d779108e500b5923272441ed2964b8c45da0f0430ed9fa807e5fb6ace75f8766ea60009d8539e00006e359f5f7bc38a76596a0a98a7938db99a2d80abea6349de42bf2576c9e51cc715c85fbacab365ec16f5ba026fadc7d124a456c62ada928eaede3e80611e3e6f99041f7393f062e9e788c8ca0ca48cad1e00d22d6146173341a6060378e738be727a7265a923cf6bfd1f8b610a0f8a4aae21a78ac28e2a61f50396f9a80f6c8232fe4afa203160361c0962242baa09a1029479959fb29b4da7239393fd6ca20bc376d860324f429a51b0e0565a158a0eefb84d3943d680e176258dffe0104ac48c171a8574a811535256a2d8ba531dea062a3d709a2f70ba1970842c4f20a602291273d1f6022e7a14cde2afdcd51e795a0397e6b9b87012cd79cbd0bb7daa4cc43830a673d80b65fb88c0449140175d89ca0f8a4c73c0078cbd32961227910e3f9315bc587716062e39f66be19747ccf9b67a0ea4bdd1b187fdba273a8625f88f284994d19c38ec58651839852665717d953d9a0319ebf356f45da83c7f106f1fd3decbf15f651fad3389a0d279602cdea8ee11480";
        proof[3] =
            hex"f8f1a069a092c7a950214e7e45b99012dc8ad112eab0fc94ae5ca9efbd6949068384f280a0b25c46db67ef7cf0c47bb400c31c85a26c5a204431527c964c8ecaf3d63e52cc80a01911a2a74db0d8d182447176e23f25556d1a1eaa0afad96453f2d64876ad88e480808080a04a0ca9e3bed1bc3e3c819384d19b6d5e523164a6520c4eb42e828a63ef730ae38080a03b598ed1b9269d4b05e2e75cfb54298d25437669870c919a59a147d2d256fdba80a0db2d655057c83107a73d086cfdd8fcc74739bb48c652eb0ce597178ecf96b39aa05c66ac392a761341b9c22b773ea19af311f34ef537640b9bb96842ec6ace913280";

        proof[4] = bytes.concat(
            hex"f69f",
            bytes1(prefix),
            hex"4dcf44e265ba93879b2da89e1b16ab48fc5eb8e31bc16b0612d6da8463f195942536c09e5f5691498805884fa37811be3b2bddb4"
        );

        bytes32 root;
        (proof[0], proof[1], proof[2], proof[3], root) = rehashOtherElements(proof[4]);

        vm.expectRevert("MerkleTrie: received a node with an unknown prefix");
        wrapper.get(key, proof, root);
    }

    /// @notice Tests that `get` reverts if a proof node is unparsable i.e list length is not 2 or 17
    function test_get_unparsableNode_reverts(uint8 listLen) external {
        listLen = uint8(bound(listLen, 1, RLPReader.MAX_LIST_LENGTH));
        if (listLen == 2 || listLen == 17) {
            listLen++;
        }

        MerkleTrieWrapper wrapper = new MerkleTrieWrapper();

        bytes memory key = abi.encodePacked(
            keccak256(abi.encodePacked(bytes32(0xa15bc60c955c405d20d9149c709e2460f1c2d9a497496a7f46004d1772c3054c)))
        );
        bytes[] memory proof = new bytes[](5);
        proof[0] =
            hex"f90211a085ed702d58e6a962ad0e785e5c9036e06d878fd065eb9669122447f6aee7957da05badb8cfd5a7493d928614730af6e14eabe2c93fbac93c853dde3270c446309da01de85a57c524ac56a5bd4bed0b0aa7d963e364ad930ea964d0a42631a77ded4da0fe3143892366faeb9fae1117b888263afe0f74e6c73555fee53a604bf7188431a0af2c79f0dddd15d6f62e3fa60d515c44d58444ad3915c7ca4bddb31c8f148d0ca08f37a2f9093a4aee39519f3a06fe4674cc670fbbbd7a5f4eb096310b7bc1fdc9a086bd12d2031d9714130c687e6822250fa24b3147824780bea96cf8a7406c8966a03e42538ba2da8adaa0eca4550ef62de4dabde8ca06b71ac1b276ff31b67a7655a04a439f7eb6a62c77ec921139925af3359f61d16e083076e0e425583289107d7da0c453a51991b5a4c6174ddff77c0b7d9cc86f05ffda6ff523e2365f19797c7a00a06f43b7b9a118264ab4b6c24d7d3de77f39071a298426bfc27180adfca57d590da0032e0db4dcf122d4bdb1d4ec3c5df5fabd3127bcefe412cb046b7f0d80d11c9fa0560c2b8c9466b8cb5ffd600f24ea0ed9838bfdab7870d505d4387c2468d3c498a0597996e939ff8c29c9e59dc47c111e760450a9c4fe2b065825762da2a1f32495a0e3411c9af104364230c49de5a4d0802c84df18beee9778673364e1747a875622a02a6928825356d8280f361a02285af30e73889f4e55ecb63ed85c8581e07061d680";
        proof[1] =
            hex"f90211a0db246208c4cef45e9aeb9f1df1baa8572675bc453f7da538165a2bb9e6a4c416a0d26d82a9ddff901d2a1f9e18506120dec0e5b3c95549c5bff0efc355061ea73fa04f1cedbb5c7df7ee5cc3210980baf5537affb29c661c6a4eeb193bc42e7fbc74a0acea389e0cf577d0e13483d98b15c82618ac18b7cc4a479981e3e672ddd16867a0ef59a06aeea1eb5ba1313bbe1fa74ff264d84d7319ab6178213734b5b5efa9c1a08f85dc6001713d77aa4e12982dfdb28fd1c7dcc290d46f2749e8a7d67ba2a694a0f6698ff794881edc50340b75311de64ce3da5f97debcfdfd4d20de57ef3ba7eba0680071ce05e9c7915f731bac8b9673332d1d77ea1f7dadab36d9b233eea32ba4a035ad3686f436232360c2aa364c9f2aa2081318b9fb33cd1050d69ee46f791d62a03b495b3d65d9ae39680a0f835c1d1378d218f7b1fb88d2b2c6ac6ef916f09172a0a808d1e8c632d9a6cfeb3c2c123a58b5b3e1998d4bd02c5fb0f7c5d4ba1338e6a0369376e9152831135ff3a902c9740cf22951d67edd51bf0541565e379d7efc25a0cc26d7fa1c326bc14950e92d9de78e4ed8372ff9727dec34602f24057b3a9b30a0278081783022e748dc70874a72377038935c00c1f0a24bbb8cd0fc208d8b68f4a06c4e83593571b94d08cb78ece0de4920b02a650a47a16583f94c8fe35f724707a0cd7eb9d730e5138fd943200b577e7bbb827d010a50d19af2f4b19ef19658156d80";
        proof[2] =
            hex"f90211a0065f58fbe63e8e3387e91047d5b7a4532e7d9de0abc47f04791aef27a140fdb5a0858beea29778551c01b0d3e542d707675856da9c3f1d065c845e55c24d77be89a0e90a410489eff6f4f8d70b0cce1fb1339117ec0f6f1db195a6cc410509a2ebaea078ba7fe504e8d01d57f6bee52c4938d779108e500b5923272441ed2964b8c45da0f0430ed9fa807e5fb6ace75f8766ea60009d8539e00006e359f5f7bc38a76596a0a98a7938db99a2d80abea6349de42bf2576c9e51cc715c85fbacab365ec16f5ba026fadc7d124a456c62ada928eaede3e80611e3e6f99041f7393f062e9e788c8ca0ca48cad1e00d22d6146173341a6060378e738be727a7265a923cf6bfd1f8b610a0f8a4aae21a78ac28e2a61f50396f9a80f6c8232fe4afa203160361c0962242baa09a1029479959fb29b4da7239393fd6ca20bc376d860324f429a51b0e0565a158a0eefb84d3943d680e176258dffe0104ac48c171a8574a811535256a2d8ba531dea062a3d709a2f70ba1970842c4f20a602291273d1f6022e7a14cde2afdcd51e795a0397e6b9b87012cd79cbd0bb7daa4cc43830a673d80b65fb88c0449140175d89ca0f8a4c73c0078cbd32961227910e3f9315bc587716062e39f66be19747ccf9b67a0ea4bdd1b187fdba273a8625f88f284994d19c38ec58651839852665717d953d9a0319ebf356f45da83c7f106f1fd3decbf15f651fad3389a0d279602cdea8ee11480";
        proof[3] =
            hex"f8f1a069a092c7a950214e7e45b99012dc8ad112eab0fc94ae5ca9efbd6949068384f280a0b25c46db67ef7cf0c47bb400c31c85a26c5a204431527c964c8ecaf3d63e52cc80a01911a2a74db0d8d182447176e23f25556d1a1eaa0afad96453f2d64876ad88e480808080a04a0ca9e3bed1bc3e3c819384d19b6d5e523164a6520c4eb42e828a63ef730ae38080a03b598ed1b9269d4b05e2e75cfb54298d25437669870c919a59a147d2d256fdba80a0db2d655057c83107a73d086cfdd8fcc74739bb48c652eb0ce597178ecf96b39aa05c66ac392a761341b9c22b773ea19af311f34ef537640b9bb96842ec6ace913280";
        proof[4] =
            hex"f69f204dcf44e265ba93879b2da89e1b16ab48fc5eb8e31bc16b0612d6da8463f195942536c09e5f5691498805884fa37811be3b2bddb4"; // Correct
            // leaf node

        bytes32 root = keccak256(proof[0]);

        // Should not revert
        wrapper.get(key, proof, root);

        if (listLen > 3) {
            // Node with list > 3
            proof[4] =
                hex"f8379f204dcf44e265ba93879b2da89e1b16ab48fc5eb8e31bc16b0612d6da8463f195942536c09e5f5691498805884fa37811be3b2bddb480";
            for (uint256 i; i < listLen - 3; i++) {
                proof[4] = bytes.concat(proof[4], hex"80");
            }
            proof[4][1] = bytes1(uint8(proof[4][1]) + (listLen - 3));
            // rehash all proof elements and insert it into the proof element above it
            (proof[0], proof[1], proof[2], proof[3], root) = rehashOtherElements(proof[4]);

            vm.expectRevert("MerkleTrie: received an unparseable node");
            wrapper.get(key, proof, root);
        } else if (listLen == 1) {
            // Node with list of 1
            proof[4] = hex"e09f204dcf44e265ba93879b2da89e1b16ab48fc5eb8e31bc16b0612d6da8463f1";
            // rehash all proof elements and insert it into the proof element above it
            (proof[0], proof[1], proof[2], proof[3], root) = rehashOtherElements(proof[4]);

            vm.expectRevert("MerkleTrie: received an unparseable node");
            wrapper.get(key, proof, root);
        } else if (listLen == 3) {
            // Node with list of 3
            proof[4] =
                hex"f79f204dcf44e265ba93879b2da89e1b16ab48fc5eb8e31bc16b0612d6da8463f195942536c09e5f5691498805884fa37811be3b2bddb480";
            // rehash all proof elements and insert it into the proof element above it
            (proof[0], proof[1], proof[2], proof[3], root) = rehashOtherElements(proof[4]);

            vm.expectRevert("MerkleTrie: received an unparseable node");
            wrapper.get(key, proof, root);
        }
    }

    function rehashOtherElements(bytes memory _proof4)
        private
        pure
        returns (bytes memory proof0_, bytes memory proof1_, bytes memory proof2_, bytes memory proof3_, bytes32 root_)
    {
        // rehash all proof elements and insert it into the proof element above it
        proof3_ = bytes.concat(
            hex"f8f1a069a092c7a950214e7e45b99012dc8ad112eab0fc94ae5ca9efbd6949068384f280a0b25c46db67ef7cf0c47bb400c31c85a26c5a204431527c964c8ecaf3d63e52cc80a0",
            keccak256(_proof4),
            hex"80808080a04a0ca9e3bed1bc3e3c819384d19b6d5e523164a6520c4eb42e828a63ef730ae38080a03b598ed1b9269d4b05e2e75cfb54298d25437669870c919a59a147d2d256fdba80a0db2d655057c83107a73d086cfdd8fcc74739bb48c652eb0ce597178ecf96b39aa05c66ac392a761341b9c22b773ea19af311f34ef537640b9bb96842ec6ace913280"
        );
        proof2_ = bytes.concat(
            hex"f90211a0065f58fbe63e8e3387e91047d5b7a4532e7d9de0abc47f04791aef27a140fdb5a0858beea29778551c01b0d3e542d707675856da9c3f1d065c845e55c24d77be89a0e90a410489eff6f4f8d70b0cce1fb1339117ec0f6f1db195a6cc410509a2ebaea078ba7fe504e8d01d57f6bee52c4938d779108e500b5923272441ed2964b8c45da0f0430ed9fa807e5fb6ace75f8766ea60009d8539e00006e359f5f7bc38a76596a0a98a7938db99a2d80abea6349de42bf2576c9e51cc715c85fbacab365ec16f5ba0",
            keccak256(proof3_),
            hex"a0ca48cad1e00d22d6146173341a6060378e738be727a7265a923cf6bfd1f8b610a0f8a4aae21a78ac28e2a61f50396f9a80f6c8232fe4afa203160361c0962242baa09a1029479959fb29b4da7239393fd6ca20bc376d860324f429a51b0e0565a158a0eefb84d3943d680e176258dffe0104ac48c171a8574a811535256a2d8ba531dea062a3d709a2f70ba1970842c4f20a602291273d1f6022e7a14cde2afdcd51e795a0397e6b9b87012cd79cbd0bb7daa4cc43830a673d80b65fb88c0449140175d89ca0f8a4c73c0078cbd32961227910e3f9315bc587716062e39f66be19747ccf9b67a0ea4bdd1b187fdba273a8625f88f284994d19c38ec58651839852665717d953d9a0319ebf356f45da83c7f106f1fd3decbf15f651fad3389a0d279602cdea8ee11480"
        );
        proof1_ = bytes.concat(
            hex"f90211a0db246208c4cef45e9aeb9f1df1baa8572675bc453f7da538165a2bb9e6a4c416a0d26d82a9ddff901d2a1f9e18506120dec0e5b3c95549c5bff0efc355061ea73fa04f1cedbb5c7df7ee5cc3210980baf5537affb29c661c6a4eeb193bc42e7fbc74a0acea389e0cf577d0e13483d98b15c82618ac18b7cc4a479981e3e672ddd16867a0ef59a06aeea1eb5ba1313bbe1fa74ff264d84d7319ab6178213734b5b5efa9c1a08f85dc6001713d77aa4e12982dfdb28fd1c7dcc290d46f2749e8a7d67ba2a694a0f6698ff794881edc50340b75311de64ce3da5f97debcfdfd4d20de57ef3ba7eba0680071ce05e9c7915f731bac8b9673332d1d77ea1f7dadab36d9b233eea32ba4a035ad3686f436232360c2aa364c9f2aa2081318b9fb33cd1050d69ee46f791d62a03b495b3d65d9ae39680a0f835c1d1378d218f7b1fb88d2b2c6ac6ef916f09172a0a808d1e8c632d9a6cfeb3c2c123a58b5b3e1998d4bd02c5fb0f7c5d4ba1338e6a0369376e9152831135ff3a902c9740cf22951d67edd51bf0541565e379d7efc25a0",
            keccak256(proof2_),
            hex"a0278081783022e748dc70874a72377038935c00c1f0a24bbb8cd0fc208d8b68f4a06c4e83593571b94d08cb78ece0de4920b02a650a47a16583f94c8fe35f724707a0cd7eb9d730e5138fd943200b577e7bbb827d010a50d19af2f4b19ef19658156d80"
        );
        proof0_ = bytes.concat(
            hex"f90211a085ed702d58e6a962ad0e785e5c9036e06d878fd065eb9669122447f6aee7957da05badb8cfd5a7493d928614730af6e14eabe2c93fbac93c853dde3270c446309da0",
            keccak256(proof1_),
            hex"a0fe3143892366faeb9fae1117b888263afe0f74e6c73555fee53a604bf7188431a0af2c79f0dddd15d6f62e3fa60d515c44d58444ad3915c7ca4bddb31c8f148d0ca08f37a2f9093a4aee39519f3a06fe4674cc670fbbbd7a5f4eb096310b7bc1fdc9a086bd12d2031d9714130c687e6822250fa24b3147824780bea96cf8a7406c8966a03e42538ba2da8adaa0eca4550ef62de4dabde8ca06b71ac1b276ff31b67a7655a04a439f7eb6a62c77ec921139925af3359f61d16e083076e0e425583289107d7da0c453a51991b5a4c6174ddff77c0b7d9cc86f05ffda6ff523e2365f19797c7a00a06f43b7b9a118264ab4b6c24d7d3de77f39071a298426bfc27180adfca57d590da0032e0db4dcf122d4bdb1d4ec3c5df5fabd3127bcefe412cb046b7f0d80d11c9fa0560c2b8c9466b8cb5ffd600f24ea0ed9838bfdab7870d505d4387c2468d3c498a0597996e939ff8c29c9e59dc47c111e760450a9c4fe2b065825762da2a1f32495a0e3411c9af104364230c49de5a4d0802c84df18beee9778673364e1747a875622a02a6928825356d8280f361a02285af30e73889f4e55ecb63ed85c8581e07061d680"
        );
        root_ = keccak256(proof0_);
    }
}

contract MerkleTrieWrapper {
    function get(bytes memory key, bytes[] memory proof, bytes32 root) external pure returns (bytes memory) {
        return MerkleTrie.get(key, proof, root);
    }
}
