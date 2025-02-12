// SPDX-License-Identifier: MIT

pragma solidity ^0.8.0;

import { Vm } from "forge-std/Vm.sol";

abstract contract AddressConditions {
    Vm private constant vm = Vm(address(bytes20(uint160(uint256(keccak256("hevm cheat code"))))));

    // This function checks whether an address, `addr`, is payable. It works by sending 1 wei to
    // `addr` and checking the `success` return value.
    // NOTE: This function may result in state changes depending on the fallback/receive logic
    // implemented by `addr`, which should be taken into account when this function is used.
    function __isPayable(address addr) internal returns (bool) {
        require(
            addr.balance < type(uint256).max,
            "TestUtils: Balance equals max uint256, so it cannot receive any more funds"
        );
        uint256 origBalanceTest = address(this).balance;
        uint256 origBalanceAddr = address(addr).balance;

        (bool success,) = payable(addr).call{ value: 1 }("");

        // reset balances
        vm.deal(address(this), origBalanceTest);
        vm.deal(addr, origBalanceAddr);

        return success;
    }

    // This function checks whether an address, `addr`, is not payable. It works by sending 1 wei to
    // `addr` and checking the `success` return value.
    // NOTE: This function may result in state changes depending on the fallback/receive logic
    // implemented by `addr`, which should be taken into account when this function is used.
    function __isNotPayable(address addr) internal returns (bool) {
        return !__isPayable(addr);
    }

    // This function checks whether an address, `addr`, is not a precompile on OP main/test net.
    function __isNotPrecompile(address addr) internal pure returns (bool) {
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
    function __isNotForgeAddress(address addr) internal pure returns (bool) {
        // vm, console, and Create2Deployer addresses
        return (
            addr != address(vm) && addr != 0x000000000000000000636F6e736F6c652e6c6f67
                && addr != 0x4e59b44847b379578588920cA78FbF26c0B4956C
        );
    }
}

interface IAddressCondition {
    function check(address _addr) external returns (bool);
}

contract IsPayableCondition is IAddressCondition, AddressConditions {
    function check(address _addr) external returns (bool) {
        return __isPayable(_addr);
    }
}

contract IsNotPayableCondition is IAddressCondition, AddressConditions {
    function check(address _addr) external returns (bool) {
        return __isNotPayable(_addr);
    }
}

contract IsNotPrecompileCondition is IAddressCondition, AddressConditions {
    function check(address _addr) external pure returns (bool) {
        return __isNotPrecompile(_addr);
    }
}

contract IsNotForgeAddressCondition is IAddressCondition, AddressConditions {
    function check(address _addr) external pure returns (bool) {
        return __isNotForgeAddress(_addr);
    }
}

contract AddressConditionChainer {
    IAddressCondition[] internal _conditions;

    function p(IAddressCondition _condition) public returns (AddressConditionChainer) {
        _conditions.push(_condition);
        return this;
    }

    function conditions() public view returns (IAddressCondition[] memory) {
        return _conditions;
    }
}
