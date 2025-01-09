// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Testing
import { stdJson } from "forge-std/StdJson.sol";

// Scripts
import { Deployer } from "scripts/deploy/Deployer.sol";

// Libraries
import { GameTypes } from "src/dispute/lib/Types.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Interfaces
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";

/// @title ForkLive
/// @notice This script is called by Setup.sol as a preparation step for the foundry test suite, and is run as an
///         alternative to Deploy.s.sol, when `FORK_TEST=true` is set in the env.
///         Like Deploy.s.sol this script saves the system addresses to disk so that they can be read into memory later
///         on, however rather than deploying new contracts from the local source code, it simply reads the addresses
///         from the superchain-registry.
///         Therefore this script can only be run against a fork of a production network which is listed in the
///         superchain-registry.
///         This contract must not have constructor logic because it is set into state using `etch`.
contract ForkLive is Deployer {
    using stdJson for string;

    /// @notice Returns the base chain name to use for forking
    /// @return The base chain name as a string
    function baseChain() internal view returns (string memory) {
        return vm.envOr("FORK_BASE_CHAIN", string("mainnet"));
    }

    /// @notice Returns the OP chain name to use for forking
    /// @return The OP chain name as a string
    function opChain() internal view returns (string memory) {
        return vm.envOr("FORK_OP_CHAIN", string("op"));
    }

    /// @notice Reads a standard chains addresses from the superchain-registry and saves them to disk.
    function run() public {
        string memory superchainBasePath = "./lib/superchain-registry/superchain/configs/";

        // Read the superchain config files
        // During development of an upgrade which adds a new contract, the contract will not yet be present in the
        // superchain-registry. In this case, the contract will be deployed by the upgrade process, and will need to
        // be saved by after the call to opcm.upgrade().
        // After the upgrade is complete, the superchain-registry will be updated and the contract will be present.
        // At this point, the test will need to be updated to read the new contract from the superchain-registry using
        // either the `saveProxyAndImpl` or `save` functions.
        string memory superchainToml = vm.readFile(string.concat(superchainBasePath, baseChain(), "/superchain.toml"));
        string memory opToml = vm.readFile(string.concat(superchainBasePath, baseChain(), "/", opChain(), ".toml"));

        // Superchain shared contracts
        saveProxyAndImpl("SuperchainConfig", superchainToml, ".superchain_config_addr");
        saveProxyAndImpl("ProtocolVersions", superchainToml, ".protocol_versions_addr");
        artifacts.save("OPContractsManager", vm.parseTomlAddress(superchainToml, ".op_contracts_manager_proxy_addr"));

        // Core contracts
        artifacts.save("ProxyAdmin", vm.parseTomlAddress(opToml, ".addresses.ProxyAdmin"));
        saveProxyAndImpl("SystemConfig", opToml, ".addresses.SystemConfigProxy");

        // Bridge contracts
        address optimismPortal = vm.parseTomlAddress(opToml, ".addresses.OptimismPortalProxy");
        artifacts.save("OptimismPortalProxy", optimismPortal);
        artifacts.save("OptimismPortal2Impl", EIP1967Helper.getImplementation(optimismPortal));

        address addressManager = vm.parseTomlAddress(opToml, ".addresses.AddressManager");
        artifacts.save("AddressManager", addressManager);
        artifacts.save(
            "L1CrossDomainMessengerImpl", IAddressManager(addressManager).getAddress("OVM_L1CrossDomainMessenger")
        );
        artifacts.save(
            "L1CrossDomainMessengerProxy", vm.parseTomlAddress(opToml, ".addresses.L1CrossDomainMessengerProxy")
        );
        saveProxyAndImpl("OptimismMintableERC20Factory", opToml, ".addresses.OptimismMintableERC20FactoryProxy");
        saveProxyAndImpl("L1StandardBridge", opToml, ".addresses.L1StandardBridgeProxy");
        saveProxyAndImpl("L1ERC721Bridge", opToml, ".addresses.L1ERC721BridgeProxy");

        // Fault proof proxied contracts
        saveProxyAndImpl("AnchorStateRegistry", opToml, ".addresses.AnchorStateRegistryProxy");
        saveProxyAndImpl("DisputeGameFactory", opToml, ".addresses.DisputeGameFactoryProxy");
        saveProxyAndImpl("DelayedWETH", opToml, ".addresses.DelayedWETHProxy");

        // Fault proof non-proxied contracts
        artifacts.save("PreimageOracle", vm.parseTomlAddress(opToml, ".addresses.PreimageOracle"));
        artifacts.save("MipsSingleton", vm.parseTomlAddress(opToml, ".addresses.MIPS"));
        IDisputeGameFactory disputeGameFactory =
            IDisputeGameFactory(artifacts.mustGetAddress("DisputeGameFactoryProxy"));
        artifacts.save("FaultDisputeGame", vm.parseTomlAddress(opToml, ".addresses.FaultDisputeGame"));
        // The PermissionedDisputeGame and PermissionedDelayedWETHProxy are not listed in the registry for OP, so we
        // look it up onchain
        IFaultDisputeGame permissionedDisputeGame =
            IFaultDisputeGame(address(disputeGameFactory.gameImpls(GameTypes.PERMISSIONED_CANNON)));
        artifacts.save("PermissionedDisputeGame", address(permissionedDisputeGame));
        artifacts.save("PermissionedDelayedWETHProxy", address(permissionedDisputeGame.weth()));
    }

    /// @notice Saves the proxy and implementation addresses for a contract name
    /// @param _contractName The name of the contract to save
    /// @param _tomlPath The path to the superchain config file
    /// @param _tomlKey The key in the superchain config file to get the proxy address
    function saveProxyAndImpl(string memory _contractName, string memory _tomlPath, string memory _tomlKey) internal {
        address proxy = vm.parseTomlAddress(_tomlPath, _tomlKey);
        artifacts.save(string.concat(_contractName, "Proxy"), proxy);
        address impl = EIP1967Helper.getImplementation(proxy);
        require(impl != address(0), "Upgrade: Implementation address is zero");
        artifacts.save(string.concat(_contractName, "Impl"), impl);
    }
}
