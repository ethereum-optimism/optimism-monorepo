// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { MIPSSyscalls as sys } from "src/cannon/libraries/MIPSSyscalls.sol";
import { MIPSInstructions as ins } from "src/cannon/libraries/MIPSInstructions.sol";
import { InvalidExitedValue, InvalidMemoryProof, InvalidSecondMemoryProof } from "src/cannon/libraries/CannonErrors.sol";
import "src/dispute/lib/Types.sol";

// Interfaces
import { IMIPS2 } from "src/cannon/interfaces/IMIPS2.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";

contract ThreadStack {
    bytes32 internal constant EMPTY_THREAD_ROOT = hex"ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5";

    struct Entry {
        IMIPS2.ThreadState thread;
        bytes32 root;
    }

    Entry[] internal stack;

    constructor() {
        stack.push();
        stack[stack.length - 1].root = EMPTY_THREAD_ROOT;
    }

    function root() public view returns (bytes32) {
        return stack[stack.length - 1].root;
    }

    function inner(uint256 _i) public view returns (bytes32 root_) {
        root_ = stack[stack.length - 1 - _i].root;
    }

    function top() public view returns (IMIPS2.ThreadState memory thread_) {
        thread_ = peek(0);
    }

    function peek(uint256 _i) public view returns (IMIPS2.ThreadState memory thread_) {
        thread_ = stack[stack.length - 1 - _i].thread;
    }

    function push(IMIPS2.ThreadState memory _thread) public {
        _push(_thread);
    }

    function pop() public {
        stack.pop();
    }

    function replace(IMIPS2.ThreadState memory _thread) public {
        stack.pop();
        _push(_thread);
    }

    function _push(IMIPS2.ThreadState memory _thread) internal {
        bytes32 newRoot = keccak256(abi.encodePacked(stack[stack.length - 1].root, keccak256(encodeThread(_thread))));
        stack.push(Entry(_thread, newRoot));
    }
}

contract Threading {
    ThreadStack public left;
    ThreadStack public right;
    bool public traverseRight;
    uint32 public nextThreadID;

    constructor() {
        left = new ThreadStack();
        right = new ThreadStack();
        traverseRight = false;
    }

    function createThread() public returns (IMIPS2.ThreadState memory thread_) {
        thread_.threadID = nextThreadID;
        if (traverseRight) {
            right.push(thread_);
        } else {
            left.push(thread_);
        }
        nextThreadID += 1;
    }

    function current() public view returns (IMIPS2.ThreadState memory out_) {
        if (traverseRight) {
            out_ = right.top();
        } else {
            out_ = left.top();
        }
    }

    function replaceCurrent(IMIPS2.ThreadState memory _thread) public {
        if (traverseRight) {
            right.replace(_thread);
        } else {
            left.replace(_thread);
        }
    }

    function witness() public view returns (bytes memory out_) {
        if (traverseRight) {
            out_ = abi.encodePacked(encodeThread(right.top()), right.inner(1));
        } else {
            out_ = abi.encodePacked(encodeThread(left.top()), left.inner(1));
        }
    }

    function setTraverseRight(bool _traverseRight) public {
        traverseRight = _traverseRight;
    }
}

contract MIPS2_Test is CommonTest {
    IMIPS2 internal mips;
    IPreimageOracle internal oracle;
    Threading internal threading;

    // keccak256(bytes32(0) ++ bytes32(0))
    bytes32 internal constant EMPTY_THREAD_ROOT = hex"ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5";

    uint32 internal constant A0_REG = 4;
    uint32 internal constant A1_REG = 5;
    uint32 internal constant A2_REG = 6;
    uint32 internal constant A3_REG = 7;
    uint32 internal constant SP_REG = 29;

    function setUp() public virtual override {
        super.setUp();
        oracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IPreimageOracle.__constructor__, (0, 0)))
            })
        );
        mips = IMIPS2(
            DeployUtils.create1({
                _name: "MIPS2",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IMIPS2.__constructor__, (IPreimageOracle(address(oracle))))
                )
            })
        );
        threading = new Threading();
        vm.store(address(mips), 0x0, bytes32(abi.encode(address(oracle))));
        vm.label(address(oracle), "PreimageOracle");
        vm.label(address(mips), "MIPS2");
        vm.label(address(threading), "Threading");
    }

    /// @notice Used to debug step() behavior given a specific input.
    /// This is useful to more easily debug non-forge tests.
    /// For example, in cannon/mipsevm/evm_test.go step input can be pulled here:
    /// https://github.com/ethereum-optimism/optimism/blob/1f64dd6db5561f3bb76ed1d1ffdaff0cde9b7c4b/cannon/mipsevm/evm_test.go#L80-L80
    function test_step_debug_succeeds() external {
        bytes memory input =
            hex"e14ced3200000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000140000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000acab5a39c6f974b22302e96dcdef1815483eaf580639bb1ee7ac98267afac2bf1ac041d3ff12045b73c86e4ff95ff662a5eee82abdf44a2d0b75fb180daf48a79e3143a81fa7c3d90b000000000000000000000078fc2ffac2fd940100000000000080c8ffffffff006504aeffb6e08baf3f85da5476a9160fa8f9f188a722fdd29268b0cbaf596736ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb500000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007c6000000000000ffffffff000000000000000000000000f1f85ff4f1f85ff8506d442dbb3938f83eb60825a7ecbff2000010185e1a31f600050f0000000064a7c3d90be5acea102ad7bda149e0bfd0e7111c77d98b335645e665389becadf140ef999cc64fbd7f04799e85c97dadc5cca510bd5b3d97166d1aec28829f3dd43d8cf1f9358e4103b16d09d466e2c7c048ea3ba1aef3141e700270581aa0b75b50e34fc926bb2d83ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb500000000000000000000000000000000000000000000000c00000000000000000000000000000000000000000000000000000000000000000000000000000000ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5b4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d3021ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85e58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a193440eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968ffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f839867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756afcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0f9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5f8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf8923490c6ceeb450aecdc82e28293031d10c7d73bf85e57bf041a97360aa2c5d99cc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8beccda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d22733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981fe1d3b5c807b281e4683cc6d6315cf95b9ade8641defcb32372f1c126e398ef7a5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0b46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0c65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2f4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd95a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e3774df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652cdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618db8cd74046ff337f0a7bf2c8e03e10f642c1886798d71806ab1e888d9e5ee87d000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000";
        (bool success, bytes memory retVal) = address(mips).call(input);
        bytes memory expectedRetVal = hex"0335fe4205f8443eefa7ac4541197874224df35e8536158c2fc2d5c8c2d2adb4";

        assertTrue(success);
        assertEq(retVal.length, 32, "Expect a bytes32 hash of the post-state to be returned");
        assertEq(retVal, expectedRetVal);
    }

    function test_stepABI_succeeds() public {
        uint32[32] memory registers;
        registers[0] = 0xdeadbeef;
        registers[16] = 0xbfff0000;
        registers[31] = 0x0badf00d;
        IMIPS2.ThreadState memory thread = IMIPS2.ThreadState({
            threadID: 0,
            exitCode: 0,
            exited: false,
            futexAddr: sys.FUTEX_EMPTY_ADDR,
            futexVal: 0,
            futexTimeoutStep: 0,
            pc: 4,
            nextPC: 8,
            lo: 0,
            hi: 0,
            registers: registers
        });
        bytes memory encodedThread = encodeThread(thread);
        bytes memory threadWitness = abi.encodePacked(encodedThread, EMPTY_THREAD_ROOT);
        bytes32 threadRoot = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodedThread)));

        IMIPS2.State memory state = IMIPS2.State({
            memRoot: hex"30be14bdf94d7a93989a6263f1e116943dc052d584730cae844bf330dfddce2f",
            preimageKey: bytes32(0),
            preimageOffset: 0,
            heap: 0,
            llReservationStatus: 0,
            llAddress: 0,
            llOwnerThread: 0,
            exitCode: 0,
            exited: false,
            step: 1,
            stepsSinceLastContextSwitch: 1,
            wakeup: sys.FUTEX_EMPTY_ADDR,
            traverseRight: false,
            leftThreadStack: threadRoot,
            rightThreadStack: EMPTY_THREAD_ROOT,
            nextThreadID: 1
        });
        bytes memory memProof =
            hex"3c10bfff3610fff0341100013c08ffff3508fffd34090003010950202d420001ae020008ae11000403e000080000000000000000000000000000000000000000ad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5b4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d3021ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85e58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a193440eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968ffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f839867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756afcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0f9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5f8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf8923490c6ceeb450aecdc82e28293031d10c7d73bf85e57bf041a97360aa2c5d99cc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8beccda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d22733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981fe1d3b5c807b281e4683cc6d6315cf95b9ade8641defcb32372f1c126e398ef7a5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0b46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0c65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2f4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd95a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e3774df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652cdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618db8cd74046ff337f0a7bf2c8e03e10f642c1886798d71806ab1e888d9e5ee87d00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000";
        bytes32 post = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertNotEq(post, bytes32(0));
    }

    /// @notice Tests that the mips step function fails when the value of the exited field is
    ///         invalid (anything greater than 1).
    function test_step_invalidExitedValueInState_fails() external {
        // Bound to invalid exited values.
        for (uint8 exited = 2; exited <= type(uint8).max && exited != 0;) {
            // Setup state
            uint32 insn = encodespec(17, 18, 8, 0x20); // Arbitrary instruction: add t0, s1, s2
            (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
                constructMIPSState(0, insn, 0x4, 0);

            // Set up step data
            bytes memory encodedThread = encodeThread(thread);
            bytes memory threadWitness = abi.encodePacked(encodedThread, EMPTY_THREAD_ROOT);
            bytes memory proofData = bytes.concat(threadWitness, memProof);
            bytes memory stateData = encodeState(state);
            assembly {
                // Manipulate state data
                // Push offset by an additional 32 bytes (0x20) to account for length prefix
                mstore8(add(add(stateData, 0x20), 82), exited)
            }

            // Call the step function and expect a revert.
            vm.expectRevert(InvalidExitedValue.selector);
            mips.step(stateData, proofData, 0);
            unchecked {
                exited++;
            }
        }
    }

    function test_step_invalidThreadWitness_reverts() public {
        IMIPS2.State memory state;
        IMIPS2.ThreadState memory thread;
        bytes memory memProof;
        bytes memory witness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        vm.expectRevert("invalid thread witness");
        mips.step(encodeState(state), bytes.concat(witness, memProof), 0);
    }

    function test_syscallNanosleep_succeeds() public {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[2] = sys.SYS_NANOSLEEP;
        thread.registers[7] = 0xdead; // should be reset to a zero errno
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 0x0;
        expectThread.registers[7] = 0x0;
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = 0;
        expect.leftThreadStack = EMPTY_THREAD_ROOT;
        expect.rightThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));
        expect.traverseRight = true;

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_syscallSchedYield_succeeds() public {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[2] = sys.SYS_SCHED_YIELD;
        thread.registers[7] = 0xdead; // should be reset to a zero errno
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 0x0;
        expectThread.registers[7] = 0x0;
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = 0;
        expect.leftThreadStack = EMPTY_THREAD_ROOT;
        expect.rightThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));
        expect.traverseRight = true;

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_syscallGetTID_succeeds() public {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.threadID = 0xbeef;
        thread.registers[2] = sys.SYS_GETTID;
        thread.registers[7] = 0xdead;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 0xbeef;
        expectThread.registers[7] = 0x0; // errno
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        expect.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_syscallClone_succeeds() public {
        uint32 insn = 0x0000000c; // syscall
        uint32 sp = 0xdead;

        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[2] = sys.SYS_CLONE;
        thread.registers[A0_REG] = sys.VALID_SYS_CLONE_FLAGS;
        thread.registers[A1_REG] = sp;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = state.nextThreadID;
        expectThread.registers[7] = 0;

        IMIPS2.ThreadState memory newThread = copyThread(thread);
        newThread.threadID = 1;
        newThread.futexAddr = sys.FUTEX_EMPTY_ADDR;
        newThread.futexVal = 0;
        newThread.futexTimeoutStep = 0;
        newThread.pc = thread.nextPC;
        newThread.nextPC = thread.nextPC + 4;
        newThread.registers[2] = 0;
        newThread.registers[7] = 0;
        newThread.registers[SP_REG] = sp;

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.nextThreadID = 2;
        expect.stepsSinceLastContextSwitch = 0;
        bytes32 innerThreadRoot = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));
        expect.leftThreadStack = keccak256(abi.encodePacked(innerThreadRoot, keccak256(encodeThread(newThread))));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting the VM exits successfully for a clone syscall with invalid clone flags.
    function test_syscallClone_invalidCloneFlags_succeeds() public {
        uint32 insn = 0x0000000c; // syscall
        uint32 sp = 0xdead;

        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[2] = sys.SYS_CLONE;
        thread.registers[A0_REG] = 0xdead; // invalid flag
        thread.registers[A1_REG] = sp;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = state.nextThreadID;
        expectThread.registers[7] = 0;

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.step + 1;
        expect.exited = true;
        expect.exitCode = VMStatuses.PANIC.raw();

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting successful futex wait syscall behavior with a timeout argument
    function test_syscallFutexWaitTimeout_succeeds() public {
        uint32 futexAddr = 0x1000;
        uint32 futexVal = 0xAA_AA_AA_AA;
        uint32 timeout = 1;

        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, futexAddr, futexVal);
        thread.registers[2] = sys.SYS_FUTEX;
        thread.registers[A0_REG] = futexAddr;
        thread.registers[A1_REG] = sys.FUTEX_WAIT_PRIVATE;
        thread.registers[A2_REG] = futexVal;
        thread.registers[A3_REG] = timeout;
        threading.createThread();
        threading.replaceCurrent(thread);
        bytes memory threadWitness = threading.witness();
        finalizeThreadingState(threading, state);

        // FUTEX_WAIT
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.futexAddr = futexAddr;
        expectThread.futexVal = futexVal;
        expectThread.futexTimeoutStep = state.step + 1 + sys.FUTEX_TIMEOUT_STEPS;
        threading.replaceCurrent(expectThread);

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting successful futex wait syscall behavior with a zero timeout argument
    function test_syscallFutexWaitNoTimeout_succeeds() public {
        uint32 futexAddr = 0x1000;
        uint32 futexVal = 0xAA_AA_AA_AA;
        uint32 timeout = 0;

        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, futexAddr, futexVal);
        thread.registers[2] = sys.SYS_FUTEX;
        thread.registers[A0_REG] = futexAddr;
        thread.registers[A1_REG] = sys.FUTEX_WAIT_PRIVATE;
        thread.registers[A2_REG] = futexVal;
        thread.registers[A3_REG] = timeout;
        threading.createThread();
        threading.replaceCurrent(thread);
        bytes memory threadWitness = threading.witness();
        finalizeThreadingState(threading, state);

        // FUTEX_WAIT
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.futexAddr = futexAddr;
        expectThread.futexVal = futexVal;
        expectThread.futexTimeoutStep = sys.FUTEX_NO_TIMEOUT;
        threading.replaceCurrent(expectThread);

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting unsuccessful spurious futex wait syscall
    function test_syscallFutexWaitErrno_succeeds() public {
        uint32 futexAddr = 0x1000;
        uint32 futexVal = 0xAA_AA_AA_AA;

        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, futexAddr, futexVal);
        thread.registers[2] = sys.SYS_FUTEX;
        thread.registers[A0_REG] = futexAddr;
        thread.registers[A1_REG] = sys.FUTEX_WAIT_PRIVATE;
        thread.registers[A2_REG] = 0xBB_BB_BB_BB;
        thread.registers[A3_REG] = 0; // timeout
        threading.createThread();
        threading.replaceCurrent(thread);
        bytes memory threadWitness = threading.witness();
        finalizeThreadingState(threading, state);

        // FUTEX_WAIT
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.futexAddr = sys.FUTEX_EMPTY_ADDR;
        expectThread.registers[2] = sys.SYS_ERROR_SIGNAL;
        expectThread.registers[7] = sys.EAGAIN; // errno
        threading.replaceCurrent(expectThread);

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_syscallFutexWake_succeeds() public {
        uint32 futexAddr = 0x1000;
        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, futexAddr, 0xAA_AA_AA_AA);
        thread.threadID = threading.nextThreadID();
        thread.registers[2] = sys.SYS_FUTEX;
        thread.registers[A0_REG] = futexAddr;
        thread.registers[A1_REG] = sys.FUTEX_WAKE_PRIVATE;
        thread.registers[A2_REG] = 1000; // ignored
        thread.registers[7] = 0xbeef; // non-zero value to check if it is cleared
        threading.createThread();
        threading.replaceCurrent(thread);
        bytes memory threadWitness = threading.witness();
        finalizeThreadingState(threading, state);

        // FUTEX_WAKE
        threading.left().pop();
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 0x0;
        expectThread.registers[7] = 0x0; // errno
        threading.right().push(expectThread);

        IMIPS2.State memory expect = copyState(state);
        expect.wakeup = futexAddr;
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = 0;
        expect.traverseRight = true;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting behavior of exit syscall when there are multiple threads present
    function test_syscallExit_multipleThreads_succeeds() public {
        uint32 insn = 0x0000000c; // syscall
        uint8 exitCode = 4;

        IMIPS2.ThreadState memory threadA = threading.createThread();
        threadA.futexAddr = sys.FUTEX_EMPTY_ADDR;
        threadA.pc = 0x1000;
        threadA.nextPC = 0x1004;
        threading.replaceCurrent(threadA);

        IMIPS2.ThreadState memory threadB = threading.createThread();
        threadB.futexAddr = sys.FUTEX_EMPTY_ADDR;
        threadB.pc = 0x100;
        threadB.nextPC = 0x104;
        threadB.registers[2] = sys.SYS_EXIT;
        threadB.registers[A0_REG] = exitCode;
        threading.replaceCurrent(threadB);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        bytes memory memProof;
        (state.memRoot, memProof) = ffi.getCannonMemoryProof(threadB.pc, insn, 0, 0);
        state.step = 20;
        state.stepsSinceLastContextSwitch = 10;
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        finalizeThreadingState(threading, state);

        // state updates
        IMIPS2.ThreadState memory expectThread = copyThread(threadB);
        expectThread.exited = true;
        expectThread.exitCode = exitCode;
        threading.replaceCurrent(expectThread);
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting behavior of exit syscall when is a single thread left
    function test_syscallExit_lastThread_succeeds() public {
        uint32 insn = 0x0000000c; // syscall
        uint8 exitCode = 4;

        IMIPS2.ThreadState memory thread = threading.createThread();
        thread.futexAddr = sys.FUTEX_EMPTY_ADDR;
        thread.pc = 0x1000;
        thread.nextPC = 0x1004;
        thread.registers[2] = sys.SYS_EXIT;
        thread.registers[A0_REG] = exitCode;
        threading.replaceCurrent(thread);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        bytes memory memProof;
        (state.memRoot, memProof) = ffi.getCannonMemoryProof(thread.pc, insn, 0, 0);
        state.step = 20;
        state.stepsSinceLastContextSwitch = 10;
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        finalizeThreadingState(threading, state);

        // state updates
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.exited = true;
        expectThread.exitCode = exitCode;
        threading.replaceCurrent(expectThread);
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        expect.exited = true;
        expect.exitCode = exitCode;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_syscallGetPid_succeeds() public {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[2] = sys.SYS_GETPID;
        thread.registers[7] = 0xdead;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 0x0;
        expectThread.registers[7] = 0x0;
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        expect.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev static unit test asserting that clock_gettime syscall for monotonic time succeeds
    function test_syscallClockGettimeMonotonic_succeeds() public {
        _test_syscallClockGettime_succeeds(sys.CLOCK_GETTIME_MONOTONIC_FLAG);
    }

    /// @dev static unit test asserting that clock_gettime syscall for real time succeeds
    function test_syscallClockGettimeRealtime_succeeds() public {
        _test_syscallClockGettime_succeeds(sys.CLOCK_GETTIME_REALTIME_FLAG);
    }

    function _test_syscallClockGettime_succeeds(uint32 clkid) internal {
        uint32 pc = 0;
        uint32 insn = 0x0000000c; // syscall
        uint32 timespecAddr = 0xb000;
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory insnAndMemProof) =
            constructMIPSState(pc, insn, timespecAddr, 0xbad);
        state.step = 100_000_004;
        thread.registers[2] = sys.SYS_CLOCKGETTIME;
        thread.registers[A0_REG] = clkid;
        thread.registers[A1_REG] = timespecAddr;
        thread.registers[7] = 0xdead;

        uint32 secs = 0;
        uint32 nsecs = 0;
        if (clkid == sys.CLOCK_GETTIME_MONOTONIC_FLAG) {
            secs = 10;
            nsecs = 500;
        }
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);
        (, bytes memory memProof2) = ffi.getCannonMemoryProof2(pc, insn, timespecAddr, secs, timespecAddr + 4);

        IMIPS2.State memory expect = copyState(state);
        (expect.memRoot,) = ffi.getCannonMemoryProof(pc, insn, timespecAddr, secs, timespecAddr + 4, nsecs);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 0x0;
        expectThread.registers[7] = 0x0;
        expect.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, insnAndMemProof, memProof2), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev static unit test asserting that clock_gettime syscall for monotonic time succeeds in writing to an
    /// unaligned address
    function test_syscallClockGettimeMonotonicUnaligned_succeeds() public {
        _test_syscallClockGettimeUnaligned_succeeds(sys.CLOCK_GETTIME_MONOTONIC_FLAG);
    }

    /// @dev static unit test asserting that clock_gettime syscall for real time succeeds in writing to an
    /// unaligned address
    function test_syscallClockGettimeRealtimeUnaligned_succeeds() public {
        _test_syscallClockGettimeUnaligned_succeeds(sys.CLOCK_GETTIME_REALTIME_FLAG);
    }

    function _test_syscallClockGettimeUnaligned_succeeds(uint32 clkid) internal {
        uint32 pc = 0;
        uint32 insn = 0x0000000c; // syscall
        uint32 timespecAddr = 0xb001;
        uint32 timespecAddrAligned = 0xb000;
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory insnAndMemProof) =
            constructMIPSState(pc, insn, timespecAddrAligned, 0xbad);
        state.step = 100_000_004;
        thread.registers[2] = sys.SYS_CLOCKGETTIME;
        thread.registers[A0_REG] = clkid;
        thread.registers[A1_REG] = timespecAddr;
        thread.registers[7] = 0xdead;

        uint32 secs = 0;
        uint32 nsecs = 0;
        if (clkid == sys.CLOCK_GETTIME_MONOTONIC_FLAG) {
            secs = 10;
            nsecs = 500;
        }
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);
        (, bytes memory memProof2) =
            ffi.getCannonMemoryProof2(pc, insn, timespecAddrAligned, secs, timespecAddrAligned + 4);

        IMIPS2.State memory expect = copyState(state);
        (expect.memRoot,) =
            ffi.getCannonMemoryProof(pc, insn, timespecAddrAligned, secs, timespecAddrAligned + 4, nsecs);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 0x0;
        expectThread.registers[7] = 0x0;
        expect.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, insnAndMemProof, memProof2), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Test asserting that an clock_gettime monotonic syscall reverts on an invalid memory proof
    function test_step_syscallClockGettimeMonotonicInvalidProof_reverts() public {
        _test_syscallClockGettimeInvalidProof_reverts(sys.CLOCK_GETTIME_MONOTONIC_FLAG);
    }

    /// @dev Test asserting that an clock_gettime realtime syscall reverts on an invalid memory proof
    function test_step_syscallClockGettimeRealtimeInvalidProof_reverts() public {
        _test_syscallClockGettimeInvalidProof_reverts(sys.CLOCK_GETTIME_REALTIME_FLAG);
    }

    function _test_syscallClockGettimeInvalidProof_reverts(uint32 clkid) internal {
        // NOTE: too slow to run this test under the forge fuzzer.
        for (uint256 proofIndex = 896 + (32 * 2); proofIndex < 896 * 2; proofIndex += 32) {
            // proofIndex points to a leaf in the index in the memory proof (in insnAndMemProof) that will be zeroed.
            // Note that the second leaf in the memory proof is already zeroed because it's the sibling of the first
            // memory write. So start from the third leaf.

            uint32 secs = 0;
            if (clkid == sys.CLOCK_GETTIME_MONOTONIC_FLAG) {
                secs = 10;
            }
            uint32 pc = 0;
            uint32 insn = 0x0000000c; // syscall
            uint32 timespecAddr = 0xb000;
            (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory insnAndMemProof) =
                constructMIPSState(pc, insn, timespecAddr, 0xbad);
            state.step = 100_000_004;
            thread.registers[2] = sys.SYS_CLOCKGETTIME;
            thread.registers[A0_REG] = sys.CLOCK_GETTIME_MONOTONIC_FLAG;
            thread.registers[A1_REG] = timespecAddr;
            thread.registers[7] = 0xdead;
            bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
            updateThreadStacks(state, thread);
            (, bytes memory memProof2) = ffi.getCannonMemoryProof2(pc, insn, timespecAddr, secs, timespecAddr + 4);

            bytes memory invalidInsnAndMemProof = new bytes(insnAndMemProof.length);
            for (uint256 i = 0; i < invalidInsnAndMemProof.length; i++) {
                // clear the 32-byte insn leaf
                if (i >= proofIndex && i < proofIndex + 32) {
                    invalidInsnAndMemProof[i] = 0x0;
                } else {
                    invalidInsnAndMemProof[i] = insnAndMemProof[i];
                }
            }
            vm.expectRevert(InvalidMemoryProof.selector);
            mips.step(encodeState(state), bytes.concat(threadWitness, invalidInsnAndMemProof, memProof2), 0);

            (, bytes memory invalidMemProof2) =
                ffi.getCannonMemoryProof2(pc, insn, timespecAddr, secs + 1, timespecAddr + 4);
            vm.expectRevert(InvalidSecondMemoryProof.selector);
            mips.step(encodeState(state), bytes.concat(threadWitness, insnAndMemProof, invalidMemProof2), 0);
        }
    }

    /// @dev static unit test asserting that clock_gettime syscall for non-realtime, non-monotonic time succeeds
    function test_syscallClockGettimeOtherFlags_succeeds() public {
        uint32 pc = 0;
        uint32 insn = 0x0000000c; // syscall
        uint32 timespecAddr = 0xb000;
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory insnAndMemProof) =
            constructMIPSState(pc, insn, timespecAddr, 0xbad);
        state.step = (sys.HZ * 10 + 5) - 1;
        thread.registers[2] = sys.SYS_CLOCKGETTIME;
        thread.registers[A0_REG] = sys.CLOCK_GETTIME_MONOTONIC_FLAG + 1;
        thread.registers[A1_REG] = timespecAddr;
        thread.registers[7] = 0xdead;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = sys.SYS_ERROR_SIGNAL;
        expectThread.registers[7] = sys.EINVAL;
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        expect.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, insnAndMemProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting that VM preempts threads after a certain number of steps
    function test_threadQuantumSchedule_succeeds() public {
        IMIPS2.ThreadState memory threadA = threading.createThread();
        threadA.threadID = 0;
        threadA.futexAddr = sys.FUTEX_EMPTY_ADDR;
        threading.replaceCurrent(threadA);
        IMIPS2.ThreadState memory threadB = threading.createThread();
        threadB.futexAddr = sys.FUTEX_EMPTY_ADDR;
        threading.replaceCurrent(threadB);
        IMIPS2.State memory state;
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        state.stepsSinceLastContextSwitch = sys.SCHED_QUANTUM;
        finalizeThreadingState(threading, state);
        bytes memory threadWitness = threading.witness();

        // Preempt the current thread after the quantum
        threading.left().pop();
        threading.right().push(threadB);

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = 0;
        finalizeThreadingState(threading, expect);

        bytes memory memProof; // unused
        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting thread left traversal without wakeups
    function test_threadTraverseLeft_succeeds() public {
        IMIPS2.State memory state;
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        state.step = 10;
        state.stepsSinceLastContextSwitch = 0;
        finalizeThreadingState(threading, state);

        uint32 pc = 0x4000;
        uint32 insn = 0x0000000c; // syscall
        bytes memory memProof;
        (state.memRoot, memProof) = ffi.getCannonMemoryProof(pc, insn);

        // Create a few threads
        for (uint256 i = 0; i < 3; i++) {
            IMIPS2.ThreadState memory thread = threading.createThread();
            thread.pc = pc;
            thread.nextPC = pc + 4;
            thread.futexAddr = sys.FUTEX_EMPTY_ADDR;
            thread.registers[2] = sys.SYS_NANOSLEEP;
            threading.replaceCurrent(thread);
        }
        finalizeThreadingState(threading, state);

        // Traverse left
        for (uint256 i = 0; i < 3; i++) {
            IMIPS2.ThreadState memory currentThread = threading.current();
            bytes memory threadWitness = threading.witness();

            // thread stack updates
            currentThread.pc = currentThread.nextPC;
            currentThread.nextPC = currentThread.nextPC + 4;
            currentThread.registers[2] = 0x0;
            currentThread.registers[7] = 0x0;
            threading.left().pop();
            threading.right().push(currentThread);

            IMIPS2.State memory expect = copyState(state);
            expect.step = state.step + 1;
            expect.stepsSinceLastContextSwitch = 0;
            finalizeThreadingState(threading, expect);
            expect.traverseRight = i == 2;

            bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
            assertEq(postState, outputState(expect), "unexpected post state");

            state = expect;
        }
    }

    /// @dev Static unit test asserting thread right traversal without wakeups
    function test_threadTraverseRight_succeeds() public {
        threading.setTraverseRight(true);

        IMIPS2.State memory state;
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        state.step = 10;
        state.stepsSinceLastContextSwitch = 0;
        state.traverseRight = true;
        finalizeThreadingState(threading, state);

        uint32 pc = 0x4000;
        uint32 insn = 0x0000000c; // syscall
        bytes memory memProof;
        (state.memRoot, memProof) = ffi.getCannonMemoryProof(pc, insn);

        // Create a few threads
        for (uint256 i = 0; i < 3; i++) {
            IMIPS2.ThreadState memory thread = threading.createThread();
            thread.pc = pc;
            thread.nextPC = pc + 4;
            thread.futexAddr = sys.FUTEX_EMPTY_ADDR;
            thread.registers[2] = sys.SYS_NANOSLEEP;
            threading.replaceCurrent(thread);
        }
        finalizeThreadingState(threading, state);

        for (uint256 i = 0; i < 3; i++) {
            IMIPS2.ThreadState memory currentThread = threading.current();
            bytes memory threadWitness = threading.witness();

            // thread stack updates
            currentThread.pc = currentThread.nextPC;
            currentThread.nextPC = currentThread.nextPC + 4;
            currentThread.registers[2] = 0x0;
            currentThread.registers[7] = 0x0;
            threading.right().pop();
            threading.left().push(currentThread);

            IMIPS2.State memory expect = copyState(state);
            expect.step = state.step + 1;
            expect.stepsSinceLastContextSwitch = 0;
            finalizeThreadingState(threading, expect);
            expect.traverseRight = i != 2;

            bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
            assertEq(postState, outputState(expect), "unexpected post state");

            state = expect;
        }
    }

    /// @dev static unit test asserting state transition of a spurious wakeup
    function test_wakeupPreemptsThread_succeeds() public {
        threading.createThread();
        threading.createThread();
        IMIPS2.ThreadState memory threadB = threading.current();
        threadB.futexAddr = 0xdead;
        threading.replaceCurrent(threadB);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        state.wakeup = 0xabba;
        finalizeThreadingState(threading, state);

        // Preempt the current thread on spurious wakeup
        threading.left().pop();
        threading.right().push(threadB);

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = 0;
        finalizeThreadingState(threading, expect);

        bytes memory memProof;
        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting successful wakeup traversal when no threads are ready to wake
    function test_threadWakeupFullTraversalNoWakeup_succeeds() public {
        IMIPS2.State memory state;
        state.wakeup = 0x1000;
        state.step = 10;
        state.stepsSinceLastContextSwitch = 10;
        finalizeThreadingState(threading, state);

        // Create a few threads that are not waiting to wake
        for (uint256 i = 0; i < 3; i++) {
            IMIPS2.ThreadState memory thread = threading.createThread();
            thread.futexAddr = sys.FUTEX_EMPTY_ADDR;
            threading.replaceCurrent(thread);
        }
        finalizeThreadingState(threading, state);

        // Traverse left
        for (uint256 i = 0; i < 3; i++) {
            IMIPS2.ThreadState memory currentThread = threading.current();

            bytes memory memProof;
            (state.memRoot, memProof) = ffi.getCannonMemoryProof(currentThread.pc, 0);
            bytes memory threadWitness = threading.witness();

            // We should preempt the current thread
            threading.left().pop();
            threading.right().push(currentThread);

            IMIPS2.State memory expect = copyState(state);
            expect.step = state.step + 1;
            expect.stepsSinceLastContextSwitch = 0;
            finalizeThreadingState(threading, expect);
            expect.traverseRight = i == 2;

            bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
            assertEq(postState, outputState(expect), "unexpected post state");

            state = expect;
        }

        // Traverse right
        threading.setTraverseRight(true);
        for (uint256 i = 0; i < 3; i++) {
            IMIPS2.ThreadState memory currentThread = threading.current();

            bytes memory memProof;
            (state.memRoot, memProof) = ffi.getCannonMemoryProof(currentThread.pc, 0);
            bytes memory threadWitness = threading.witness();

            // We should preempt the current thread
            threading.right().pop();
            threading.left().push(currentThread);

            IMIPS2.State memory expect = copyState(state);
            expect.step = state.step + 1;
            expect.stepsSinceLastContextSwitch = 0;
            finalizeThreadingState(threading, expect);
            expect.traverseRight = true;
            if (i == 2) {
                // When we reach the last thread, we should clear the wakeup and resume normal execution
                expect.traverseRight = false;
                expect.wakeup = sys.FUTEX_EMPTY_ADDR;
            }

            bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
            assertEq(postState, outputState(expect), "unexpected post state");

            state = expect;
        }
    }

    /// @dev static unit test asserting state transition of an empty right thread stack
    ///      This occurs during wakeup traversal
    function test_wakeup_traversalEnds_succeeds() public {
        threading.setTraverseRight(true);
        IMIPS2.ThreadState memory thread = threading.createThread();
        IMIPS2.State memory state;
        state.traverseRight = true;
        state.wakeup = 0x1000;
        state.stepsSinceLastContextSwitch = 10;
        finalizeThreadingState(threading, state);
        bytes memory threadWitness = threading.witness();

        // state changes
        threading.right().pop();
        threading.left().push(thread);
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        // Note that this does not change. The next thread scheduled (on the left stack) was the last thread on the
        // right stack.
        expect.stepsSinceLastContextSwitch = 0;
        expect.wakeup = sys.FUTEX_EMPTY_ADDR;
        expect.traverseRight = false;
        finalizeThreadingState(threading, expect);

        bytes memory memProof; // unused
        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev static unit test asserting state transition of completion of a timed-out futex waiter
    function test_futexTimeoutCompletion_succeeds() public {
        threading.createThread();
        threading.createThread();
        IMIPS2.ThreadState memory threadB = threading.current();
        threadB.futexAddr = 0x1000;
        threadB.futexVal = 0xdead;
        threadB.futexTimeoutStep = 10;
        threading.replaceCurrent(threadB);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        state.step = 10;
        state.stepsSinceLastContextSwitch = 10; // must be unchanged
        finalizeThreadingState(threading, state);

        // Resume the current blocked thread on futex timeout
        IMIPS2.ThreadState memory expectThread = copyThread(threadB);
        expectThread.pc = threadB.nextPC;
        expectThread.nextPC = threadB.nextPC + 4;
        expectThread.futexAddr = sys.FUTEX_EMPTY_ADDR;
        expectThread.futexVal = 0x0;
        expectThread.futexTimeoutStep = 0;
        expectThread.registers[2] = sys.SYS_ERROR_SIGNAL;
        expectThread.registers[7] = sys.ETIMEDOUT;
        threading.replaceCurrent(expectThread);
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.wakeup = sys.FUTEX_EMPTY_ADDR;
        finalizeThreadingState(threading, expect);

        bytes memory memProof;
        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting wakeup where the current thread is ready to be woken up
    function testFuzz_wakeupWaiter_succeeds(
        uint32 _wakeup,
        uint32 _futexVal,
        uint32 _futexTimeoutStep,
        bool _threadExited
    )
        public
    {
        vm.assume(_wakeup != sys.FUTEX_EMPTY_ADDR);

        threading.createThread();
        threading.createThread();
        IMIPS2.ThreadState memory threadB = threading.current();

        threadB.futexAddr = _wakeup;
        threadB.futexVal = _futexVal;
        threadB.futexTimeoutStep = _futexTimeoutStep;
        // A thread exit cannot interrupt wakeup traversal. thread.exited during wakeup is technically not a valid
        // state.
        // But we fuzz this anyways to ensure the VM only traverses threads during wakeup
        threadB.exited = _threadExited;
        threadB.exitCode = _threadExited ? 1 : 0;
        threading.replaceCurrent(threadB);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        bytes memory memProof; // unused
        state.wakeup = _wakeup;
        state.step = 10;
        state.stepsSinceLastContextSwitch = 20; // must be unchanged
        finalizeThreadingState(threading, state);

        // Resume the current thread that is blocked
        IMIPS2.ThreadState memory expectThread = copyThread(threadB);
        // no changes on thread since we're in wakeup traversal
        threading.replaceCurrent(expectThread);

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.wakeup = sys.FUTEX_EMPTY_ADDR;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting wakeup where the current thread isn't ready
    function testFuzz_wakeupNonWaiter_succeeds(
        uint32 _wakeup,
        uint32 _futexAddr,
        uint32 _futexVal,
        uint32 _futexTimeoutStep,
        bool _threadExited
    )
        public
    {
        vm.assume(_wakeup != sys.FUTEX_EMPTY_ADDR);
        vm.assume(_wakeup != _futexAddr);

        threading.createThread();
        threading.createThread();
        IMIPS2.ThreadState memory threadB = threading.current();
        threadB.futexAddr = _futexAddr;
        threadB.futexVal = _futexVal;
        threadB.futexTimeoutStep = _futexTimeoutStep;
        threadB.exited = _threadExited;
        threadB.exitCode = _threadExited ? 1 : 0;
        threading.replaceCurrent(threadB);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        bytes memory memProof; // unused
        state.wakeup = _wakeup;
        state.step = 10;
        state.stepsSinceLastContextSwitch = 20;
        finalizeThreadingState(threading, state);

        // state changes
        IMIPS2.ThreadState memory expectThread = copyThread(threadB);
        // thread internal state is unchanged since we're in wakeup traversal
        threading.replaceCurrent(expectThread);
        threading.left().pop();
        threading.right().push(expectThread);

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = 0;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev static unit test asserting state transition of completion of a non-timed-out futex waiter
    function test_futexNoTimeoutCompletion_succeeds() public {
        threading.createThread();
        threading.createThread();
        IMIPS2.ThreadState memory threadB = threading.current();
        threadB.futexAddr = 0x1000;
        threadB.futexVal = 0xdead;
        threadB.futexTimeoutStep = 100;
        threading.replaceCurrent(threadB);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        bytes memory memProof;
        (state.memRoot, memProof) = ffi.getCannonMemoryProof(0, 0, threadB.futexAddr, threadB.futexVal + 1);
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        state.step = 10;
        state.stepsSinceLastContextSwitch = 10; // must be unchanged
        finalizeThreadingState(threading, state);

        // Resume the current thread that is blocked
        IMIPS2.ThreadState memory expectThread = copyThread(threadB);
        expectThread.pc = threadB.nextPC;
        expectThread.nextPC = threadB.nextPC + 4;
        expectThread.futexAddr = sys.FUTEX_EMPTY_ADDR;
        expectThread.futexVal = 0x0;
        expectThread.futexTimeoutStep = 0;
        expectThread.registers[2] = 0;
        expectThread.registers[7] = 0; // errno
        threading.replaceCurrent(expectThread);

        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.wakeup = sys.FUTEX_EMPTY_ADDR;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev static unit test asserting state transition of futex waiter that isn't ready
    function test_futexNoTimeoutPreemptsThread_succeeds() public {
        threading.createThread();
        threading.createThread();
        IMIPS2.ThreadState memory threadB = threading.current();
        threadB.futexAddr = 0x1000;
        threadB.futexVal = 0xdead;
        threadB.futexTimeoutStep = sys.FUTEX_NO_TIMEOUT;
        threading.replaceCurrent(threadB);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        bytes memory memProof;
        (state.memRoot, memProof) = ffi.getCannonMemoryProof(0, 0, threadB.futexAddr, threadB.futexVal);
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        state.stepsSinceLastContextSwitch = 10;
        finalizeThreadingState(threading, state);

        // Expect the thread to be moved from the left to right stack
        threading.left().pop();
        threading.right().push(threadB);
        IMIPS2.State memory expect = copyState(state);
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = 0;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting VM behavior when the current thread has exited
    function test_threadExit_succeeds() public {
        threading.createThread();
        threading.createThread();
        IMIPS2.ThreadState memory threadB = threading.current();
        threadB.exited = true;
        threading.replaceCurrent(threadB);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        state.stepsSinceLastContextSwitch = 10;
        finalizeThreadingState(threading, state);

        // Expect the thread to be popped from the left stack
        threading.left().pop();
        IMIPS2.State memory expect = copyState(state);
        expect.stepsSinceLastContextSwitch = 0;
        expect.step = state.step + 1;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), threadWitness, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Static unit test asserting VM behavior when the current thread has exited and the current thread stack is
    /// almost empty
    function test_threadExit_swapStacks_succeeds() public {
        threading.setTraverseRight(true);
        threading.createThread();
        threading.setTraverseRight(false);
        threading.createThread();
        IMIPS2.ThreadState memory threadL = threading.current();
        threadL.exited = true;
        threading.replaceCurrent(threadL);
        bytes memory threadWitness = threading.witness();

        IMIPS2.State memory state;
        state.wakeup = sys.FUTEX_EMPTY_ADDR;
        state.stepsSinceLastContextSwitch = 10;
        finalizeThreadingState(threading, state);

        threading.left().pop();
        IMIPS2.State memory expect = copyState(state);
        expect.stepsSinceLastContextSwitch = 0;
        expect.step = state.step + 1;
        expect.traverseRight = true;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), threadWitness, 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mmap_simple_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);

        state.heap = 4096;
        thread.nextPC = 4;
        thread.registers[2] = sys.SYS_MMAP; // syscall num
        thread.registers[4] = 0x0; // a0
        thread.registers[5] = 4095; // a1
        updateThreadStacks(state, thread);

        // Set up step data
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        bytes memory encodedState = encodeState(state);

        IMIPS2.State memory expect = copyState(state);
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expect.memRoot = state.memRoot;
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        expect.heap = state.heap + 4096;
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = state.heap; // return old heap
        expectThread.registers[7] = 0; // No error
        updateThreadStacks(expect, expectThread);

        bytes32 postState = mips.step(encodedState, bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mmap_justWithinMemLimit_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);

        state.heap = sys.HEAP_END - 4096; // Set up to increase heap to its limit
        thread.nextPC = 4;
        thread.registers[2] = sys.SYS_MMAP; // syscall num
        thread.registers[4] = 0x0; // a0
        thread.registers[5] = 4095; // a1
        updateThreadStacks(state, thread);

        // Set up step data
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        bytes memory encodedState = encodeState(state);

        IMIPS2.State memory expect = copyState(state);
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expect.memRoot = state.memRoot;
        expect.step += 1;
        expect.stepsSinceLastContextSwitch += 1;
        expect.heap = sys.HEAP_END;
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = state.heap; // Return the old heap value
        expectThread.registers[7] = 0; // No error
        updateThreadStacks(expect, expectThread);

        bytes32 postState = mips.step(encodedState, bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_step_mmap_fails() external {
        uint32 insn = 0x0000000c; // syscall
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);

        state.heap = sys.HEAP_END - 4096; // Set up to increase heap beyond its limit
        thread.nextPC = 4;
        thread.registers[2] = sys.SYS_MMAP; // syscall num
        thread.registers[4] = 0x0; // a0
        thread.registers[5] = 4097; // a1
        updateThreadStacks(state, thread);

        // Set up step data
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        bytes memory encodedState = encodeState(state);

        IMIPS2.State memory expect = copyState(state);
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expect.memRoot = state.memRoot;
        expect.step += 1;
        expect.stepsSinceLastContextSwitch += 1;
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = sys.SYS_ERROR_SIGNAL; // signal an stdError
        expectThread.registers[7] = sys.EINVAL; // Return error value
        expectThread.registers[4] = thread.registers[4]; // a0
        expectThread.registers[5] = thread.registers[5]; // a1
        updateThreadStacks(expect, expectThread);

        bytes32 postState = mips.step(encodedState, bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_srav_succeeds() external {
        uint32 insn = encodespec(0xa, 0x9, 0x8, 7); // srav t0, t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0xdeafbeef; // t1
        thread.registers[10] = 12; // t2
        updateThreadStacks(state, thread);

        // Set up step data
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        bytes memory encodedState = encodeState(state);

        IMIPS2.State memory expect = copyState(state);
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expect.memRoot = state.memRoot;
        expect.step += 1;
        expect.stepsSinceLastContextSwitch += 1;
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[8] = 0xfffdeafb; // t0
        updateThreadStacks(expect, expectThread);

        bytes32 postState = mips.step(encodedState, bytes.concat(threadWitness, memProof), 0);
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
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0xdeadbeef; // t1
        thread.registers[10] = _rs; // t2
        updateThreadStacks(state, thread);

        // Set up step data
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        bytes memory encodedState = encodeState(state);

        // Calculate shamt
        uint32 shamt = thread.registers[10] & 0x1F;

        IMIPS2.State memory expect = copyState(state);
        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expect.memRoot = state.memRoot;
        expect.step += 1;
        expect.stepsSinceLastContextSwitch += 1;
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[8] = ins.signExtend(thread.registers[9] >> shamt, 32 - shamt); // t0
        updateThreadStacks(expect, expectThread);

        bytes32 postState = mips.step(encodedState, bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_add_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x20); // add t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 12;
        thread.registers[18] = 20;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] + thread.registers[18]; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_addu_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x21); // addu t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 12;
        thread.registers[18] = 20;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] + thread.registers[18]; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_addi_succeeds() public {
        uint16 imm = 40;
        uint32 insn = encodeitype(0x8, 17, 8, imm); // addi t0, s1, 40
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 1; // t0
        thread.registers[17] = 4; // t1
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] + imm; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_addiSign_succeeds() public {
        uint16 imm = 0xfffe; // -2
        uint32 insn = encodeitype(0x8, 17, 8, imm); // addi t0, s1, 40
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 1; // s0
        thread.registers[17] = 2; // s1
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = 0; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_addui_succeeds() public {
        // copy the existing corresponding test in MIPS.t.sol and adapt for MIPS2
        uint16 imm = 40;
        uint32 insn = encodeitype(0x9, 17, 8, imm); // addui t0, s1, 40
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 1; // t0
        thread.registers[17] = 4; // t1
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] + imm; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sub_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x22); // sub t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 20;
        thread.registers[18] = 12;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] - thread.registers[18]; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_subu_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x23); // subu t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 20;
        thread.registers[18] = 12;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] - thread.registers[18]; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_and_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x24); // and t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 1200;
        thread.registers[18] = 490;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] & thread.registers[18]; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_andi_succeeds() public {
        uint16 imm = 40;
        uint32 insn = encodeitype(0xc, 17, 8, imm); // andi t0, s1, 40
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 1; // t0
        thread.registers[17] = 4; // s1
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] & imm; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_or_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x25); // or t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 1200;
        thread.registers[18] = 490;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] | thread.registers[18]; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_ori_succeeds() public {
        uint16 imm = 40;
        uint32 insn = encodeitype(0xd, 17, 8, imm); // ori t0, s1, 40
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 1; // t0
        thread.registers[17] = 4; // s1
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] | imm; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_xor_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x26); // xor t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 1200;
        thread.registers[18] = 490;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] ^ thread.registers[18]; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_xori_succeeds() public {
        uint16 imm = 40;
        uint32 insn = encodeitype(0xe, 17, 8, imm); // xori t0, s1, 40
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 1; // t0
        thread.registers[17] = 4; // s1
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] ^ imm; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_nor_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x27); // nor t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 1200;
        thread.registers[18] = 490;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = ~(thread.registers[17] | thread.registers[18]); // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_slt_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x2a); // slt t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 0xFF_FF_FF_FE; // -2
        thread.registers[18] = 5;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = 1; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        // swap operands and check again
        uint32 tmp = thread.registers[17];
        thread.registers[17] = thread.registers[18];
        thread.registers[18] = tmp;
        threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        result = 0; // t0
        expect = arithmeticPostState(state, thread, 8, /* t0 */ result);
        postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sltu_succeeds() public {
        uint32 insn = encodespec(17, 18, 8, 0x2b); // sltu t0, s1, s2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[17] = 1200;
        thread.registers[18] = 490;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[17] < thread.registers[18] ? 1 : 0; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lb_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x20, 0x9, 0x8, 0x4); // lb $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0x12_00_00_00);
        thread.registers[8] = 0; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = 0x12; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lh_succeeds() public {
        uint32 t1 = 0x100;
        uint32 val = 0x12_23_00_00;
        uint32 insn = encodeitype(0x21, 0x9, 0x8, 0x4); // lh $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, val);
        thread.registers[8] = 0; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = 0x12_23; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lw_succeeds() public {
        uint32 t1 = 0x100;
        uint32 val = 0x12_23_45_67;
        uint32 insn = encodeitype(0x23, 0x9, 0x8, 0x4); // lw $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, val);
        thread.registers[8] = 0; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = val; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lbu_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x24, 0x9, 0x8, 0x4); // lbu $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0x12_23_00_00);
        thread.registers[8] = 0; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = 0x12; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lhu_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x25, 0x9, 0x8, 0x4); // lhu $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0x12_23_00_00);
        thread.registers[8] = 0; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = 0x12_23; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lwl_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x22, 0x9, 0x8, 0x4); // lwl $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0x12_34_56_78);
        thread.registers[8] = 0xaa_bb_cc_dd; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.registers[8] = 0x12_34_56_78; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lwl_unaligned_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x22, 0x9, 0x8, 0x5); // lwl $t0, 5($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0x12_34_56_78);
        thread.registers[8] = 0x34_56_78_dd; // t0
        thread.registers[9] = t1; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        (state.memRoot, memProof) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0x12_34_56_78);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);
        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lwr_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x26, 0x9, 0x8, 0x4); // lwr $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0x12_34_56_78);
        thread.registers[8] = 0xaa_bb_cc_dd; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.registers[8] = 0xaa_bb_cc_12; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lwr_unaligned_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x26, 0x9, 0x8, 0x5); // lwr $t0, 5($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0x12_34_56_78);
        thread.registers[8] = 0xaa_bb_cc_dd; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        (state.memRoot, memProof) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0x12_34_56_78);
        updateThreadStacks(state, thread);

        thread.registers[8] = 0xaa_bb_12_34; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sb_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x28, 0x9, 0x8, 0x4); // sb $t0, 4($t1)
        // note. cannon memory is zero-initialized. mem[t+4] = 0 is a no-op
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0);
        thread.registers[8] = 0xaa_bb_cc_dd; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xdd_00_00_00);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sh_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x29, 0x9, 0x8, 0x4); // sh $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0);
        thread.registers[8] = 0xaa_bb_cc_dd; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xcc_dd_00_00);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_swl_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x2a, 0x9, 0x8, 0x4); // swl $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0);
        thread.registers[8] = 0xaa_bb_cc_dd; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xaa_bb_cc_dd);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sw_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x2b, 0x9, 0x8, 0x4); // sw $t0, 4($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0);
        thread.registers[8] = 0xaa_bb_cc_dd; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xaa_bb_cc_dd);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_swr_succeeds() public {
        uint32 t1 = 0x100;
        uint32 insn = encodeitype(0x2e, 0x9, 0x8, 0x5); // swr $t0, 5($t1)
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, t1 + 4, 0);
        thread.registers[8] = 0xaa_bb_cc_dd; // t0
        thread.registers[9] = t1;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, t1 + 4, 0xcc_dd_00_00);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_ll_succeeds() public {
        uint32 base = 0x100;
        uint32 memVal = 0x12_23_45_67;
        uint16 offset = 0x4;
        uint32 effAddr = base + offset;
        uint32 insn = encodeitype(0x30, 0x9, 0x8, offset); // ll baseReg, rtReg, offset

        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, effAddr, memVal);
        thread.registers[8] = 0; // rtReg
        thread.registers[9] = base;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, memVal);
        expect.llReservationStatus = 1;
        expect.llAddress = effAddr;
        expect.llOwnerThread = thread.threadID;

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sc_succeeds() public {
        uint32 base = 0x100;
        uint16 offset = 0x4;
        uint32 effAddr = base + offset;
        uint32 writeMemVal = 0xaa_bb_cc_dd;
        uint32 insn = encodeitype(0x38, 0x9, 0x8, offset); // ll baseReg, rtReg, offset

        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, effAddr, 0);
        state.llReservationStatus = 1;
        state.llAddress = effAddr;
        state.llOwnerThread = thread.threadID;
        thread.registers[8] = writeMemVal;
        thread.registers[9] = base;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, 0x1);
        (expect.memRoot,) = ffi.getCannonMemoryProof(0, insn, effAddr, writeMemVal);
        expect.llReservationStatus = 0;
        expect.llAddress = 0;
        expect.llOwnerThread = 0;

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_movn_succeeds() public {
        uint32 insn = encodespec(0x9, 0xa, 0x8, 0xb); // movn $t0, $t1, $t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0xa; // t0
        thread.registers[9] = 0xb; // t1
        thread.registers[10] = 0x1; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[9]; // t1
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);
        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        thread.registers[10] = 0x0; // t2
        updateThreadStacks(state, thread);
        expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);
        threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_movz_succeeds() public {
        uint32 insn = encodespec(0x9, 0xa, 0x8, 0xa); // movz $t0, $t1, $t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0xa; // t0
        thread.registers[9] = 0xb; // t1
        thread.registers[10] = 0x0; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[9]; // t1
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);
        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");

        thread.registers[10] = 0x1; // t2
        updateThreadStacks(state, thread);
        expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);
        threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mflo_succeeds() public {
        uint32 insn = encodespec(0x0, 0x0, 0x8, 0x12); // mflo $t0
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.lo = 0xdeadbeef;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.lo);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mfhi_succeeds() public {
        uint32 insn = encodespec(0x0, 0x0, 0x8, 0x10); // mfhi $t0
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.hi = 0xdeadbeef;
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.hi);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mthi_succeeds() public {
        uint32 insn = encodespec(0x8, 0x0, 0x0, 0x11); // mthi $t0
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0xdeadbeef; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.hi = thread.registers[8];
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mtlo_succeeds() public {
        uint32 insn = encodespec(0x8, 0x0, 0x0, 0x13); // mtlo $t0
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0xdeadbeef; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.lo = thread.registers[8];
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ thread.registers[8]);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mul_succeeds() public {
        uint32 insn = encodespec2(0x9, 0xa, 0x8, 0x2); // mul t0, t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 5; // t1
        thread.registers[10] = 2; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[9] * thread.registers[10]; // t0
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_mult_succeeds() public {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x18); // mult t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0x0F_FF_00_00; // t1
        thread.registers[10] = 100; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 loResult = 0x3F_9C_00_00;
        uint32 hiResult = 0x6;
        thread.lo = loResult;
        thread.hi = hiResult;
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 0, /* t0 */ 0); // no update on t0

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_multu_succeeds() public {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x19); // multu t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0x0F_FF_00_00; // t1
        thread.registers[10] = 100; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        uint32 loResult = 0x3F_9C_00_00;
        uint32 hiResult = 0x6;
        thread.lo = loResult;
        thread.hi = hiResult;
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 0, /* t0 */ 0); // no update on t0

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_div_succeeds() public {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x1a); // div t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 5; // t1
        thread.registers[10] = 2; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.lo = 2;
        thread.hi = 1;
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 0, /* t0 */ 0); // no update on t0

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_divu_succeeds() public {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x1b); // divu t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 5; // t1
        thread.registers[10] = 2; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.lo = 2;
        thread.hi = 1;
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 0, /* t0 */ 0); // no update on t0

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_div_byZero_fails() public {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x1a); // div t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 5; // t1
        thread.registers[10] = 0; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        vm.expectRevert("MIPS: division by zero");
        mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
    }

    function test_divu_byZero_fails() public {
        uint32 insn = encodespec(0x9, 0xa, 0x0, 0x1b); // divu t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 5; // t1
        thread.registers[10] = 0; // t2
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        vm.expectRevert("MIPS: division by zero");
        mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
    }

    function test_beq_succeeds() public {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x4, 0x9, 0x8, boff); // beq $t0, $t1, 16
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0xdeadbeef; // t0
        thread.registers[9] = 0xdeadbeef; // t1
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, thread.nextPC + (uint32(boff) << 2));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_beq_notTaken_succeeds() public {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x4, 0x9, 0x8, boff); // beq $t0, $t1, 16
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0xaa; // t0
        thread.registers[9] = 0xdeadbeef; // t1
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, thread.nextPC + 4);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_bne_succeeds() public {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x5, 0x9, 0x8, boff); // bne $t0, $t1, 16
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0xdeadbeef; // t0
        thread.registers[9] = 0xaa; // t1
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, thread.nextPC + (uint32(boff) << 2));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_blez_succeeds() public {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x6, 0x8, 0x0, boff); // blez $t0, 16
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, thread.nextPC + (uint32(boff) << 2));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_bgtz_succeeds() public {
        uint16 boff = 0xa0;
        uint32 insn = encodeitype(0x7, 0x8, 0x0, boff); // bgtz $t0, 0xa0
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 1; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, thread.nextPC + (uint32(boff) << 2));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_bltz_succeeds() public {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x1, 0x8, 0x0, boff); // bltz $t0, 16
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0xF0_00_00_00; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, thread.nextPC + (uint32(boff) << 2));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_bgez_succeeds() public {
        uint16 boff = 0x10;
        uint32 insn = encodeitype(0x1, 0x8, 0x1, boff); // bgez $t0, 16
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = 0x00_00_00_01; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, thread.nextPC + (uint32(boff) << 2));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jump_succeeds() public {
        uint32 label = 0x02_00_00_02; // set the 26th bit to assert no sign extension
        uint32 insn = uint32(0x08_00_00_00) | label; // j label
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, label << 2);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jump_nonzeroRegion_succeeds() public {
        uint32 pcRegion1 = 0x10000000;
        uint32 label = 0x2;
        uint32 insn = uint32(0x08_00_00_00) | label; // j label
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(pcRegion1, insn, 0x4, 0);
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect =
            controlFlowPostState(state, thread, (thread.nextPC & 0xF0_00_00_00) | (uint32(label) << 2));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jal_succeeds() public {
        uint32 label = 0x02_00_00_02; // set the 26th bit to assert no sign extension
        uint32 insn = uint32(0x0c_00_00_00) | label; // jal label
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.registers[31] = thread.pc + 8; // ra
        IMIPS2.State memory expect = controlFlowPostState(state, thread, label << 2);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jal_nonzeroRegion_succeeds() public {
        uint32 pcRegion1 = 0x10000000;
        uint32 label = 0x2;
        uint32 insn = uint32(0x0c_00_00_00) | label; // jal label
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(pcRegion1, insn, 0x4, 0);
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.registers[31] = thread.pc + 8; // ra
        IMIPS2.State memory expect =
            controlFlowPostState(state, thread, (thread.nextPC & 0xF0_00_00_00) | (uint32(label) << 2));

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jr_succeeds() public {
        uint16 tgt = 0x34;
        uint32 insn = encodespec(0x8, 0, 0, 0x8); // jr t0
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = tgt; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = controlFlowPostState(state, thread, tgt);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_jalr_succeeds() public {
        uint16 tgt = 0x34;
        uint32 insn = encodespec(0x8, 0, 0x9, 0x9); // jalr t1, t0
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[8] = tgt; // t0
        bytes memory threadWitness = abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT);
        updateThreadStacks(state, thread);

        thread.registers[9] = thread.pc + 8; // t1
        IMIPS2.State memory expect = controlFlowPostState(state, thread, tgt);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sll_succeeds() external {
        uint8 shiftamt = 4;
        uint32 insn = encodespec(0x0, 0x9, 0x8, uint16(shiftamt) << 6); // sll t0, t1, 3
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0x20; // t1
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[9] << shiftamt;
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_srl_succeeds() external {
        uint8 shiftamt = 4;
        uint32 insn = encodespec(0x0, 0x9, 0x8, uint16(shiftamt) << 6 | 2); // srl t0, t1, 3
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0x20; // t1
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[9] >> shiftamt;
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sra_succeeds() external {
        uint8 shiftamt = 4;
        uint32 insn = encodespec(0x0, 0x9, 0x8, uint16(shiftamt) << 6 | 3); // sra t0, t1, 3
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0x80_00_00_20; // t1
        updateThreadStacks(state, thread);

        uint32 result = 0xF8_00_00_02; // 4 shifts while preserving sign bit
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_sllv_succeeds() external {
        uint32 insn = encodespec(0xa, 0x9, 0x8, 4); // sllv t0, t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0x20; // t1
        thread.registers[10] = 4; // t2
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[9] << thread.registers[10];
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_srlv_succeeds() external {
        uint32 insn = encodespec(0xa, 0x9, 0x8, 6); // srlv t0, t1, t2
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0x20_00; // t1
        thread.registers[10] = 4; // t2
        updateThreadStacks(state, thread);

        uint32 result = thread.registers[9] >> thread.registers[10];
        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ result);

        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_lui_succeeds() external {
        uint32 insn = encodeitype(0xf, 0x0, 0x8, 0x4); // lui $t0, 0x04
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ 0x00_04_00_00);
        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_clo_succeeds() external {
        uint32 insn = encodespec2(0x9, 0x0, 0x8, 0x21); // clo t0, t1
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0xFF_00_00_00; // t1
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ 8);
        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_clz_succeeds() external {
        uint32 insn = encodespec2(0x9, 0x0, 0x8, 0x20); // clz t0, t1
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, 0x4, 0);
        thread.registers[9] = 0x00_00_F0_00; // t1
        updateThreadStacks(state, thread);

        IMIPS2.State memory expect = arithmeticPostState(state, thread, 8, /* t0 */ 16);
        bytes32 postState = mips.step(
            encodeState(state), bytes.concat(abi.encodePacked(encodeThread(thread), EMPTY_THREAD_ROOT), memProof), 0
        );
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_preimage_read_succeeds() external {
        uint32 pc = 0x0;
        uint32 insn = 0x0000000c; // syscall
        uint32 a1 = 0x4;
        uint32 a1_val = 0x0000abba;
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, a1, a1_val);
        state.preimageKey = bytes32(uint256(1) << 248 | 0x01);
        state.preimageOffset = 8; // start reading past the pre-image length prefix
        thread.registers[2] = 4003; // read syscall
        thread.registers[4] = 5; // fd
        thread.registers[5] = a1; // addr
        thread.registers[6] = 4; // count
        threading.createThread();
        threading.replaceCurrent(thread);
        bytes memory threadWitness = threading.witness();
        finalizeThreadingState(threading, state);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 4; // return
        expectThread.registers[7] = 0; // errno
        threading.replaceCurrent(expectThread);

        // prime the pre-image oracle
        bytes32 word = bytes32(uint256(0xdeadbeef) << 224);
        uint8 size = 4;
        uint8 partOffset = 8;
        oracle.loadLocalData(uint256(state.preimageKey), 0, word, size, partOffset);

        IMIPS2.State memory expect = copyState(state);
        expect.preimageOffset += 4;
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        // recompute merkle root of written pre-image
        (expect.memRoot,) = ffi.getCannonMemoryProof(pc, insn, a1, 0xdeadbeef);
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    function test_preimage_write_succeeds() external {
        uint32 insn = 0x0000000c; // syscall
        uint32 a1 = 0x4;
        uint32 a1_val = 0x0000abba;
        (IMIPS2.State memory state, IMIPS2.ThreadState memory thread, bytes memory memProof) =
            constructMIPSState(0, insn, a1, a1_val);
        state.preimageKey = bytes32(0);
        state.preimageOffset = 1;
        thread.registers[2] = 4004; // write syscall
        thread.registers[4] = 6; // fd
        thread.registers[5] = a1; // addr
        thread.registers[6] = 4; // count
        threading.createThread();
        threading.replaceCurrent(thread);
        bytes memory threadWitness = threading.witness();
        finalizeThreadingState(threading, state);

        IMIPS2.ThreadState memory expectThread = copyThread(thread);
        expectThread.pc = thread.nextPC;
        expectThread.nextPC = thread.nextPC + 4;
        expectThread.registers[2] = 4; // return
        expectThread.registers[7] = 0; // errno
        threading.replaceCurrent(expectThread);

        IMIPS2.State memory expect = copyState(state);
        expect.preimageKey = bytes32(uint256(0xabba));
        expect.preimageOffset = 0;
        expect.step = state.step + 1;
        expect.stepsSinceLastContextSwitch = state.stepsSinceLastContextSwitch + 1;
        finalizeThreadingState(threading, expect);

        bytes32 postState = mips.step(encodeState(state), bytes.concat(threadWitness, memProof), 0);
        assertEq(postState, outputState(expect), "unexpected post state");
    }

    /// @dev Modifies the MIPS2 State based on threading state
    function finalizeThreadingState(Threading _threading, IMIPS2.State memory _state) internal view {
        _state.leftThreadStack = _threading.left().root();
        _state.rightThreadStack = _threading.right().root();
        _state.nextThreadID = uint32(_threading.nextThreadID());
    }

    /// @dev constructs a generic MIPS2 state for single-threaded execution.
    function constructMIPSState(
        uint32 pc,
        uint32 insn,
        uint32 addr,
        uint32 val
    )
        internal
        returns (IMIPS2.State memory state_, IMIPS2.ThreadState memory thread_, bytes memory proof_)
    {
        (state_.memRoot, proof_) = ffi.getCannonMemoryProof(pc, insn, addr, val);
        state_.nextThreadID = 1;
        state_.wakeup = sys.FUTEX_EMPTY_ADDR;
        thread_.pc = pc;
        thread_.nextPC = pc + 4;
        thread_.futexAddr = sys.FUTEX_EMPTY_ADDR;
        state_.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(thread_))));
        state_.rightThreadStack = EMPTY_THREAD_ROOT;
    }

    /// @dev Updates the state stack roots with a single thread
    function updateThreadStacks(IMIPS2.State memory _state, IMIPS2.ThreadState memory _thread) internal pure {
        if (_state.traverseRight) {
            _state.rightThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(_thread))));
        } else {
            _state.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(_thread))));
        }
    }

    /// @dev Constructs a post-state after an arithmetic or logical instruction
    function arithmeticPostState(
        IMIPS2.State memory _state,
        IMIPS2.ThreadState memory _thread,
        uint32 reg,
        uint32 regVal
    )
        internal
        pure
        returns (IMIPS2.State memory out_)
    {
        IMIPS2.ThreadState memory expectThread = copyThread(_thread);
        expectThread.pc = _thread.nextPC;
        expectThread.nextPC = _thread.nextPC + 4;
        expectThread.registers[reg] = regVal;

        out_ = copyState(_state);
        out_.step = _state.step + 1;
        out_.stepsSinceLastContextSwitch = _state.stepsSinceLastContextSwitch + 1;
        out_.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));
    }

    /// @dev Constructs a post-state after a branch instruction
    function controlFlowPostState(
        IMIPS2.State memory _state,
        IMIPS2.ThreadState memory _thread,
        uint32 branchTarget
    )
        internal
        pure
        returns (IMIPS2.State memory out_)
    {
        IMIPS2.ThreadState memory expectThread = copyThread(_thread);
        expectThread.pc = _thread.nextPC;
        expectThread.nextPC = branchTarget;

        out_ = copyState(_state);
        out_.step = _state.step + 1;
        out_.stepsSinceLastContextSwitch = _state.stepsSinceLastContextSwitch + 1;
        out_.leftThreadStack = keccak256(abi.encodePacked(EMPTY_THREAD_ROOT, keccak256(encodeThread(expectThread))));
    }

    function encodeState(IMIPS2.State memory _state) internal pure returns (bytes memory) {
        // Split up encoding to get around stack-too-deep error
        return abi.encodePacked(encodeStateA(_state), encodeStateB(_state));
    }

    function encodeStateA(IMIPS2.State memory _state) internal pure returns (bytes memory) {
        return abi.encodePacked(
            _state.memRoot,
            _state.preimageKey,
            _state.preimageOffset,
            _state.heap,
            _state.llReservationStatus,
            _state.llAddress,
            _state.llOwnerThread,
            _state.exitCode,
            _state.exited,
            _state.step,
            _state.stepsSinceLastContextSwitch,
            _state.wakeup,
            _state.traverseRight,
            _state.leftThreadStack
        );
    }

    function encodeStateB(IMIPS2.State memory _state) internal pure returns (bytes memory) {
        return abi.encodePacked(_state.rightThreadStack, _state.nextThreadID);
    }

    function copyState(IMIPS2.State memory _state) internal pure returns (IMIPS2.State memory out_) {
        bytes memory data = abi.encode(_state);
        return abi.decode(data, (IMIPS2.State));
    }

    function copyThread(IMIPS2.ThreadState memory _thread) internal pure returns (IMIPS2.ThreadState memory out_) {
        bytes memory data = abi.encode(_thread);
        return abi.decode(data, (IMIPS2.ThreadState));
    }

    /// @dev MIPS VM status codes:
    ///      0. Exited with success (Valid)
    ///      1. Exited with success (Invalid)
    ///      2. Exited with failure (Panic)
    ///      3. Unfinished
    function vmStatus(IMIPS2.State memory state) internal pure returns (VMStatus out_) {
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

    event ExpectedOutputState(bytes encoded, IMIPS2.State state);

    function outputState(IMIPS2.State memory state) internal returns (bytes32 out_) {
        bytes memory enc = encodeState(state);
        emit ExpectedOutputState(enc, state);
        VMStatus status = vmStatus(state);
        out_ = keccak256(enc);
        assembly {
            out_ := or(and(not(shl(248, 0xFF)), out_), shl(248, status))
        }
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

function encodeThread(IMIPS2.ThreadState memory _thread) pure returns (bytes memory) {
    bytes memory registers;
    for (uint256 i = 0; i < _thread.registers.length; i++) {
        registers = bytes.concat(registers, abi.encodePacked(_thread.registers[i]));
    }
    return abi.encodePacked(
        _thread.threadID,
        _thread.exitCode,
        _thread.exited,
        _thread.futexAddr,
        _thread.futexVal,
        _thread.futexTimeoutStep,
        _thread.pc,
        _thread.nextPC,
        _thread.lo,
        _thread.hi,
        registers
    );
}
