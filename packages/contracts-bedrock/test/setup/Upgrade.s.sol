// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Testing
import { VmSafe } from "forge-std/Vm.sol";
import { console2 as console } from "forge-std/console2.sol";
import { stdJson } from "forge-std/StdJson.sol";

// Scripts
import { Deployer } from "scripts/deploy/Deployer.sol";
import { DeploySuperchainInput, DeploySuperchain, DeploySuperchainOutput } from "scripts/deploy/DeploySuperchain.s.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";
import { GameTypes } from "src/dispute/lib/Types.sol";

// Interfaces
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";

/// @title Upgrade
/// @notice A script to read superchain configs and save relevant addresses

contract Upgrade is Deployer {
    using stdJson for string;

    /// @notice Deploy all of the L1 contracts necessary for a full Superchain with a single Op Chain.
    function run() public {
        // Read the superchain config files
        string memory superchainPath = "./lib/superchain-registry/superchain/configs/mainnet/";
        string memory superchainToml = vm.readFile(string.concat(superchainPath, "superchain.toml"));
        string memory opToml = vm.readFile(string.concat(superchainPath, "op.toml"));

        // Continue with saving addresses
        saveProxyAndImpl("SuperchainConfig", vm.parseTomlAddress(superchainToml, ".superchain_config_addr"));
        saveProxyAndImpl("ProtocolVersions", vm.parseTomlAddress(superchainToml, ".protocol_versions_addr"));

        saveProxyAndImpl("OptimismPortal", vm.parseTomlAddress(opToml, ".addresses.OptimismPortalProxy"));
        save("OptimismPortal2", vm.parseTomlAddress(opToml, ".addresses.OptimismPortalProxy"));

        saveProxyAndImpl(
            "L1CrossDomainMessenger", vm.parseTomlAddress(opToml, ".addresses.L1CrossDomainMessengerProxy")
        );
        saveProxyAndImpl(
            "OptimismMintableERC20Factory", vm.parseTomlAddress(opToml, ".addresses.OptimismMintableERC20FactoryProxy")
        );
        saveProxyAndImpl("SystemConfig", vm.parseTomlAddress(opToml, ".addresses.SystemConfigProxy"));
        saveProxyAndImpl("L1StandardBridge", vm.parseTomlAddress(opToml, ".addresses.L1StandardBridgeProxy"));
        saveProxyAndImpl("L1ERC721Bridge", vm.parseTomlAddress(opToml, ".addresses.L1ERC721BridgeProxy"));

        save("PreimageOracle", vm.parseTomlAddress(opToml, ".addresses.PreimageOracle"));
        save("Mips", vm.parseTomlAddress(opToml, ".addresses.MIPS"));
        save("ProxyAdmin", vm.parseTomlAddress(opToml, ".addresses.ProxyAdmin"));
        save("AddressManager", vm.parseTomlAddress(opToml, ".addresses.AddressManager"));

        saveProxyAndImpl("AnchorStateRegistryProxy", vm.parseTomlAddress(opToml, ".addresses.AnchorStateRegistryProxy"));
        saveProxyAndImpl("DisputeGameFactory", vm.parseTomlAddress(opToml, ".addresses.DisputeGameFactoryProxy"));

        save(
            "FaultDisputeGame",
            address(IDisputeGameFactory(mustGetAddress("DisputeGameFactory")).gameImpls(GameTypes.PERMISSIONED_CANNON))
        );
        // TODO: Where do we get the OPContractsManager address from?
        // save("OPContractsManager", x);

        // TODO: The superchain-registry doesn't seem to differentiate between Permissioned and Permissionless
        // DelayedWETH. In the case of op mainnet, it's Permissionless.
        // So we need to determine how to save the PermissionedDelayedWETHProxy and PermissionedDisputeGame
        // addresses, or if we can skip them.
        saveProxyAndImpl("DelayedWETH", vm.parseTomlAddress(opToml, ".addresses.DelayedWETHProxy"));
    }

    /// @notice Saves the proxy and implementation addresses for a given proxy contract
    /// @param implName The name to save the implementation address under
    /// @param proxyAddr The address of the proxy contract
    function saveProxyAndImpl(string memory implName, address proxyAddr) internal {
        save(string.concat(implName, "Proxy"), proxyAddr);
        save(implName, address(uint160(uint256(vm.load(proxyAddr, Constants.PROXY_IMPLEMENTATION_ADDRESS)))));
    }
}
