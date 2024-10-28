// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { CommonBase } from "forge-std/Base.sol";
import { stdToml } from "forge-std/StdToml.sol";

import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Solarray } from "scripts/libraries/Solarray.sol";

// This file follows the pattern of Superchain.s.sol. Refer to that file for more details.
contract DeployAuthSystemInput is CommonBase {
    using stdToml for string;

    // Generic safe inputs
    // Note: these will need to be replaced with settings specific to the different Safes in the system.
    uint256 internal _threshold;
    address[] internal _owners;

    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.threshold.selector) _threshold = _value;
        else revert("DeployAuthSystemInput: unknown selector");
    }

    function set(bytes4 _sel, address[] memory _addrs) public {
        if (_sel == this.owners.selector) {
            require(_owners.length == 0, "DeployAuthSystemInput: owners already set");
            for (uint256 i = 0; i < _addrs.length; i++) {
                _owners.push(_addrs[i]);
            }
        } else {
            revert("DeployAuthSystemInput: unknown selector");
        }
    }

    function threshold() public view returns (uint256) {
        require(_threshold != 0, "DeployAuthSystemInput: threshold not set");
        return _threshold;
    }

    function owners() public view returns (address[] memory) {
        require(_owners.length != 0, "DeployAuthSystemInput: owners not set");
        return _owners;
    }
}

contract DeployAuthSystemOutput is CommonBase {
    Safe internal _safe;

    function set(bytes4 _sel, address _address) public {
        if (_sel == this.safe.selector) _safe = Safe(payable(_address));
        else revert("DeployAuthSystemOutput: unknown selector");
    }

    function checkOutput() public view {
        address[] memory addrs = Solarray.addresses(address(this.safe()));
        DeployUtils.assertValidContractAddresses(addrs);
    }

    function safe() public view returns (Safe) {
        DeployUtils.assertValidContractAddress(address(_safe));
        return _safe;
    }
}

contract DeployAuthSystem is Script {
    function run(DeployAuthSystemInput _dasi, DeployAuthSystemOutput _daso) public {
        deploySafe(_dasi, _daso);
    }

    function deploySafe(DeployAuthSystemInput _dasi, DeployAuthSystemOutput _daso) public {
        address[] memory owners = _dasi.owners();
        uint256 threshold = _dasi.threshold();

        // TODO: replace with a real deployment. The safe deployment logic is fairly complex, so for the purposes of
        // this scaffolding PR we'll just etch the code.
        address safe = makeAddr("safe");
        vm.etch(safe, type(Safe).runtimeCode);
        vm.store(safe, bytes32(uint256(3)), bytes32(uint256(owners.length)));
        vm.store(safe, bytes32(uint256(4)), bytes32(uint256(threshold)));

        _daso.set(_daso.safe.selector, safe);
    }

    function etchIOContracts() public returns (DeployAuthSystemInput dasi_, DeployAuthSystemOutput daso_) {
        (dasi_, daso_) = getIOContracts();
        vm.etch(address(dasi_), type(DeployAuthSystemInput).runtimeCode);
        vm.etch(address(daso_), type(DeployAuthSystemOutput).runtimeCode);
        vm.allowCheatcodes(address(dasi_));
        vm.allowCheatcodes(address(daso_));
    }

    function getIOContracts() public view returns (DeployAuthSystemInput dasi_, DeployAuthSystemOutput daso_) {
        dasi_ = DeployAuthSystemInput(DeployUtils.toIOAddress(msg.sender, "optimism.DeployAuthSystemInput"));
        daso_ = DeployAuthSystemOutput(DeployUtils.toIOAddress(msg.sender, "optimism.DeployAuthSystemOutput"));
    }
}
