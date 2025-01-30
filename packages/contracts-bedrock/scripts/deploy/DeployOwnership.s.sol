// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { console2 as console } from "forge-std/console2.sol";

import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { GnosisSafeProxyFactory as SafeProxyFactory } from "safe-contracts/proxies/GnosisSafeProxyFactory.sol";
import { OwnerManager } from "safe-contracts/base/OwnerManager.sol";
import { ModuleManager } from "safe-contracts/base/ModuleManager.sol";
import { GuardManager } from "safe-contracts/base/GuardManager.sol";
import { Enum as SafeOps } from "safe-contracts/common/Enum.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

import { LivenessGuard } from "src/safe/LivenessGuard.sol";
import { LivenessModule } from "src/safe/LivenessModule.sol";
import { DeputyGuardianModule } from "src/safe/DeputyGuardianModule.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

import { Deploy } from "./Deploy.s.sol";

/// @notice Configuration for a Safe
struct SafeConfig {
    uint256 threshold;
    address[] owners;
}

/// @notice Configuration for the Liveness Module
struct LivenessModuleConfig {
    uint256 livenessInterval;
    uint256 thresholdPercentage;
    uint256 minOwners;
    address fallbackOwner;
}

/// @notice Configuration for the Security Council Safe.
struct SecurityCouncilConfig {
    SafeConfig safeConfig;
    LivenessModuleConfig livenessModuleConfig;
}

/// @notice Configuration for the Deputy Guardian Module
struct DeputyGuardianModuleConfig {
    address deputyGuardian;
    ISuperchainConfig superchainConfig;
}

/// @notice Configuration for the Guardian Safe.
struct GuardianConfig {
    SafeConfig safeConfig;
    DeputyGuardianModuleConfig deputyGuardianModuleConfig;
}

/// @title DeployOwnership
/// @notice Script used to deploy and configure the Safe contracts which are used to manage the Superchain,
///         as the ProxyAdminOwner and other roles in the system. Note that this script is not executable in a
///         production environment as some steps depend on having a quorum of signers available. This script is meant to
///         be used as an example to guide the setup and configuration of the Safe contracts.
contract DeployOwnership is Deploy {
    /// @notice Internal function containing the deploy logic.
    function _run(bool) internal override {
        console.log("start of Ownership Deployment");
        // The SuperchainConfig is needed as a constructor argument to the Deputy Guardian Module
        deploySuperchainConfig();

        deployFoundationOperationsSafe();
        deployFoundationUpgradeSafe();
        deploySecurityCouncilSafe();
        deployGuardianSafe();
        configureGuardianSafe();
        configureSecurityCouncilSafe();

        console.log("Ownership contracts completed");
    }

    /// @notice Returns a SafeConfig similar to that of the Foundation Safe on Mainnet.
    function _getExampleFoundationConfig() internal returns (SafeConfig memory safeConfig_) {
        address[] memory exampleFoundationOwners = new address[](7);
        for (uint256 i; i < exampleFoundationOwners.length; i++) {
            exampleFoundationOwners[i] = makeAddr(string.concat("fnd-", vm.toString(i)));
        }
        safeConfig_ = SafeConfig({ threshold: 5, owners: exampleFoundationOwners });
    }

    /// @notice Returns a GuardianConfig similar to that of the Guardian Safe on Mainnet.
    function _getExampleGuardianConfig() internal view returns (GuardianConfig memory guardianConfig_) {
        address[] memory exampleGuardianOwners = new address[](1);
        exampleGuardianOwners[0] = artifacts.mustGetAddress("SecurityCouncilSafe");
        guardianConfig_ = GuardianConfig({
            safeConfig: SafeConfig({ threshold: 1, owners: exampleGuardianOwners }),
            deputyGuardianModuleConfig: DeputyGuardianModuleConfig({
                deputyGuardian: artifacts.mustGetAddress("FoundationOperationsSafe"),
                superchainConfig: ISuperchainConfig(artifacts.mustGetAddress("SuperchainConfigImpl"))
            })
        });
    }

    /// @notice Returns a SafeConfig similar to that of the Security Council Safe on Mainnet.
    function _getExampleCouncilConfig() internal returns (SecurityCouncilConfig memory councilConfig_) {
        address[] memory exampleCouncilOwners = new address[](13);
        for (uint256 i; i < exampleCouncilOwners.length; i++) {
            exampleCouncilOwners[i] = makeAddr(string.concat("sc-", vm.toString(i)));
        }
        SafeConfig memory safeConfig = SafeConfig({ threshold: 10, owners: exampleCouncilOwners });
        councilConfig_ = SecurityCouncilConfig({
            safeConfig: safeConfig,
            livenessModuleConfig: LivenessModuleConfig({
                livenessInterval: 14 weeks,
                thresholdPercentage: 75,
                minOwners: 8,
                fallbackOwner: artifacts.mustGetAddress("FoundationUpgradeSafe")
            })
        });
    }

    /// @notice Make a call from the Safe contract to an arbitrary address with arbitrary data
    function _callViaSafe(Safe _safe, address _target, bytes memory _data) internal {
        // This is the signature format used when the caller is also the signer.
        bytes memory signature = abi.encodePacked(uint256(uint160(msg.sender)), bytes32(0), uint8(1));

        _safe.execTransaction({
            to: _target,
            value: 0,
            data: _data,
            operation: SafeOps.Operation.Call,
            safeTxGas: 0,
            baseGas: 0,
            gasPrice: 0,
            gasToken: address(0),
            refundReceiver: payable(address(0)),
            signatures: signature
        });
    }

    /// @notice Deploy the Safe
    function deploySafe(string memory _name) public broadcast returns (address addr_) {
        address[] memory owners = new address[](0);
        addr_ = deploySafe(_name, owners, 1, true);
    }

    /// @notice Deploy a new Safe contract. If the keepDeployer option is used to enable further setup actions, then
    ///         the removeDeployerFromSafe() function should be called on that safe after setup is complete.
    ///         Note this function does not have the broadcast modifier.
    /// @param _name The name of the Safe to deploy.
    /// @param _owners The owners of the Safe.
    /// @param _threshold The threshold of the Safe.
    /// @param _keepDeployer Whether or not the deployer address will be added as an owner of the Safe.
    function deploySafe(
        string memory _name,
        address[] memory _owners,
        uint256 _threshold,
        bool _keepDeployer
    )
        public
        returns (address addr_)
    {
        bytes32 salt = keccak256(abi.encode(_name, _implSalt()));
        console.log("Deploying safe: %s with salt %s", _name, vm.toString(salt));
        (SafeProxyFactory safeProxyFactory, Safe safeSingleton) = _getSafeFactory();

        if (_keepDeployer) {
            address[] memory expandedOwners = new address[](_owners.length + 1);
            // By always adding msg.sender first we know that the previousOwner will be SENTINEL_OWNERS, which makes it
            // easier to call removeOwner later.
            expandedOwners[0] = msg.sender;
            for (uint256 i = 0; i < _owners.length; i++) {
                expandedOwners[i + 1] = _owners[i];
            }
            _owners = expandedOwners;
        }

        bytes memory initData = abi.encodeCall(
            Safe.setup, (_owners, _threshold, address(0), hex"", address(0), address(0), 0, payable(address(0)))
        );
        addr_ = address(safeProxyFactory.createProxyWithNonce(address(safeSingleton), initData, uint256(salt)));

        artifacts.save(_name, addr_);
        console.log("New safe: %s deployed at %s\n    Note that this safe is owned by the deployer key", _name, addr_);
    }

    /// @notice If the keepDeployer option was used with deploySafe(), this function can be used to remove the deployer.
    ///         Note this function does not have the broadcast modifier.
    function removeDeployerFromSafe(string memory _name, uint256 _newThreshold) public {
        Safe safe = Safe(artifacts.mustGetAddress(_name));

        // The sentinel address is used to mark the start and end of the linked list of owners in the Safe.
        address sentinelOwners = address(0x1);

        // Because deploySafe() always adds msg.sender first (if keepDeployer is true), we know that the previousOwner
        // will be sentinelOwners.
        _callViaSafe({
            _safe: safe,
            _target: address(safe),
            _data: abi.encodeCall(OwnerManager.removeOwner, (sentinelOwners, msg.sender, _newThreshold))
        });
        console.log("Removed deployer owner from ", _name);
    }

    /// @notice Gets the address of the SafeProxyFactory and Safe singleton for use in deploying a new GnosisSafe.
    function _getSafeFactory() internal returns (SafeProxyFactory safeProxyFactory_, Safe safeSingleton_) {
        if (artifacts.getAddress("SafeProxyFactory") != address(0)) {
            // The SafeProxyFactory is already saved, we can just use it.
            safeProxyFactory_ = SafeProxyFactory(artifacts.getAddress("SafeProxyFactory"));
            safeSingleton_ = Safe(artifacts.getAddress("SafeSingleton"));
            return (safeProxyFactory_, safeSingleton_);
        }

        // These are the standard create2 deployed contracts. First we'll check if they are deployed,
        // if not we'll deploy new ones, though not at these addresses.
        address safeProxyFactory = 0xa6B71E26C5e0845f74c812102Ca7114b6a896AB2;
        address safeSingleton = 0xd9Db270c1B5E3Bd161E8c8503c55cEABeE709552;

        safeProxyFactory.code.length == 0
            ? safeProxyFactory_ = new SafeProxyFactory()
            : safeProxyFactory_ = SafeProxyFactory(safeProxyFactory);

        safeSingleton.code.length == 0 ? safeSingleton_ = new Safe() : safeSingleton_ = Safe(payable(safeSingleton));

        artifacts.save("SafeProxyFactory", address(safeProxyFactory_));
        artifacts.save("SafeSingleton", address(safeSingleton_));
    }

    /// @notice Deploys a Safe with a configuration similar to that of the Foundation Safe on Mainnet.
    function deployFoundationOperationsSafe() public broadcast returns (address addr_) {
        SafeConfig memory exampleFoundationConfig = _getExampleFoundationConfig();
        addr_ = deploySafe({
            _name: "FoundationOperationsSafe",
            _owners: exampleFoundationConfig.owners,
            _threshold: exampleFoundationConfig.threshold,
            _keepDeployer: false
        });
    }

    /// @notice Deploys a Safe with a configuration similar to that of the Foundation Safe on Mainnet.
    function deployFoundationUpgradeSafe() public broadcast returns (address addr_) {
        SafeConfig memory exampleFoundationConfig = _getExampleFoundationConfig();
        addr_ = deploySafe({
            _name: "FoundationUpgradeSafe",
            _owners: exampleFoundationConfig.owners,
            _threshold: exampleFoundationConfig.threshold,
            _keepDeployer: false
        });
    }

    /// @notice Deploy a LivenessGuard for use on the Security Council Safe.
    ///         Note this function does not have the broadcast modifier.
    function deployLivenessGuard() public returns (address addr_) {
        Safe councilSafe = Safe(payable(artifacts.mustGetAddress("SecurityCouncilSafe")));
        addr_ = address(new LivenessGuard(councilSafe));

        artifacts.save("LivenessGuard", address(addr_));
        console.log("New LivenessGuard deployed at %s", address(addr_));
    }

    /// @notice Deploy a LivenessModule for use on the Security Council Safe
    ///         Note this function does not have the broadcast modifier.
    function deployLivenessModule() public returns (address addr_) {
        Safe councilSafe = Safe(payable(artifacts.mustGetAddress("SecurityCouncilSafe")));
        address guard = artifacts.mustGetAddress("LivenessGuard");
        LivenessModuleConfig memory livenessModuleConfig = _getExampleCouncilConfig().livenessModuleConfig;

        addr_ = address(
            new LivenessModule({
                _safe: councilSafe,
                _livenessGuard: LivenessGuard(guard),
                _livenessInterval: livenessModuleConfig.livenessInterval,
                _thresholdPercentage: livenessModuleConfig.thresholdPercentage,
                _minOwners: livenessModuleConfig.minOwners,
                _fallbackOwner: livenessModuleConfig.fallbackOwner
            })
        );

        artifacts.save("LivenessModule", address(addr_));
        console.log("New LivenessModule deployed at %s", address(addr_));
    }

    /// @notice Deploy a DeputyGuardianModule for use on the Security Council Safe.
    ///         Note this function does not have the broadcast modifier.
    function deployDeputyGuardianModule() public returns (address addr_) {
        Safe guardianSafe = Safe(payable(artifacts.mustGetAddress("GuardianSafe")));
        DeputyGuardianModuleConfig memory deputyGuardianModuleConfig =
            _getExampleGuardianConfig().deputyGuardianModuleConfig;
        addr_ = address(
            new DeputyGuardianModule({
                _safe: guardianSafe,
                _superchainConfig: deputyGuardianModuleConfig.superchainConfig,
                _deputyGuardian: deputyGuardianModuleConfig.deputyGuardian
            })
        );

        artifacts.save("DeputyGuardianModule", addr_);
        console.log("New DeputyGuardianModule deployed at %s", addr_);
    }

    /// @notice Deploy a Security Council Safe.
    function deploySecurityCouncilSafe() public broadcast returns (address addr_) {
        // Deploy the safe with the extra deployer key, and keep the threshold at 1 to allow for further setup.
        SecurityCouncilConfig memory exampleCouncilConfig = _getExampleCouncilConfig();
        addr_ = payable(
            deploySafe({
                _name: "SecurityCouncilSafe",
                _owners: exampleCouncilConfig.safeConfig.owners,
                _threshold: 1,
                _keepDeployer: true
            })
        );
    }

    /// @notice Deploy Guardian Safe.
    function deployGuardianSafe() public broadcast returns (address addr_) {
        // Config is hardcoded here as the Guardian Safe's configuration is inflexible.
        address[] memory owners = new address[](1);
        owners[0] = artifacts.mustGetAddress("SecurityCouncilSafe");
        addr_ = deploySafe({ _name: "GuardianSafe", _owners: owners, _threshold: 1, _keepDeployer: true });

        console.log("Deployed and configured the Guardian Safe!");
    }

    /// @notice Deploy the SuperchainConfig contract
    function deploySuperchainConfig() public broadcast {
        ISuperchainConfig superchainConfig = ISuperchainConfig(
            DeployUtils.create2AndSave({
                _save: artifacts,
                _salt: _implSalt(),
                _name: "SuperchainConfig",
                _nick: "SuperchainConfigImpl",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(ISuperchainConfig.__constructor__, ()))
            })
        );

        require(superchainConfig.guardian() == address(0), "SuperchainConfig: guardian must be address(0)");
        bytes32 initialized = vm.load(address(superchainConfig), bytes32(0));
        require(initialized != 0, "SuperchainConfig: must be initialized");
    }

    /// @notice Configure the Guardian Safe with the DeputyGuardianModule.
    function configureGuardianSafe() public broadcast returns (address addr_) {
        addr_ = artifacts.mustGetAddress("GuardianSafe");
        address deputyGuardianModule = deployDeputyGuardianModule();
        _callViaSafe({
            _safe: Safe(payable(addr_)),
            _target: addr_,
            _data: abi.encodeCall(ModuleManager.enableModule, (deputyGuardianModule))
        });

        // Finalize configuration by removing the additional deployer key.
        removeDeployerFromSafe({ _name: "GuardianSafe", _newThreshold: 1 });
        console.log("DeputyGuardianModule enabled on GuardianSafe");
    }

    /// @notice Configure the Security Council Safe with the LivenessModule and LivenessGuard.
    function configureSecurityCouncilSafe() public broadcast returns (address addr_) {
        // Deploy and add the Deputy Guardian Module.
        SecurityCouncilConfig memory exampleCouncilConfig = _getExampleCouncilConfig();
        Safe safe = Safe(artifacts.mustGetAddress("SecurityCouncilSafe"));

        // Deploy and add the Liveness Guard.
        address guard = deployLivenessGuard();
        _callViaSafe({ _safe: safe, _target: address(safe), _data: abi.encodeCall(GuardManager.setGuard, (guard)) });
        console.log("LivenessGuard setup on SecurityCouncilSafe");

        // Deploy and add the Liveness Module.
        address livenessModule = deployLivenessModule();
        _callViaSafe({
            _safe: safe,
            _target: address(safe),
            _data: abi.encodeCall(ModuleManager.enableModule, (livenessModule))
        });

        // Finalize configuration by removing the additional deployer key.
        removeDeployerFromSafe({ _name: "SecurityCouncilSafe", _newThreshold: exampleCouncilConfig.safeConfig.threshold });

        address[] memory owners = safe.getOwners();
        require(
            safe.getThreshold() == LivenessModule(livenessModule).getRequiredThreshold(owners.length),
            "DeployOwnership: safe threshold must be equal to the LivenessModule's required threshold"
        );

        addr_ = address(safe);
        console.log("Deployed and configured the Security Council Safe!");
    }
}
