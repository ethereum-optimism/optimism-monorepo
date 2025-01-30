// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Script } from "forge-std/Script.sol";
import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";

contract UpgradeOPChainInput is BaseDeployIO {
    address internal _prank;
    OPContractsManager internal _opcm;
    bytes _opChain;

    // Setter for OPContractsManager type
    function set(bytes4 _sel, address _value) public {
        require(address(_value) != address(0), "UpgradeOPCMInput: cannot set zero address");

        if (_sel == this.prank.selector) _prank = _value;
        else if (_sel == this.opcm.selector) _opcm = OPContractsManager(_value);
        else revert("UpgradeOPCMInput: unknown selector");
    }

    function set(bytes4 _sel, OPContractsManager.OpChain[] memory _value) public {
        require(_value.length > 0, "UpgradeOPCMInput: cannot set empty array");

        if (_sel == this.opChain.selector) _opChain = abi.encode(_value);
        else revert("UpgradeOPCMInput: unknown selector");
    }

    function prank() public view returns (address) {
        require(address(_prank) != address(0), "UpgradeOPCMInput: prank not set");
        return _prank;
    }

    function opcm() public view returns (OPContractsManager) {
        require(address(_opcm) != address(0), "UpgradeOPCMInput: not set");
        return _opcm;
    }

    function opChain() public view returns (bytes memory) {
        require(_opChain.length > 0, "UpgradeOPCMInput: not set");
        return _opChain;
    }
}

contract UpgradeOPChain is Script {
    function run(UpgradeOPChainInput _uoci) external {
        OPContractsManager opcm = _uoci.opcm();
        OPContractsManager.OpChain[] memory opChain = abi.decode(_uoci.opChain(), (OPContractsManager.OpChain[]));

        // Etch DummyCaller contract. This contract is used to mimic the contract that is used
        // as the source of the delegatecall to the OPCM. In practice this will be the governance
        // 2/2 or similar.
        address prank = _uoci.prank();
        bytes memory code = vm.getDeployedCode("UpgradeOPChain.s.sol:DummyCaller");
        vm.etch(prank, code);
        vm.store(prank, bytes32(0), bytes32(uint256(uint160(address(opcm)))));
        vm.label(prank, "DummyCaller");

        // Call into the DummyCaller. This will perform the delegatecall under the hood and
        // return the result.
        vm.broadcast(msg.sender);
        (bool success,) = DummyCaller(prank).upgrade(opChain);
        require(success, "UpgradeChain: upgrade failed");
    }
}

contract DummyCaller {
    address internal _opcmAddr;

    function upgrade(OPContractsManager.OpChain[] memory _opChain) external returns (bool, bytes memory) {
        bytes memory data = abi.encodeCall(DummyCaller.upgrade, _opChain);
        (bool success, bytes memory result) = _opcmAddr.delegatecall(data);
        return (success, result);
    }
}
