// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { MIPSInstructions } from "src/cannon/libraries/MIPSInstructions.sol";
import { MIPSSyscalls as sys } from "src/cannon/libraries/MIPSSyscalls.sol";
import { InvalidExitedValue, InvalidMemoryProof } from "src/cannon/libraries/CannonErrors.sol";
import "src/dispute/lib/Types.sol";

// Interfaces
import { IMIPS } from "src/cannon/interfaces/IMIPS.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";

contract MIPS_Test is CommonTest {
    IMIPS internal mips;
    IPreimageOracle internal oracle;

    function setUp() public virtual override {
        super.setUp();
        oracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
            })
        );
        mips = IMIPS(
            DeployUtils.create1({
                _name: "MIPS",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IMIPS.__constructor__, (oracle)))
            })
        );
        vm.store(address(mips), 0x0, bytes32(abi.encode(address(oracle))));
        vm.label(address(oracle), "PreimageOracle");
        vm.label(address(mips), "MIPS");
    }

    /// @notice Used to debug step() behavior given a specific input.
    /// This is useful to more easily debug non-forge tests.
    /// For example, in cannon/mipsevm/evm_test.go step input can be pulled here:
    /// https://github.com/ethereum-optimism/optimism/blob/1f64dd6db5561f3bb76ed1d1ffdaff0cde9b7c4b/cannon/mipsevm/evm_test.go#L80-L80
    function test_step_debug_succeeds() external {
        bytes memory oracleInput =
            hex"e15926110000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000081234567898765432000000000000000000000000000000000000000000000000";
        (bool oracleSuccess,) = address(oracle).call(oracleInput);
        assertTrue(oracleSuccess);

        bytes memory input =
            hex"e14ced3200000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000180000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e2d0637506e48299469d82d71a947c6771e7d3eaa3db30b44b30b3ac6ba6596c79029df7322a2404a59aebdffb81ab72dd22cc4459131675e63231b2c693676cf100000008f1f85ff4f1f85ff82ad7bda1e5acea1049e0bfd000001f5b0412ffd341c045e665389becadf100000fa3c64fbd7f000000050000ff00000000015b3d97166d1aec28829f3dd43d8cf1f9358e4103b16d09d466e2c7c048ea3ba1aef3141e700270581aa0b75b50e34fc926bb2d83bb3938f8506d442d5e545ba3a5d214515c11955d8ad50cfb04a6a0e484a2a29f1d688138c1883f289a45a6d5d9c37ebe000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000070000000000000000000000000000000000000000000000000c00000000000000000000000000000000000000000000000000000000000000000000000000000000ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5b4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d3021ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85e58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a193440eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968ffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f839867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756afcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0f9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5f8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf8923490c6ceeb450aecdc82e28293031d10c7d73bf85e57bf041a97360aa2c5d99cc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8beccda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d22733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981fe1d3b5c807b281e4683cc6d6315cf95b9ade8641defcb32372f1c126e398ef7a5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0b46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0c65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2f4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd95a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e3774df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652cdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618df7a66599e9dd7409a7d8de62e29bea7821f0f19cfb783952bff507c87eba2365ffffffff000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5b4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d3021ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85e58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a193440eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968ffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f839867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756afcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0f9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5f8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf8923490c6ceeb450aecdc82e28293031d10c7d73bf85e57bf041a97360aa2c5d99cc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8beccda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d22733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981fe1d3b5c807b281e4683cc6d6315cf95b9ade8641defcb32372f1c126e398ef7a5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0b46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0c65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2f4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd95a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e3774df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652cdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618d53cee4e252442dad999b85c644aa0a6fd8bca90f35f1a5ae696cb8d9eb5a35be";
        (bool success, bytes memory retVal) = address(mips).call(input);
        bytes memory expectedRetVal = hex"03dacdac4e61d89774a305dd0828063706ad878bb6353c0c2cd787d1e5cddd67";

        assertTrue(success);
        assertEq(retVal.length, 32, "Expect a bytes32 hash of the post-state to be returned");
        assertEq(retVal, expectedRetVal);
    }

    function test_step_abi_succeeds() external {
        uint32[32] memory registers;
        registers[16] = 0xbfff0000;
        IMIPS.State memory state = IMIPS.State({
            memRoot: hex"30be14bdf94d7a93989a6263f1e116943dc052d584730cae844bf330dfddce2f",
            preimageKey: bytes32(0),
            preimageOffset: 0,
            pc: 4,
            nextPC: 8,
            lo: 0,
            hi: 0,
            heap: 0,
            exitCode: 0,
            exited: false,
            step: 1,
            registers: registers
        });
        bytes memory proof =
            hex"3c10bfff3610fff0341100013c08ffff3508fffd34090003010950202d420001ae020008ae11000403e000080000000000000000000000000000000000000000ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5b4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d3021ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85e58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a193440eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968ffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f839867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756afcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0f9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5f8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf8923490c6ceeb450aecdc82e28293031d10c7d73bf85e57bf041a97360aa2c5d99cc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8beccda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d22733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981fe1d3b5c807b281e4683cc6d6315cf95b9ade8641defcb32372f1c126e398ef7a5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0b46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0c65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2f4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd95a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e3774df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652cdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618db8cd74046ff337f0a7bf2c8e03e10f642c1886798d71806ab1e888d9e5ee87d00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000";

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertNotEq(postState, bytes32(0));
    }

    /// @notice Tests that the mips step function fails when the value of the exited field is
    ///         invalid (anything greater than 1).
    function test_step_invalidExitedValue_fails() external {
        // Bound to invalid exited values.
        for (uint8 exited = 2; exited <= type(uint8).max && exited != 0;) {
            // Rest of this stuff doesn't matter very much, just setting up some state to edit.
            // Here just using the parameters for the ADD test below.
            uint32 insn = encodespec(17, 18, 8, 0x20);
            (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);

            // Compute the encoded state and manipulate it.
            bytes memory enc = encodeState(state);
            assembly {
                // Push offset by an additional 32 bytes (0x20) to account for length prefix
                mstore8(add(add(enc, 0x20), 89), exited)
            }

            // Call the step function and expect a revert.
            vm.expectRevert(InvalidExitedValue.selector);
            mips.step(enc, proof, 0);
            unchecked {
                exited++;
            }
        }
    }

    function test_add_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x20); // add t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 12;
        state.registers[18] = 20;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] + state.registers[18]; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_addu_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x21); // addu t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 12;
        state.registers[18] = 20;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] + state.registers[18]; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_addi_succeeds() external {
        uint16 imm = 40;
        uint32 insn = encodeitype(0x8, 17, 8, imm); // addi t0, s1, 40
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 1; // t0
        state.registers[17] = 4; // s1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] + imm;
        expect.registers[17] = state.registers[17];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_addiSign_succeeds() external {
        uint16 imm = 0xfffe; // -2
        uint32 insn = encodeitype(0x8, 17, 8, imm); // addi t0, s1, 40
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 1; // t0
        state.registers[17] = 2; // s1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0;
        expect.registers[17] = state.registers[17];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_addui_succeeds() external {
        uint16 imm = 40;
        uint32 insn = encodeitype(0x9, 17, 8, imm); // addui t0, s1, 40
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 1; // t0
        state.registers[17] = 4; // s1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] + imm;
        expect.registers[17] = state.registers[17];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sub_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x22); // sub t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 20;
        state.registers[18] = 12;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] - state.registers[18]; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_subu_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x23); // subu t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 20;
        state.registers[18] = 12;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] - state.registers[18]; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_and_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x24); // and t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 1200;
        state.registers[18] = 490;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] & state.registers[18]; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_andi_succeeds() external {
        uint16 imm = 40;
        uint32 insn = encodeitype(0xc, 17, 8, imm); // andi t0, s1, 40
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 1; // t0
        state.registers[17] = 4; // s1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] & imm;
        expect.registers[17] = state.registers[17];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_or_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x25); // or t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 1200;
        state.registers[18] = 490;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] | state.registers[18]; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_ori_succeeds() external {
        uint16 imm = 40;
        uint32 insn = encodeitype(0xd, 17, 8, imm); // ori t0, s1, 40
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 1; // t0
        state.registers[17] = 4; // s1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] | imm;
        expect.registers[17] = state.registers[17];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_xor_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x26); // xor t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 1200;
        state.registers[18] = 490;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] ^ state.registers[18]; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_xori_succeeds() external {
        uint16 imm = 40;
        uint32 insn = encodeitype(0xe, 17, 8, imm); // xori t0, s1, 40
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 1; // t0
        state.registers[17] = 4; // s1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] ^ imm;
        expect.registers[17] = state.registers[17];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_nor_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x27); // nor t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 1200;
        state.registers[18] = 490;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = ~(state.registers[17] | state.registers[18]); // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_slt_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x2a); // slt t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 0xFF_FF_FF_FE; // -2
        state.registers[18] = 5;

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 1; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        // swap and check again
        uint32 tmp = state.registers[17];
        state.registers[17] = state.registers[18];
        state.registers[18] = tmp;
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];
        expect.registers[8] = 0; // t0
        postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sltu_succeeds() external {
        uint32 insn = encodespec(17, 18, 8, 0x2b); // sltu t0, s1, s2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[17] = 1200;
        state.registers[18] = 490;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[17] < state.registers[18] ? 1 : 0; // t0
        expect.registers[17] = state.registers[17];
        expect.registers[18] = state.registers[18];

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lb_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x20, 0x9, 0x8, 0x4); // lb $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0x12_00_00_00);
        state.registers[8] = 0; // t0
        state.registers[9] = t1;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0x12; // t0
        expect.registers[9] = t1;

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lh_succeeds() external {
        uint32 t1 = 0x100;
        uint32 val = 0x12_23_00_00;
        uint32 insn = encodeitype(0x21, 0x9, 0x8, 0x4); // lh $t0, 4($t1)

        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, val);
        state.registers[8] = 0; // t0
        state.registers[9] = t1;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0x12_23; // t0
        expect.registers[9] = t1;

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lw_succeeds() external {
        uint32 t1 = 0x100;
        uint32 val = 0x12_23_45_67;
        uint32 insn = encodeitype(0x23, 0x9, 0x8, 0x4); // lw $t0, 4($t1)

        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, val);
        state.registers[8] = 0; // t0
        state.registers[9] = t1;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = val; // t0
        expect.registers[9] = t1;

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lbu_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x24, 0x9, 0x8, 0x4); // lbu $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0x12_23_00_00);
        state.registers[8] = 0; // t0
        state.registers[9] = t1;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0x12; // t0
        expect.registers[9] = t1;

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lhu_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x25, 0x9, 0x8, 0x4); // lhu $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0x12_23_00_00);
        state.registers[8] = 0; // t0
        state.registers[9] = t1;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0x12_23; // t0
        expect.registers[9] = t1;

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lwl_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x22, 0x9, 0x8, 0x4); // lwl $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0x12_34_56_78);
        state.registers[8] = 0xaa_bb_cc_dd; // t0
        state.registers[9] = t1;

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0x12_34_56_78; // t0
        expect.registers[9] = t1;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        // test unaligned address
        insn = encodeitype(0x22, 0x9, 0x8, 0x5); // lwl $t0, 5($t1)
        (state.memRoot, proof) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0x12_34_56_78);
        expect.memRoot = state.memRoot;
        expect.registers[8] = 0x34_56_78_dd; // t0
        postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lwr_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x26, 0x9, 0x8, 0x4); // lwr $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0x12_34_56_78);
        state.registers[8] = 0xaa_bb_cc_dd; // t0
        state.registers[9] = t1;

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0xaa_bb_cc_12; // t0
        expect.registers[9] = t1;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        // test unaligned address
        insn = encodeitype(0x26, 0x9, 0x8, 0x5); // lwr $t0, 5($t1)
        (state.memRoot, proof) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0x12_34_56_78);
        expect.memRoot = state.memRoot;
        expect.registers[8] = 0xaa_bb_12_34; // t0
        postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sb_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x28, 0x9, 0x8, 0x4); // sb $t0, 4($t1)
        // note. cannon memory is zero-initalized. mem[t+4] = 0 is a no-op
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0);
        state.registers[8] = 0xaa_bb_cc_dd; // t0
        state.registers[9] = t1;

        IMIPS.State memory expect;
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xdd_00_00_00);
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[8];
        expect.registers[9] = state.registers[9];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sh_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x29, 0x9, 0x8, 0x4); // sh $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0);
        state.registers[8] = 0xaa_bb_cc_dd; // t0
        state.registers[9] = t1;

        IMIPS.State memory expect;
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xcc_dd_00_00);
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[8];
        expect.registers[9] = state.registers[9];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_swl_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x2a, 0x9, 0x8, 0x4); // swl $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0);
        state.registers[8] = 0xaa_bb_cc_dd; // t0
        state.registers[9] = t1;

        IMIPS.State memory expect;
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xaa_bb_cc_dd);
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[8];
        expect.registers[9] = state.registers[9];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sw_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x2b, 0x9, 0x8, 0x4); // sw $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0);
        state.registers[8] = 0xaa_bb_cc_dd; // t0
        state.registers[9] = t1;

        IMIPS.State memory expect;
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xaa_bb_cc_dd);
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[8];
        expect.registers[9] = state.registers[9];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_swr_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x2e, 0x9, 0x8, 0x5); // swr $t0, 5($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0);
        state.registers[8] = 0xaa_bb_cc_dd; // t0
        state.registers[9] = t1;

        IMIPS.State memory expect;
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xcc_dd_00_00);
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[8];
        expect.registers[9] = state.registers[9];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_ll_succeeds() external {
        uint32 t1 = 0x100;
        uint32 val = 0x12_23_45_67;
        uint32 insn = encodeitype(0x30, 0x9, 0x8, 0x4); // ll $t0, 4($t1)

        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, val);
        state.registers[8] = 0; // t0
        state.registers[9] = t1;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = val; // t0
        expect.registers[9] = t1;

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sc_succeeds() external {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x38, 0x9, 0x8, 0x4); // sc $t0, 4($t1)
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, t1 + 4, 0);
        state.registers[8] = 0xaa_bb_cc_dd; // t0
        state.registers[9] = t1;

        IMIPS.State memory expect;
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xaa_bb_cc_dd);
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0x1;
        expect.registers[9] = state.registers[9];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_movn_succeeds() external {
        // test mips mov instruction
        uint32 insn = encodespec(0x9, 0xa, 0x8, 0xb); // movn $t0, $t1, $t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0xa; // t0
        state.registers[9] = 0xb; // t1
        state.registers[10] = 0x1; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[9];
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        state.registers[10] = 0x0; // t2
        expect.registers[10] = 0x0; // t2
        expect.registers[8] = state.registers[8];
        postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_movz_succeeds() external {
        // test mips mov instruction
        uint32 insn = encodespec(0x9, 0xa, 0x8, 0xa); // movz $t0, $t1, $t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0xa; // t0
        state.registers[9] = 0xb; // t1
        state.registers[10] = 0x0; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[9];
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        state.registers[10] = 0x1; // t2
        expect.registers[10] = 0x1; // t2
        expect.registers[8] = state.registers[8];
        postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mflo_succeeds() external {
        uint32 insn = encodespec(0x0, 0x0, 0x8, 0x12); // mflo $t0
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.lo = 0xdeadbeef;

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.lo = state.lo;
        expect.registers[8] = state.lo;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mfhi_succeeds() external {
        uint32 insn = encodespec(0x0, 0x0, 0x8, 0x10); // mfhi $t0
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.hi = 0xdeadbeef;

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.hi = state.hi;
        expect.registers[8] = state.hi;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mthi_succeeds() external {
        uint32 insn = encodespec(0x8, 0x0, 0x0, 0x11); // mthi $t0
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0xdeadbeef; // t0

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.hi = state.registers[8];
        expect.registers[8] = state.registers[8];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mtlo_succeeds() external {
        uint32 insn = encodespec(0x8, 0x0, 0x0, 0x13); // mtlo $t0
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0xdeadbeef; // t0

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.lo = state.registers[8];
        expect.registers[8] = state.registers[8];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mul_succeeds() external {
        uint32 insn = encodespec2(0x9, 0xa, 0x8, 0x2); // mul t0, t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 5; // t1
        state.registers[10] = 2; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[9] * state.registers[10]; // t0
        expect.registers[9] = 5;
        expect.registers[10] = 2;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mult_succeeds() external {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x18); // mult t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0x0F_FF_00_00; // t1
        state.registers[10] = 100; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];
        expect.lo = 0x3F_9C_00_00;
        expect.hi = 0x6;

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_multu_succeeds() external {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x19); // multu t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0x0F_FF_00_00; // t1
        state.registers[10] = 100; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];
        expect.lo = 0x3F_9C_00_00;
        expect.hi = 0x6;

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_div_succeeds() external {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x1a); // div t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 5; // t1
        state.registers[10] = 2; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];
        expect.lo = 2;
        expect.hi = 1;

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_divu_succeeds() external {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x1b); // divu t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 5; // t1
        state.registers[10] = 2; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];
        expect.lo = 2;
        expect.hi = 1;

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_div_byZero_fails() external {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x1a); // div t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 5; // t1
        state.registers[10] = 0; // t2

        vm.expectRevert("MIPS: division by zero");
        mips.step(encodeState(state), proof, 0);
    }

    function test_divu_byZero_fails() external {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x1b); // divu t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 5; // t1
        state.registers[10] = 0; // t2

        vm.expectRevert("MIPS: division by zero");
        mips.step(encodeState(state), proof, 0);
    }

    function test_beq_succeeds() external {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x4, 0x9, 0x8, boff); // beq $t0, $t1, 16
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0xdeadbeef; // t0
        state.registers[9] = 0xdeadbeef; // t1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + (uint32(boff) << 2);
        expect.step = state.step + 1;
        expect.registers[8] = 0xdeadbeef;
        expect.registers[9] = 0xdeadbeef;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        // branch not taken
        state.registers[8] = 0xaa;
        expect.registers[8] = 0xaa;
        expect.nextPC = state.nextPC + 4;
        postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_bne_succeeds() external {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x5, 0x9, 0x8, boff); // bne $t0, $t1, 16
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0xdeadbeef; // t0
        state.registers[9] = 0xaa; // t1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + (uint32(boff) << 2);
        expect.step = state.step + 1;
        expect.registers[8] = 0xdeadbeef;
        expect.registers[9] = 0xaa;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_blez_succeeds() external {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x6, 0x8, 0x0, boff); // blez $t0, 16
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0; // t0

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + (uint32(boff) << 2);
        expect.step = state.step + 1;
        expect.registers[8] = 0;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_bgtz_succeeds() external {
        uint16 boff = 0xa0;
        uint32 insn = encodeitype(0x7, 0x8, 0x0, boff); // bgtz $t0, 0xa0
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 1; // t0

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + (uint32(boff) << 2);
        expect.step = state.step + 1;
        expect.registers[8] = 1;

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_bltz_succeeds() external {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x1, 0x8, 0x0, boff); // bltz $t0, 16
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0xF0_00_00_00; // t0

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + (uint32(boff) << 2);
        expect.step = state.step + 1;
        expect.registers[8] = 0xF0_00_00_00;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_bgez_succeeds() external {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x1, 0x8, 0x1, boff); // bgez $t0, 16
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0x00_00_00_01; // t0

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + (uint32(boff) << 2);
        expect.step = state.step + 1;
        expect.registers[8] = 0x00_00_00_01;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jump_succeeds() external {
        uint32 label = 0x02_00_00_02; // set the 26th bit to assert no sign extension
        uint32 insn = uint32(0x08_00_00_00) | label; // j label
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = label << 2;
        expect.step = state.step + 1;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jump_nonzeroRegion_succeeds() external {
        uint32 pcRegion1 = 0x10000000;
        uint32 label = 0x2;
        uint32 insn = uint32(0x08_00_00_00) | label; // j label
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(pcRegion1, insn, 0x4, 0);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = (state.nextPC & 0xF0_00_00_00) | (uint32(label) << 2);
        expect.step = state.step + 1;

        bytes memory witness = encodeState(state);
        bytes32 postState = mips.step(witness, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jal_succeeds() external {
        uint32 label = 0x02_00_00_02; // set the 26th bit to assert no sign extension
        uint32 insn = uint32(0x0c_00_00_00) | label; // jal label
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = label << 2;
        expect.step = state.step + 1;
        expect.registers[31] = state.pc + 8; // ra

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jal_nonzeroRegion_succeeds() external {
        uint32 pcRegion1 = 0x10000000;
        uint32 label = 0x2;
        uint32 insn = uint32(0x0c_00_00_00) | label; // jal label
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(pcRegion1, insn, 0x4, 0);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = (state.nextPC & 0xF0_00_00_00) | (uint32(label) << 2);
        expect.step = state.step + 1;
        expect.registers[31] = state.pc + 8; // ra

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jr_succeeds() external {
        uint16 tgt = 0x34;
        uint32 insn = encodespec(0x8, 0, 0, 0x8); // jr t0
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = tgt;

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = tgt;
        expect.step = state.step + 1;
        expect.registers[8] = tgt;

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jalr_succeeds() external {
        uint16 tgt = 0x34;
        uint32 insn = encodespec(0x8, 0, 0x9, 0x9); // jalr t1, t0
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = tgt; // t0

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = tgt;
        expect.step = state.step + 1;
        expect.registers[8] = tgt;
        expect.registers[9] = state.pc + 8; // t1

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sll_succeeds() external {
        uint8 shiftamt = 4;
        uint32 insn = encodespec(0x0, 0x9, 0x8, uint16(shiftamt) << 6); // sll t0, t1, 3
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0x20; // t1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[9] << shiftamt;
        expect.registers[9] = state.registers[9];

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_srl_succeeds() external {
        uint8 shiftamt = 4;
        uint32 insn = encodespec(0x0, 0x9, 0x8, uint16(shiftamt) << 6 | 2); // srl t0, t1, 3
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0x20; // t1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[9] >> shiftamt;
        expect.registers[9] = state.registers[9];

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sra_succeeds() external {
        uint8 shiftamt = 4;
        uint32 insn = encodespec(0x0, 0x9, 0x8, uint16(shiftamt) << 6 | 3); // sra t0, t1, 3
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0x80_00_00_20; // t1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0xF8_00_00_02; // 4 shifts while preserving sign bit
        expect.registers[9] = state.registers[9];

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sllv_succeeds() external {
        uint32 insn = encodespec(0xa, 0x9, 0x8, 4); // sllv t0, t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0x20; // t1
        state.registers[10] = 4; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[9] << state.registers[10]; // t0
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_srlv_succeeds() external {
        uint32 insn = encodespec(0xa, 0x9, 0x8, 6); // srlv t0, t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0x20_00; // t1
        state.registers[10] = 4; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = state.registers[9] >> state.registers[10]; // t0
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_srav_succeeds() external {
        uint32 insn = encodespec(0xa, 0x9, 0x8, 7); // srav t0, t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0xdeafbeef; // t1
        state.registers[10] = 12; // t2

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0xfffdeafb; // t0
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @notice Tests that the SRAV instruction succeeds when it includes extra bits in the shift
    ///         amount beyond the lower 5 bits that are actually used for the shift. Extra bits
    ///         need to be ignored but the instruction should still succeed.
    /// @param _rs Value to set in the shift register $rs.
    function testFuzz_srav_withExtraBits_succeeds(uint32 _rs) external {
        // Assume
        // Force _rs to have more than 5 bits set.
        _rs = uint32(bound(uint256(_rs), 0x20, type(uint32).max));

        uint32 insn = encodespec(0xa, 0x9, 0x8, 7); // srav t0, t1, t2
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0xdeadbeef; // t1
        state.registers[10] = _rs; // t2

        // Calculate shamt
        uint32 shamt = state.registers[10] & 0x1F;

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = MIPSInstructions.signExtend(state.registers[9] >> shamt, 32 - shamt); // t0
        expect.registers[9] = state.registers[9];
        expect.registers[10] = state.registers[10];

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lui_succeeds() external {
        uint32 insn = encodeitype(0xf, 0x0, 0x8, 0x4); // lui $t0, 0x04
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 0x00_04_00_00; // t0

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_clo_succeeds() external {
        uint32 insn = encodespec2(0x9, 0x0, 0x8, 0x21); // clo t0, t1
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0xFF_00_00_00; // t1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 8; // t0
        expect.registers[9] = state.registers[9];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_clz_succeeds() external {
        uint32 insn = encodespec2(0x9, 0x0, 0x8, 0x20); // clz t0, t1
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[9] = 0x00_00_F0_00; // t1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[8] = 16; // t0
        expect.registers[9] = state.registers[9];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_preimage_read_succeeds() external {
        uint32 pc = 0x0;
        uint32 insn = 0x0000000c; // syscall
        uint32 a1 = 0x4;
        uint32 a1_val = 0x0000abba;
        (bytes32 memRoot, bytes memory proof) = ffi.getCannonMemoryProof(pc, insn, a1, a1_val);

        uint32[32] memory registers;
        registers[2] = 4003; // read syscall
        registers[4] = 5; // fd
        registers[5] = a1; // addr
        registers[6] = 4; // count

        IMIPS.State memory state = IMIPS.State({
            memRoot: memRoot,
            preimageKey: bytes32(uint256(1) << 248 | 0x01),
            preimageOffset: 8, // start reading past the pre-image length prefix
            pc: pc,
            nextPC: pc + 4,
            lo: 0,
            hi: 0,
            heap: 0,
            exitCode: 0,
            exited: false,
            step: 1,
            registers: registers
        });
        bytes memory encodedState = encodeState(state);

        // prime the pre-image oracle
        bytes32 word = bytes32(uint256(0xdeadbeef) << 224);
        uint8 size = 4;
        uint8 partOffset = 8;
        oracle.loadLocalData(uint256(state.preimageKey), 0, word, size, partOffset);

        IMIPS.State memory expect = state;
        expect.preimageOffset += 4;
        expect.pc = state.nextPC;
        expect.nextPC += 4;
        expect.step += 1;
        expect.registers[2] = 4; // return
        expect.registers[7] = 0; // errno
        // recompute merkle root of written pre-image
        (expect.memRoot,) = ffi.getCannonMemoryProof(pc, insn, a1, 0xdeadbeef);

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_preimage_write_succeeds() external {
        uint32 pc = 0x0;
        uint32 insn = 0x0000000c; // syscall
        uint32 a1 = 0x4;
        uint32 a1_val = 0x0000abba;
        (bytes32 memRoot, bytes memory proof) = ffi.getCannonMemoryProof(pc, insn, a1, a1_val);

        uint32[32] memory registers;
        registers[2] = 4004; // write syscall
        registers[4] = 6; // fd
        registers[5] = a1; // addr
        registers[6] = 4; // count

        IMIPS.State memory state = IMIPS.State({
            memRoot: memRoot,
            preimageKey: bytes32(0),
            preimageOffset: 1,
            pc: pc,
            nextPC: 4,
            lo: 0,
            hi: 0,
            heap: 0,
            exitCode: 0,
            exited: false,
            step: 1,
            registers: registers
        });
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect = state;
        expect.preimageOffset = 0; // preimage write resets offset
        expect.pc = state.nextPC;
        expect.nextPC += 4;
        expect.step += 1;
        expect.preimageKey = bytes32(uint256(0xabba));
        expect.registers[2] = 4; // return
        expect.registers[7] = 0; // errno

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mmap_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (bytes32 memRoot, bytes memory proof) = ffi.getCannonMemoryProof(0, insn);

        IMIPS.State memory state;
        state.memRoot = memRoot;
        state.nextPC = 4;
        state.registers[2] = 4090; // mmap syscall
        state.registers[4] = 0x0; // a0
        state.registers[5] = 4095; // a1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        // assert page allocation is aligned to 4k
        expect.step = state.step + 1;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.heap = state.heap + 4096;
        expect.registers[2] = 0; // return old heap
        expect.registers[4] = 0x0; // a0
        expect.registers[5] = 4095; // a1

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mmap_justWithinMemLimit_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (bytes32 memRoot, bytes memory proof) = ffi.getCannonMemoryProof(0, insn);

        IMIPS.State memory state;
        state.memRoot = memRoot;
        state.nextPC = 4;
        state.heap = sys.HEAP_END - 4096; // Set up to increase heap to its limit
        state.registers[2] = 4090; // mmap syscall
        state.registers[4] = 0x0; // a0
        state.registers[5] = 4095; // a1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        // assert page allocation is aligned to 4k
        expect.step = state.step + 1;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.heap = sys.HEAP_END;
        expect.registers[2] = state.heap; // Return the old heap value
        expect.registers[7] = 0; // No error
        expect.registers[4] = state.registers[4]; // a0
        expect.registers[5] = state.registers[5]; // a1

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_step_mmap_fails() external {
        uint32 insn = 0x0000000c; // syscall
        (bytes32 memRoot, bytes memory proof) = ffi.getCannonMemoryProof(0, insn);

        IMIPS.State memory state;
        state.memRoot = memRoot;
        state.nextPC = 4;
        state.heap = sys.HEAP_END - 4096; // Set up to increase heap beyond its limit
        state.registers[2] = 4090; // mmap syscall
        state.registers[4] = 0x0; // a0
        state.registers[5] = 4097; // a1
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        // assert page allocation is aligned to 4k
        expect.step = state.step + 1;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.heap = state.heap;
        expect.registers[2] = sys.SYS_ERROR_SIGNAL; // signal an stdError
        expect.registers[7] = sys.EINVAL; // Return error value
        expect.registers[4] = state.registers[4]; // a0
        expect.registers[5] = state.registers[5]; // a1

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_brk_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[2] = 4045; // brk syscall
        state.registers[4] = 0xdead;
        bytes memory encodedState = encodeState(state);

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.step = state.step + 1;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.registers[2] = 0x40000000;
        expect.registers[4] = state.registers[4]; // registers unchanged

        bytes32 postState = mips.step(encodedState, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_clone_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[2] = 4120; // clone syscall

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.step = state.step + 1;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.registers[2] = 1;

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_exit_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[2] = 4246; // exit_group syscall
        state.registers[4] = 0x5; // a0

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.pc;
        expect.nextPC = state.nextPC;
        expect.step = state.step + 1;
        expect.registers[2] = state.registers[2]; // unchanged
        expect.registers[4] = state.registers[4]; // unchanged
        expect.exited = true;
        expect.exitCode = uint8(state.registers[4]);

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_fcntl_getfl_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[2] = 4055; // fcntl syscall
        state.registers[4] = 0x0; // a0
        state.registers[5] = 0x3; // a1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[2] = 0;
        expect.registers[5] = state.registers[5];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        // assert O_WRONLY
        state.registers[4] = 0x1; // a0
        expect.registers[4] = state.registers[4];
        expect.registers[2] = 1;
        postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_fcntl_getfd_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[2] = 4055; // fcntl syscall
        state.registers[4] = 0x0; // a0
        state.registers[5] = 0x1; // a1

        IMIPS.State memory expect;
        expect.memRoot = state.memRoot;
        expect.pc = state.nextPC;
        expect.nextPC = state.nextPC + 4;
        expect.step = state.step + 1;
        expect.registers[2] = 0;
        expect.registers[5] = state.registers[5];

        bytes32 postState = mips.step(encodeState(state), proof, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_prestate_exited_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.exited = true;

        bytes memory enc = encodeState(state);
        bytes32 postState = mips.step(enc, proof, 0);
        assertEq(postState, outputState(state), "unexpected post state");
    }

    function test_illegal_instruction_fails() external {
        uint32 illegal_insn = 0xFF_FF_FF_FF;
        // the illegal instruction is partially decoded as containing a memory operand
        // so we stuff random data to the expected address
        uint32 addr = 0xFF_FF_FF_FC; // 4-byte aligned ff..ff
        (bytes32 memRoot, bytes memory proof) = ffi.getCannonMemoryProof(0, illegal_insn, addr, 0);

        IMIPS.State memory state;
        state.memRoot = memRoot;
        bytes memory encodedState = encodeState(state);
        vm.expectRevert("invalid instruction");
        mips.step(encodedState, proof, 0);
    }

    function test_step_invalidRoot_fails() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[2] = 4246; // exit_group syscall
        state.registers[4] = 0x5; // a0

        // invalidate proof
        for (uint256 i = 0; i < proof.length; i++) {
            proof[i] = 0x0;
        }
        vm.expectRevert(InvalidMemoryProof.selector);
        mips.step(encodeState(state), proof, 0);
    }

    function test_step_invalidRootDifferentLeaf_fails() external {
        uint32 insn = 0x0000000c; // syscall

        // Initialize the state, though for the proof, use valid proofs for the instruction
        // and the memory address, but for a different leaf that does not contain the
        // instruction @ pc nor the memory address being read.
        uint32 pc = 0;
        IMIPS.State memory state;
        bytes memory proof;
        (state.memRoot, proof) = ffi.getCannonMemoryProofWrongLeaf(pc, insn, 0x4, 0);
        state.pc = pc;
        state.nextPC = pc + 4;
        state.registers[2] = 4246; // exit_group syscall
        state.registers[4] = 0x5; // a0

        vm.expectRevert(InvalidMemoryProof.selector);
        mips.step(encodeState(state), proof, 0);
    }

    function test_jump_inDelaySlot_fails() external {
        uint16 label = 0x2;
        uint32 insn = uint32(0x08_00_00_00) | label; // j label
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.nextPC = 0xa;

        vm.expectRevert("jump in delay slot");
        mips.step(encodeState(state), proof, 0);
    }

    function test_branch_inDelaySlot_fails() external {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x4, 0x9, 0x8, boff); // beq $t0, $t1, 16
        (IMIPS.State memory state, bytes memory proof) = constructMIPSState(0, insn, 0x4, 0);
        state.registers[8] = 0xdeadbeef; // t0
        state.registers[9] = 0xdeadbeef; // t1
        state.nextPC = 0xa;

        vm.expectRevert("branch in delay slot");
        mips.step(encodeState(state), proof, 0);
    }

    function encodeState(IMIPS.State memory state) internal pure returns (bytes memory) {
        bytes memory registers;
        for (uint256 i = 0; i < state.registers.length; i++) {
            registers = bytes.concat(registers, abi.encodePacked(state.registers[i]));
        }
        return abi.encodePacked(
            state.memRoot,
            state.preimageKey,
            state.preimageOffset,
            state.pc,
            state.nextPC,
            state.lo,
            state.hi,
            state.heap,
            state.exitCode,
            state.exited,
            state.step,
            registers
        );
    }

    /// @dev MIPS VM status codes:
    ///      0. Exited with success (Valid)
    ///      1. Exited with success (Invalid)
    ///      2. Exited with failure (Panic)
    ///      3. Unfinished
    function vmStatus(IMIPS.State memory state) internal pure returns (VMStatus out_) {
        if (!state.exited) {
            return VMStatuses.UNFINISHED;
        } else if (state.exitCode == 0) {
            return VMStatuses.VALID;
        } else if (state.exitCode == 1) {
            return VMStatuses.INVALID;
        } else {
            return VMStatuses.PANIC;
        }
    }

    function outputState(IMIPS.State memory state) internal pure returns (bytes32 out_) {
        bytes memory enc = encodeState(state);
        VMStatus status = vmStatus(state);
        assembly {
            out_ := keccak256(add(enc, 0x20), 226)
            out_ := or(and(not(shl(248, 0xFF)), out_), shl(248, status))
        }
    }

    function constructMIPSState(
        uint32 pc,
        uint32 insn,
        uint32 addr,
        uint32 val
    )
        internal
        returns (IMIPS.State memory state_, bytes memory proof_)
    {
        (state_.memRoot, proof_) = ffi.getCannonMemoryProof(pc, insn, addr, val);
        state_.pc = pc;
        state_.nextPC = pc + 4;
    }

    function encodeitype(uint8 opcode, uint8 rs, uint8 rt, uint16 imm) internal pure returns (uint32 insn_) {
        insn_ = uint32(opcode) << 26 | uint32(rs) << 21 | uint32(rt) << 16 | imm;
    }

    function encodespec(uint8 rs, uint8 rt, uint8 rd, uint16 funct) internal pure returns (uint32 insn_) {
        insn_ = uint32(rs) << 21 | uint32(rt) << 16 | uint32(rd) << 11 | uint32(funct);
    }

    function encodespec2(uint8 rs, uint8 rt, uint8 rd, uint8 funct) internal pure returns (uint32 insn_) {
        insn_ = uint32(28) << 26 | uint32(rs) << 21 | uint32(rt) << 16 | uint32(rd) << 11 | uint32(funct);
    }
}
