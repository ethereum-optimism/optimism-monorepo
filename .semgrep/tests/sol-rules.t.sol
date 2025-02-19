// Semgrep tests for Solidity rules are defined in this file.
// Semgrep tests do not need to be valid Solidity code but should be syntactically correct so that
// Semgrep can parse them. You don't need to be able to *run* the code here but it should look like
// the code that you expect to catch with the rule.
//
// Semgrep testing 101
// Use comments like "ruleid: <rule-id>" to assert that the rule catches the code.
// Use comments like "ok: <rule-id>" to assert that the rule does not catch the code.

/// begin SemgrepTest__sol-style-no-bare-imports
// ok: sol-style-no-bare-imports
import { SomeStruct } from "some-library.sol";

// ok: sol-style-no-bare-imports
import { SomeStruct, AnotherThing } from "some-library.sol";

// ok: sol-style-no-bare-imports
import { SomeStruct as SomeOtherStruct } from "some-library.sol";

// ok: sol-style-no-bare-imports
import { SomeStruct as SomeOtherStruct, AnotherThing as AnotherOtherThing } from "some-library.sol";

// ok: sol-style-no-bare-imports
import { SomeStruct as SomeOtherStruct, AnotherThing } from "some-library.sol";

// ok: sol-style-no-bare-imports
import { AnotherThing, SomeStruct as SomeOtherStruct } from "some-library.sol";

// ruleid: sol-style-no-bare-imports
import "some-library.sol";
/// end   SemgrepTest__sol-style-no-bare-imports

contract SemgrepTest__sol_safety_deployutils_args {
    function test() {
        // ruleid: sol-safety-deployutils-args
        DeployUtils.create1AndSave({
            _save: this,
            _name: "SuperchainConfig",
            _args: abi.encodeCall(ISuperchainConfig.__constructor__, ())
        });

        // ruleid: sol-safety-deployutils-args
        DeployUtils.create1({ _name: "SuperchainConfig", _args: abi.encodeCall(ISuperchainConfig.__constructor__, ()) });

        // ruleid: sol-safety-deployutils-args
        DeployUtils.create2AndSave({
            _save: this,
            _salt: _implSalt(),
            _name: "SuperchainConfig",
            _args: abi.encodeCall(ISuperchainConfig.__constructor__, ())
        });

        // ruleid: sol-safety-deployutils-args
        DeployUtils.create2({
            _salt: _implSalt(),
            _name: "SuperchainConfig",
            _args: abi.encodeCall(ISuperchainConfig.__constructor__, ())
        });

        // ruleid: sol-safety-deployutils-args
        DeployUtils.createDeterministic({
            _name: "SuperchainConfig",
            _args: abi.encodeCall(ISuperchainConfig.__constructor__, ()),
            _salt: _implSalt()
        });

        // ok: sol-safety-deployutils-args
        DeployUtils.create1AndSave({
            _save: this,
            _name: "Proxy",
            _nick: "DataAvailabilityChallengeProxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        });

        // ok: sol-safety-deployutils-args
        DeployUtils.create1({
            _name: "Proxy",
            _nick: "DataAvailabilityChallengeProxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        });

        // ok: sol-safety-deployutils-args
        DeployUtils.create2AndSave({
            _save: this,
            _salt: _implSalt(),
            _name: "Proxy",
            _nick: "DataAvailabilityChallengeProxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        });

        // ok: sol-safety-deployutils-args
        DeployUtils.create2({
            _salt: _implSalt(),
            _name: "Proxy",
            _nick: "DataAvailabilityChallengeProxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        });

        // ok: sol-safety-deployutils-args
        DeployUtils.createDeterministic({
            _name: "Proxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin))),
            _salt: _implSalt()
        });
    }
}

contract SemgrepTest__sol_safety_deployutils_named_args_parameter {
    function test() {
        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create1AndSave(
            this,
            "Proxy",
            "DataAvailabilityChallengeProxy",
            DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        );

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create1(
            "Proxy",
            "DataAvailabilityChallengeProxy",
            DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        );

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create2AndSave(
            this,
            _implSalt(),
            "Proxy",
            "DataAvailabilityChallengeProxy",
            DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        );

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create2(
            _implSalt(),
            "Proxy",
            "DataAvailabilityChallengeProxy",
            DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        );

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create1({ _save: _args, _name: "Proxy", _nick: "DataAvailabilityChallengeProxy" });

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.createDeterministic(
            "Proxy", DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin))), _implSalt()
        );

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create1AndSave({ _save: this, _name: "Proxy", _nick: "DataAvailabilityChallengeProxy" });

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create1({ _save: this, _name: "Proxy", _nick: "DataAvailabilityChallengeProxy" });

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create2AndSave({ _save: this, _name: "Proxy", _nick: "DataAvailabilityChallengeProxy" });

        // ruleid: sol-safety-deployutils-named-args-parameter
        DeployUtils.create2({ _save: this, _name: "Proxy", _nick: "DataAvailabilityChallengeProxy" });

        // ok: sol-safety-deployutils-named-args-parameter
        DeployUtils.create1AndSave({
            _save: this,
            _name: "Proxy",
            _nick: "DataAvailabilityChallengeProxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        });

        // ok: sol-safety-deployutils-named-args-parameter
        DeployUtils.create1({
            _name: "Proxy",
            _nick: "DataAvailabilityChallengeProxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        });

        // ok: sol-safety-deployutils-named-args-parameter
        DeployUtils.create2AndSave({
            _save: this,
            _salt: _implSalt(),
            _name: "Proxy",
            _nick: "DataAvailabilityChallengeProxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        });

        // ok: sol-safety-deployutils-named-args-parameter
        DeployUtils.create2({
            _salt: _implSalt(),
            _name: "Proxy",
            _nick: "DataAvailabilityChallengeProxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin)))
        });

        // ok: sol-safety-deployutils-named-args-parameter
        DeployUtils.createDeterministic({
            _name: "Proxy",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (proxyAdmin))),
            _salt: _implSalt()
        });
    }
}

contract SemgrepTest__sol_safety_expectrevert_before_ll_call {
    function test() {
        // ok: sol-safety-expectrevert-before-ll-call
        vm.expectRevert("some revert");
        (bool revertsAsExpected,) = target.call(hex"");
        assertTrue(revertsAsExpected);

        // ok: sol-safety-expectrevert-before-ll-call
        vm.expectRevert("some revert");
        (bool revertsAsExpected,) = target.delegatecall(hex"");
        assertTrue(revertsAsExpected);

        // ok: sol-safety-expectrevert-before-ll-call
        vm.expectRevert("some revert");
        target.someFunction();

        // ruleid: sol-safety-expectrevert-before-ll-call
        vm.expectRevert("some revert");
        (bool success,) = target.call(hex"");

        // ruleid: sol-safety-expectrevert-before-ll-call
        vm.expectRevert("some revert");
        (bool success,) = target.call(hex"");
        assertTrue(success);

        // ruleid: sol-safety-expectrevert-before-ll-call
        vm.expectRevert("some revert");
        (bool success,) = target.delegatecall(hex"");
        assertTrue(success);

        // ruleid: sol-safety-expectrevert-before-ll-call
        vm.expectRevert("some revert");
        target.call(hex"");

        // ruleid: sol-safety-expectrevert-before-ll-call
        vm.expectRevert("some revert");
        target.delegatecall(hex"");
    }
}

contract SemgrepTest__sol_safety_expectrevert_no_args {
    function test() {
        // ok: sol-safety-expectrevert-no-args
        vm.expectRevert("some revert");
        target.someFunction();

        // ruleid: sol-safety-expectrevert-no-args
        vm.expectRevert();
        target.someFunction();
    }
}

contract SemgrepTest__sol_safety_natspec_semver_match {
    // ok: sol-safety-natspec-semver-match
    /// @custom:semver 2.8.1-beta.4
    string public constant version = "2.8.1-beta.4";

    // ok: sol-safety-natspec-semver-match
    /// @custom:semver 2.8.1-beta.3
    function version() public pure virtual returns (string memory) {
        return "2.8.1-beta.3";
    }

    // ok: sol-safety-natspec-semver-match
    /// @custom:semver +interop-beta.1
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.1");
    }

    // ruleid: sol-safety-natspec-semver-match
    /// @custom:semver 2.8.1-beta.5
    string public constant version = "2.8.1-beta.4";

    // ruleid: sol-safety-natspec-semver-match
    /// @custom:semver 2.8.1-beta.4
    function version() public pure virtual returns (string memory) {
        return "2.8.1-beta.3";
    }

    // ruleid: sol-safety-natspec-semver-match
    /// @custom:semver +interop-beta.2
    function version() public pure override returns (string memory) {
        return string.concat(super.version(), "+interop-beta.1");
    }
}

library SemgrepTest__sol_safety_no_public_in_libraries {
    // ok: sol-safety-no-public-in-libraries
    function test() internal {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test() private {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test(uint256 _value, address _addr) internal {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test(uint256 _value, address _addr) private {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test() internal pure returns (uint256) {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test() private pure returns (uint256) {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test() internal view returns (uint256, address) {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test() private view returns (uint256, address) {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test() internal returns (uint256 amount_, bool success_) {
        // ...
    }

    // ok: sol-safety-no-public-in-libraries
    function test() private returns (uint256 amount_, bool success_) {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() public {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() external {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() public pure {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() external pure {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() public view {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() external view {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test(uint256 _value, address _addr) public {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test(uint256 _value, address _addr) external {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() public pure returns (uint256) {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() external pure returns (uint256) {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() public view returns (uint256, address) {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() external view returns (uint256, address) {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() public returns (uint256 amount_, bool success_) {
        // ...
    }

    // ruleid: sol-safety-no-public-in-libraries
    function test() external returns (uint256 amount_, bool success_) {
        // ...
    }
}

contract SemgrepTest__sol_style_input_arg_fmt {
    // ok: sol-style-input-arg-fmt
    event Test(address indexed src, address indexed guy, uint256 wad);

    // ok: sol-style-input-arg-fmt
    function test() public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(address payable) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(uint256 _a, uint256 _b) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(uint256 _a, uint256 _b) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(bytes memory _a) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(bytes memory _a, uint256 _b) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(Contract.Struct memory _a) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(uint256 _b, bytes memory) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(bytes memory, uint256 _b) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test(bytes memory) public {
        // ...
    }

    // ok: sol-style-input-arg-fmt
    function test() public returns (bytes memory b_) {
        // ...
    }

    // ruleid: sol-style-input-arg-fmt
    function test(uint256 a) public {
        // ...
    }

    // ruleid: sol-style-input-arg-fmt
    function test(uint256 a, uint256 b) public {
        // ...
    }

    // ruleid: sol-style-input-arg-fmt
    function test(bytes memory a) public {
        // ...
    }

    // ruleid: sol-style-input-arg-fmt
    function testg(bytes memory a, uint256 b) public {
        // ...
    }

    // ruleid: sol-style-input-arg-fmt
    function test(uint256 b, bytes memory a) public {
        // ...
    }

    // ruleid: sol-style-input-arg-fmt
    function test(Contract.Struct memory a) public {
        // ...
    }

    // ruleid: sol-style-input-arg-fmt
    function test(uint256 _a, uint256 b) public {
        // ...
    }

    // ruleid: sol-style-input-arg-fmt
    function test(uint256 a, uint256 _b) public {
        // ...
    }
}

contract SemgrepTest__sol_style_return_arg_fmt {
    // ok: sol-style-return-arg-fmt
    function test() returns (uint256 a_) {
        // ...
    }

    // ok: sol-style-return-arg-fmt
    function test() returns (address payable) {
        // ...
    }

    // ok: sol-style-return-arg-fmt
    function test() returns (uint256 a_, bytes memory b_) {
        // ...
    }

    // ok: sol-style-return-arg-fmt
    function test() returns (Contract.Struct memory ab_) {
        // ...
    }

    // ok: sol-style-return-arg-fmt
    function test() returns (uint256, bool) {
        // ...
    }

    // ok: sol-style-return-arg-fmt
    function test() returns (uint256) {
        // ...
    }

    // ruleid: sol-style-return-arg-fmt
    function test() returns (uint256 a) {
        // ...
    }

    // ruleid: sol-style-return-arg-fmt
    function test() returns (uint256 a, bytes memory b) {
        // ...
    }

    // ruleid: sol-style-return-arg-fmt
    function test() returns (Contract.Struct memory b) {
        // ...
    }

    // ruleid: sol-style-return-arg-fmt
    function test() returns (Contract.Struct memory b, bool xyz) {
        // ...
    }
}

contract SemgrepTest__sol_style_doc_comment {
    function test() {
        // ok: sol-style-doc-comment
        /// Good comment

        // ok: sol-style-doc-comment
        /// Multiline
        /// Good
        /// comment
        /// @notice with natspec

        // ruleid: sol-style-doc-comment
        /**
         * Example bad comment
         */

        // ruleid: sol-style-doc-comment
        /**
         * Example
         * bad
         * Multiline
         * comment
         */

        // ruleid: sol-style-doc-comment
        /**
         * Example
         * bad
         * Multiline
         * comment
         * @notice with natspec
         */
    }
}

contract SemgrepTest__sol_style_malformed_require {
    function test() {
        // ok: sol-style-malformed-require
        require(cond, "MyContract: test message good");

        // ok: sol-style-malformed-require
        require(cond, "MyContract: test message good");

        // ok: sol-style-malformed-require
        require(!LibString.eq(_standardVersionsToml, ""), "DeployImplementationsInput: not set");

        // ok: sol-style-malformed-require
        require(cond, "MyContract: Test message");

        // ok: sol-style-malformed-require
        require(cond, "L1SB-10");

        // ok: sol-style-malformed-require
        require(cond, "CHECK-L2OO-140");

        // ok: sol-style-malformed-require
        require(bytes(env_).length > 0, "Config: must set DEPLOY_CONFIG_PATH to filesystem path of deploy config");

        // ok: sol-style-malformed-require
        require(false, string.concat("DeployConfig: cannot find deploy config file at ", _path));

        // ok: sol-style-malformed-require
        require(
            _addrs[i] != _addrs[j],
            string.concat(
                "DeployUtils: check failed, duplicates at ", LibString.toString(i), ",", LibString.toString(j)
            )
        );

        // ruleid: sol-style-malformed-require
        require(cond, "MyContract: ");

        // ruleid: sol-style-malformed-require
        require(cond, "test");
    }
}

contract SemgrepTest__sol_style_malformed_revert {
    function test() {
        // ok: sol-style-malformed-revert
        revert("MyContract: test message good");

        // ok: sol-style-malformed-revert
        revert("MyContract: test message good");

        // ok: sol-style-malformed-revert
        revert("DeployImplementationsInput: not set");

        // ok: sol-style-malformed-revert
        revert("MyContract: Test message");

        // ok: sol-style-malformed-revert
        revert("L1SB-10");

        // ok: sol-style-malformed-revert
        revert("CHECK-L2OO-140");

        // ok: sol-style-malformed-revert
        revert();

        // ok: sol-style-malformed-revert
        revert("Config: must set DEPLOY_CONFIG_PATH to filesystem path of deploy config");

        // ok: sol-style-malformed-revert
        revert(string.concat("DeployConfig: cannot find deploy config file at ", _path));

        // ok: sol-style-malformed-revert
        revert(
            string.concat(
                "DeployUtils: check failed, duplicates at ", LibString.toString(i), ",", LibString.toString(j)
            )
        );

        // ok: sol-style-malformed-revert
        revert(string.concat("StandardValidatorV180: ", _errors));

        // ruleid: sol-style-malformed-revert
        revert("MyContract: ");

        // ruleid: sol-style-malformed-revert
        revert("test");
    }
}

contract SemgrepTest__sol_style_enforce_require_msg {
    function test() {
        // ok: sol-style-enforce-require-msg
        require(cond, "MyContract: test message good");

        // ruleid: sol-style-enforce-require-msg
        require(cond);
    }
}

contract SemgrepTest__sol_safety_try_catch_eip_150 {
    function test() {
        // ok: sol-safety-trycatch-eip150
        // eip150-safe
        try someContract.someFunction() {
            // ...
        } catch {
            // ...
        }

        // ruleid: sol-safety-trycatch-eip150
        try someContract.someFunction() {
            // ...
        } catch {
            // ...
        }
    }
}
