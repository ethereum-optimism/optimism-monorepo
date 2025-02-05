// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IRISCV } from "interfaces/vendor/asterisc/IRISCV.sol";

/// @title DeployAsteriscInput
contract DeployAsteriscInput is BaseDeployIO {
    // Specify the PreimageOracle to use
    address internal _preimageOracle;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.preimageOracle.selector) {
            require(_value != address(0), "DeployAsterisc: preimageOracle cannot be empty");
            _preimageOracle = _value;
        } else {
            revert("DeployAsterisc: unknown selector");
        }
    }

    function preimageOracle() public view returns (address) {
        require(_preimageOracle != address(0), "DeployAsterisc: preimageOracle not set");
        return _preimageOracle;
    }
}

/// @title DeployAsteriscOutput
contract DeployAsteriscOutput is BaseDeployIO {
    IRISCV internal _asteriscSingleton;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.asteriscSingleton.selector) {
            require(_value != address(0), "DeployAsterisc: asteriscSingleton cannot be zero address");
            _asteriscSingleton = IRISCV(_value);
        } else {
            revert("DeployAsterisc: unknown selector");
        }
    }

    function checkOutput(DeployAsteriscInput _mi) public view {
        DeployUtils.assertValidContractAddress(address(_asteriscSingleton));
        assertValidDeploy(_mi);
    }

    function asteriscSingleton() public view returns (IRISCV) {
        DeployUtils.assertValidContractAddress(address(_asteriscSingleton));
        return _asteriscSingleton;
    }

    function assertValidDeploy(DeployAsteriscInput _mi) public view {
        assertValidAsteriscSingleton(_mi);
    }

    function assertValidAsteriscSingleton(DeployAsteriscInput _mi) internal view {
        IRISCV asterisc = asteriscSingleton();

        require(address(asterisc.oracle()) == address(_mi.preimageOracle()), "ASTERISC-10");
    }
}

/// @title DeployAsterisc
contract DeployAsterisc is Script {
    function run(DeployAsteriscInput _mi, DeployAsteriscOutput _mo) public {
        DeployAsteriscSingleton(_mi, _mo);
        _mo.checkOutput(_mi);
    }

    function DeployAsteriscSingleton(DeployAsteriscInput _mi, DeployAsteriscOutput _mo) internal {
        IPreimageOracle preimageOracle = IPreimageOracle(_mi.preimageOracle());
        vm.broadcast(msg.sender);
        IRISCV singleton = IRISCV(
            DeployUtils.create1({
                _name: "RISCV",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IRISCV.__constructor__, (preimageOracle)))
            })
        );

        vm.label(address(singleton), "AsteriscSingleton");
        _mo.set(_mo.asteriscSingleton.selector, address(singleton));
    }
}
