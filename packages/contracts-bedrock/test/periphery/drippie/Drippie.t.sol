// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { Drippie } from "src/periphery/drippie/Drippie.sol";
import { IDripCheck } from "src/periphery/drippie/IDripCheck.sol";
import { CheckTrue } from "src/periphery/drippie/dripchecks/CheckTrue.sol";
import { SimpleStorage } from "test/mocks/SimpleStorage.sol";

/// @title  TestDrippie
/// @notice This is a wrapper contract around Drippie used for testing.
///         Returning an entire `Drippie.DripState` causes stack too
///         deep errors without `--vir-ir` which causes the compile time
///         to go up by ~4x. Each of the methods is a simple getter around
///         parts of the `DripState`.
contract TestDrippie is Drippie {
    constructor(address owner) Drippie(owner) { }

    function dripStatus(string memory name) external view returns (Drippie.DripStatus) {
        return drips[name].status;
    }

    function dripStateLast(string memory name) external view returns (uint256) {
        return drips[name].last;
    }

    function dripConfig(string memory name) external view returns (Drippie.DripConfig memory) {
        return drips[name].config;
    }

    function dripConfigActions(string memory name) external view returns (Drippie.DripAction[] memory) {
        return drips[name].config.actions;
    }

    function dripConfigCheckParams(string memory name) external view returns (bytes memory) {
        return drips[name].config.checkparams;
    }

    function dripConfigCheckAddress(string memory name) external view returns (address) {
        return address(drips[name].config.dripcheck);
    }
}

/// @title  Drippie_Test
/// @notice Test coverage of the Drippie contract.
contract Drippie_Test is Test {
    /// @notice Emitted when a drip is executed.
    event DripExecuted(string indexed nameref, string name, address executor, uint256 timestamp);

    /// @notice Emitted when a drip's status is updated.
    event DripStatusUpdated(string indexed nameref, string name, Drippie.DripStatus status);

    /// @notice Emitted when a drip is created.
    event DripCreated(string indexed nameref, string name, Drippie.DripConfig config);

    /// @notice Address of the test DripCheck. CheckTrue is deployed
    ///         here so it always returns true.
    IDripCheck check;

    /// @notice Address of a SimpleStorage contract. Used to test that
    ///         calls are actually made by Drippie.
    SimpleStorage simpleStorage;

    /// @notice Address of the Drippie contract.
    TestDrippie drippie;

    /// @notice Address of the Drippie owner
    address constant alice = address(0x42);

    /// @notice The name of the default drip
    string constant dripName = "foo";

    /// @notice Set up the test suite by deploying a CheckTrue DripCheck.
    ///         This is a mock that always returns true. Alice is the owner
    ///         and also give drippie 1 ether so that it can balance transfer.
    function setUp() external {
        check = IDripCheck(new CheckTrue());
        simpleStorage = new SimpleStorage();

        drippie = new TestDrippie(alice);
        vm.deal(address(drippie), 1 ether);
    }

    /// @notice Builds a default Drippie.DripConfig. Uses CheckTrue as the
    ///         dripcheck so it will always be able to do its DripActions.
    ///         Gives a dummy DripAction that does a simple transfer to
    ///         a dummy address.
    function _defaultConfig() internal view returns (Drippie.DripConfig memory) {
        Drippie.DripAction[] memory actions = new Drippie.DripAction[](1);
        actions[0] = Drippie.DripAction({ target: payable(address(0x44)), data: hex"", value: 1 });

        return Drippie.DripConfig({
            interval: 100,
            dripcheck: check,
            reentrant: false,
            checkparams: hex"",
            actions: actions
        });
    }

    /// @notice Creates a default drip using the default drip config.
    function _createDefaultDrip(string memory name) internal {
        address owner = drippie.owner();
        Drippie.DripConfig memory cfg = _defaultConfig();
        vm.prank(owner);
        drippie.create(name, cfg);
    }

    /// @notice Moves the block's timestamp forward so that it is
    ///         possible to execute a drip.
    function _warpToExecutable(string memory name) internal {
        Drippie.DripConfig memory config = drippie.dripConfig(name);
        vm.warp(config.interval + drippie.dripStateLast(name));
    }

    /// @notice Creates a drip and asserts that it was configured as expected.
    function test_create_succeeds() external {
        Drippie.DripConfig memory cfg = _defaultConfig();
        vm.expectEmit(address(drippie));
        emit DripCreated(dripName, dripName, cfg);

        if (cfg.reentrant) {
            assertEq(cfg.interval, 0);
        } else {
            assertTrue(cfg.interval > 0);
        }

        // Drip count is 0 before creating the drip.
        assertEq(drippie.getDripCount(), 0);

        vm.prank(drippie.owner());
        drippie.create(dripName, cfg);

        Drippie.DripStatus status = drippie.dripStatus(dripName);
        Drippie.DripConfig memory config = drippie.dripConfig(dripName);

        assertEq(uint256(status), uint256(Drippie.DripStatus.PAUSED));

        assertEq(config.interval, cfg.interval);
        assertEq(config.reentrant, cfg.reentrant);
        assertEq(address(config.dripcheck), address(cfg.dripcheck));
        assertEq(config.checkparams, cfg.checkparams);

        assertEq(config.actions.length, cfg.actions.length);

        for (uint256 i; i < config.actions.length; i++) {
            Drippie.DripAction memory a = config.actions[i];
            Drippie.DripAction memory b = cfg.actions[i];

            assertEq(a.target, b.target);
            assertEq(a.data, b.data);
            assertEq(a.value, b.value);
        }

        // Drip count is 1 after creating the drip.
        assertEq(drippie.getDripCount(), 1);

        // Name of the first created drip is the same as the name of the drip.
        assertEq(drippie.created(0), dripName);
    }

    /// @notice Ensures that the same drip cannot be created two times.
    function test_create_calledTwice_reverts() external {
        vm.startPrank(drippie.owner());
        Drippie.DripConfig memory cfg = _defaultConfig();
        drippie.create(dripName, cfg);
        vm.expectRevert("Drippie: drip with that name already exists");
        drippie.create(dripName, cfg);
        vm.stopPrank();
    }

    /// @notice Ensures that only the owner of Drippie can create a drip.
    function testFuzz_owner_unauthorized_reverts(address caller) external {
        vm.assume(caller != drippie.owner());
        vm.prank(caller);
        vm.expectRevert("UNAUTHORIZED");
        drippie.create(dripName, _defaultConfig());
    }

    /// @notice The owner should be able to set the status of the drip.
    function test_set_status_succeeds() external {
        vm.expectEmit(address(drippie));
        emit DripCreated(dripName, dripName, _defaultConfig());
        _createDefaultDrip(dripName);

        address owner = drippie.owner();

        {
            Drippie.DripStatus status = drippie.dripStatus(dripName);
            assertEq(uint256(status), uint256(Drippie.DripStatus.PAUSED));
        }

        vm.prank(owner);

        vm.expectEmit(address(drippie));
        emit DripStatusUpdated({ nameref: dripName, name: dripName, status: Drippie.DripStatus.ACTIVE });

        drippie.status(dripName, Drippie.DripStatus.ACTIVE);

        {
            Drippie.DripStatus status = drippie.dripStatus(dripName);
            assertEq(uint256(status), uint256(Drippie.DripStatus.ACTIVE));
        }

        vm.prank(owner);

        vm.expectEmit(address(drippie));
        emit DripStatusUpdated({ nameref: dripName, name: dripName, status: Drippie.DripStatus.PAUSED });

        drippie.status(dripName, Drippie.DripStatus.PAUSED);

        {
            Drippie.DripStatus status = drippie.dripStatus(dripName);
            assertEq(uint256(status), uint256(Drippie.DripStatus.PAUSED));
        }
    }

    /// @notice The drip status cannot be set back to NONE after it is created.
    function test_set_statusNone_reverts() external {
        _createDefaultDrip(dripName);

        vm.prank(drippie.owner());

        vm.expectRevert("Drippie: drip status can never be set back to NONE after creation");

        drippie.status(dripName, Drippie.DripStatus.NONE);
    }

    /// @notice The owner cannot set the status of the drip to the status that
    ///         it is already set as.
    function test_set_statusSame_reverts() external {
        _createDefaultDrip(dripName);

        vm.prank(drippie.owner());

        vm.expectRevert("Drippie: cannot set drip status to the same status as its current status");

        drippie.status(dripName, Drippie.DripStatus.PAUSED);
    }

    /// @notice The owner should be able to archive the drip if it is in the
    ///         paused state.
    function test_shouldArchive_ifPaused_succeeds() external {
        _createDefaultDrip(dripName);

        address owner = drippie.owner();

        vm.prank(owner);

        vm.expectEmit(address(drippie));
        emit DripStatusUpdated({ nameref: dripName, name: dripName, status: Drippie.DripStatus.ARCHIVED });

        drippie.status(dripName, Drippie.DripStatus.ARCHIVED);

        Drippie.DripStatus status = drippie.dripStatus(dripName);
        assertEq(uint256(status), uint256(Drippie.DripStatus.ARCHIVED));
    }

    /// @notice The owner should not be able to archive the drip if it is in the
    ///         active state.
    function test_shouldNotArchive_ifActive_reverts() external {
        _createDefaultDrip(dripName);

        vm.prank(drippie.owner());
        drippie.status(dripName, Drippie.DripStatus.ACTIVE);

        vm.prank(drippie.owner());

        vm.expectRevert("Drippie: drip must first be paused before being archived");

        drippie.status(dripName, Drippie.DripStatus.ARCHIVED);
    }

    /// @notice The owner should not be allowed to pause the drip if it
    ///         has already been archived.
    function test_shouldNotAllowPaused_ifArchived_reverts() external {
        _createDefaultDrip(dripName);

        _notAllowFromArchive(dripName, Drippie.DripStatus.PAUSED);
    }

    /// @notice The owner should not be allowed to make the drip active again if
    ///         it has already been archived.
    function test_shouldNotAllowActive_ifArchived_reverts() external {
        _createDefaultDrip(dripName);

        _notAllowFromArchive(dripName, Drippie.DripStatus.ACTIVE);
    }

    /// @notice Archive the drip and then attempt to set the status to the passed
    ///         in status.
    function _notAllowFromArchive(string memory name, Drippie.DripStatus status) internal {
        address owner = drippie.owner();
        vm.prank(owner);
        drippie.status(name, Drippie.DripStatus.ARCHIVED);

        vm.expectRevert("Drippie: drip with that name has been archived and cannot be updated");

        vm.prank(owner);
        drippie.status(name, status);
    }

    /// @notice Attempt to update a drip that does not exist.
    function test_name_notExist_reverts() external {
        string memory otherName = "bar";

        vm.prank(drippie.owner());

        vm.expectRevert("Drippie: drip with that name does not exist and cannot be updated");

        assertFalse(keccak256(abi.encode(dripName)) == keccak256(abi.encode(otherName)));

        drippie.status(otherName, Drippie.DripStatus.ARCHIVED);
    }

    /// @notice Expect a revert when attempting to set the status when not the owner.
    function test_status_unauthorized_reverts() external {
        _createDefaultDrip(dripName);

        vm.expectRevert("UNAUTHORIZED");
        drippie.status(dripName, Drippie.DripStatus.ACTIVE);
    }

    /// @notice The drip should execute and be able to transfer value.
    function test_drip_amount_succeeds() external {
        _createDefaultDrip(dripName);

        vm.prank(drippie.owner());
        drippie.status(dripName, Drippie.DripStatus.ACTIVE);

        _warpToExecutable(dripName);

        vm.expectEmit(address(drippie));
        emit DripExecuted({ nameref: dripName, name: dripName, executor: address(this), timestamp: block.timestamp });

        Drippie.DripAction[] memory actions = drippie.dripConfigActions(dripName);
        assertEq(actions.length, 1);

        vm.expectCall(drippie.dripConfigCheckAddress(dripName), drippie.dripConfigCheckParams(dripName));

        vm.expectCall(actions[0].target, actions[0].value, actions[0].data);

        drippie.drip(dripName);
    }

    /// @notice A single DripAction should be able to make a state modifying call.
    function test_trigger_oneFunction_succeeds() external {
        Drippie.DripConfig memory cfg = _defaultConfig();

        bytes32 key = bytes32(uint256(2));
        bytes32 value = bytes32(uint256(3));

        // Add in an action
        cfg.actions[0] = Drippie.DripAction({
            target: payable(address(simpleStorage)),
            data: abi.encodeCall(SimpleStorage.set, (key, value)),
            value: 0
        });

        vm.prank(drippie.owner());
        drippie.create(dripName, cfg);

        _warpToExecutable(dripName);

        vm.prank(drippie.owner());
        drippie.status(dripName, Drippie.DripStatus.ACTIVE);

        vm.expectCall(address(simpleStorage), 0, abi.encodeCall(SimpleStorage.set, (key, value)));

        vm.expectEmit(address(drippie));
        emit DripExecuted(dripName, dripName, address(this), block.timestamp);
        drippie.drip(dripName);

        assertEq(simpleStorage.get(key), value);
    }

    /// @notice Multiple drip actions should be able to be triggered with the same check.
    function test_trigger_twoFunctions_succeeds() external {
        Drippie.DripConfig memory cfg = _defaultConfig();
        Drippie.DripAction[] memory actions = new Drippie.DripAction[](2);

        bytes32 keyOne = bytes32(uint256(2));
        bytes32 valueOne = bytes32(uint256(3));
        actions[0] = Drippie.DripAction({
            target: payable(address(simpleStorage)),
            data: abi.encodeCall(SimpleStorage.set, (keyOne, valueOne)),
            value: 0
        });

        bytes32 keyTwo = bytes32(uint256(4));
        bytes32 valueTwo = bytes32(uint256(5));
        actions[1] = Drippie.DripAction({
            target: payable(address(simpleStorage)),
            data: abi.encodeCall(SimpleStorage.set, (keyTwo, valueTwo)),
            value: 0
        });

        cfg.actions = actions;

        vm.prank(drippie.owner());
        drippie.create(dripName, cfg);

        _warpToExecutable(dripName);

        vm.prank(drippie.owner());
        drippie.status(dripName, Drippie.DripStatus.ACTIVE);

        vm.expectCall(drippie.dripConfigCheckAddress(dripName), drippie.dripConfigCheckParams(dripName));

        vm.expectCall(address(simpleStorage), 0, abi.encodeCall(SimpleStorage.set, (keyOne, valueOne)));

        vm.expectCall(address(simpleStorage), 0, abi.encodeCall(SimpleStorage.set, (keyTwo, valueTwo)));

        vm.expectEmit(address(drippie));
        emit DripExecuted(dripName, dripName, address(this), block.timestamp);
        drippie.drip(dripName);

        assertEq(simpleStorage.get(keyOne), valueOne);

        assertEq(simpleStorage.get(keyTwo), valueTwo);
    }

    /// @notice The drips can only be triggered once per interval. Attempt to
    ///         trigger the same drip multiple times in the same interval. Then
    ///         move forward to the next interval and it should trigger.
    function test_twice_inOneInterval_reverts() external {
        _createDefaultDrip(dripName);

        vm.prank(drippie.owner());
        drippie.status(dripName, Drippie.DripStatus.ACTIVE);

        _warpToExecutable(dripName);

        vm.prank(drippie.owner());

        drippie.drip(dripName);

        vm.prank(drippie.owner());

        vm.expectRevert("Drippie: drip interval has not elapsed since last drip");

        drippie.drip(dripName);

        _warpToExecutable(dripName);

        vm.expectEmit(address(drippie));
        emit DripExecuted({ nameref: dripName, name: dripName, executor: address(this), timestamp: block.timestamp });

        drippie.drip(dripName);
    }

    /// @notice It should revert if attempting to trigger a drip that does not exist.
    ///         Note that the drip was never created at the beginning of the test.
    function test_drip_notExist_reverts() external {
        vm.prank(drippie.owner());

        vm.expectRevert("Drippie: selected drip does not exist or is not currently active");

        drippie.drip(dripName);
    }

    /// @notice The owner cannot trigger the drip when it is paused.
    function test_not_active_reverts() external {
        _createDefaultDrip(dripName);

        Drippie.DripStatus status = drippie.dripStatus(dripName);
        assertEq(uint256(status), uint256(Drippie.DripStatus.PAUSED));

        vm.prank(drippie.owner());

        vm.expectRevert("Drippie: selected drip does not exist or is not currently active");

        drippie.drip(dripName);
    }

    /// @notice The interval must be 0 if reentrant is set on the config.
    function test_reentrant_succeeds() external {
        address owner = drippie.owner();
        Drippie.DripConfig memory cfg = _defaultConfig();
        cfg.reentrant = true;
        cfg.interval = 0;

        vm.prank(owner);
        vm.expectEmit(address(drippie));
        emit DripCreated(dripName, dripName, cfg);
        drippie.create(dripName, cfg);

        Drippie.DripConfig memory _cfg = drippie.dripConfig(dripName);
        assertEq(_cfg.reentrant, true);
        assertEq(_cfg.interval, 0);
    }

    /// @notice A non zero interval when reentrant is true will cause a revert
    ///         when creating a drip.
    function test_drip_reentrant_reverts() external {
        address owner = drippie.owner();
        Drippie.DripConfig memory cfg = _defaultConfig();
        cfg.reentrant = true;
        cfg.interval = 1;

        vm.prank(owner);

        vm.expectRevert("Drippie: if allowing reentrant drip, must set interval to zero");
        drippie.create(dripName, cfg);
    }

    /// @notice If reentrant is false and the interval is 0 then it should
    ///         revert when the drip is created.
    function test_notReentrant_zeroInterval_reverts() external {
        address owner = drippie.owner();
        Drippie.DripConfig memory cfg = _defaultConfig();
        cfg.reentrant = false;
        cfg.interval = 0;

        vm.prank(owner);

        vm.expectRevert("Drippie: interval must be greater than zero if drip is not reentrant");
        drippie.create(dripName, cfg);
    }
}
