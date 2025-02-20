// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IMIPS } from "interfaces/cannon/IMIPS.sol";

/// @title DeployMIPSInput
contract DeployMIPSInput is BaseDeployIO {
    // Specify the PreimageOracle to use
    address internal _preimageOracle;

    // Specify which MIPS version to use.
    uint256 internal _mipsVersion;

    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.mipsVersion.selector) {
            require(_value == 1 || _value == 2, "DeployMIPS: unknown mips version");
            _mipsVersion = _value;
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.preimageOracle.selector) {
            require(_value != address(0), "DeployMIPS: preimageOracle cannot be empty");
            _preimageOracle = _value;
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function mipsVersion() public view returns (uint256) {
        require(_mipsVersion != 0, "DeployMIPS: mipsVersion not set");
        require(_mipsVersion == 1 || _mipsVersion == 2, "DeployMIPS: unknown mips version");
        return _mipsVersion;
    }

    function preimageOracle() public view returns (address) {
        require(_preimageOracle != address(0), "DeployMIPS: preimageOracle not set");
        return _preimageOracle;
    }
}

/// @title DeployMIPSOutput
contract DeployMIPSOutput is BaseDeployIO {
    IMIPS internal _mipsSingleton;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.mipsSingleton.selector) {
            require(_value != address(0), "DeployMIPS: mipsSingleton cannot be zero address");
            _mipsSingleton = IMIPS(_value);
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function mipsSingleton() public view returns (IMIPS) {
        DeployUtils.assertValidContractAddress(address(_mipsSingleton));
        return _mipsSingleton;
    }
}

/// @title DeployMIPS
contract DeployMIPS is Script {
    function run(DeployMIPSInput _mi, DeployMIPSOutput _mo) public {
        deployMipsSingleton(_mi, _mo);
        assertValidDeploy(_mi, _mo);
    }

    function deployMipsSingleton(DeployMIPSInput _mi, DeployMIPSOutput _mo) internal {
        IMIPS singleton;
        uint256 mipsVersion = _mi.mipsVersion();
        IPreimageOracle preimageOracle = IPreimageOracle(_mi.preimageOracle());
        singleton = IMIPS(
            DeployUtils.createDeterministic({
                _name: mipsVersion == 1 ? "MIPS" : "MIPS64",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IMIPS.__constructor__, (preimageOracle))),
                _salt: DeployUtils.DEFAULT_SALT
            })
        );

        vm.label(address(singleton), "MIPSSingleton");
        _mo.set(_mo.mipsSingleton.selector, address(singleton));
    }

    function assertValidDeploy(DeployMIPSInput _mi, DeployMIPSOutput _mo) public view {
        DeployUtils.assertValidContractAddress(address(_mo.mipsSingleton()));
        assertValidMipsSingleton(_mi, _mo);
    }

    function assertValidMipsSingleton(DeployMIPSInput _mi, DeployMIPSOutput _mo) internal view {
        IMIPS mips = _mo.mipsSingleton();
        require(address(mips.oracle()) == address(_mi.preimageOracle()), "MIPS-10");
    }
}
