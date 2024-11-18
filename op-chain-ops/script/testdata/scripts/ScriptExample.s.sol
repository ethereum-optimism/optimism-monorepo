// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Vm is a minimal interface to the forge cheatcode precompile
interface Vm {
    function envOr(string calldata name, bool defaultValue) external view returns (bool value);
    function getNonce(address account) external view returns (uint64 nonce);
    function parseJsonKeys(string calldata json, string calldata key) external pure returns (string[] memory keys);
    function startPrank(address msgSender) external;
    function stopPrank() external;
    function broadcast() external;
    function broadcast(address msgSender) external;
    function startBroadcast(address msgSender) external;
    function startBroadcast() external;
    function stopBroadcast() external;
    function getDeployedCode(string calldata artifactPath) external view returns (bytes memory runtimeBytecode);
    function etch(address target, bytes calldata newRuntimeBytecode) external;
    function allowCheatcodes(address account) external;
    function createSelectFork(string calldata forkName, uint256 blockNumber) external returns (uint256);
}

// console is a minimal version of the console2 lib.
library console {
    address constant CONSOLE_ADDRESS = address(0x000000000000000000636F6e736F6c652e6c6f67);

    function _castLogPayloadViewToPure(function(bytes memory) internal view fnIn)
        internal
        pure
        returns (function(bytes memory) internal pure fnOut)
    {
        assembly {
            fnOut := fnIn
        }
    }

    function _sendLogPayload(bytes memory payload) internal pure {
        _castLogPayloadViewToPure(_sendLogPayloadView)(payload);
    }

    function _sendLogPayloadView(bytes memory payload) private view {
        uint256 payloadLength = payload.length;
        address consoleAddress = CONSOLE_ADDRESS;
        /// @solidity memory-safe-assembly
        assembly {
            let payloadStart := add(payload, 32)
            let r := staticcall(gas(), consoleAddress, payloadStart, payloadLength, 0, 0)
        }
    }

    function log(string memory p0) internal pure {
        _sendLogPayload(abi.encodeWithSignature("log(string)", p0)); // nosemgrep: sol-style-use-abi-encodecall
    }

    function log(string memory p0, bool p1) internal pure {
        _sendLogPayload(abi.encodeWithSignature("log(string,bool)", p0, p1)); // nosemgrep: sol-style-use-abi-encodecall
    }

    function log(string memory p0, uint256 p1) internal pure {
        _sendLogPayload(abi.encodeWithSignature("log(string,uint256)", p0, p1)); // nosemgrep: sol-style-use-abi-encodecall
    }

    function log(string memory p0, address p1) internal pure {
        _sendLogPayload(abi.encodeWithSignature("log(string,address)", p0, p1)); // nosemgrep: sol-style-use-abi-encodecall
    }

    function log(string memory p0, string memory p1, string memory p2) internal pure {
        _sendLogPayload(abi.encodeWithSignature("log(string,string,string)", p0, p1, p2)); // nosemgrep: sol-style-use-abi-encodecall
    }
}

/// @title ScriptExample
/// @notice ScriptExample is an example script. The Go forge script code tests that it can run this.
contract ScriptExample {
    address internal constant VM_ADDRESS = address(uint160(uint256(keccak256("hevm cheat code"))));
    Vm internal constant vm = Vm(VM_ADDRESS);

    // @notice counter variable to force non-pure calls.
    uint256 public counter;

    /// @notice example function, runs through basic cheat-codes and console logs.
    function run() public {
        bool x = vm.envOr("EXAMPLE_BOOL", false);
        console.log("bool value from env", x);

        console.log("contract addr", address(this));
        console.log("contract nonce", vm.getNonce(address(this)));
        console.log("sender addr", address(msg.sender));
        console.log("sender nonce", vm.getNonce(address(msg.sender)));

        string memory json = '{"root_key": [{"a": 1, "b": 2}]}';
        string[] memory keys = vm.parseJsonKeys(json, ".root_key[0]");
        console.log("keys", keys[0], keys[1]);

        this.hello("from original");
        vm.startPrank(address(uint160(0x42)));
        this.hello("from prank 1");
        console.log("parent scope msg.sender", address(msg.sender));
        console.log("parent scope contract.addr", address(this));
        this.hello("from prank 2");
        vm.stopPrank();
        this.hello("from original again");

        // vm.etch should not give cheatcode access, unless allowed to afterwards
        address tmpNonceGetter = address(uint160(uint256(keccak256("temp nonce test getter"))));
        vm.etch(tmpNonceGetter, vm.getDeployedCode("ScriptExample.s.sol:NonceGetter"));
        vm.allowCheatcodes(tmpNonceGetter);
        uint256 v = NonceGetter(tmpNonceGetter).getNonce(address(this));
        console.log("nonce from nonce getter, no explicit access required with vm.etch:", v);

        console.log("done!");
    }

    /// @notice example function, to test vm.broadcast with.
    function runBroadcast() public {
        console.log("nonce start", uint256(vm.getNonce(address(this))));

        console.log("testing single");
        vm.broadcast();
        this.call1("single_call1");
        this.call2("single_call2");

        console.log("testing start/stop");
        vm.startBroadcast(address(uint160(0xc0ffee)));
        this.call1("startstop_call1");
        this.call2("startstop_call2");
        this.callPure("startstop_pure");
        vm.stopBroadcast();
        this.call1("startstop_call3");

        console.log("testing nested");
        vm.startBroadcast(address(uint160(0x1234)));
        this.nested1("nested");
        vm.stopBroadcast();

        console.log("contract deployment");
        vm.broadcast(address(uint160(0x123456)));
        FooBar x = new FooBar(1234);
        require(x.foo() == 1234, "FooBar: foo in create is not 1234");

        console.log("create 2");
        vm.broadcast(address(uint160(0xcafe)));
        FooBar y = new FooBar{salt: bytes32(uint256(42))}(1234);
        require(y.foo() == 1234, "FooBar: foo in create2 is not 1234");
        console.log("done!");

        // Deploy a script without a pranked sender and check the nonce.
        vm.broadcast();
        new FooBar(1234);

        console.log("nonce end", uint256(vm.getNonce(address(this))));
    }

    /// @notice example external function, to force a CALL, and test vm.startPrank with.
    function hello(string calldata _v) external view {
        console.log(_v);
        console.log("hello msg.sender", address(msg.sender));
    }

    function call1(string calldata _v) external {
        counter++;
        console.log(_v);
    }

    function call2(string calldata _v) external {
        counter++;
        console.log(_v);
    }

    function nested1(string calldata _v) external {
        counter++;
        this.nested2(_v);
    }

    function nested2(string calldata _v) external {
        counter++;
        console.log(_v);
    }

    function callPure(string calldata _v) external pure {
        console.log(_v);
    }
}

contract FooBar {
    uint256 public foo;

    constructor(uint256 v) {
        foo = v;
    }
}

contract NonceGetter {
    address internal constant VM_ADDRESS = address(uint160(uint256(keccak256("hevm cheat code"))));
    Vm internal constant vm = Vm(VM_ADDRESS);

    function getNonce(address _addr) public view returns (uint256) {
        return vm.getNonce(_addr);
    }
}

contract ForkedContract {
    uint256 internal v;

    constructor() {
        v = 1;
    }

    function getValue() public view returns (uint256) {
        return v;
    }
}

contract ForkTester {
    address internal constant VM_ADDRESS = address(uint160(uint256(keccak256("hevm cheat code"))));
    Vm internal constant vm = Vm(VM_ADDRESS);

    function run() external {
        address testAddr = address(uint160(0x1234));
        ForkedContract fc = ForkedContract(testAddr);

        vm.createSelectFork("fork1", 12345);
        require(vm.getNonce(testAddr) == 12345, "nonce should be 12345");
        require(fc.getValue() == 1, "value should be 1");
        require(testAddr.balance == uint256(1), "balance should be 1");

        vm.createSelectFork("fork2", 23456);
        require(vm.getNonce(testAddr) == 23456, "nonce should be 12345");
        require(fc.getValue() == 2, "value should be 2");
        require(testAddr.balance == uint256(2), "balance should be 2");
    }
}