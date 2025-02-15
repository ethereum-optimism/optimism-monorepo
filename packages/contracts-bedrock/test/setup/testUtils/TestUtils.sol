// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";
import {
    IAddressCondition,
    AddressConditionChainer,
    AddressConditions,
    IsPayableCondition,
    IsNotPayableCondition,
    IsNotPrecompileCondition,
    IsNotForgeAddressCondition
} from "test/setup/testUtils/conditions/AddressConditions.sol";
import { ForbiddenAddresses, ForbiddenUints, ForbiddenInts } from "test/setup/testUtils/Forbiddens.sol";

abstract contract TestUtils is Test {
    // This function returns _addr if it satisfies all given conditions and is not forbidden,
    // otherwise it will generate a new random address that satisfies the conditions and is not
    // forbidden.
    // NOTE: This function will resort to vm.assume() if it does not find a valid address within
    // the given number of _attempts.
    function __randomAddress(
        address _addr,
        AddressConditionChainer _conditions,
        ForbiddenAddresses _forbiddenAddresses,
        uint256 _attempts
    )
        internal
        returns (address)
    {
        IAddressCondition[] memory conditions = _conditions.conditions();

        // if _attempts is 0, vm.assume below will fail
        bool pass = false;
        for (uint256 i; i < _attempts; i++) {
            if (_forbiddenAddresses.forbiddenAddresses(_addr)) {
                _addr = vm.randomAddress();
                continue;
            }

            pass = true;
            for (uint256 j; j < conditions.length; j++) {
                if (!conditions[j].check(_addr)) {
                    pass = false;
                    break;
                }
            }
            if (pass) break;
            _addr = vm.randomAddress();
        }
        vm.assume(pass);

        return _addr;
    }

    // This function returns _addr if it is not forbidden by _forbiddenAddresses, otherwise it
    // will generate a new random address that is not forbidden.
    // NOTE: This function will resort to vm.assume() if it does not find a valid address within
    // the given number of _attempts.
    function __randomAddress(
        address _addr,
        ForbiddenAddresses _forbiddenAddresses,
        uint256 _attempts
    )
        internal
        returns (address)
    {
        // if _attempts is 0, vm.assume below will fail
        bool pass = false;
        for (uint256 i; i < _attempts; i++) {
            if (_forbiddenAddresses.forbiddenAddresses(_addr)) {
                _addr = vm.randomAddress();
                continue;
            } else {
                pass = true;
                break;
            }
        }
        vm.assume(pass);

        return _addr;
    }

    // This function returns _addr if it satisfies all given conditions, otherwise it will generate
    // a new random address that satisfies the conditions.
    // NOTE: This function will resort to vm.assume() if it does not find a valid address within
    // the given number of _attempts.
    function __randomAddress(
        address _addr,
        AddressConditionChainer _conditions,
        uint256 _attempts
    )
        internal
        returns (address)
    {
        IAddressCondition[] memory conditions = _conditions.conditions();

        // if _attempts is 0, vm.assume below will fail
        bool pass = false;
        for (uint256 i; i < _attempts; i++) {
            pass = true;
            for (uint256 j; j < conditions.length; j++) {
                if (!conditions[j].check(_addr)) {
                    pass = false;
                    break;
                }
            }
            if (pass) break;
            _addr = vm.randomAddress();
        }
        vm.assume(pass);

        return _addr;
    }

    // This function returns _bound(_value, _min, _max) unless _bound(_value, _min, _max) is
    // forbidden by _forbiddenUint256, in which case it will generate a new random uint256 that
    // is not forbidden in the _forbiddenUint256 contract.
    // NOTE: This function will resort to vm.assume() if it does not find a valid uint256 within
    // the given number of _attempts.
    function __boundExcept(
        uint256 _value,
        uint256 _min,
        uint256 _max,
        ForbiddenUints _forbiddenUints,
        uint256 _attempts
    )
        internal
        returns (uint256)
    {
        uint256 value_ = _bound(_value, _min, _max);

        // if _attempts is 0, vm.assume below will fail
        bool pass = false;
        for (uint256 i; i < _attempts; i++) {
            if (_forbiddenUints.forbiddenUints(value_)) {
                value_ = vm.randomUint(_min, _max);
                continue;
            } else {
                pass = true;
                break;
            }
        }
        vm.assume(pass);
        return value_;
    }

    // This function returns _bound(_value, _min, _max) unless _bound(_value, _min, _max) is
    // forbidden by _forbiddenUint256, in which case it will generate a new random uint256 that
    // is not forbidden in the _forbiddenUint256 contract.
    // NOTE: This function will resort to vm.assume() if it does not find a valid uint256 within
    // the given number of _attempts.
    function __boundExcept(
        int256 _value,
        int256 _min,
        int256 _max,
        ForbiddenInts _forbiddenInts,
        uint256 _attempts
    )
        internal
        view
        returns (int256)
    {
        int256 value_ = _bound(_value, _min, _max);
        bool pass = false;
        for (uint256 i; i < _attempts; i++) {
            if (_forbiddenInts.forbiddenInts(value_)) {
                value_ = _bound(vm.randomInt(), _min, _max); // no support for `vm.randomInt(_min, _max)` yet
                continue;
            } else {
                pass = true;
                break;
            }
        }
        vm.assume(pass);
        return value_;
    }

    function __randomBytes(uint256 _minLength, uint256 _maxLength) internal returns (bytes memory bytes_) {
        uint256 length = vm.randomUint(_minLength, _maxLength);
        bytes_ = vm.randomBytes(length);
    }

    uint256 private isPayableConditionCounter;

    function newIsPayableCondition() internal returns (IsPayableCondition) {
        IsPayableCondition condition = new IsPayableCondition();
        vm.label(address(condition), string.concat("IsPayableCondition:", vm.toString(isPayableConditionCounter++)));
        return condition;
    }

    uint256 private isNotPayableConditionCounter;

    function newIsNotPayableCondition() internal returns (IsNotPayableCondition) {
        IsNotPayableCondition condition = new IsNotPayableCondition();
        vm.label(
            address(condition), string.concat("IsNotPayableCondition:", vm.toString(isNotPayableConditionCounter++))
        );
        return condition;
    }

    uint256 private isNotPrecompileConditionCounter;

    function newIsNotPrecompileCondition() internal returns (IsNotPrecompileCondition) {
        IsNotPrecompileCondition condition = new IsNotPrecompileCondition();
        vm.label(
            address(condition),
            string.concat("IsNotPrecompileCondition:", vm.toString(isNotPrecompileConditionCounter++))
        );
        return condition;
    }

    uint256 private isNotForgeAddressConditionCounter;

    function newIsNotForgeAddressCondition() internal returns (IsNotForgeAddressCondition) {
        IsNotForgeAddressCondition condition = new IsNotForgeAddressCondition();
        vm.label(
            address(condition),
            string.concat("IsNotForgeAddressCondition:", vm.toString(isNotForgeAddressConditionCounter++))
        );
        return condition;
    }

    uint256 private conditionsCounter;

    function newAddressConditionChainer() internal returns (AddressConditionChainer) {
        AddressConditionChainer conditions = new AddressConditionChainer();
        vm.label(address(conditions), string.concat("AddressConditionChainer:", vm.toString(conditionsCounter++)));
        return conditions;
    }

    uint256 private forbiddenAddressesCounter;

    function newForbiddenAddresses() internal returns (ForbiddenAddresses) {
        ForbiddenAddresses forbiddenAddresses = new ForbiddenAddresses();
        vm.label(
            address(forbiddenAddresses), string.concat("ForbiddenAddresses:", vm.toString(forbiddenAddressesCounter++))
        );
        return forbiddenAddresses;
    }

    uint256 private forbiddenUint256Counter;

    function newForbiddenUints() internal returns (ForbiddenUints) {
        ForbiddenUints forbiddenUints = new ForbiddenUints();
        vm.label(address(forbiddenUints), string.concat("ForbiddenUints:", vm.toString(forbiddenUint256Counter++)));
        return forbiddenUints;
    }

    uint256 private forbiddenIntsCounter;

    function newForbiddenInts() internal returns (ForbiddenInts) {
        ForbiddenInts forbiddenInts = new ForbiddenInts();
        vm.label(address(forbiddenInts), string.concat("ForbiddenInts:", vm.toString(forbiddenIntsCounter++)));
        return forbiddenInts;
    }
}
