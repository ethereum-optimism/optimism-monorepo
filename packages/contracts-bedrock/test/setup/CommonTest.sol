// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Forge
import { Test } from "forge-std/Test.sol";

// Testing
import { Setup } from "test/setup/Setup.sol";
import { Events } from "test/setup/Events.sol";
import { FFIInterface } from "test/setup/FFIInterface.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Contracts
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// Libraries
import { console } from "forge-std/console.sol";

// Interfaces
import { IOptimismMintableERC20Full } from "interfaces/universal/IOptimismMintableERC20Full.sol";
import { ILegacyMintableERC20Full } from "interfaces/legacy/ILegacyMintableERC20Full.sol";

/// @title CommonTest
/// @dev An extenstion to `Test` that sets up the optimism smart contracts.
contract CommonTest is Test, Setup, Events {
    address alice;
    address bob;

    bytes32 constant nonZeroHash = keccak256(abi.encode("NON_ZERO"));

    FFIInterface constant ffi = FFIInterface(address(uint160(uint256(keccak256(abi.encode("optimism.ffi"))))));

    bool useAltDAOverride;
    bool useInteropOverride;

    /// @dev This value is only used in forked tests. During forked tests, the default is to perform the upgrade before
    ///      running the tests.
    ///      This value should only be set to false in forked tests which are specifically testing the upgrade path
    ///      itself, rather than simply ensuring that the tests pass after the upgrade.
    bool useUpgradedFork = true;

    ERC20 L1Token;
    ERC20 BadL1Token;
    IOptimismMintableERC20Full L2Token;
    ILegacyMintableERC20Full LegacyL2Token;
    ERC20 NativeL2Token;
    IOptimismMintableERC20Full RemoteL1Token;

    function setUp() public virtual override {
        // Setup.setup() may switch the tests over to a newly forked network. Therefore
        // state modifying cheatcodes must be run after Setup.setup(), otherwise the
        // changes will not be persisted into the new network.
        Setup.setUp();

        alice = makeAddr("alice");
        bob = makeAddr("bob");
        vm.deal(alice, 10000 ether);
        vm.deal(bob, 10000 ether);

        // Override the config after the deploy script initialized the config
        if (useAltDAOverride) {
            deploy.cfg().setUseAltDA(true);
        }
        if (useInteropOverride) {
            deploy.cfg().setUseInterop(true);
        }
        if (useUpgradedFork) {
            deploy.cfg().setUseUpgradedFork(true);
        }

        if (isForkTest()) {
            // Skip any test suite which uses a nonstandard configuration.
            if (useAltDAOverride || useInteropOverride) {
                vm.skip(true);
            }
        } else {
            // Modifying these values on a fork test causes issues.
            vm.warp(deploy.cfg().l2OutputOracleStartingTimestamp() + 1);
            vm.roll(deploy.cfg().l2OutputOracleStartingBlockNumber() + 1);
            vm.fee(1 gwei);
        }

        vm.etch(address(ffi), vm.getDeployedCode("FFIInterface.sol:FFIInterface"));
        vm.allowCheatcodes(address(ffi));
        vm.label(address(ffi), "FFIInterface");

        // Exclude contracts for the invariant tests
        excludeContract(address(ffi));
        excludeContract(address(deploy));
        excludeContract(address(deploy.cfg()));

        // Deploy L1
        Setup.L1();
        // Deploy L2
        Setup.L2();

        // Call bridge initializer setup function
        bridgeInitializerSetUp();
    }

    function bridgeInitializerSetUp() public {
        L1Token = new ERC20("Native L1 Token", "L1T");

        LegacyL2Token = ILegacyMintableERC20Full(
            DeployUtils.create1({
                _name: "LegacyMintableERC20",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(
                        ILegacyMintableERC20Full.__constructor__,
                        (
                            address(l2StandardBridge),
                            address(L1Token),
                            string.concat("LegacyL2-", L1Token.name()),
                            string.concat("LegacyL2-", L1Token.symbol())
                        )
                    )
                )
            })
        );
        vm.label(address(LegacyL2Token), "LegacyMintableERC20");

        if (isForkTest()) {
            console.log("CommonTest: fork test detected, skipping L2 setup");
            L2Token = IOptimismMintableERC20Full(makeAddr("L2Token"));
        } else {
            // Deploy the L2 ERC20 now
            L2Token = IOptimismMintableERC20Full(
                l2OptimismMintableERC20Factory.createStandardL2Token(
                    address(L1Token),
                    string(abi.encodePacked("L2-", L1Token.name())),
                    string(abi.encodePacked("L2-", L1Token.symbol()))
                )
            );
        }

        NativeL2Token = new ERC20("Native L2 Token", "L2T");

        RemoteL1Token = IOptimismMintableERC20Full(
            l1OptimismMintableERC20Factory.createStandardL2Token(
                address(NativeL2Token),
                string(abi.encodePacked("L1-", NativeL2Token.name())),
                string(abi.encodePacked("L1-", NativeL2Token.symbol()))
            )
        );

        BadL1Token = ERC20(
            l1OptimismMintableERC20Factory.createStandardL2Token(
                address(1),
                string(abi.encodePacked("L1-", NativeL2Token.name())),
                string(abi.encodePacked("L1-", NativeL2Token.symbol()))
            )
        );

        console.log("CommonTest: SetUp complete!");
    }

    /// @dev Helper function that wraps `TransactionDeposited` event.
    ///      The magic `0` is the version.
    function emitTransactionDeposited(
        address _from,
        address _to,
        uint256 _mint,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        internal
    {
        emit TransactionDeposited(_from, _to, 0, abi.encodePacked(_mint, _value, _gasLimit, _isCreation, _data));
    }

    /// @dev Checks if the system has already been deployed, based off of the heuristic that alice and bob have not been
    ///      set by the `setUp` function yet.
    function _checkNotDeployed(string memory _feature) internal view {
        if (alice != address(0) && bob != address(0)) {
            revert(
                string.concat("CommonTest: Cannot enable ", _feature, " after deployment. Consider overriding `setUp`.")
            );
        }
        console.log("CommonTest: enabling", _feature);
    }

    /// @dev Enables alternative data availability mode for testing
    function enableAltDA() public {
        _checkNotDeployed("altda");
        useAltDAOverride = true;
    }

    /// @dev Enables interoperability mode for testing
    function enableInterop() public {
        _checkNotDeployed("interop");
        useInteropOverride = true;
    }

    /// @dev Disables upgrade mode for testing. By default the fork testing env will be upgraded to the latest
    ///      implementation. This can be used to disable the upgrade which, is useful for tests targeting the upgrade
    ///      process itself.
    function disableUpgradedFork() public {
        _checkNotDeployed("non-upgraded fork");

        useUpgradedFork = false;
    }
}
