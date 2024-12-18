// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";

import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { AlphabetVM } from "test/mocks/AlphabetVM.sol";
import { Claim } from "src/dispute/lib/Types.sol";

contract DeployAlphabetVMInput is BaseDeployIO {
    bytes32 internal _absolutePrestate;
    IPreimageOracle internal _preimageOracle;

    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployAlphabetVMInput: cannot set zero address");

        if (_sel == this.preimageOracle.selector) _preimageOracle = IPreimageOracle(_addr);
        else revert("DeployAlphabetVMInput: unknown selector");
    }

    function set(bytes4 _sel, bytes32 _value) public {
        if (_sel == this.absolutePrestate.selector) _absolutePrestate = _value;
        else revert("DeployAlphabetVMInput: unknown selector");
    }

    function absolutePrestate() public view returns (bytes32) {
        require(_absolutePrestate != bytes32(0), "DeployAlphabetVMInput: not set");
        return _absolutePrestate;
    }

    function preimageOracle() public view returns (IPreimageOracle) {
        require(address(_preimageOracle) != address(0), "DeployAlphabetVMInput: not set");
        return _preimageOracle;
    }
}

contract DeployAlphabetVMOutput is BaseDeployIO {
    AlphabetVM internal _alphabetVM;

    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployAlphabetVMOutput: cannot set zero address");
        if (_sel == this.alphabetVM.selector) _alphabetVM = AlphabetVM(_addr);
        else revert("DeployAlphabetVMOutput: unknown selector");
    }

    function alphabetVM() public view returns (AlphabetVM) {
        require(address(_alphabetVM) != address(0), "DeployAlphabetVMOutput: not set");
        return _alphabetVM;
    }
}

contract DeployAlphabetVM is Script {
    function run(DeployAlphabetVMInput _input, DeployAlphabetVMOutput _output) public {
        Claim absolutePrestate = Claim.wrap(_input.absolutePrestate());
        IPreimageOracle preimageOracle = _input.preimageOracle();

        vm.broadcast(msg.sender);
        AlphabetVM alphabetVM = new AlphabetVM(absolutePrestate, preimageOracle);

        _output.set(_output.alphabetVM.selector, address(alphabetVM));
    }
}
