// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { console2 as console } from "forge-std/console2.sol";
import { stdJson } from "forge-std/StdJson.sol";
import { Vm } from "forge-std/Vm.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Config } from "scripts/libraries/Config.sol";
import { StorageSlot } from "scripts/libraries/ForgeArtifacts.sol";
import { LibString } from "@solady/utils/LibString.sol";
import { ForgeArtifacts } from "scripts/libraries/ForgeArtifacts.sol";
import { Process } from "scripts/libraries/Process.sol";

/// @notice Represents a deployment. Is serialized to JSON as a key/value
///         pair. Can be accessed from within scripts.
struct Deployment {
    string name;
    address payable addr;
}

/// @title Artifacts
/// @notice Useful for accessing deployment artifacts from within scripts.
///         When a contract is deployed, call the `save` function to write its name and
///         contract address to disk. Inspired by `forge-deploy`.
abstract contract Artifacts {
    /// @notice Foundry cheatcode VM.
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    /// @notice Error for when attempting to fetch a deployment and it does not exist
    error DeploymentDoesNotExist(string);
    /// @notice Error for when trying to save an invalid deployment
    error InvalidDeployment(string);
    /// @notice Error for when attempting to load the initialized slot of an unsupported contract.
    error UnsupportedInitializableContract(string);

    /// @notice The set of deployments that have been done during execution.
    mapping(string => Deployment) internal _namedDeployments;
    /// @notice The same as `_namedDeployments` but as an array.
    Deployment[] internal _newDeployments;
    /// @notice Path to the directory containing the hh deploy style artifacts
    string internal deploymentsDir;
    /// @notice The path to the deployment artifact that is being written to.
    string internal deploymentOutfile;
    /// @notice The namespace for the deployment. Can be set with the env var DEPLOYMENT_CONTEXT.
    string internal deploymentContext;

    /// @notice Setup function. The arguments here
    function setUp() public virtual {
        deploymentOutfile = Config.deploymentOutfile();
        console.log("Writing artifact to %s", deploymentOutfile);
        ForgeArtifacts.ensurePath(deploymentOutfile);

        uint256 chainId = Config.chainID();
        console.log("Connected to network with chainid %s", chainId);

        // Load addresses from a JSON file if the CONTRACT_ADDRESSES_PATH environment variable
        // is set. Great for loading addresses from `superchain-registry`.
        string memory addresses = Config.contractAddressesPath();
        if (bytes(addresses).length > 0) {
            console.log("Loading addresses from %s", addresses);
            _loadAddresses(addresses);
        }
    }

    /// @notice Populates the addresses to be used in a script based on a JSON file.
    ///         The format of the JSON file is the same that it output by this script
    ///         as well as the JSON files that contain addresses in the `superchain-registry`
    ///         repo. The JSON key is the name of the contract and the value is an address.
    function _loadAddresses(string memory _path) internal {
        string memory json = Process.bash(string.concat("jq -cr < ", _path));
        string[] memory keys = vm.parseJsonKeys(json, "");
        for (uint256 i; i < keys.length; i++) {
            string memory key = keys[i];
            address addr = stdJson.readAddress(json, string.concat("$.", key));
            save(key, addr);
        }
    }

    /// @notice Returns all of the deployments done in the current context.
    function newDeployments() external view returns (Deployment[] memory) {
        return _newDeployments;
    }

    /// @notice Returns whether or not a particular deployment exists.
    /// @param _name The name of the deployment.
    /// @return Whether the deployment exists or not.
    function has(string memory _name) public view returns (bool) {
        Deployment memory existing = _namedDeployments[_name];
        return bytes(existing.name).length > 0;
    }

    /// @notice Returns the address of a deployment. Also handles the predeploys.
    /// @param _name The name of the deployment.
    /// @return The address of the deployment. May be `address(0)` if the deployment does not
    ///         exist.
    function getAddress(string memory _name) public view returns (address payable) {
        Deployment memory existing = _namedDeployments[_name];
        if (existing.addr != address(0)) {
            if (bytes(existing.name).length == 0) {
                return payable(address(0));
            }
            return existing.addr;
        }

        bytes32 digest = keccak256(bytes(_name));
        if (digest == keccak256(bytes("L2CrossDomainMessenger"))) {
            return payable(Predeploys.L2_CROSS_DOMAIN_MESSENGER);
        } else if (digest == keccak256(bytes("L2ToL1MessagePasser"))) {
            return payable(Predeploys.L2_TO_L1_MESSAGE_PASSER);
        } else if (digest == keccak256(bytes("L2StandardBridge"))) {
            return payable(Predeploys.L2_STANDARD_BRIDGE);
        } else if (digest == keccak256(bytes("L2StandardBridgeInterop"))) {
            return payable(Predeploys.L2_STANDARD_BRIDGE);
        } else if (digest == keccak256(bytes("L2ERC721Bridge"))) {
            return payable(Predeploys.L2_ERC721_BRIDGE);
        } else if (digest == keccak256(bytes("SequencerFeeWallet"))) {
            return payable(Predeploys.SEQUENCER_FEE_WALLET);
        } else if (digest == keccak256(bytes("OptimismMintableERC20Factory"))) {
            return payable(Predeploys.OPTIMISM_MINTABLE_ERC20_FACTORY);
        } else if (digest == keccak256(bytes("OptimismMintableERC721Factory"))) {
            return payable(Predeploys.OPTIMISM_MINTABLE_ERC721_FACTORY);
        } else if (digest == keccak256(bytes("L1Block"))) {
            return payable(Predeploys.L1_BLOCK_ATTRIBUTES);
        } else if (digest == keccak256(bytes("GasPriceOracle"))) {
            return payable(Predeploys.GAS_PRICE_ORACLE);
        } else if (digest == keccak256(bytes("L1MessageSender"))) {
            return payable(Predeploys.L1_MESSAGE_SENDER);
        } else if (digest == keccak256(bytes("DeployerWhitelist"))) {
            return payable(Predeploys.DEPLOYER_WHITELIST);
        } else if (digest == keccak256(bytes("WETH"))) {
            return payable(Predeploys.WETH);
        } else if (digest == keccak256(bytes("LegacyERC20ETH"))) {
            return payable(Predeploys.LEGACY_ERC20_ETH);
        } else if (digest == keccak256(bytes("L1BlockNumber"))) {
            return payable(Predeploys.L1_BLOCK_NUMBER);
        } else if (digest == keccak256(bytes("LegacyMessagePasser"))) {
            return payable(Predeploys.LEGACY_MESSAGE_PASSER);
        } else if (digest == keccak256(bytes("ProxyAdmin"))) {
            return payable(Predeploys.PROXY_ADMIN);
        } else if (digest == keccak256(bytes("BaseFeeVault"))) {
            return payable(Predeploys.BASE_FEE_VAULT);
        } else if (digest == keccak256(bytes("L1FeeVault"))) {
            return payable(Predeploys.L1_FEE_VAULT);
        } else if (digest == keccak256(bytes("GovernanceToken"))) {
            return payable(Predeploys.GOVERNANCE_TOKEN);
        } else if (digest == keccak256(bytes("SchemaRegistry"))) {
            return payable(Predeploys.SCHEMA_REGISTRY);
        } else if (digest == keccak256(bytes("EAS"))) {
            return payable(Predeploys.EAS);
        } else if (digest == keccak256(bytes("OptimismSuperchainERC20Factory"))) {
            return payable(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_FACTORY);
        } else if (digest == keccak256(bytes("OptimismSuperchainERC20Beacon"))) {
            return payable(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_BEACON);
        } else if (digest == keccak256(bytes("SuperchainTokenBridge"))) {
            return payable(Predeploys.SUPERCHAIN_TOKEN_BRIDGE);
        }
        return payable(address(0));
    }

    /// @notice Returns the address of a deployment and reverts if the deployment
    ///         does not exist.
    /// @return The address of the deployment.
    function mustGetAddress(string memory _name) public view returns (address payable) {
        address addr = getAddress(_name);
        if (addr == address(0)) {
            revert DeploymentDoesNotExist(_name);
        }
        return payable(addr);
    }

    /// @notice Returns a deployment that is suitable to be used to interact with contracts.
    /// @param _name The name of the deployment.
    /// @return The deployment.
    function get(string memory _name) public view returns (Deployment memory) {
        return _namedDeployments[_name];
    }

    /// @notice Appends a deployment to disk as a JSON deploy artifact.
    /// @param _name The name of the deployment.
    /// @param _deployed The address of the deployment.
    function save(string memory _name, address _deployed) public {
        console.log("Saving %s: %s", _name, _deployed);
        if (bytes(_name).length == 0) {
            revert InvalidDeployment("EmptyName");
        }
        if (bytes(_namedDeployments[_name].name).length > 0) {
            revert InvalidDeployment("AlreadyExists");
        }

        Deployment memory deployment = Deployment({ name: _name, addr: payable(_deployed) });
        _namedDeployments[_name] = deployment;
        _newDeployments.push(deployment);
        _appendDeployment(_name, _deployed);
    }

    /// @notice Adds a deployment to the temp deployments file
    function _appendDeployment(string memory _name, address _deployed) internal {
        vm.writeJson({ json: stdJson.serialize("", _name, _deployed), path: deploymentOutfile });
    }

    /// @notice Stubs a deployment retrieved through `get`.
    /// @param _name The name of the deployment.
    /// @param _addr The mock address of the deployment.
    function prankDeployment(string memory _name, address _addr) public {
        if (bytes(_name).length == 0) {
            revert InvalidDeployment("EmptyName");
        }

        Deployment memory deployment = Deployment({ name: _name, addr: payable(_addr) });
        _namedDeployments[_name] = deployment;
    }

    /// @notice Returns the value of the internal `_initialized` storage slot for a given contract.
    function loadInitializedSlot(string memory _contractName) public returns (uint8 initialized_) {
        address contractAddress = mustGetAddress(_contractName);

        // Check if the contract name ends with `Proxy` and, if so override the contract name which is used to
        // retrieve the storage layout.
        if (LibString.endsWith(_contractName, "Proxy")) {
            _contractName = LibString.slice(_contractName, 0, bytes(_contractName).length - 5);
        }

        StorageSlot memory slot = ForgeArtifacts.getInitializedSlot(_contractName);
        bytes32 slotVal = vm.load(contractAddress, bytes32(vm.parseUint(slot.slot)));
        initialized_ = uint8((uint256(slotVal) >> (slot.offset * 8)) & 0xFF);
    }
}
