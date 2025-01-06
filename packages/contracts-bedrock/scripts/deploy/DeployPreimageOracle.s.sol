// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";

import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

contract DeployPreimageOracleInput is BaseDeployIO {
    uint256 internal _minProposalSize;
    uint256 internal _challengePeriod;

    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.minProposalSize.selector) _minProposalSize = _value;
        else if (_sel == this.challengePeriod.selector) _challengePeriod = _value;
        else revert("DeployPreimageOracleInput: unknown selector");
    }

    function minProposalSize() public view returns (uint256) {
        require(_minProposalSize > 0, "DeployPreimageOracleInput: not set");
        return _minProposalSize;
    }

    function challengePeriod() public view returns (uint256) {
        require(_challengePeriod > 0, "DeployPreimageOracleInput: not set");
        return _challengePeriod;
    }
}

contract DeployPreimageOracleOutput is BaseDeployIO {
    IPreimageOracle internal _preimageOracle;

    function set(bytes4 _sel, address _addr) public {
        require(_addr != address(0), "DeployPreimageOracleOutput: cannot set zero address");

        if (_sel == this.preimageOracle.selector) _preimageOracle = IPreimageOracle(_addr);
        else revert("DeployPreimageOracleOutput: unknown selector");
    }

    function preimageOracle() public view returns (IPreimageOracle) {
        require(address(_preimageOracle) != address(0), "DeployPreimageOracleOutput: not set");
        return _preimageOracle;
    }
}

contract DeployPreimageOracle is Script {
    function run(DeployPreimageOracleInput _input, DeployPreimageOracleOutput _output) public {
        uint256 minProposalSize = _input.minProposalSize();
        uint256 challengePeriod = _input.challengePeriod();

        vm.broadcast(msg.sender);
        IPreimageOracle preimageOracle = IPreimageOracle(
            DeployUtils.create1({
                _name: "PreimageOracle",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IPreimageOracle.__constructor__, (minProposalSize, challengePeriod))
                )
            })
        );

        _output.set(_output.preimageOracle.selector, address(preimageOracle));
        assertValid(_input, _output);
    }

    function assertValid(DeployPreimageOracleInput _input, DeployPreimageOracleOutput _output) public view {
        IPreimageOracle oracle = _output.preimageOracle();
        require(address(oracle) != address(0), "DPO-10");
        require(oracle.minProposalSize() == _input.minProposalSize(), "DPO-20");
        require(oracle.challengePeriod() == _input.challengePeriod(), "DPO-30");
    }
}
