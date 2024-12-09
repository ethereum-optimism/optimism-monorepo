// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Testing
import { stdJson } from "forge-std/StdJson.sol";

// Scripts
import { Deployer } from "scripts/deploy/Deployer.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";
import { GameTypes } from "src/dispute/lib/Types.sol";

// Interfaces
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";

/// @title Upgrade
/// @notice This script is called by Setup.sol as a preparation step for the foundry test suite, and is run as an
///         alternative to Deploy.s.sol, when `UPGRADE_TEST=true` is set in the env.
///         Like Deploy.s.sol this script saves the system addresses to disk so that they can be read into memory later
///         on, however rather than deploying new contracts from the local source code, it simply reads the addresses
///         from the superchain-registry.
///         Therefore this script can only be run against a fork of a production network which is listed in the
///         superchain-registry.
///         This contract must not have constructor logic because it is set into state using `etch`.
contract Upgrade is Deployer {
    using stdJson for string;

    /// @notice Reads a standard chains addresses from the superchain-registry and saves them to disk.
    function run() public {
        string memory superchainBasePath = "./lib/superchain-registry/superchain/configs/";
        string memory forkBaseChain = "mainnet";
        string memory forkOpChain = "op";

        // Read the superchain config files
        string memory superchainToml = vm.readFile(string.concat(superchainBasePath, forkBaseChain, "/superchain.toml"));
        string memory opToml = vm.readFile(string.concat(superchainBasePath, forkBaseChain, "/", forkOpChain, ".toml"));

        // Superchain shared contracts
        saveProxyAndImpl("SuperchainConfig", superchainToml, ".superchain_config_addr");
        saveProxyAndImpl("ProtocolVersions", superchainToml, ".protocol_versions_addr");
        save("OPContractsManager", vm.parseTomlAddress(superchainToml, ".op_contracts_manager_proxy_addr"));

        // Core contracts
        save("ProxyAdmin", vm.parseTomlAddress(opToml, ".addresses.ProxyAdmin"));
        saveProxyAndImpl("SystemConfig", opToml, ".addresses.SystemConfigProxy");

        // Bridge contracts
        address optimismPortal = vm.parseTomlAddress(opToml, ".addresses.OptimismPortalProxy");
        save("OptimismPortalProxy", optimismPortal);
        save(
            "OptimismPortal", address(uint160(uint256(vm.load(optimismPortal, Constants.PROXY_IMPLEMENTATION_ADDRESS))))
        );
        save("OptimismPortal2", optimismPortal);
        address addressManager = vm.parseTomlAddress(opToml, ".addresses.AddressManager");
        save("AddressManager", addressManager);
        save("L1CrossDomainMessenger", IAddressManager(addressManager).getAddress("OVM_L1CrossDomainMessenger"));
        save("L1CrossDomainMessengerProxy", vm.parseTomlAddress(opToml, ".addresses.L1CrossDomainMessengerProxy"));
        saveProxyAndImpl("OptimismMintableERC20Factory", opToml, ".addresses.OptimismMintableERC20FactoryProxy");
        saveProxyAndImpl("L1StandardBridge", opToml, ".addresses.L1StandardBridgeProxy");
        saveProxyAndImpl("L1ERC721Bridge", opToml, ".addresses.L1ERC721BridgeProxy");

        // Fault proof proxied contracts
        saveProxyAndImpl("AnchorStateRegistry", opToml, ".addresses.AnchorStateRegistryProxy");
        saveProxyAndImpl("DisputeGameFactory", opToml, ".addresses.DisputeGameFactoryProxy");
        saveProxyAndImpl("DelayedWETH", opToml, ".addresses.DelayedWETHProxy");

        // Fault proof non-proxied contracts
        save("PreimageOracle", vm.parseTomlAddress(opToml, ".addresses.PreimageOracle"));
        save("Mips", vm.parseTomlAddress(opToml, ".addresses.MIPS"));
        IDisputeGameFactory disputeGameFactory = IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy"));
        save("FaultDisputeGame", vm.parseTomlAddress(opToml, ".addresses.FaultDisputeGame"));
        // The PermissionedDisputeGame and PermissionedDelayedWETHProxy are not listed in the registry for OP, so we
        // look it up onchain
        IFaultDisputeGame permissionedDisputeGame =
            IFaultDisputeGame(address(disputeGameFactory.gameImpls(GameTypes.PERMISSIONED_CANNON)));
        save("PermissionedDisputeGame", address(permissionedDisputeGame));
        save("PermissionedDelayedWETHProxy", address(permissionedDisputeGame.weth()));
    }

    /// @notice Saves the proxy and implementation addresses for a contract name
    /// @param _contractName The name of the contract to save
    /// @param _tomlPath The path to the superchain config file
    /// @param _tomlKey The key in the superchain config file to get the proxy address
    function saveProxyAndImpl(string memory _contractName, string memory _tomlPath, string memory _tomlKey) internal {
        address proxy = vm.parseTomlAddress(_tomlPath, _tomlKey);
        save(string.concat(_contractName, "Proxy"), proxy);
        save(_contractName, address(uint160(uint256(vm.load(proxy, Constants.PROXY_IMPLEMENTATION_ADDRESS)))));
    }
}
