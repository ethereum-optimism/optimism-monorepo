// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { console2 as console } from "forge-std/console2.sol";

// Testing
import { stdToml } from "forge-std/StdToml.sol";
import { DelegateCaller } from "test/mocks/Callers.sol";

// Scripts
import { Deployer } from "scripts/deploy/Deployer.sol";
import { Deploy } from "scripts/deploy/Deploy.s.sol";

// Libraries
import { GameTypes, Claim } from "src/dispute/lib/Types.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Interfaces
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";

/// @title ForkLive
/// @notice This script is called by Setup.sol as a preparation step for the foundry test suite, and is run as an
///         alternative to Deploy.s.sol, when `FORK_TEST=true` is set in the env.
///         Like Deploy.s.sol this script saves the system addresses to the Artifacts contract so that they can be
///         read by other contracts. However, rather than deploying new contracts from the local source code, it
///         simply reads the addresses from the superchain-registry.
///         Therefore this script can only be run against a fork of a production network which is listed in the
///         superchain-registry.
///         This contract must not have constructor logic because it is set into state using `etch`.
contract ForkLive is Deployer {
    using stdToml for string;

    bool public useOpsRepo;

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

    /// @dev This function sets up the system to test it as follows:
    ///      1. Check if the SUPERCHAIN_OPS_ALLOCS_PATH environment variable was set from superchain ops.
    ///      2. If set, load the state from the given path.
    ///      3. Read the superchain-registry to get the contract addresses we wish to test from that network.
    ///      4. If the environment variable wasn't set, deploy the updated OPCM and implementations of the contracts.
    ///      5. Upgrade the system using the OPCM.upgrade() function if useUpgradedFork is true.
    function run() public {
        string memory superchainOpsAllocsPath = vm.envOr("SUPERCHAIN_OPS_ALLOCS_PATH", string(""));

        useOpsRepo = bytes(superchainOpsAllocsPath).length > 0;
        if (useOpsRepo) {
            console.log("ForkLive: loading state from %s", superchainOpsAllocsPath);
            // Set the resultant state from the superchain ops repo upgrades.
            // The allocs are generated when simulating an upgrade task that runs vm.dumpState.
            // These allocs represent the state of the EVM after the upgrade has been simulated.
            vm.loadAllocs(superchainOpsAllocsPath);
            // Next, fetch the addresses from the superchain registry. This function uses a local EVM
            // to retrieve implementation addresses by reading from proxy addresses provided by the registry.
            // Setting the allocs first ensures the correct implementation addresses are retrieved.
            _readSuperchainRegistry();
        } else {
            // Read the superchain registry and save the addresses to the Artifacts contract.
            _readSuperchainRegistry();
            // Now deploy the updated OPCM and implementations of the contracts.
            _deployNewImplementations();
        }

        // Now upgrade the contracts (if the config is set to do so)
        if (useOpsRepo) {
            console.log("ForkLive: using ops repo to upgrade");
        } else if (cfg.useUpgradedFork()) {
            console.log("ForkLive: upgrading");
            _upgrade();
        }
    }

    /// @notice Reads the superchain config files and saves the addresses to disk.
    /// @dev During development of an upgrade which adds a new contract, the contract will not yet be present in the
    ///      superchain-registry. In this case, the contract will be deployed by the upgrade process, and will need to
    ///      be stored by artifacts.save() after the call to opcm.upgrade().
    ///      After the upgrade is complete, the superchain-registry will be updated and the contract will be present. At
    ///      that point, this function will need to be updated to read the new contract from the superchain-registry
    ///      using either the `saveProxyAndImpl` or `artifacts.save()` functions.
    function _readSuperchainRegistry() internal {
        string memory superchainBasePath = "./lib/superchain-registry/superchain/configs/";

        string memory superchainToml = vm.readFile(string.concat(superchainBasePath, baseChain(), "/superchain.toml"));
        string memory opToml = vm.readFile(string.concat(superchainBasePath, baseChain(), "/", opChain(), ".toml"));

        // Slightly hacky, we encode the uint chainId as an address to save it in Artifacts
        artifacts.save("L2ChainId", address(uint160(vm.parseTomlUint(opToml, ".chain_id"))));
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
        // For chains that don't have a permissionless game, we save the dispute game and WETH
        // addresses as the zero address.
        artifacts.save("PreimageOracle", vm.parseTomlAddress(opToml, ".addresses.PreimageOracle"));
        artifacts.save("MipsSingleton", vm.parseTomlAddress(opToml, ".addresses.MIPS"));
        IDisputeGameFactory disputeGameFactory =
            IDisputeGameFactory(artifacts.mustGetAddress("DisputeGameFactoryProxy"));
        IFaultDisputeGame faultDisputeGame =
            IFaultDisputeGame(opToml.readAddressOr(".addresses.FaultDisputeGame", address(0)));
        artifacts.save("FaultDisputeGame", address(faultDisputeGame));
        artifacts.save("PermissionlessDelayedWETHProxy", address(faultDisputeGame.weth()));

        // The PermissionedDisputeGame and PermissionedDelayedWETHProxy are not listed in the registry for OP, so we
        // look it up onchain
        IFaultDisputeGame permissionedDisputeGame =
            IFaultDisputeGame(address(disputeGameFactory.gameImpls(GameTypes.PERMISSIONED_CANNON)));
        artifacts.save("PermissionedDisputeGame", address(permissionedDisputeGame));
        artifacts.save("PermissionedDelayedWETHProxy", address(permissionedDisputeGame.weth()));
    }

    /// @notice Calls to the Deploy.s.sol contract etched by Setup.sol to a deterministic address, sets up the
    /// environment, and deploys new implementations.
    function _deployNewImplementations() internal {
        Deploy deploy = Deploy(address(uint160(uint256(keccak256(abi.encode("optimism.deploy"))))));
        deploy.deployImplementations({ _isInterop: false });
    }

    /// @notice Upgrades the contracts using the OPCM.
    function _upgrade() internal {
        IOPContractsManager opcm = IOPContractsManager(artifacts.mustGetAddress("OPContractsManager"));

        ISystemConfig systemConfig = ISystemConfig(artifacts.mustGetAddress("SystemConfigProxy"));
        IProxyAdmin proxyAdmin = IProxyAdmin(EIP1967Helper.getAdmin(address(systemConfig)));

        address upgrader = proxyAdmin.owner();
        vm.label(upgrader, "ProxyAdmin Owner");

        IOPContractsManager.OpChainConfig[] memory opChains = new IOPContractsManager.OpChainConfig[](1);
        opChains[0] = IOPContractsManager.OpChainConfig({
            systemConfigProxy: systemConfig,
            proxyAdmin: proxyAdmin,
            absolutePrestate: Claim.wrap(bytes32(keccak256("absolutePrestate")))
        });

        // Temporarily replace the upgrader with a DelegateCaller so we can test the upgrade,
        // then reset its code to the original code.
        bytes memory upgraderCode = address(upgrader).code;
        vm.etch(upgrader, vm.getDeployedCode("test/mocks/Callers.sol:DelegateCaller"));
        DelegateCaller(upgrader).dcForward(address(opcm), abi.encodeCall(IOPContractsManager.upgrade, (opChains)));
        vm.etch(upgrader, upgraderCode);

        console.log("ForkLive: Saving newly deployed contracts");
        // A new ASR and new dispute games were deployed, so we need to update them
        IDisputeGameFactory disputeGameFactory =
            IDisputeGameFactory(artifacts.mustGetAddress("DisputeGameFactoryProxy"));
        address permissionedDisputeGame = address(disputeGameFactory.gameImpls(GameTypes.PERMISSIONED_CANNON));
        artifacts.save("PermissionedDisputeGame", permissionedDisputeGame);

        address permissionlessDisputeGame = address(disputeGameFactory.gameImpls(GameTypes.CANNON));
        if (permissionlessDisputeGame != address(0)) {
            artifacts.save("PermissionlessDisputeGame", address(permissionlessDisputeGame));
        }

        IAnchorStateRegistry newAnchorStateRegistry =
            IPermissionedDisputeGame(permissionedDisputeGame).anchorStateRegistry();
        artifacts.save("AnchorStateRegistryProxy", address(newAnchorStateRegistry));
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
