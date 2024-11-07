// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test, stdStorage, StdStorage } from "forge-std/Test.sol";
import { stdToml } from "forge-std/StdToml.sol";
import { Solarray } from "scripts/libraries/Solarray.sol";

import { DeployAuthSystemInput, DeployAuthSystem, DeployAuthSystemOutput } from "scripts/deploy/DeployAuthSystem.s.sol";

contract DeployAuthSystemInput_Test is Test {
    DeployAuthSystemInput dasi;

    uint256 threshold = 5;
    address[] owners;

    function setUp() public {
        dasi = new DeployAuthSystemInput();
        address[] memory _owners = Solarray.addresses(
            0x1111111111111111111111111111111111111111,
            0x2222222222222222222222222222222222222222,
            0x3333333333333333333333333333333333333333,
            0x4444444444444444444444444444444444444444,
            0x5555555555555555555555555555555555555555,
            0x6666666666666666666666666666666666666666,
            0x7777777777777777777777777777777777777777
        );

        for (uint256 i = 0; i < _owners.length; i++) {
            owners.push(_owners[i]);
        }
    }

    function test_getters_whenNotSet_reverts() public {
        vm.expectRevert("DeployAuthSystemInput: threshold not set");
        dasi.threshold();

        vm.expectRevert("DeployAuthSystemInput: owners not set");
        dasi.owners();
    }

    function test_setters_ownerAlreadySet_reverts() public {
        dasi.set(dasi.owners.selector, owners);

        vm.expectRevert("DeployAuthSystemInput: owners already set");
        dasi.set(dasi.owners.selector, owners);
    }
}

contract DeployAuthSystemOutput_Test is Test {
    using stdToml for string;

    DeployAuthSystemOutput daso;

    function setUp() public {
        daso = new DeployAuthSystemOutput();
    }

    function test_set_succeeds() public {
        address safeAddr = makeAddr("safe");

        vm.etch(safeAddr, hex"01");

        daso.set(daso.safe.selector, safeAddr);

        assertEq(safeAddr, address(daso.safe()), "100");
    }

    function test_getter_whenNotSet_reverts() public {
        vm.expectRevert("DeployUtils: zero address");
        daso.safe();
    }

    function test_getter_whenAddrHasNoCode_reverts() public {
        address emptyAddr = makeAddr("emptyAddr");
        bytes memory expectedErr = bytes(string.concat("DeployUtils: no code at ", vm.toString(emptyAddr)));

        daso.set(daso.safe.selector, emptyAddr);
        vm.expectRevert(expectedErr);
        daso.safe();
    }
}

contract DeployAuthSystem_Test is Test {
    using stdStorage for StdStorage;

    DeployAuthSystem deployAuthSystem;
    DeployAuthSystemInput dasi;
    DeployAuthSystemOutput daso;

    // Define default input variables for testing.
    uint256 defaultThreshold = 5;
    uint256 defaultOwnersLength = 7;
    address[] defaultOwners;

    function setUp() public {
        deployAuthSystem = new DeployAuthSystem();
        (dasi, daso) = deployAuthSystem.etchIOContracts();
        for (uint256 i = 0; i < defaultOwnersLength; i++) {
            defaultOwners.push(makeAddr(string.concat("owner", vm.toString(i))));
        }
    }

    function hash(bytes32 _seed, uint256 _i) internal pure returns (bytes32) {
        return keccak256(abi.encode(_seed, _i));
    }

    function testFuzz_run_memory_succeeds(bytes32 _seed) public {
        address[] memory _owners = Solarray.addresses(
            address(uint160(uint256(hash(_seed, 0)))),
            address(uint160(uint256(hash(_seed, 1)))),
            address(uint160(uint256(hash(_seed, 2)))),
            address(uint160(uint256(hash(_seed, 3)))),
            address(uint160(uint256(hash(_seed, 4)))),
            address(uint160(uint256(hash(_seed, 5)))),
            address(uint160(uint256(hash(_seed, 6))))
        );

        uint256 threshold = bound(uint256(_seed), 1, _owners.length - 1);

        dasi.set(dasi.owners.selector, _owners);
        dasi.set(dasi.threshold.selector, threshold);

        deployAuthSystem.run(dasi, daso);

        assertNotEq(address(daso.safe()), address(0), "100");
        assertEq(daso.safe().getThreshold(), threshold, "200");
        // TODO: the getOwners() method requires iterating over the owners linked list.
        // Since we're not yet performing a proper deployment of the Safe, this call will revert.
        // assertEq(daso.safe().getOwners().length, _owners.length, "300");

        // Architecture assertions.
        // TODO: these will become relevant as we add more contracts to the auth system, and need to test their
        // relationships.

        daso.checkOutput();
    }

    function test_run_nullInput_reverts() public {
        dasi.set(dasi.owners.selector, defaultOwners);
        dasi.set(dasi.threshold.selector, defaultThreshold);

        // Zero out the owners length slot
        uint256 slot = 9;
        vm.store(address(dasi), bytes32(uint256(9)), bytes32(0));
        vm.expectRevert("DeployAuthSystemInput: owners not set");
        deployAuthSystem.run(dasi, daso);
        vm.store(address(dasi), bytes32(uint256(9)), bytes32(defaultOwnersLength));

        slot = zeroOutSlotForSelector(dasi.threshold.selector);
        vm.expectRevert("DeployAuthSystemInput: threshold not set");
        deployAuthSystem.run(dasi, daso);
        vm.store(address(dasi), bytes32(slot), bytes32(defaultThreshold));
    }

    function zeroOutSlotForSelector(bytes4 _selector) internal returns (uint256 slot_) {
        slot_ = stdstore.enable_packed_slots().target(address(dasi)).sig(_selector).find();
        vm.store(address(dasi), bytes32(slot_), bytes32(0));
    }
}
