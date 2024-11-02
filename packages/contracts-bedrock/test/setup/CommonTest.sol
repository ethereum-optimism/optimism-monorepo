// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { Setup } from "test/setup/Setup.sol";
import { Events } from "test/setup/Events.sol";
import { FFIInterface } from "test/setup/FFIInterface.sol";
import { Constants } from "src/libraries/Constants.sol";
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { OptimismMintableERC20 } from "src/universal/OptimismMintableERC20.sol";
import { LegacyMintableERC20 } from "src/legacy/LegacyMintableERC20.sol";

/// @title CommonTest
/// @dev An extenstion to `Test` that sets up the optimism smart contracts.
contract CommonTest is Test, Setup, Events {
    address alice;
    address bob;

    bytes32 constant nonZeroHash = keccak256(abi.encode("NON_ZERO"));

    FFIInterface constant ffi = FFIInterface(address(uint160(uint256(keccak256(abi.encode("optimism.ffi"))))));

    bool useAltDAOverride;
    bool useLegacyContracts;
    address customGasToken;
    bool useInteropOverride;

    ERC20 L1Token;
    ERC20 BadL1Token;
    OptimismMintableERC20 L2Token;
    LegacyMintableERC20 LegacyL2Token;
    ERC20 NativeL2Token;
    ERC20 BadL2Token;
    OptimismMintableERC20 RemoteL1Token;

    function setUp() public virtual override {
        alice = makeAddr("alice");
        bob = makeAddr("bob");
        vm.deal(alice, 10000 ether);
        vm.deal(bob, 10000 ether);

        Setup.setUp();

        // Override the config after the deploy script initialized the config
        if (useAltDAOverride) {
            deploy.cfg().setUseAltDA(true);
        }
        // We default to fault proofs unless explicitly disabled by useLegacyContracts
        if (!useLegacyContracts) {
            deploy.cfg().setUseFaultProofs(true);
        }
        if (customGasToken != address(0)) {
            deploy.cfg().setUseCustomGasToken(customGasToken);
        }
        if (useInteropOverride) {
            deploy.cfg().setUseInterop(true);
        }

        vm.etch(address(ffi), vm.getDeployedCode("FFIInterface.sol:FFIInterface"));
        vm.label(address(ffi), "FFIInterface");

        // Exclude contracts for the invariant tests
        excludeContract(address(ffi));
        excludeContract(address(deploy));
        excludeContract(address(deploy.cfg()));

        // Make sure the base fee is non zero
        vm.fee(1 gwei);

        // Set sane initialize block numbers
        vm.warp(deploy.cfg().l2OutputOracleStartingTimestamp() + 1);
        vm.roll(deploy.cfg().l2OutputOracleStartingBlockNumber() + 1);

        // Deploy L1
        Setup.L1();
        // Deploy L2
        Setup.L2();

        // Call bridge initializer setup function
        bridgeInitializerSetUp();
    }

    function bridgeInitializerSetUp() public {
        L1Token = new ERC20("Native L1 Token", "L1T");

        LegacyL2Token = new LegacyMintableERC20({
            _l2Bridge: address(l2StandardBridge),
            _l1Token: address(L1Token),
            _name: string.concat("LegacyL2-", L1Token.name()),
            _symbol: string.concat("LegacyL2-", L1Token.symbol())
        });
        vm.label(address(LegacyL2Token), "LegacyMintableERC20");

        // Deploy the L2 ERC20 now
        L2Token = OptimismMintableERC20(
            l2OptimismMintableERC20Factory.createStandardL2Token(
                address(L1Token),
                string(abi.encodePacked("L2-", L1Token.name())),
                string(abi.encodePacked("L2-", L1Token.symbol()))
            )
        );

        BadL2Token = OptimismMintableERC20(
            l2OptimismMintableERC20Factory.createStandardL2Token(
                address(1),
                string(abi.encodePacked("L2-", L1Token.name())),
                string(abi.encodePacked("L2-", L1Token.symbol()))
            )
        );

        NativeL2Token = new ERC20("Native L2 Token", "L2T");

        RemoteL1Token = OptimismMintableERC20(
            l1OptimismMintableERC20Factory.createStandardL2Token(
                address(NativeL2Token),
                string(abi.encodePacked("L1-", NativeL2Token.name())),
                string(abi.encodePacked("L1-", NativeL2Token.symbol()))
            )
        );

        BadL1Token = OptimismMintableERC20(
            l1OptimismMintableERC20Factory.createStandardL2Token(
                address(1),
                string(abi.encodePacked("L1-", NativeL2Token.name())),
                string(abi.encodePacked("L1-", NativeL2Token.symbol()))
            )
        );
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

    // @dev Advance the evm's time to meet the L2OutputOracle's requirements for proposeL2Output
    function warpToProposeTime(uint256 _nextBlockNumber) public {
        vm.warp(l2OutputOracle.computeL2Timestamp(_nextBlockNumber) + 1);
    }

    /// @dev Helper function to propose an output.
    function proposeAnotherOutput() public {
        bytes32 proposedOutput2 = keccak256(abi.encode());
        uint256 nextBlockNumber = l2OutputOracle.nextBlockNumber();
        uint256 nextOutputIndex = l2OutputOracle.nextOutputIndex();
        warpToProposeTime(nextBlockNumber);
        uint256 proposedNumber = l2OutputOracle.latestBlockNumber();

        uint256 submissionInterval = deploy.cfg().l2OutputOracleSubmissionInterval();
        // Ensure the submissionInterval is enforced
        assertEq(nextBlockNumber, proposedNumber + submissionInterval);

        vm.roll(nextBlockNumber + 1);

        vm.expectEmit(true, true, true, true);
        emit OutputProposed(proposedOutput2, nextOutputIndex, nextBlockNumber, block.timestamp);

        address proposer = deploy.cfg().l2OutputOracleProposer();
        vm.prank(proposer);
        l2OutputOracle.proposeL2Output(proposedOutput2, nextBlockNumber, 0, 0);
    }

    function enableLegacyContracts() public {
        // Check if the system has already been deployed, based off of the heuristic that alice and bob have not been
        // set by the `setUp` function yet.
        if (!(alice == address(0) && bob == address(0))) {
            revert("CommonTest: Cannot enable fault proofs after deployment. Consider overriding `setUp`.");
        }

        useLegacyContracts = true;
    }

    function enableAltDA() public {
        // Check if the system has already been deployed, based off of the heuristic that alice and bob have not been
        // set by the `setUp` function yet.
        if (!(alice == address(0) && bob == address(0))) {
            revert("CommonTest: Cannot enable altda after deployment. Consider overriding `setUp`.");
        }

        useAltDAOverride = true;
    }

    function enableCustomGasToken(address _token) public {
        // Check if the system has already been deployed, based off of the heuristic that alice and bob have not been
        // set by the `setUp` function yet.
        if (!(alice == address(0) && bob == address(0))) {
            revert("CommonTest: Cannot enable custom gas token after deployment. Consider overriding `setUp`.");
        }
        require(_token != Constants.ETHER);

        customGasToken = _token;
    }

    function enableInterop() public {
        // Check if the system has already been deployed, based off of the heuristic that alice and bob have not been
        // set by the `setUp` function yet.
        if (!(alice == address(0) && bob == address(0))) {
            revert("CommonTest: Cannot enable interop after deployment. Consider overriding `setUp`.");
        }

        useInteropOverride = true;
    }
}
