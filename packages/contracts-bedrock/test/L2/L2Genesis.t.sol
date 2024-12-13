// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { L2Genesis, L1Dependencies } from "scripts/L2Genesis.s.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Process } from "scripts/libraries/Process.sol";

/// @title L2GenesisTest
/// @notice Test suite for L2Genesis script.
contract L2GenesisTest is Test {
    L2Genesis genesis;

    function setUp() public {
        genesis = new L2Genesis();
        // Note: to customize L1 addresses,
        // simply pass in the L1 addresses argument for Genesis setup functions that depend on it.
        // L1 addresses, or L1 artifacts, are not stored globally.
        genesis.setUp();
    }

    /// @notice Creates a temp file and returns the path to it.
    function tmpfile() internal returns (string memory) {
        return Process.bash("mktemp");
    }

    /// @notice Deletes a file at a given filesystem path. Does not force delete
    ///         and does not recursively delete.
    function deleteFile(string memory path) internal {
        Process.bash(string.concat("rm ", path), true);
    }

    /// @notice Returns the number of top level keys in a JSON object at a given
    ///         file path.
    function getJSONKeyCount(string memory path) internal returns (uint256) {
        bytes memory result =
            bytes(Process.bash(string.concat("jq 'keys | length' < ", path, " | xargs cast abi-encode 'f(uint256)'")));
        return abi.decode(result, (uint256));
    }

    /// @notice Helper function to run a function with a temporary dump file.
    function withTempDump(function (string memory) internal f) internal {
        string memory path = tmpfile();
        f(path);
        deleteFile(path);
    }

    /// @notice Helper function for reading the number of storage keys for a given account.
    function getStorageKeysCount(string memory _path, address _addr) internal returns (uint256) {
        return vm.parseUint(
            Process.bash(
                string.concat("jq -r '.[\"", vm.toLowercase(vm.toString(_addr)), "\"].storage | length' < ", _path)
            )
        );
    }

    /// @notice Returns the number of accounts that contain particular code at a given path to a genesis file.
    function getCodeCount(string memory path, string memory name) internal returns (uint256) {
        bytes memory code = vm.getDeployedCode(name);
        bytes memory result = bytes(
            Process.bash(
                string.concat(
                    "jq -r 'map_values(select(.code == \"",
                    vm.toString(code),
                    "\")) | length' < ",
                    path,
                    " | xargs cast abi-encode 'f(uint256)'"
                )
            )
        );
        return abi.decode(result, (uint256));
    }

    /// @notice Returns the number of accounts that have a particular slot set.
    function getPredeployCountWithSlotSet(string memory path, bytes32 slot) internal returns (uint256) {
        bytes memory result = bytes(
            Process.bash(
                string.concat(
                    "jq 'map_values(.storage | select(has(\"",
                    vm.toString(slot),
                    "\"))) | keys | length' < ",
                    path,
                    " | xargs cast abi-encode 'f(uint256)'"
                )
            )
        );
        return abi.decode(result, (uint256));
    }

    /// @notice Returns the number of accounts that have a particular slot set to a particular value.
    function getPredeployCountWithSlotSetToValue(
        string memory path,
        bytes32 slot,
        bytes32 value
    )
        internal
        returns (uint256)
    {
        bytes memory result = bytes(
            Process.bash(
                string.concat(
                    "jq 'map_values(.storage | select(.\"",
                    vm.toString(slot),
                    "\" == \"",
                    vm.toString(value),
                    "\")) | length' < ",
                    path,
                    " | xargs cast abi-encode 'f(uint256)'"
                )
            )
        );
        return abi.decode(result, (uint256));
    }

    /// @notice Tests the genesis predeploys setup using a temp file for the case where useInterop is false.
    function test_genesisPredeploys_notUsingInterop_works() external {
        string memory path = tmpfile();
        _test_genesis_predeploys(path, false);
        deleteFile(path);
    }

    /// @notice Tests the genesis predeploys setup using a temp file for the case where useInterop is true.
    function test_genesisPredeploys_usingInterop_works() external {
        string memory path = tmpfile();
        _test_genesis_predeploys(path, true);
        deleteFile(path);
    }

    /// @notice Tests the genesis predeploys setup.
    function _test_genesis_predeploys(string memory _path, bool _useInterop) internal {
        // Set the useInterop value
        vm.mockCall(address(genesis.cfg()), abi.encodeCall(genesis.cfg().useInterop, ()), abi.encode(_useInterop));

        // Set the predeploy proxies into state
        genesis.setPredeployProxies();
        genesis.writeGenesisAllocs(_path);

        // 2 predeploys do not have proxies
        assertEq(getCodeCount(_path, "Proxy.sol:Proxy"), Predeploys.PREDEPLOY_COUNT - 2);

        // 24 proxies have the implementation set if useInterop is true and 17 if useInterop is false
        assertEq(getPredeployCountWithSlotSet(_path, Constants.PROXY_IMPLEMENTATION_ADDRESS), _useInterop ? 24 : 17);

        // All proxies except 2 have the proxy 1967 admin slot set to the proxy admin
        assertEq(
            getPredeployCountWithSlotSetToValue(
                _path, Constants.PROXY_OWNER_ADDRESS, bytes32(uint256(uint160(Predeploys.PROXY_ADMIN)))
            ),
            Predeploys.PREDEPLOY_COUNT - 2
        );

        // Also see Predeploys.t.test_predeploysSet_succeeds which uses L1Genesis for the CommonTest prestate.
    }

    /// @notice Tests the number of accounts in the genesis setup
    function test_allocs_size_works() external {
        withTempDump(_test_allocs_size);
    }

    /// @notice Creates mock L1Dependencies for testing purposes.
    function _dummyL1Deps() internal pure returns (L1Dependencies memory deps_) {
        return L1Dependencies({
            l1CrossDomainMessengerProxy: payable(address(0x100000)),
            l1StandardBridgeProxy: payable(address(0x100001)),
            l1ERC721BridgeProxy: payable(address(0x100002))
        });
    }

    /// @notice Tests the number of accounts in the genesis setup
    function _test_allocs_size(string memory _path) internal {
        genesis.cfg().setFundDevAccounts(false);
        genesis.runWithLatestLocal(_dummyL1Deps());
        genesis.writeGenesisAllocs(_path);

        uint256 expected = 0;
        expected += 2048 - 2; // predeploy proxies
        expected += 21; // predeploy implementations (excl. legacy erc20-style eth and legacy message sender)
        expected += 256; // precompiles
        expected += 13; // preinstalls
        expected += 1; // 4788 deployer account
        // 16 prefunded dev accounts are excluded
        assertEq(expected, getJSONKeyCount(_path), "key count check");

        // 3 slots: implementation, owner, admin
        assertEq(3, getStorageKeysCount(_path, Predeploys.PROXY_ADMIN), "proxy admin storage check");
    }
}
