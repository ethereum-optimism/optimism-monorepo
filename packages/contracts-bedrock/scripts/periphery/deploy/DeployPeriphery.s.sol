// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;
// TODO: Migrate this script to use DeployUtils

import { console2 as console } from "forge-std/console2.sol";
import { Script } from "forge-std/Script.sol";

import { Config } from "scripts/libraries/Config.sol";
import { Artifacts } from "scripts/Artifacts.s.sol";
import { PeripheryDeployConfig } from "scripts/periphery/deploy/PeripheryDeployConfig.s.sol";

import { ProxyAdmin } from "src/universal/ProxyAdmin.sol";
import { Proxy } from "src/universal/Proxy.sol";
import { Faucet } from "src/periphery/faucet/Faucet.sol";
import { Drippie } from "src/periphery/drippie/Drippie.sol";
import { CheckGelatoLow } from "src/periphery/drippie/dripchecks/CheckGelatoLow.sol";
import { CheckBalanceLow } from "src/periphery/drippie/dripchecks/CheckBalanceLow.sol";
import { CheckTrue } from "src/periphery/drippie/dripchecks/CheckTrue.sol";
import { CheckSecrets } from "src/periphery/drippie/dripchecks/CheckSecrets.sol";
import { AdminFaucetAuthModule } from "src/periphery/faucet/authmodules/AdminFaucetAuthModule.sol";

import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

/// @title DeployPeriphery
/// @notice Script used to deploy periphery contracts.
contract DeployPeriphery is Script, Artifacts {
    /// @notice Error emitted when an address mismatch is detected.
    error AddressMismatch(string, address, address);

    /// @notice Deployment configuration.
    PeripheryDeployConfig cfg;

    /// @notice Sets up the deployment script.
    function setUp() public override {
        Artifacts.setUp();
        cfg = new PeripheryDeployConfig(Config.deployConfigPath());
        console.log("Config path: %s", Config.deployConfigPath());
    }

    /// @notice Deploy all of the periphery contracts.
    function run() public {
        console.log("Deploying periphery contracts");

        // Optionally deploy the base dripcheck contracts.
        if (cfg.deployDripchecks()) {
            deployCheckTrue();
            deployCheckBalanceLow();
            deployCheckGelatoLow();
            deployCheckSecrets();
        }

        // Optionally deploy the faucet contracts.
        if (cfg.deployFaucetContracts()) {
            // Deploy faucet contracts.
            deployProxyAdmin();
            deployFaucetProxy();
            deployFaucet();
            deployFaucetDrippie();
            deployOnChainAuthModule();
            deployOffChainAuthModule();

            // Initialize the faucet.
            initializeFaucet();
            installFaucetAuthModulesConfigs();
        }

        // Optionally deploy the operations contracts.
        if (cfg.deployOperationsContracts()) {
            deployOperationsDrippie();
        }
    }

    /// @notice Modifier that wraps a function in broadcasting.
    modifier broadcast() {
        vm.startBroadcast();
        _;
        vm.stopBroadcast();
    }

    /// @notice Deploy ProxyAdmin.
    function deployProxyAdmin() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "ProxyAdmin",
            _creationCode: type(ProxyAdmin).creationCode,
            _constructorParams: abi.encode(msg.sender)
        });

        ProxyAdmin admin = ProxyAdmin(addr_);
        require(admin.owner() == msg.sender);
    }

    /// @notice Deploy FaucetProxy.
    function deployFaucetProxy() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "FaucetProxy",
            _creationCode: type(Proxy).creationCode,
            _constructorParams: abi.encode(mustGetAddress("ProxyAdmin"))
        });

        Proxy proxy = Proxy(payable(addr_));
        require(EIP1967Helper.getAdmin(address(proxy)) == mustGetAddress("ProxyAdmin"));
    }

    /// @notice Deploy the Faucet contract.
    function deployFaucet() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "Faucet",
            _creationCode: type(Faucet).creationCode,
            _constructorParams: abi.encode(cfg.faucetAdmin())
        });

        Faucet faucet = Faucet(payable(addr_));
        require(faucet.ADMIN() == cfg.faucetAdmin());
    }

    /// @notice Deploy the Drippie contract.
    function deployFaucetDrippie() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "FaucetDrippie",
            _creationCode: type(Drippie).creationCode,
            _constructorParams: abi.encode(cfg.faucetDrippieOwner())
        });

        Drippie drippie = Drippie(payable(addr_));
        require(drippie.owner() == cfg.faucetDrippieOwner());
    }

    /// @notice Deploy the Drippie contract for standard operations.
    function deployOperationsDrippie() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "OperationsDrippie",
            _creationCode: type(Drippie).creationCode,
            _constructorParams: abi.encode(cfg.operationsDrippieOwner())
        });

        Drippie drippie = Drippie(payable(addr_));
        require(drippie.owner() == cfg.operationsDrippieOwner());
    }

    /// @notice Deploy On-Chain Authentication Module.
    function deployOnChainAuthModule() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "OnChainAuthModule",
            _creationCode: type(AdminFaucetAuthModule).creationCode,
            _constructorParams: abi.encode(cfg.faucetOnchainAuthModuleAdmin(), "OnChainAuthModule", "1")
        });

        AdminFaucetAuthModule module = AdminFaucetAuthModule(addr_);
        require(module.ADMIN() == cfg.faucetOnchainAuthModuleAdmin());
    }

    /// @notice Deploy Off-Chain Authentication Module.
    function deployOffChainAuthModule() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "OffChainAuthModule",
            _creationCode: type(AdminFaucetAuthModule).creationCode,
            _constructorParams: abi.encode(cfg.faucetOffchainAuthModuleAdmin(), "OffChainAuthModule", "1")
        });

        AdminFaucetAuthModule module = AdminFaucetAuthModule(addr_);
        require(module.ADMIN() == cfg.faucetOffchainAuthModuleAdmin());
    }

    /// @notice Deploy CheckTrue contract.
    function deployCheckTrue() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "CheckTrue",
            _creationCode: type(CheckTrue).creationCode,
            _constructorParams: hex""
        });
    }

    /// @notice Deploy CheckBalanceLow contract.
    function deployCheckBalanceLow() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "CheckBalanceLow",
            _creationCode: type(CheckBalanceLow).creationCode,
            _constructorParams: hex""
        });
    }

    /// @notice Deploy CheckGelatoLow contract.
    function deployCheckGelatoLow() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "CheckGelatoLow",
            _creationCode: type(CheckGelatoLow).creationCode,
            _constructorParams: hex""
        });
    }

    /// @notice Deploy CheckSecrets contract.
    function deployCheckSecrets() public broadcast returns (address addr_) {
        addr_ = _deployCreate2({
            _name: "CheckSecrets",
            _creationCode: type(CheckSecrets).creationCode,
            _constructorParams: hex""
        });
    }

    /// @notice Initialize the Faucet.
    function initializeFaucet() public broadcast {
        ProxyAdmin proxyAdmin = ProxyAdmin(mustGetAddress("ProxyAdmin"));
        address faucetProxy = mustGetAddress("FaucetProxy");
        address faucet = mustGetAddress("Faucet");
        address implementationAddress = proxyAdmin.getProxyImplementation(faucetProxy);
        if (implementationAddress == faucet) {
            console.log("Faucet proxy implementation already set");
        } else {
            proxyAdmin.upgrade({ _proxy: payable(faucetProxy), _implementation: faucet });
        }

        require(Faucet(payable(faucetProxy)).ADMIN() == Faucet(payable(faucet)).ADMIN());
    }

    /// @notice Installs the OnChain AuthModule on the Faucet contract.
    function installOnChainAuthModule() public broadcast {
        _installAuthModule({
            _faucet: Faucet(mustGetAddress("FaucetProxy")),
            _name: "OnChainAuthModule",
            _config: Faucet.ModuleConfig({
                name: "OnChainAuthModule",
                enabled: true,
                ttl: cfg.faucetOnchainAuthModuleTtl(),
                amount: cfg.faucetOnchainAuthModuleAmount()
            })
        });
    }

    /// @notice Installs the OffChain AuthModule on the Faucet contract.
    function installOffChainAuthModule() public broadcast {
        _installAuthModule({
            _faucet: Faucet(mustGetAddress("FaucetProxy")),
            _name: "OffChainAuthModule",
            _config: Faucet.ModuleConfig({
                name: "OffChainAuthModule",
                enabled: true,
                ttl: cfg.faucetOffchainAuthModuleTtl(),
                amount: cfg.faucetOffchainAuthModuleAmount()
            })
        });
    }

    /// @notice Installs all of the auth modules in the faucet contract.
    function installFaucetAuthModulesConfigs() public {
        Faucet faucet = Faucet(mustGetAddress("FaucetProxy"));
        console.log("Installing auth modules at %s", address(faucet));
        installOnChainAuthModule();
        installOffChainAuthModule();
        console.log("Faucet Auth Module configs successfully installed");
    }

    /// @notice Deploys a contract using the CREATE2 opcode.
    /// @param _name The name of the contract.
    /// @param _creationCode The contract creation code.
    /// @param _constructorParams The constructor parameters.
    function _deployCreate2(
        string memory _name,
        bytes memory _creationCode,
        bytes memory _constructorParams
    )
        internal
        returns (address addr_)
    {
        bytes32 salt = keccak256(abi.encodePacked(bytes(_name), cfg.create2DeploymentSalt()));
        bytes memory initCode = abi.encodePacked(_creationCode, _constructorParams);
        address preComputedAddress = vm.computeCreate2Address(salt, keccak256(initCode));
        if (preComputedAddress.code.length > 0) {
            console.log("%s already deployed at %s", _name, preComputedAddress);
            address savedAddress = getAddress(_name);
            if (savedAddress == address(0)) {
                save(_name, preComputedAddress);
            } else if (savedAddress != preComputedAddress) {
                revert AddressMismatch(_name, preComputedAddress, savedAddress);
            }
            addr_ = preComputedAddress;
        } else {
            assembly {
                addr_ := create2(0, add(initCode, 0x20), mload(initCode), salt)
            }
            require(addr_ != address(0), "DeployPeriphery: deployment failed");
            save(_name, addr_);
            console.log("%s deployed at %s", _name, addr_);
        }
    }

    /// @notice Installs an auth module in the faucet.
    /// @param _faucet The faucet contract.
    /// @param _name The name of the auth module.
    /// @param _config The configuration of the auth module.
    function _installAuthModule(Faucet _faucet, string memory _name, Faucet.ModuleConfig memory _config) internal {
        AdminFaucetAuthModule module = AdminFaucetAuthModule(mustGetAddress(_name));
        if (_faucet.isModuleEnabled(module)) {
            console.log("%s already installed.", _name);
        } else {
            console.log("Installing %s", _name);
            _faucet.configure(module, _config);
            console.log("%s installed successfully", _name);
        }
    }
}
