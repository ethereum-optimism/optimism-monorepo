// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { stdError } from "forge-std/Test.sol";
import { VmSafe } from "forge-std/Vm.sol";
import { MockERC20 } from "solmate/test/utils/mocks/MockERC20.sol";
import { CommonTest } from "test/setup/CommonTest.sol";
import { NextImpl } from "test/mocks/NextImpl.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Contracts
import { SuperchainConfig } from "src/L1/SuperchainConfig.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { GasPayingToken } from "src/libraries/GasPayingToken.sol";
import { AddressAliasHelper } from "src/vendor/AddressAliasHelper.sol";
import "src/dispute/lib/Types.sol";
import "src/libraries/PortalErrors.sol";

// Interfaces
import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";
import { IL1Block } from "src/L2/interfaces/IL1Block.sol";
import { IOptimismPortal2 } from "src/L1/interfaces/IOptimismPortal2.sol";
import { IDisputeGame } from "src/dispute/interfaces/IDisputeGame.sol";
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";
import { IProxy } from "src/universal/interfaces/IProxy.sol";

contract OptimismPortal2_Test is CommonTest {
    address depositor;

    function setUp() public virtual override {
        super.setUp();

        // zero out contracts that should not be used
        assembly {
            sstore(l2OutputOracle.slot, 0)
            sstore(optimismPortal.slot, 0)
        }

        depositor = makeAddr("depositor");
    }

    /// @dev Tests that the constructor sets the correct values.
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function test_constructor_succeeds() external virtual {
        IOptimismPortal2 opImpl = IOptimismPortal2(payable(deploy.mustGetAddress("OptimismPortal2")));
        assertEq(address(opImpl.disputeGameFactory()), address(0));
        assertEq(address(opImpl.systemConfig()), address(0));
        assertEq(address(opImpl.superchainConfig()), address(0));
        assertEq(opImpl.l2Sender(), Constants.DEFAULT_L2_SENDER);
        assertEq(opImpl.respectedGameType().raw(), deploy.cfg().respectedGameType());
    }

    /// @dev Tests that the initializer sets the correct values.
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function test_initialize_succeeds() external virtual {
        address guardian = deploy.cfg().superchainConfigGuardian();
        assertEq(address(optimismPortal2.disputeGameFactory()), address(disputeGameFactory));
        assertEq(address(optimismPortal2.systemConfig()), address(systemConfig));
        assertEq(optimismPortal2.guardian(), guardian);
        assertEq(address(optimismPortal2.superchainConfig()), address(superchainConfig));
        assertEq(optimismPortal2.l2Sender(), Constants.DEFAULT_L2_SENDER);
        assertEq(optimismPortal2.paused(), false);
        assertEq(optimismPortal2.respectedGameType().raw(), deploy.cfg().respectedGameType());
    }

    /// @dev Tests that `pause` successfully pauses
    ///      when called by the GUARDIAN.
    function test_pause_succeeds() external {
        address guardian = optimismPortal2.guardian();

        assertEq(optimismPortal2.paused(), false);

        vm.expectEmit(address(superchainConfig));
        emit Paused("identifier");

        vm.prank(guardian);
        superchainConfig.pause("identifier");

        assertEq(optimismPortal2.paused(), true);
    }

    /// @dev Tests that `pause` reverts when called by a non-GUARDIAN.
    function test_pause_onlyGuardian_reverts() external {
        assertEq(optimismPortal2.paused(), false);

        assertTrue(optimismPortal2.guardian() != alice);
        vm.expectRevert("SuperchainConfig: only guardian can pause");
        vm.prank(alice);
        superchainConfig.pause("identifier");

        assertEq(optimismPortal2.paused(), false);
    }

    /// @dev Tests that `unpause` successfully unpauses
    ///      when called by the GUARDIAN.
    function test_unpause_succeeds() external {
        address guardian = optimismPortal2.guardian();

        vm.prank(guardian);
        superchainConfig.pause("identifier");
        assertEq(optimismPortal2.paused(), true);

        vm.expectEmit(address(superchainConfig));
        emit Unpaused();
        vm.prank(guardian);
        superchainConfig.unpause();

        assertEq(optimismPortal2.paused(), false);
    }

    /// @dev Tests that `unpause` reverts when called by a non-GUARDIAN.
    function test_unpause_onlyGuardian_reverts() external {
        address guardian = optimismPortal2.guardian();

        vm.prank(guardian);
        superchainConfig.pause("identifier");
        assertEq(optimismPortal2.paused(), true);

        assertTrue(optimismPortal2.guardian() != alice);
        vm.expectRevert("SuperchainConfig: only guardian can unpause");
        vm.prank(alice);
        superchainConfig.unpause();

        assertEq(optimismPortal2.paused(), true);
    }

    /// @dev Tests that `receive` successdully deposits ETH.
    function testFuzz_receive_succeeds(uint256 _value) external {
        vm.expectEmit(address(optimismPortal2));
        emitTransactionDeposited({
            _from: alice,
            _to: alice,
            _value: _value,
            _mint: _value,
            _gasLimit: 100_000,
            _isCreation: false,
            _data: hex""
        });

        // give alice money and send as an eoa
        vm.deal(alice, _value);
        vm.prank(alice, alice);
        (bool s,) = address(optimismPortal2).call{ value: _value }(hex"");

        assertTrue(s);
        assertEq(address(optimismPortal2).balance, _value);
    }

    /// @dev Tests that `depositTransaction` reverts when the destination address is non-zero
    ///      for a contract creation deposit.
    function test_depositTransaction_contractCreation_reverts() external {
        // contract creation must have a target of address(0)
        vm.expectRevert(BadTarget.selector);
        optimismPortal2.depositTransaction(address(1), 1, 0, true, hex"");
    }

    /// @dev Tests that `depositTransaction` reverts when the data is too large.
    ///      This places an upper bound on unsafe blocks sent over p2p.
    function test_depositTransaction_largeData_reverts() external {
        uint256 size = 120_001;
        uint64 gasLimit = optimismPortal2.minimumGasLimit(uint64(size));
        vm.expectRevert(LargeCalldata.selector);
        optimismPortal2.depositTransaction({
            _to: address(0),
            _value: 0,
            _gasLimit: gasLimit,
            _isCreation: false,
            _data: new bytes(size)
        });
    }

    /// @dev Tests that `depositTransaction` reverts when the gas limit is too small.
    function test_depositTransaction_smallGasLimit_reverts() external {
        vm.expectRevert(SmallGasLimit.selector);
        optimismPortal2.depositTransaction({ _to: address(1), _value: 0, _gasLimit: 0, _isCreation: false, _data: hex"" });
    }

    /// @dev Tests that `depositTransaction` succeeds for small,
    ///      but sufficient, gas limits.
    function testFuzz_depositTransaction_smallGasLimit_succeeds(bytes memory _data, bool _shouldFail) external {
        uint64 gasLimit = optimismPortal2.minimumGasLimit(uint64(_data.length));
        if (_shouldFail) {
            gasLimit = uint64(bound(gasLimit, 0, gasLimit - 1));
            vm.expectRevert(SmallGasLimit.selector);
        }

        optimismPortal2.depositTransaction({
            _to: address(0x40),
            _value: 0,
            _gasLimit: gasLimit,
            _isCreation: false,
            _data: _data
        });
    }

    /// @dev Tests that `minimumGasLimit` succeeds for small calldata sizes.
    ///      The gas limit should be 21k for 0 calldata and increase linearly
    ///      for larger calldata sizes.
    function test_minimumGasLimit_succeeds() external view {
        assertEq(optimismPortal2.minimumGasLimit(0), 21_000);
        assertTrue(optimismPortal2.minimumGasLimit(2) > optimismPortal2.minimumGasLimit(1));
        assertTrue(optimismPortal2.minimumGasLimit(3) > optimismPortal2.minimumGasLimit(2));
    }

    /// @dev Tests that `depositTransaction` succeeds for an EOA.
    /// forge-config: ciheavy.fuzz.runs = 8192
    function testFuzz_depositTransaction_eoa_succeeds(
        address _to,
        uint64 _gasLimit,
        uint256 _value,
        uint256 _mint,
        bool _isCreation,
        bytes memory _data
    )
        external
    {
        _gasLimit = uint64(
            bound(
                _gasLimit,
                optimismPortal2.minimumGasLimit(uint64(_data.length)),
                systemConfig.resourceConfig().maxResourceLimit
            )
        );
        if (_isCreation) _to = address(0);

        // EOA emulation
        vm.expectEmit(address(optimismPortal2));
        emitTransactionDeposited({
            _from: depositor,
            _to: _to,
            _value: _value,
            _mint: _mint,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });

        vm.deal(depositor, _mint);
        vm.prank(depositor, depositor);
        optimismPortal2.depositTransaction{ value: _mint }({
            _to: _to,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });
        assertEq(address(optimismPortal2).balance, _mint);
    }

    /// @dev Tests that `depositTransaction` succeeds for a contract.
    /// forge-config: ciheavy.fuzz.runs = 8192
    function testFuzz_depositTransaction_contract_succeeds(
        address _to,
        uint64 _gasLimit,
        uint256 _value,
        uint256 _mint,
        bool _isCreation,
        bytes memory _data
    )
        external
    {
        _gasLimit = uint64(
            bound(
                _gasLimit,
                optimismPortal2.minimumGasLimit(uint64(_data.length)),
                systemConfig.resourceConfig().maxResourceLimit
            )
        );
        if (_isCreation) _to = address(0);

        vm.expectEmit(address(optimismPortal2));
        emitTransactionDeposited({
            _from: AddressAliasHelper.applyL1ToL2Alias(address(this)),
            _to: _to,
            _value: _value,
            _mint: _mint,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });

        vm.deal(address(this), _mint);
        vm.prank(address(this));
        optimismPortal2.depositTransaction{ value: _mint }({
            _to: _to,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });
        assertEq(address(optimismPortal2).balance, _mint);
    }

    /// @dev Tests that the gas paying token can be set.
    function testFuzz_setGasPayingToken_succeeds(
        address _token,
        uint8 _decimals,
        bytes32 _name,
        bytes32 _symbol
    )
        external
    {
        vm.expectEmit(address(optimismPortal2));
        emit TransactionDeposited(
            0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAd0001,
            Predeploys.L1_BLOCK_ATTRIBUTES,
            0,
            abi.encodePacked(
                uint256(0), // mint
                uint256(0), // value
                uint64(200_000), // gasLimit
                false, // isCreation,
                abi.encodeCall(IL1Block.setGasPayingToken, (_token, _decimals, _name, _symbol))
            )
        );

        vm.prank(address(systemConfig));
        optimismPortal2.setGasPayingToken({ _token: _token, _decimals: _decimals, _name: _name, _symbol: _symbol });
    }

    /// @notice Ensures that the deposit event is correct for the `setGasPayingToken`
    ///         code path that manually emits a deposit transaction outside of the
    ///         `depositTransaction` function. This is a simple differential test.
    function test_setGasPayingToken_correctEvent_succeeds(
        address _token,
        string memory _name,
        string memory _symbol
    )
        external
    {
        vm.assume(bytes(_name).length <= 32);
        vm.assume(bytes(_symbol).length <= 32);

        bytes32 name = GasPayingToken.sanitize(_name);
        bytes32 symbol = GasPayingToken.sanitize(_symbol);

        vm.recordLogs();

        vm.prank(address(systemConfig));
        optimismPortal2.setGasPayingToken({ _token: _token, _decimals: 18, _name: name, _symbol: symbol });

        vm.prank(Constants.DEPOSITOR_ACCOUNT, Constants.DEPOSITOR_ACCOUNT);
        optimismPortal2.depositTransaction({
            _to: Predeploys.L1_BLOCK_ATTRIBUTES,
            _value: 0,
            _gasLimit: 200_000,
            _isCreation: false,
            _data: abi.encodeCall(IL1Block.setGasPayingToken, (_token, 18, name, symbol))
        });

        VmSafe.Log[] memory logs = vm.getRecordedLogs();
        assertEq(logs.length, 2);

        VmSafe.Log memory systemPath = logs[0];
        VmSafe.Log memory userPath = logs[1];

        assertEq(systemPath.topics.length, 4);
        assertEq(systemPath.topics.length, userPath.topics.length);
        assertEq(systemPath.topics[0], userPath.topics[0]);
        assertEq(systemPath.topics[1], userPath.topics[1]);
        assertEq(systemPath.topics[2], userPath.topics[2]);
        assertEq(systemPath.topics[3], userPath.topics[3]);
        assertEq(systemPath.data, userPath.data);
    }

    /// @dev Tests that the gas paying token cannot be set by a non-system config.
    function test_setGasPayingToken_notSystemConfig_fails(address _caller) external {
        vm.assume(_caller != address(systemConfig));
        vm.prank(_caller);
        vm.expectRevert(Unauthorized.selector);
        optimismPortal2.setGasPayingToken({ _token: address(0), _decimals: 0, _name: "", _symbol: "" });
    }

    /// @dev Tests that `depositERC20Transaction` reverts when the gas paying token is ether.
    function test_depositERC20Transaction_noCustomGasToken_reverts() external {
        // Check that the gas paying token is set to ether
        (address token,) = systemConfig.gasPayingToken();
        assertEq(token, Constants.ETHER);

        vm.expectRevert(OnlyCustomGasToken.selector);
        optimismPortal2.depositERC20Transaction(address(0), 0, 0, 0, false, "");
    }

    function test_depositERC20Transaction_balanceOverflow_reverts() external {
        vm.mockCall(address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(42), 18));

        // The balance slot
        vm.store(address(optimismPortal2), bytes32(uint256(61)), bytes32(type(uint256).max));
        assertEq(optimismPortal2.balance(), type(uint256).max);

        vm.expectRevert(stdError.arithmeticError);
        optimismPortal2.depositERC20Transaction({
            _to: address(0),
            _mint: 1,
            _value: 1,
            _gasLimit: 10_000,
            _isCreation: false,
            _data: ""
        });
    }

    /// @dev Tests that `balance()` returns the correct balance when the gas paying token is ether.
    function testFuzz_balance_ether_succeeds(uint256 _amount) external {
        // Check that the gas paying token is set to ether
        (address token,) = systemConfig.gasPayingToken();
        assertEq(token, Constants.ETHER);

        // Increase the balance of the gas paying token
        vm.deal(address(optimismPortal2), _amount);

        // Check that the balance has been correctly updated
        assertEq(optimismPortal2.balance(), address(optimismPortal2).balance);
    }
}

contract OptimismPortal2_FinalizeWithdrawal_Test is CommonTest {
    // Reusable default values for a test withdrawal
    Types.WithdrawalTransaction _defaultTx;

    IFaultDisputeGame game;
    uint256 _proposedGameIndex;
    uint256 _proposedBlockNumber;
    bytes32 _stateRoot;
    bytes32 _storageRoot;
    bytes32 _outputRoot;
    bytes32 _withdrawalHash;
    bytes[] _withdrawalProof;
    Types.OutputRootProof internal _outputRootProof;

    // Use a constructor to set the storage vars above, so as to minimize the number of ffi calls.
    constructor() {
        super.setUp();

        _defaultTx = Types.WithdrawalTransaction({
            nonce: 0,
            sender: alice,
            target: bob,
            value: 100,
            gasLimit: 100_000,
            data: hex"aa" // includes calldata for ERC20 withdrawal test
         });
        // Get withdrawal proof data we can use for testing.
        (_stateRoot, _storageRoot, _outputRoot, _withdrawalHash, _withdrawalProof) =
            ffi.getProveWithdrawalTransactionInputs(_defaultTx);

        // Setup a dummy output root proof for reuse.
        _outputRootProof = Types.OutputRootProof({
            version: bytes32(uint256(0)),
            stateRoot: _stateRoot,
            messagePasserStorageRoot: _storageRoot,
            latestBlockhash: bytes32(uint256(0))
        });
    }

    /// @dev Setup the system for a ready-to-use state.
    function setUp() public virtual override {
        _proposedBlockNumber = 0xFF;
        game = IFaultDisputeGame(
            payable(
                address(
                    disputeGameFactory.create(
                        optimismPortal2.respectedGameType(), Claim.wrap(_outputRoot), abi.encode(_proposedBlockNumber)
                    )
                )
            )
        );
        _proposedGameIndex = disputeGameFactory.gameCount() - 1;

        // Warp beyond the chess clocks and finalize the game.
        vm.warp(block.timestamp + game.maxClockDuration().raw() + 1 seconds);

        // Fund the portal so that we can withdraw ETH.
        vm.deal(address(optimismPortal2), 0xFFFFFFFF);
    }

    /// @dev Asserts that the reentrant call will revert.
    function callPortalAndExpectRevert() external payable {
        vm.expectRevert(NonReentrant.selector);
        // Arguments here don't matter, as the require check is the first thing that happens.
        // We assume that this has already been proven.
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
        // Assert that the withdrawal was not finalized.
        assertFalse(optimismPortal2.finalizedWithdrawals(Hashing.hashWithdrawal(_defaultTx)));
    }

    /// @dev Tests that `blacklistDisputeGame` reverts when called by a non-guardian.
    function testFuzz_blacklist_onlyGuardian_reverts(address _act) external {
        vm.assume(_act != address(optimismPortal2.guardian()));

        vm.expectRevert(Unauthorized.selector);
        optimismPortal2.blacklistDisputeGame(IDisputeGame(address(0xdead)));
    }

    /// @dev Tests that the guardian role can blacklist any dispute game.
    function testFuzz_blacklist_guardian_succeeds(IDisputeGame _addr) external {
        vm.expectEmit(address(optimismPortal2));
        emit DisputeGameBlacklisted(_addr);

        vm.prank(optimismPortal2.guardian());
        optimismPortal2.blacklistDisputeGame(_addr);

        assertTrue(optimismPortal2.disputeGameBlacklist(_addr));
    }

    /// @dev Tests that `setRespectedGameType` reverts when called by a non-guardian.
    function testFuzz_setRespectedGameType_onlyGuardian_reverts(address _act, GameType _ty) external {
        vm.assume(_act != address(optimismPortal2.guardian()));

        vm.prank(_act);
        vm.expectRevert(Unauthorized.selector);
        optimismPortal2.setRespectedGameType(_ty);
    }

    /// @dev Tests that the guardian role can set the respected game type to anything they want.
    function testFuzz_setRespectedGameType_guardian_succeeds(GameType _ty) external {
        vm.expectEmit(address(optimismPortal2));
        emit RespectedGameTypeSet(_ty, Timestamp.wrap(uint64(block.timestamp)));
        vm.prank(optimismPortal2.guardian());
        optimismPortal2.setRespectedGameType(_ty);

        assertEq(optimismPortal2.respectedGameType().raw(), _ty.raw());
    }

    /// @dev Tests that `proveWithdrawalTransaction` reverts when paused.
    function test_proveWithdrawalTransaction_paused_reverts() external {
        vm.prank(optimismPortal2.guardian());
        superchainConfig.pause("identifier");

        vm.expectRevert(CallPaused.selector);
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` reverts when the target is the portal contract.
    function test_proveWithdrawalTransaction_onSelfCall_reverts() external {
        _defaultTx.target = address(optimismPortal2);
        vm.expectRevert(BadTarget.selector);
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` reverts when the outputRootProof does not match the output root
    function test_proveWithdrawalTransaction_onInvalidOutputRootProof_reverts() external {
        // Modify the version to invalidate the withdrawal proof.
        _outputRootProof.version = bytes32(uint256(1));
        vm.expectRevert(InvalidProof.selector);
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` reverts when the withdrawal is missing.
    function test_proveWithdrawalTransaction_onInvalidWithdrawalProof_reverts() external {
        // modify the default test values to invalidate the proof.
        _defaultTx.data = hex"abcd";
        vm.expectRevert("MerkleTrie: path remainder must share all nibbles with key");
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` reverts when the withdrawal has already been proven, and the new
    ///      game has the `CHALLENGER_WINS` status.
    function test_proveWithdrawalTransaction_replayProveDifferentGameChallengerWins_reverts() external {
        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Create a new dispute game, and mock both games to be CHALLENGER_WINS.
        IDisputeGame game2 = disputeGameFactory.create(
            optimismPortal2.respectedGameType(), Claim.wrap(_outputRoot), abi.encode(_proposedBlockNumber + 1)
        );
        _proposedGameIndex = disputeGameFactory.gameCount() - 1;
        vm.mockCall(address(game), abi.encodeCall(game.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));
        vm.mockCall(address(game2), abi.encodeCall(game.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));

        vm.expectRevert(InvalidDisputeGame.selector);
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` reverts if the dispute game being proven against is not of the
    ///      respected game type.
    function test_proveWithdrawalTransaction_badGameType_reverts() external {
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(disputeGameFactory.gameAtIndex, (_proposedGameIndex)),
            abi.encode(GameType.wrap(0xFF), Timestamp.wrap(uint64(block.timestamp)), IDisputeGame(address(game)))
        );

        vm.expectRevert(InvalidGameType.selector);
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` can be re-executed if the dispute game proven against has been
    ///      blacklisted.
    function test_proveWithdrawalTransaction_replayProveBlacklisted_succeeds() external {
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Blacklist the dispute dispute game.
        vm.prank(optimismPortal2.guardian());
        optimismPortal2.blacklistDisputeGame(IDisputeGame(address(game)));

        // Mock the status of the dispute game we just proved against to be CHALLENGER_WINS.
        vm.mockCall(address(game), abi.encodeCall(game.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));
        // Create a new game to re-prove against
        disputeGameFactory.create(
            optimismPortal2.respectedGameType(), Claim.wrap(_outputRoot), abi.encode(_proposedBlockNumber + 1)
        );
        _proposedGameIndex = disputeGameFactory.gameCount() - 1;

        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` can be re-executed if the dispute game proven against has resolved
    ///      against the favor of the root claim.
    function test_proveWithdrawalTransaction_replayProveBadProposal_succeeds() external {
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Mock the status of the dispute game we just proved against to be CHALLENGER_WINS.
        vm.mockCall(address(game), abi.encodeCall(game.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));
        // Create a new game to re-prove against
        disputeGameFactory.create(
            optimismPortal2.respectedGameType(), Claim.wrap(_outputRoot), abi.encode(_proposedBlockNumber + 1)
        );
        _proposedGameIndex = disputeGameFactory.gameCount() - 1;

        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` can be re-executed if the dispute game proven against is no longer
    ///      of the respected game type.
    function test_proveWithdrawalTransaction_replayRespectedGameTypeChanged_succeeds() external {
        // Prove the withdrawal against a game with the current respected game type.
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Update the respected game type to 0xbeef.
        vm.prank(optimismPortal2.guardian());
        optimismPortal2.setRespectedGameType(GameType.wrap(0xbeef));

        // Create a new game and mock the game type as 0xbeef in the factory.
        IDisputeGame newGame =
            disputeGameFactory.create(GameType.wrap(0), Claim.wrap(_outputRoot), abi.encode(_proposedBlockNumber + 1));
        vm.mockCall(
            address(disputeGameFactory),
            abi.encodeCall(disputeGameFactory.gameAtIndex, (_proposedGameIndex + 1)),
            abi.encode(GameType.wrap(0xbeef), Timestamp.wrap(uint64(block.timestamp)), IDisputeGame(address(newGame)))
        );

        // Re-proving should be successful against the new game.
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex + 1,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `proveWithdrawalTransaction` succeeds.
    function test_proveWithdrawalTransaction_validWithdrawalProof_succeeds() external {
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts when attempting to replay using a secondary proof
    ///      submitter.
    function test_finalizeWithdrawalTransaction_secondProofReplay_reverts() external {
        uint256 bobBalanceBefore = address(bob).balance;

        // Submit the first proof for the withdrawal hash.
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Submit a second proof for the same withdrawal hash.
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(0xb0b));
        vm.prank(address(0xb0b));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp and resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1 seconds);

        vm.expectEmit(true, true, false, true);
        emit WithdrawalFinalized(_withdrawalHash, true);
        optimismPortal2.finalizeWithdrawalTransactionExternalProof(_defaultTx, address(0xb0b));

        vm.expectRevert(AlreadyFinalized.selector);
        optimismPortal2.finalizeWithdrawalTransactionExternalProof(_defaultTx, address(this));

        assert(address(bob).balance == bobBalanceBefore + 100);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` succeeds.
    function test_finalizeWithdrawalTransaction_provenWithdrawalHashEther_succeeds() external {
        uint256 bobBalanceBefore = address(bob).balance;

        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp and resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1 seconds);

        vm.expectEmit(true, true, false, true);
        emit WithdrawalFinalized(_withdrawalHash, true);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        assert(address(bob).balance == bobBalanceBefore + 100);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` succeeds using a different proof than an earlier one by another
    ///      party.
    function test_finalizeWithdrawalTransaction_secondaryProof_succeeds() external {
        uint256 bobBalanceBefore = address(bob).balance;

        // Create a secondary dispute game.
        IDisputeGame secondGame = disputeGameFactory.create(
            optimismPortal2.respectedGameType(), Claim.wrap(_outputRoot), abi.encode(_proposedBlockNumber + 1)
        );

        // Warp 1 second into the future so that the proof is submitted after the timestamp of game creation.
        vm.warp(block.timestamp + 1 seconds);

        // Prove the withdrawal transaction against the invalid dispute game, as 0xb0b.
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(0xb0b));
        vm.prank(address(0xb0b));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex + 1,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Mock the status of the dispute game 0xb0b proves against to be CHALLENGER_WINS.
        vm.mockCall(address(secondGame), abi.encodeCall(game.status, ()), abi.encode(GameStatus.CHALLENGER_WINS));

        // Prove the withdrawal transaction against the invalid dispute game, as the test contract, against the original
        // game.
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp and resolve the original dispute game.
        game.resolveClaim(0, 0);
        game.resolve();
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1 seconds);

        // Ensure both proofs are registered successfully.
        assertEq(optimismPortal2.numProofSubmitters(_withdrawalHash), 2);

        vm.expectRevert(ProposalNotValidated.selector);
        vm.prank(address(0xb0b));
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        vm.expectEmit(true, true, false, true);
        emit WithdrawalFinalized(_withdrawalHash, true);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        assert(address(bob).balance == bobBalanceBefore + 100);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` succeeds.
    function test_finalizeWithdrawalTransaction_provenWithdrawalHashNonEtherTargetToken_reverts() external {
        vm.mockCall(
            address(systemConfig),
            abi.encodeCall(systemConfig.gasPayingToken, ()),
            abi.encode(address(_defaultTx.target), 18)
        );

        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp to after the finalization period
        game.resolveClaim(0, 0);
        game.resolve();
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        vm.expectRevert(BadTarget.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the contract is paused.
    function test_finalizeWithdrawalTransaction_paused_reverts() external {
        vm.prank(optimismPortal2.guardian());
        superchainConfig.pause("identifier");

        vm.expectRevert(CallPaused.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the withdrawal has not been
    function test_finalizeWithdrawalTransaction_ifWithdrawalNotProven_reverts() external {
        uint256 bobBalanceBefore = address(bob).balance;

        vm.expectRevert(Unproven.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        assert(address(bob).balance == bobBalanceBefore);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the withdrawal has not been
    ///      proven long enough ago.
    function test_finalizeWithdrawalTransaction_ifWithdrawalProofNotOldEnough_reverts() external {
        uint256 bobBalanceBefore = address(bob).balance;

        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        vm.expectRevert("OptimismPortal: proven withdrawal has not matured yet");
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        assert(address(bob).balance == bobBalanceBefore);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the provenWithdrawal's timestamp
    ///      is less than the dispute game's creation timestamp.
    function test_finalizeWithdrawalTransaction_timestampLessThanGameCreation_reverts() external {
        uint256 bobBalanceBefore = address(bob).balance;

        // Prove our withdrawal
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp to after the finalization period
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        // Mock a createdAt change in the dispute game.
        vm.mockCall(address(game), abi.encodeCall(game.createdAt, ()), abi.encode(block.timestamp + 1));

        // Attempt to finalize the withdrawal
        vm.expectRevert("OptimismPortal: withdrawal timestamp less than dispute game creation timestamp");
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        // Ensure that bob's balance has remained the same
        assertEq(bobBalanceBefore, address(bob).balance);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the dispute game has not resolved in favor of the
    ///      root claim.
    function test_finalizeWithdrawalTransaction_ifDisputeGameNotResolved_reverts() external {
        uint256 bobBalanceBefore = address(bob).balance;

        // Prove our withdrawal
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp to after the finalization period
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        // Attempt to finalize the withdrawal
        vm.expectRevert(ProposalNotValidated.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        // Ensure that bob's balance has remained the same
        assertEq(bobBalanceBefore, address(bob).balance);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the target reverts.
    function test_finalizeWithdrawalTransaction_targetFails_fails() external {
        uint256 bobBalanceBefore = address(bob).balance;
        vm.etch(bob, hex"fe"); // Contract with just the invalid opcode.

        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();

        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalFinalized(_withdrawalHash, false);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        assert(address(bob).balance == bobBalanceBefore);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the withdrawal has already been
    ///      finalized.
    function test_finalizeWithdrawalTransaction_onReplay_reverts() external {
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();

        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalFinalized(_withdrawalHash, true);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        vm.expectRevert(AlreadyFinalized.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the withdrawal transaction
    ///      does not have enough gas to execute.
    function test_finalizeWithdrawalTransaction_onInsufficientGas_reverts() external {
        // This number was identified through trial and error.
        uint256 gasLimit = 150_000;
        Types.WithdrawalTransaction memory insufficientGasTx = Types.WithdrawalTransaction({
            nonce: 0,
            sender: alice,
            target: bob,
            value: 100,
            gasLimit: gasLimit,
            data: hex""
        });

        // Get updated proof inputs.
        (bytes32 stateRoot, bytes32 storageRoot,,, bytes[] memory withdrawalProof) =
            ffi.getProveWithdrawalTransactionInputs(insufficientGasTx);
        Types.OutputRootProof memory outputRootProof = Types.OutputRootProof({
            version: bytes32(0),
            stateRoot: stateRoot,
            messagePasserStorageRoot: storageRoot,
            latestBlockhash: bytes32(0)
        });

        vm.mockCall(
            address(game), abi.encodeCall(game.rootClaim, ()), abi.encode(Hashing.hashOutputRootProof(outputRootProof))
        );

        optimismPortal2.proveWithdrawalTransaction({
            _tx: insufficientGasTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: outputRootProof,
            _withdrawalProof: withdrawalProof
        });

        // Resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();

        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);
        vm.expectRevert("SafeCall: Not enough gas");
        optimismPortal2.finalizeWithdrawalTransaction{ gas: gasLimit }(insufficientGasTx);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if a sub-call attempts to finalize
    ///      another withdrawal.
    function test_finalizeWithdrawalTransaction_onReentrancy_reverts() external {
        uint256 bobBalanceBefore = address(bob).balance;

        // Copy and modify the default test values to attempt a reentrant call by first calling to
        // this contract's callPortalAndExpectRevert() function above.
        Types.WithdrawalTransaction memory _testTx = _defaultTx;
        _testTx.target = address(this);
        _testTx.data = abi.encodeCall(this.callPortalAndExpectRevert, ());

        // Get modified proof inputs.
        (
            bytes32 stateRoot,
            bytes32 storageRoot,
            bytes32 outputRoot,
            bytes32 withdrawalHash,
            bytes[] memory withdrawalProof
        ) = ffi.getProveWithdrawalTransactionInputs(_testTx);
        Types.OutputRootProof memory outputRootProof = Types.OutputRootProof({
            version: bytes32(0),
            stateRoot: stateRoot,
            messagePasserStorageRoot: storageRoot,
            latestBlockhash: bytes32(0)
        });

        // Return a mock output root from the game.
        vm.mockCall(address(game), abi.encodeCall(game.rootClaim, ()), abi.encode(outputRoot));

        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(withdrawalHash, alice, address(this));
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction(_testTx, _proposedGameIndex, outputRootProof, withdrawalProof);

        // Resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();

        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);
        vm.expectCall(address(this), _testTx.data);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalFinalized(withdrawalHash, true);
        optimismPortal2.finalizeWithdrawalTransaction(_testTx);

        // Ensure that bob's balance was not changed by the reentrant call.
        assert(address(bob).balance == bobBalanceBefore);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` succeeds.
    /// forge-config: ciheavy.fuzz.runs = 8192
    function testDiff_finalizeWithdrawalTransaction_succeeds(
        address _sender,
        address _target,
        uint256 _value,
        uint256 _gasLimit,
        bytes memory _data
    )
        external
    {
        vm.assume(
            _target != address(optimismPortal2) // Cannot call the optimism portal or a contract
                && _target.code.length == 0 // No accounts with code
                && _target != CONSOLE // The console has no code but behaves like a contract
                && uint160(_target) > 9 // No precompiles (or zero address)
        );

        // Total ETH supply is currently about 120M ETH.
        uint256 value = bound(_value, 0, 200_000_000 ether);
        vm.deal(address(optimismPortal2), value);

        uint256 gasLimit = bound(_gasLimit, 0, 50_000_000);
        uint256 nonce = l2ToL1MessagePasser.messageNonce();

        // Get a withdrawal transaction and mock proof from the differential testing script.
        Types.WithdrawalTransaction memory _tx = Types.WithdrawalTransaction({
            nonce: nonce,
            sender: _sender,
            target: _target,
            value: value,
            gasLimit: gasLimit,
            data: _data
        });
        (
            bytes32 stateRoot,
            bytes32 storageRoot,
            bytes32 outputRoot,
            bytes32 withdrawalHash,
            bytes[] memory withdrawalProof
        ) = ffi.getProveWithdrawalTransactionInputs(_tx);

        // Create the output root proof
        Types.OutputRootProof memory proof = Types.OutputRootProof({
            version: bytes32(uint256(0)),
            stateRoot: stateRoot,
            messagePasserStorageRoot: storageRoot,
            latestBlockhash: bytes32(uint256(0))
        });

        // Ensure the values returned from ffi are correct
        assertEq(outputRoot, Hashing.hashOutputRootProof(proof));
        assertEq(withdrawalHash, Hashing.hashWithdrawal(_tx));

        // Setup the dispute game to return the output root
        vm.mockCall(address(game), abi.encodeCall(game.rootClaim, ()), abi.encode(outputRoot));

        // Prove the withdrawal transaction
        optimismPortal2.proveWithdrawalTransaction(_tx, _proposedGameIndex, proof, withdrawalProof);
        (IDisputeGame _game,) = optimismPortal2.provenWithdrawals(withdrawalHash, address(this));
        assertTrue(_game.rootClaim().raw() != bytes32(0));

        // Resolve the dispute game
        game.resolveClaim(0, 0);
        game.resolve();

        // Warp past the finalization period
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        // Finalize the withdrawal transaction
        vm.expectCallMinGas(_tx.target, _tx.value, uint64(_tx.gasLimit), _tx.data);
        optimismPortal2.finalizeWithdrawalTransaction(_tx);
        assertTrue(optimismPortal2.finalizedWithdrawals(withdrawalHash));
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the withdrawal's dispute game has been blacklisted.
    function test_finalizeWithdrawalTransaction_blacklisted_reverts() external {
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();

        vm.prank(optimismPortal2.guardian());
        optimismPortal2.blacklistDisputeGame(IDisputeGame(address(game)));

        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        vm.expectRevert(Blacklisted.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the withdrawal's dispute game is still in the air
    ///      gap.
    function test_finalizeWithdrawalTransaction_gameInAirGap_reverts() external {
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp past the finalization period.
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        // Resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();

        // Attempt to finalize the withdrawal directly after the game resolves. This should fail.
        vm.expectRevert("OptimismPortal: output proposal in air-gap");
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        // Finalize the withdrawal transaction. This should succeed.
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds() + 1);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
        assertTrue(optimismPortal2.finalizedWithdrawals(_withdrawalHash));
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the respected game type has changed since the
    ///      withdrawal was proven.
    function test_finalizeWithdrawalTransaction_respectedTypeChangedSinceProving_reverts() external {
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp past the finalization period.
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        // Resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();

        // Change the respected game type in the portal.
        vm.prank(optimismPortal2.guardian());
        optimismPortal2.setRespectedGameType(GameType.wrap(0xFF));

        vm.expectRevert(InvalidGameType.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` reverts if the respected game type was updated after the
    ///      dispute game was created.
    function test_finalizeWithdrawalTransaction_gameOlderThanRespectedGameTypeUpdate_reverts() external {
        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp past the finalization period.
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        // Resolve the dispute game.
        game.resolveClaim(0, 0);
        game.resolve();

        // Change the respected game type in the portal.
        vm.prank(optimismPortal2.guardian());
        optimismPortal2.setRespectedGameType(GameType.wrap(0xFF));

        // Mock the game's type so that we pass the correct game type check.
        vm.mockCall(address(game), abi.encodeCall(game.gameType, ()), abi.encode(GameType.wrap(0xFF)));

        vm.expectRevert("OptimismPortal: dispute game created before respected game type was updated");
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
    }

    /// @dev Tests an e2e prove -> finalize path, checking the edges of each delay for correctness.
    function test_finalizeWithdrawalTransaction_delayEdges_succeeds() external {
        // Prove the withdrawal transaction.
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        vm.expectEmit(true, true, true, true);
        emit WithdrawalProvenExtension1(_withdrawalHash, address(this));
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Attempt to finalize the withdrawal transaction 1 second before the proof has matured. This should fail.
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds());
        vm.expectRevert("OptimismPortal: proven withdrawal has not matured yet");
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        // Warp 1 second in the future, past the proof maturity delay, and attempt to finalize the withdrawal.
        // This should also fail, since the dispute game has not resolved yet.
        vm.warp(block.timestamp + 1 seconds);
        vm.expectRevert(ProposalNotValidated.selector);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        // Finalize the dispute game and attempt to finalize the withdrawal again. This should also fail, since the
        // air gap dispute game delay has not elapsed.
        game.resolveClaim(0, 0);
        game.resolve();
        vm.warp(block.timestamp + optimismPortal2.disputeGameFinalityDelaySeconds());
        vm.expectRevert("OptimismPortal: output proposal in air-gap");
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        // Warp 1 second in the future, past the air gap dispute game delay, and attempt to finalize the withdrawal.
        // This should succeed.
        vm.warp(block.timestamp + 1 seconds);
        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);
        assertTrue(optimismPortal2.finalizedWithdrawals(_withdrawalHash));
    }
}

contract OptimismPortal2_Upgradeable_Test is CommonTest {
    function setUp() public override {
        super.setUp();
    }

    /// @dev Tests that the proxy is initialized correctly.
    function test_params_initValuesOnProxy_succeeds() external view {
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = optimismPortal2.params();
        IResourceMetering.ResourceConfig memory rcfg = systemConfig.resourceConfig();

        assertEq(prevBaseFee, rcfg.minimumBaseFee);
        assertEq(prevBoughtGas, 0);
        assertEq(prevBlockNum, block.number);
    }

    /// @dev Tests that the proxy can be upgraded.
    function test_upgradeToAndCall_upgrading_succeeds() external {
        // Check an unused slot before upgrading.
        bytes32 slot21Before = vm.load(address(optimismPortal2), bytes32(uint256(21)));
        assertEq(bytes32(0), slot21Before);

        NextImpl nextImpl = new NextImpl();

        vm.startPrank(EIP1967Helper.getAdmin(address(optimismPortal2)));
        // The value passed to the initialize must be larger than the last value
        // that initialize was called with.
        IProxy(payable(address(optimismPortal2))).upgradeToAndCall(
            address(nextImpl), abi.encodeCall(NextImpl.initialize, (2))
        );
        assertEq(IProxy(payable(address(optimismPortal2))).implementation(), address(nextImpl));

        // Verify that the NextImpl contract initialized its values according as expected
        bytes32 slot21After = vm.load(address(optimismPortal2), bytes32(uint256(21)));
        bytes32 slot21Expected = NextImpl(address(optimismPortal2)).slot21Init();
        assertEq(slot21Expected, slot21After);
    }
}

/// @title OptimismPortal2_ResourceFuzz_Test
/// @dev Test various values of the resource metering config to ensure that deposits cannot be
///      broken by changing the config.
contract OptimismPortal2_ResourceFuzz_Test is CommonTest {
    /// @dev The max gas limit observed throughout this test. Setting this too high can cause
    ///      the test to take too long to run.
    uint256 constant MAX_GAS_LIMIT = 30_000_000;

    function setUp() public override {
        super.setUp();
    }

    /// @dev Test that various values of the resource metering config will not break deposits.
    /// forge-config: ciheavy.fuzz.runs = 10000
    function testFuzz_systemConfigDeposit_succeeds(
        uint32 _maxResourceLimit,
        uint8 _elasticityMultiplier,
        uint8 _baseFeeMaxChangeDenominator,
        uint32 _minimumBaseFee,
        uint32 _systemTxMaxGas,
        uint128 _maximumBaseFee,
        uint64 _gasLimit,
        uint64 _prevBoughtGas,
        uint128 _prevBaseFee,
        uint8 _blockDiff
    )
        external
    {
        // Get the set system gas limit
        uint64 gasLimit = systemConfig.gasLimit();

        // Bound resource config
        _maxResourceLimit = uint32(bound(_maxResourceLimit, 21000, MAX_GAS_LIMIT / 8));
        _gasLimit = uint64(bound(_gasLimit, 21000, _maxResourceLimit));
        _prevBaseFee = uint128(bound(_prevBaseFee, 0, 3 gwei));
        _prevBoughtGas = uint64(bound(_prevBoughtGas, 0, _maxResourceLimit - _gasLimit));
        _blockDiff = uint8(bound(_blockDiff, 0, 3));
        _baseFeeMaxChangeDenominator = uint8(bound(_baseFeeMaxChangeDenominator, 2, type(uint8).max));
        _elasticityMultiplier = uint8(bound(_elasticityMultiplier, 1, type(uint8).max));

        // Prevent values that would cause reverts
        vm.assume(gasLimit >= _gasLimit);
        vm.assume(_minimumBaseFee < _maximumBaseFee);
        vm.assume(_baseFeeMaxChangeDenominator > 1);
        vm.assume(uint256(_maxResourceLimit) + uint256(_systemTxMaxGas) <= gasLimit);
        vm.assume(((_maxResourceLimit / _elasticityMultiplier) * _elasticityMultiplier) == _maxResourceLimit);

        // Base fee can increase quickly and mean that we can't buy the amount of gas we want.
        // Here we add a VM assumption to bound the potential increase.
        // Compute the maximum possible increase in base fee.
        uint256 maxPercentIncrease = uint256(_elasticityMultiplier - 1) * 100 / uint256(_baseFeeMaxChangeDenominator);
        // Assume that we have enough gas to burn.
        // Compute the maximum amount of gas we'd need to burn.
        // Assume we need 1/5 of our gas to do other stuff.
        vm.assume(_prevBaseFee * maxPercentIncrease * _gasLimit / 100 < MAX_GAS_LIMIT * 4 / 5);

        // Pick a pseudorandom block number
        vm.roll(uint256(keccak256(abi.encode(_blockDiff))) % uint256(type(uint16).max) + uint256(_blockDiff));

        // Create a resource config to mock the call to the system config with
        IResourceMetering.ResourceConfig memory rcfg = IResourceMetering.ResourceConfig({
            maxResourceLimit: _maxResourceLimit,
            elasticityMultiplier: _elasticityMultiplier,
            baseFeeMaxChangeDenominator: _baseFeeMaxChangeDenominator,
            minimumBaseFee: _minimumBaseFee,
            systemTxMaxGas: _systemTxMaxGas,
            maximumBaseFee: _maximumBaseFee
        });
        vm.mockCall(address(systemConfig), abi.encodeCall(systemConfig.resourceConfig, ()), abi.encode(rcfg));

        // Set the resource params
        uint256 _prevBlockNum = block.number - _blockDiff;
        vm.store(
            address(optimismPortal2),
            bytes32(uint256(1)),
            bytes32((_prevBlockNum << 192) | (uint256(_prevBoughtGas) << 128) | _prevBaseFee)
        );
        // Ensure that the storage setting is correct
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = optimismPortal2.params();
        assertEq(prevBaseFee, _prevBaseFee);
        assertEq(prevBoughtGas, _prevBoughtGas);
        assertEq(prevBlockNum, _prevBlockNum);

        // Do a deposit, should not revert
        optimismPortal2.depositTransaction{ gas: MAX_GAS_LIMIT }({
            _to: address(0x20),
            _value: 0x40,
            _gasLimit: _gasLimit,
            _isCreation: false,
            _data: hex""
        });
    }
}

contract OptimismPortal2WithMockERC20_Test is OptimismPortal2_FinalizeWithdrawal_Test {
    MockERC20 token;

    function setUp() public virtual override {
        super.setUp();
        token = new MockERC20("Test", "TST", 18);
    }

    function depositERC20Transaction(
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
        if (_isCreation) {
            _to = address(0);
        }
        vm.assume(_data.length <= 120_000);
        IResourceMetering.ResourceConfig memory rcfg = systemConfig.resourceConfig();
        _gasLimit =
            uint64(bound(_gasLimit, optimismPortal2.minimumGasLimit(uint64(_data.length)), rcfg.maxResourceLimit));

        // Mint the token to the contract and approve the token for the portal
        token.mint(address(this), _mint);
        token.approve(address(optimismPortal2), _mint);

        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        bytes memory opaqueData = abi.encodePacked(_mint, _value, _gasLimit, _isCreation, _data);

        vm.expectEmit(address(optimismPortal2));
        emit TransactionDeposited(
            _from, // from
            _to,
            uint256(0), // DEPOSIT_VERSION
            opaqueData
        );

        // Deposit the token into the portal
        optimismPortal.depositERC20Transaction(_to, _mint, _value, _gasLimit, _isCreation, _data);

        // Assert final balance equals the deposited amount
        assertEq(token.balanceOf(address(optimismPortal2)), _mint);
        assertEq(optimismPortal2.balance(), _mint);
    }

    /// @dev Tests that `depositERC20Transaction` succeeds when msg.sender == tx.origin.
    /// forge-config: ciheavy.fuzz.runs = 8192
    function testFuzz_depositERC20Transaction_senderIsOrigin_succeeds(
        address _to,
        uint256 _mint,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        external
    {
        // Ensure that msg.sender == tx.origin
        vm.startPrank(address(this), address(this));

        depositERC20Transaction({
            _from: address(this),
            _to: _to,
            _mint: _mint,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });
    }

    /// @dev Tests that `depositERC20Transaction` succeeds when msg.sender != tx.origin.
    /// forge-config: ciheavy.fuzz.runs = 8192
    function testFuzz_depositERC20Transaction_senderNotOrigin_succeeds(
        address _to,
        uint256 _mint,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        external
    {
        // Ensure that msg.sender != tx.origin
        vm.startPrank(address(this), address(1));

        depositERC20Transaction({
            _from: AddressAliasHelper.applyL1ToL2Alias(address(this)),
            _to: _to,
            _mint: _mint,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });
    }

    /// @dev Tests that `depositERC20Transaction` reverts when not enough of the token is approved.
    function test_depositERC20Transaction_notEnoughAmount_reverts() external {
        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );
        vm.expectRevert(stdError.arithmeticError);
        // Deposit the token into the portal
        optimismPortal2.depositERC20Transaction(address(0), 1, 0, 0, false, "");
    }

    /// @dev Tests that `depositERC20Transaction` reverts when token balance does not update correctly after transfer.
    function test_depositERC20Transaction_incorrectTokenBalance_reverts() external {
        // Mint the token to the contract and approve the token for the portal
        token.mint(address(this), 100);
        token.approve(address(optimismPortal2), 100);

        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        // Mock the token balance
        vm.mockCall(address(token), abi.encodeCall(token.balanceOf, (address(optimismPortal))), abi.encode(0));

        // Call minimumGasLimit(0) before vm.expectRevert to ensure vm.expectRevert is for depositERC20Transaction
        uint64 gasLimit = optimismPortal2.minimumGasLimit(0);

        vm.expectRevert(TransferFailed.selector);

        // Deposit the token into the portal
        optimismPortal2.depositERC20Transaction(address(1), 100, 0, gasLimit, false, "");
    }

    /// @dev Tests that `depositERC20Transaction` reverts when creating a contract with a non-zero target.
    function test_depositERC20Transaction_isCreationNotZeroTarget_reverts() external {
        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        // Call minimumGasLimit(0) before vm.expectRevert to ensure vm.expectRevert is for depositERC20Transaction
        uint64 gasLimit = optimismPortal2.minimumGasLimit(0);

        vm.expectRevert(BadTarget.selector);
        // Deposit the token into the portal
        optimismPortal2.depositERC20Transaction(address(1), 0, 0, gasLimit, true, "");
    }

    /// @dev Tests that `depositERC20Transaction` reverts when the gas limit is too low.
    function test_depositERC20Transaction_gasLimitTooLow_reverts() external {
        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        vm.expectRevert(SmallGasLimit.selector);
        // Deposit the token into the portal
        optimismPortal2.depositERC20Transaction(address(0), 0, 0, 0, false, "");
    }

    /// @dev Tests that `depositERC20Transaction` reverts when the data is too large.
    function test_depositERC20Transaction_dataTooLarge_reverts() external {
        bytes memory data = new bytes(120_001);
        data[120_000] = 0x01;

        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        uint64 gasLimit = optimismPortal2.minimumGasLimit(120_001);
        vm.expectRevert(LargeCalldata.selector);
        // Deposit the token into the portal
        optimismPortal2.depositERC20Transaction(address(0), 0, 0, gasLimit, false, data);
    }

    /// @dev Tests that `balance()` returns the correct balance when the gas paying token is not ether.
    function testFuzz_balance_nonEther_succeeds(uint256 _amount) external {
        // Mint the token to the contract and approve the token for the portal
        token.mint(address(this), _amount);
        token.approve(address(optimismPortal2), _amount);

        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        // Deposit the token into the portal
        optimismPortal2.depositERC20Transaction(address(0), _amount, 0, optimismPortal.minimumGasLimit(0), false, "");

        // Check that the balance has been correctly updated
        assertEq(optimismPortal2.balance(), _amount);
    }

    /// @dev Tests that `finalizeWithdrawalTransaction` succeeds.
    function test_finalizeWithdrawalTransaction_provenWithdrawalHashWithNonEther_succeeds() external {
        // Mint the token to the contract and approve the token for the portal
        token.mint(address(this), _defaultTx.value);
        token.approve(address(optimismPortal2), _defaultTx.value);

        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        // Deposit the token into the portal
        optimismPortal2.depositERC20Transaction(
            address(bob), _defaultTx.value, 0, optimismPortal.minimumGasLimit(0), false, ""
        );

        assertEq(optimismPortal2.balance(), _defaultTx.value);

        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalProven(_withdrawalHash, alice, bob);
        optimismPortal2.proveWithdrawalTransaction({
            _tx: _defaultTx,
            _disputeGameIndex: _proposedGameIndex,
            _outputRootProof: _outputRootProof,
            _withdrawalProof: _withdrawalProof
        });

        // Warp past the finalization period.
        game.resolveClaim(0, 0);
        game.resolve();
        vm.warp(block.timestamp + optimismPortal2.proofMaturityDelaySeconds() + 1);

        vm.expectEmit(address(optimismPortal2));
        emit WithdrawalFinalized(_withdrawalHash, true);

        vm.expectCall(_defaultTx.target, 0, _defaultTx.data);

        vm.expectCall(address(token), 0, abi.encodeCall(token.transfer, (_defaultTx.target, _defaultTx.value)));

        optimismPortal2.finalizeWithdrawalTransaction(_defaultTx);

        assertEq(optimismPortal2.balance(), 0);
        assertEq(token.balanceOf(address(bob)), 100);
    }

    /// @dev Helper for depositing a transaction.
    function depositTransaction(
        address _from,
        address _to,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        internal
    {
        if (_isCreation) {
            _to = address(0);
        }
        vm.assume(_data.length <= 120_000);
        IResourceMetering.ResourceConfig memory rcfg = systemConfig.resourceConfig();
        _gasLimit =
            uint64(bound(_gasLimit, optimismPortal2.minimumGasLimit(uint64(_data.length)), rcfg.maxResourceLimit));

        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        bytes memory opaqueData = abi.encodePacked(uint256(0), _value, _gasLimit, _isCreation, _data);

        vm.expectEmit(address(optimismPortal2));
        emit TransactionDeposited(
            _from, // from
            _to,
            uint256(0), // DEPOSIT_VERSION
            opaqueData
        );

        // Deposit the token into the portal
        optimismPortal2.depositTransaction(_to, _value, _gasLimit, _isCreation, _data);

        // Assert final balance equals the deposited amount
        assertEq(token.balanceOf(address(optimismPortal2)), 0);
        assertEq(optimismPortal2.balance(), 0);
    }

    /// @dev Tests that `depositTransaction` succeeds when a custom gas token is used but the msg.value is zero.
    /// forge-config: ciheavy.fuzz.runs = 8192
    function testFuzz_depositTransaction_customGasTokenWithNoValueAndSenderIsOrigin_succeeds(
        address _to,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        external
    {
        // Ensure that msg.sender == tx.origin
        vm.startPrank(address(this), address(this));

        depositTransaction({
            _from: address(this),
            _to: _to,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });
    }

    /// @dev Tests that `depositTransaction` succeeds when a custom gas token is used but the msg.value is zero.
    /// forge-config: ciheavy.fuzz.runs = 8192
    function testFuzz_depositTransaction_customGasTokenWithNoValueAndSenderNotOrigin_succeeds(
        address _to,
        uint256 _value,
        uint64 _gasLimit,
        bool _isCreation,
        bytes memory _data
    )
        external
    {
        // Ensure that msg.sender != tx.origin
        vm.startPrank(address(this), address(1));

        depositTransaction({
            _from: AddressAliasHelper.applyL1ToL2Alias(address(this)),
            _to: _to,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: _isCreation,
            _data: _data
        });
    }

    /// @dev Tests that `depositTransaction` fails when a custom gas token is used and msg.value is non-zero.
    function test_depositTransaction_customGasTokenWithValue_reverts() external {
        // Mock the gas paying token to be the ERC20 token
        vm.mockCall(
            address(systemConfig), abi.encodeCall(systemConfig.gasPayingToken, ()), abi.encode(address(token), 18)
        );

        vm.expectRevert(NoValue.selector);

        // Deposit the token into the portal
        optimismPortal2.depositTransaction{ value: 100 }(address(0), 0, 0, false, "");
    }
}
