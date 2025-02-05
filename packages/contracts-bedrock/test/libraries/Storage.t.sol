// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Target contract
import { StorageSetter } from "src/universal/StorageSetter.sol";
import { Test } from "forge-std/Test.sol";

/// @title Storage_Roundtrip_Test
/// @notice Tests the storage setting and getting through the StorageSetter contract.
///         This contract simply wraps the Storage library, this is required as to
///         not poison the storage of the `Test` contract.
contract Storage_Roundtrip_Test is Test {
    StorageSetter setter;

    /// @notice A set of storage slots to pass to `setBytes32`.
    StorageSetter.Slot[] slots;
    /// @notice Used to deduplicate slots passed to `setBytes32`.
    mapping(bytes32 => bool) keys;

    function setUp() external {
        setter = new StorageSetter();
    }

    function test_setGetUint_succeeds(bytes32 slot, uint256 num) external {
        setter.setUint(slot, num);
        assertEq(setter.getUint(slot), num);
        assertEq(num, uint256(vm.load(address(setter), slot)));
    }

    function test_setGetAddress_succeeds(bytes32 slot, address addr) external {
        setter.setAddress(slot, addr);
        assertEq(setter.getAddress(slot), addr);
        assertEq(addr, address(uint160(uint256(vm.load(address(setter), slot)))));
    }

    function test_setGetBytes32_succeeds(bytes32 slot, bytes32 hash) external {
        setter.setBytes32(slot, hash);
        assertEq(setter.getBytes32(slot), hash);
        assertEq(hash, vm.load(address(setter), slot));
    }

    function test_setGetBool_succeeds(bytes32 slot, bool value) external {
        setter.setBool(slot, value);
        assertEq(setter.getBool(slot), value);
        assertEq(value, vm.load(address(setter), slot) == bytes32(uint256(1)));
    }

    /// @dev All keys must be unique in the input so deduplication is required.
    function testFuzz_setGetBytes32Multi_succeeds(StorageSetter.Slot[] calldata _slots) external {
        for (uint256 i; i < _slots.length; i++) {
            if (keys[_slots[i].key]) {
                continue;
            }
            slots.push(_slots[i]);
            keys[_slots[i].key] = true;
        }

        setter.setBytes32(slots);
        for (uint256 i; i < slots.length; i++) {
            assertEq(setter.getBytes32(slots[i].key), slots[i].value);
            assertEq(slots[i].value, vm.load(address(setter), slots[i].key));
        }
    }
}
