// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { LibString } from "@solady/utils/LibString.sol";

// Interfaces
import { IBigStepper } from "src/dispute/interfaces/IBigStepper.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";
import { IMIPS } from "src/cannon/interfaces/IMIPS.sol";

/// @title DeployMIPSInput
contract DeployMIPSInput is BaseDeployIO {
    // Common inputs.
    string internal _release;
    string internal _standardVersionsToml;

    // All inputs required to deploy PreimageOracle.
    uint256 internal _minProposalSizeBytes;
    uint256 internal _challengePeriodSeconds;

    // Specify which MIPS version to use.
    uint256 internal _mipsVersion;

    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.mipsVersion.selector) {
            require(_value == 1 || _value == 2, "DeployMIPS: unknown mips version");
            _mipsVersion = _value;
        } else if (_sel == this.minProposalSizeBytes.selector) {
            require(_value != 0, "DeployMIPS: minProposalSizeBytes cannot be zero");
            _minProposalSizeBytes = _value;
        } else if (_sel == this.challengePeriodSeconds.selector) {
            require(_value != 0, "DeployMIPS: challengePeriodSeconds cannot be zero");
            _challengePeriodSeconds = _value;
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function set(bytes4 _sel, string memory _value) public {
        if (_sel == this.release.selector) {
            require(!LibString.eq(_value, ""), "DeployMIPS: release cannot be empty");
            _release = _value;
        } else if (_sel == this.standardVersionsToml.selector) {
            require(!LibString.eq(_value, ""), "DeployMIPS: standardVersionsToml cannot be empty");
            _standardVersionsToml = _value;
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function release() public view returns (string memory) {
        require(!LibString.eq(_release, ""), "DeployMIPS: release not set");
        return _release;
    }

    function standardVersionsToml() public view returns (string memory) {
        require(!LibString.eq(_standardVersionsToml, ""), "DeployMIPS: standardVersionsToml not set");
        return _standardVersionsToml;
    }

    function mipsVersion() public view returns (uint256) {
        require(_mipsVersion != 0, "DeployMIPS: mipsVersion not set");
        require(_mipsVersion == 1 || _mipsVersion == 2, "DeployMIPS: unknown mips version");
        return _mipsVersion;
    }

    function minProposalSizeBytes() public view returns (uint256) {
        require(_minProposalSizeBytes != 0, "DeployMIPS: minProposalSizeBytes not set");
        return _minProposalSizeBytes;
    }

    function challengePeriodSeconds() public view returns (uint256) {
        require(_challengePeriodSeconds != 0, "DeployMIPS: challengePeriodSeconds not set");
        return _challengePeriodSeconds;
    }
}

/// @title DeployMIPSOutput
contract DeployMIPSOutput is BaseDeployIO {
    IMIPS internal _mipsSingleton;
    IPreimageOracle internal _preimageOracleSingleton;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.mipsSingleton.selector) {
            require(_value != address(0), "DeployMIPS: mipsSingleton cannot be zero address");
            _mipsSingleton = IMIPS(_value);
        } else if (_sel == this.preimageOracleSingleton.selector) {
            require(_value != address(0), "DeployMIPS: preimageOracleSingleton cannot be zero address");
            _preimageOracleSingleton = IPreimageOracle(_value);
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function checkOutput(DeployMIPSInput _mi) public view {
        DeployUtils.assertValidContractAddress(address(_preimageOracleSingleton));
        DeployUtils.assertValidContractAddress(address(_mipsSingleton));
        assertValidDeploy(_mi);
    }

    function preimageOracleSingleton() public view returns (IPreimageOracle) {
        DeployUtils.assertValidContractAddress(address(_preimageOracleSingleton));
        return _preimageOracleSingleton;
    }

    function mipsSingleton() public view returns (IMIPS) {
        DeployUtils.assertValidContractAddress(address(_mipsSingleton));
        return _mipsSingleton;
    }

    function assertValidDeploy(DeployMIPSInput _mi) public view {
        assertValidPreimageOracleSingleton(_mi);
        assertValidMipsSingleton(_mi);
    }

    function assertValidPreimageOracleSingleton(DeployMIPSInput _mi) internal view {
        IPreimageOracle oracle = preimageOracleSingleton();

        require(oracle.minProposalSize() == _mi.minProposalSizeBytes(), "PO-10");
        require(oracle.challengePeriod() == _mi.challengePeriodSeconds(), "PO-20");
    }

    function assertValidMipsSingleton(DeployMIPSInput) internal view {
        IMIPS mips = mipsSingleton();

        require(address(mips.oracle()) == address(preimageOracleSingleton()), "MIPS-10");
    }
}

/// @title DeployMIPS
contract DeployMIPS is Script {
    function run(DeployMIPSInput _mi, DeployMIPSOutput _dgo) public {
        deployPreimageOracleSingleton(_mi, _dgo);
        deployMipsSingleton(_mi, _dgo);
        _dgo.checkOutput(_mi);
    }

    function deployPreimageOracleSingleton(DeployMIPSInput _mi, DeployMIPSOutput _mo) internal {
        string memory release = _mi.release();
        string memory stdVerToml = _mi.standardVersionsToml();
        string memory contractName = "preimage_oracle";
        IPreimageOracle singleton;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            singleton = IPreimageOracle(payable(existingImplementation));
        } else if (isDevelopRelease(release)) {
            uint256 minProposalSizeBytes = _mi.minProposalSizeBytes();
            uint256 challengePeriodSeconds = _mi.challengePeriodSeconds();
            vm.broadcast(msg.sender);
            singleton = IPreimageOracle(
                DeployUtils.create1({
                    _name: "PreimageOracle",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(IPreimageOracle.__constructor__, (minProposalSizeBytes, challengePeriodSeconds))
                    )
                })
            );
        } else {
            revert(string.concat("DeployImplementations: failed to deploy release ", release));
        }

        vm.label(address(singleton), "PreimageOracleSingleton");
        _mo.set(_mo.preimageOracleSingleton.selector, address(singleton));
    }

    function deployMipsSingleton(DeployMIPSInput _dgi, DeployMIPSOutput _dgo) internal {
        string memory release = _dgi.release();
        string memory stdVerToml = _dgi.standardVersionsToml();
        string memory contractName = "mips";
        IMIPS singleton;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            singleton = IMIPS(payable(existingImplementation));
        } else if (isDevelopRelease(release)) {
            uint256 mipsVersion = _dgi.mipsVersion();
            IPreimageOracle preimageOracle = IPreimageOracle(address(_dgo.preimageOracleSingleton()));
            vm.broadcast(msg.sender);
            singleton = IMIPS(
                DeployUtils.create1({
                    _name: mipsVersion == 1 ? "MIPS" : "MIPS2",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IMIPS.__constructor__, (preimageOracle)))
                })
            );
        } else {
            revert(string.concat("DeployImplementations: failed to deploy release ", release));
        }

        vm.label(address(singleton), "MIPSSingleton");
        _dgo.set(_dgo.mipsSingleton.selector, address(singleton));
    }

    // Zero address is returned if the address is not found in '_standardVersionsToml'.
    function getReleaseAddress(
        string memory _version,
        string memory _contractName,
        string memory _standardVersionsToml
    )
        internal
        pure
        returns (address addr_)
    {
        string memory baseKey = string.concat('.releases["', _version, '"].', _contractName);
        string memory implAddressKey = string.concat(baseKey, ".implementation_address");
        string memory addressKey = string.concat(baseKey, ".address");
        try vm.parseTomlAddress(_standardVersionsToml, implAddressKey) returns (address parsedAddr_) {
            addr_ = parsedAddr_;
        } catch {
            try vm.parseTomlAddress(_standardVersionsToml, addressKey) returns (address parsedAddr_) {
                addr_ = parsedAddr_;
            } catch {
                addr_ = address(0);
            }
        }
    }

    // A release is considered a 'develop' release if it does not start with 'op-contracts'.
    function isDevelopRelease(string memory _release) internal pure returns (bool) {
        return !LibString.startsWith(_release, "op-contracts");
    }
}
