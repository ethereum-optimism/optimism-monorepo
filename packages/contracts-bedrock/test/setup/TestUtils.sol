// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { TestPlus } from "solady/test/utils/TestPlus.sol";
import { Vm } from "forge-std/Vm.sol";

contract TestUtils is TestPlus {
    Vm private constant _vm = Vm(address(bytes20(uint160(uint256(keccak256("hevm cheat code"))))));

    // This function checks whether an address, `addr`, is payable. It works by sending 1 wei to
    // `addr` and checking the `success` return value.
    // NOTE: This function may result in state changes depending on the fallback/receive logic
    // implemented by `addr`, which should be taken into account when this function is used.
    function _isPayable(address addr) private returns (bool) {
        require(
            addr.balance < type(uint256).max,
            "TestUtils: Balance equals max uint256, so it cannot receive any more funds"
        );
        uint256 origBalanceTest = address(this).balance;
        uint256 origBalanceAddr = address(addr).balance;

        (bool success,) = payable(addr).call{ value: 1 }("");

        // reset balances
        _vm.deal(address(this), origBalanceTest);
        _vm.deal(addr, origBalanceAddr);

        return success;
    }

    // This function checks whether an address, `addr`, is not payable. It works by sending 1 wei to
    // `addr` and checking the `success` return value.
    // NOTE: This function may result in state changes depending on the fallback/receive logic
    // implemented by `addr`, which should be taken into account when this function is used.
    function _isNotPayable(address addr) private returns (bool) {
        return !_isPayable(addr);
    }

    // This function checks whether an address, `addr`, is not a precompile on OP main/test net.
    function _isNotPrecompile(address addr) private pure returns (bool) {
        // Note: For some chains like Optimism these are technically predeploys (i.e. bytecode placed at a specific
        // address), but the same rationale for excluding them applies so we include those too.

        // These should be present on all EVM-compatible chains.
        if (addr >= address(0x01) && addr <= address(0x09)) return false;
        // forgefmt: disable-start
        // https://github.com/ethereum-optimism/optimism/blob/eaa371a0184b56b7ca6d9eb9cb0a2b78b2ccd864/op-bindings/predeploys/addresses.go#L6-L21
        return (addr < address(0x4200000000000000000000000000000000000000) || addr > address(0x4200000000000000000000000000000000000800));
        // forgefmt: disable-end
    }

    // This function checks whether an address, `addr`, is not the vm, console, or Create2Deployer addresses.
    function _isNotForgeAddress(address addr) internal pure returns (bool) {
        // vm, console, and Create2Deployer addresses
        return (
            addr != address(_vm) && addr != 0x000000000000000000636F6e736F6c652e6c6f67
                && addr != 0x4e59b44847b379578588920cA78FbF26c0B4956C
        );
    }

    // This function returns addr if it satisfies all given conditions and is not forbidden,
    // otherwise it will generate a new random address that satisfies the conditions and is not
    // forbidden.
    // NOTE: This function will resort to vm.assume() if it does not find a valid address within
    // the given number of attempts.
    function _randomAddress(
        address addr,
        function(address) internal returns (bool)[] memory _conditions,
        ForbiddenAddresses _forbiddenAddresses,
        uint256 attempts
    )
        internal
        returns (address)
    {
        bool pass = false;

        for (uint256 i; i < attempts; i++) {
            pass = true;
            if (_forbiddenAddresses.forbiddenAddresses(addr)) continue;
            for (uint256 j; j < _conditions.length; j++) {
                if (!_conditions[j](addr)) {
                    pass = false;
                    break;
                }
            }
            if (pass) break;
            addr = _randomAddress();
        }
        _vm.assume(pass);

        return addr;
    }

    // This function returns addr if it is not forbidden by _forbiddenAddresses, otherwise it
    // will generate a new random address that is not forbidden.
    // NOTE: This function will resort to vm.assume() if it does not find a valid address within
    // the given number of attempts.
    function _randomAddress(
        address addr,
        ForbiddenAddresses _forbiddenAddresses,
        uint256 attempts
    )
        internal
        returns (address)
    {
        bool pass = false;

        for (uint256 i; i < attempts; i++) {
            if (_forbiddenAddresses.forbiddenAddresses(addr)) {
                pass = false;
                addr = _randomAddress();
            } else {
                pass = true;
            }
            if (pass) break;
        }
        _vm.assume(pass);

        return addr;
    }

    // This function returns addr if it satisfies all given conditions, otherwise it will generate
    // a new random address that satisfies the conditions.
    // NOTE: This function will resort to vm.assume() if it does not find a valid address within
    // the given number of attempts.
    function _randomAddress(
        address addr,
        function(address) internal returns (bool)[] memory _conditions,
        uint256 attempts
    )
        internal
        returns (address)
    {
        bool pass = false;

        for (uint256 i; i < attempts; i++) {
            pass = true;
            for (uint256 j; j < _conditions.length; j++) {
                if (!_conditions[j](addr)) {
                    pass = false;
                    break;
                }
            }
            if (pass) break;
            addr = _randomAddress();
        }
        _vm.assume(pass);

        return addr;
    }

    // This function returns _bound(_value, _min, _max) unless _bound(_value, _min, _max) is
    // forbidden by _forbiddenUint256, in which case it will generate a new random uint256 that
    // is not forbidden in the _forbiddenUint256 contract.
    // NOTE: This function will resort to vm.assume() if it does not find a valid uint256 within
    // the given number of attempts.
    function _boundExcept(
        uint256 _value,
        uint256 _min,
        uint256 _max,
        ForbiddenUint256 _forbiddenUint256,
        uint256 _attempts
    )
        internal
        returns (uint256)
    {
        uint256 rand = _bound(_value, _min, _max);
        bool pass = false;
        for (uint256 i; i < _attempts; i++) {
            if (_forbiddenUint256.forbiddenUint256(rand)) {
                pass = false;
                rand = _bound(_random(), _min, _max);
            } else {
                pass = true;
            }
            if (pass) break;
        }
        _vm.assume(pass);
        return rand;
    }

    function _bound(uint256 _value, uint256 _min, uint256 _max) private pure returns (uint256 rand_) {
        rand_ = (_value % (_max - _min)) + _min;
    }

    // example usage:
    function _randomAddressNotPrecompileNotPayableNotForgeAddress() private returns (address) {
        function(address) internal returns (bool)[] memory _conditions =
            new function(address) internal returns (bool)[](3);
        _conditions[0] = _isNotPrecompile;
        _conditions[1] = _isNotPayable;
        _conditions[2] = _isNotForgeAddress;
        ForbiddenAddresses forbiddenAddresses = (new ForbiddenAddresses()).forbid(address(123)).forbid(address(456))
            .forbid(address(789)).forbid(address(101)).forbid(address(112)).forbid(address(133)).forbid(address(144)).forbid(
            address(155)
        ).forbid(address(166)).forbid(address(177));

        // assume it's a random fuzz input
        address fuzzInput = address(0x1);
        return _randomAddress(fuzzInput, _conditions, forbiddenAddresses, 1_000);
    }

    // example usage:
    function _randomUint256Forbid0to10(uint256 _min, uint256 _max) private returns (uint256) {
        ForbiddenUint256 forbiddenUint256 = new ForbiddenUint256().forbid(0).forbid(1).forbid(2).forbid(3).forbid(4)
            .forbid(5).forbid(6).forbid(7).forbid(8).forbid(9).forbid(10);

        // assume it's a random fuzz input
        uint256 fuzzInput = 2;
        return _boundExcept(fuzzInput, _min, _max, forbiddenUint256, 1_000);
    }
}

// Ephemeral contract that stores a list of forbidden addresses.
contract ForbiddenAddresses {
    mapping(address => bool) public forbiddenAddresses;

    // chainable
    function forbid(address _addr) public returns (ForbiddenAddresses) {
        forbiddenAddresses[_addr] = true;
        return this;
    }
}

// Ephemeral contract that stores a list of forbidden uint256 values.
contract ForbiddenUint256 {
    mapping(uint256 => bool) public forbiddenUint256;

    // chainable
    function forbid(uint256 _value) public returns (ForbiddenUint256) {
        forbiddenUint256[_value] = true;
        return this;
    }
}
