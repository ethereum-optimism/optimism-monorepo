// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Scripts
import { ForgeArtifacts } from "scripts/libraries/ForgeArtifacts.sol";
import { Process } from "scripts/libraries/Process.sol";

// Libraries
import { LibString } from "@solady/utils/LibString.sol";
import { Constants } from "src/libraries/Constants.sol";
import { GameType } from "src/dispute/lib/Types.sol";

// Interfaces
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { ProtocolVersion } from "src/L1/interfaces/IProtocolVersions.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";

/// @title Initializer_Test
/// @dev Ensures that the `initialize()` function on contracts cannot be called more than
///      once. This contract inherits from `ERC721Bridge_Initializer` because it is the
///      deepest contract in the inheritance chain for setting up the system contracts.
///      For each L1 contract both the implementation and the proxy are tested.
contract Initializer_Test is CommonTest {
    /// @notice Contains the address of an `Initializable` contract and the calldata
    ///         used to initialize it.
    struct InitializeableContract {
        string name;
        address target;
        bytes initCalldata;
    }

    /// @notice Array of contracts to test.
    InitializeableContract[] contracts;

    /// @notice Mapping of nickname to actual contract name.
    /// @dev Nicknames are only used when one proxy contract has multiple potential implementations
    ///      as can happen when a new implementation is being developed.
    mapping(string => string) nicknames;

    function setUp() public override {
        super.enableAltDA();
        super.enableLegacyContracts();
        super.setUp();

        // Initialize the `contracts` array with the addresses of the contracts to test, the
        // calldata used to initialize them, and the storage slot of their `_initialized` flag.

        // SuperchainConfigImpl
        contracts.push(
            InitializeableContract({
                name: "SuperchainConfig",
                target: deploy.mustGetAddress("SuperchainConfig"),
                initCalldata: abi.encodeCall(superchainConfig.initialize, (address(0), false))
            })
        );
        // SuperchainConfigProxy
        contracts.push(
            InitializeableContract({
                name: "SuperchainConfigProxy",
                target: address(superchainConfig),
                initCalldata: abi.encodeCall(superchainConfig.initialize, (address(0), false))
            })
        );
        // L1CrossDomainMessengerImpl
        contracts.push(
            InitializeableContract({
                name: "L1CrossDomainMessenger",
                target: deploy.mustGetAddress("L1CrossDomainMessenger"),
                initCalldata: abi.encodeCall(
                    l1CrossDomainMessenger.initialize, (superchainConfig, optimismPortal, systemConfig)
                )
            })
        );
        // L1CrossDomainMessengerProxy
        contracts.push(
            InitializeableContract({
                name: "L1CrossDomainMessengerProxy",
                target: address(l1CrossDomainMessenger),
                initCalldata: abi.encodeCall(
                    l1CrossDomainMessenger.initialize, (superchainConfig, optimismPortal, systemConfig)
                )
            })
        );
        // DisputeGameFactoryImpl
        contracts.push(
            InitializeableContract({
                name: "DisputeGameFactory",
                target: deploy.mustGetAddress("DisputeGameFactory"),
                initCalldata: abi.encodeCall(disputeGameFactory.initialize, (address(0)))
            })
        );
        // DisputeGameFactoryProxy
        contracts.push(
            InitializeableContract({
                name: "DisputeGameFactoryProxy",
                target: address(disputeGameFactory),
                initCalldata: abi.encodeCall(disputeGameFactory.initialize, (address(0)))
            })
        );
        // DelayedWETHImpl
        contracts.push(
            InitializeableContract({
                name: "DelayedWETH",
                target: deploy.mustGetAddress("DelayedWETH"),
                initCalldata: abi.encodeCall(delayedWeth.initialize, (address(0), ISuperchainConfig(address(0))))
            })
        );
        // DelayedWETHProxy
        contracts.push(
            InitializeableContract({
                name: "DelayedWETHProxy",
                target: address(delayedWeth),
                initCalldata: abi.encodeCall(delayedWeth.initialize, (address(0), ISuperchainConfig(address(0))))
            })
        );
        // L2OutputOracleImpl
        contracts.push(
            InitializeableContract({
                name: "L2OutputOracle",
                target: deploy.mustGetAddress("L2OutputOracle"),
                initCalldata: abi.encodeCall(l2OutputOracle.initialize, (0, 0, 0, 0, address(0), address(0), 0))
            })
        );
        // L2OutputOracleProxy
        contracts.push(
            InitializeableContract({
                name: "L2OutputOracleProxy",
                target: address(l2OutputOracle),
                initCalldata: abi.encodeCall(l2OutputOracle.initialize, (0, 0, 0, 0, address(0), address(0), 0))
            })
        );
        // OptimismPortalImpl
        contracts.push(
            InitializeableContract({
                name: "OptimismPortal",
                target: deploy.mustGetAddress("OptimismPortal"),
                initCalldata: abi.encodeCall(optimismPortal.initialize, (l2OutputOracle, systemConfig, superchainConfig))
            })
        );
        // OptimismPortalProxy
        contracts.push(
            InitializeableContract({
                name: "OptimismPortalProxy",
                target: address(optimismPortal),
                initCalldata: abi.encodeCall(optimismPortal.initialize, (l2OutputOracle, systemConfig, superchainConfig))
            })
        );
        // OptimismPortal2Impl
        contracts.push(
            InitializeableContract({
                name: "OptimismPortal2",
                target: deploy.mustGetAddress("OptimismPortal2"),
                initCalldata: abi.encodeCall(
                    optimismPortal2.initialize,
                    (
                        disputeGameFactory,
                        systemConfig,
                        superchainConfig,
                        GameType.wrap(uint32(deploy.cfg().respectedGameType()))
                    )
                )
            })
        );
        // SystemConfigImpl
        contracts.push(
            InitializeableContract({
                name: "SystemConfig",
                target: deploy.mustGetAddress("SystemConfig"),
                initCalldata: abi.encodeCall(
                    systemConfig.initialize,
                    (
                        address(0xdead),
                        0,
                        0,
                        bytes32(0),
                        1,
                        address(0),
                        IResourceMetering.ResourceConfig({
                            maxResourceLimit: 1,
                            elasticityMultiplier: 1,
                            baseFeeMaxChangeDenominator: 2,
                            minimumBaseFee: 0,
                            systemTxMaxGas: 0,
                            maximumBaseFee: 0
                        }),
                        address(0),
                        ISystemConfig.Addresses({
                            l1CrossDomainMessenger: address(0),
                            l1ERC721Bridge: address(0),
                            l1StandardBridge: address(0),
                            disputeGameFactory: address(0),
                            optimismPortal: address(0),
                            optimismMintableERC20Factory: address(0),
                            gasPayingToken: Constants.ETHER
                        })
                    )
                )
            })
        );
        // SystemConfigProxy
        contracts.push(
            InitializeableContract({
                name: "SystemConfigProxy",
                target: address(systemConfig),
                initCalldata: abi.encodeCall(
                    systemConfig.initialize,
                    (
                        address(0xdead),
                        0,
                        0,
                        bytes32(0),
                        1,
                        address(0),
                        IResourceMetering.ResourceConfig({
                            maxResourceLimit: 1,
                            elasticityMultiplier: 1,
                            baseFeeMaxChangeDenominator: 2,
                            minimumBaseFee: 0,
                            systemTxMaxGas: 0,
                            maximumBaseFee: 0
                        }),
                        address(0),
                        ISystemConfig.Addresses({
                            l1CrossDomainMessenger: address(0),
                            l1ERC721Bridge: address(0),
                            l1StandardBridge: address(0),
                            disputeGameFactory: address(0),
                            optimismPortal: address(0),
                            optimismMintableERC20Factory: address(0),
                            gasPayingToken: Constants.ETHER
                        })
                    )
                )
            })
        );
        // ProtocolVersionsImpl
        contracts.push(
            InitializeableContract({
                name: "ProtocolVersions",
                target: deploy.mustGetAddress("ProtocolVersions"),
                initCalldata: abi.encodeCall(
                    protocolVersions.initialize, (address(0), ProtocolVersion.wrap(1), ProtocolVersion.wrap(2))
                )
            })
        );
        // ProtocolVersionsProxy
        contracts.push(
            InitializeableContract({
                name: "ProtocolVersionsProxy",
                target: address(protocolVersions),
                initCalldata: abi.encodeCall(
                    protocolVersions.initialize, (address(0), ProtocolVersion.wrap(1), ProtocolVersion.wrap(2))
                )
            })
        );
        // L2CrossDomainMessenger
        contracts.push(
            InitializeableContract({
                name: "L2CrossDomainMessenger",
                target: address(l2CrossDomainMessenger),
                initCalldata: abi.encodeCall(l2CrossDomainMessenger.initialize, (l1CrossDomainMessenger))
            })
        );
        // L1StandardBridgeImpl
        contracts.push(
            InitializeableContract({
                name: "L1StandardBridge",
                target: deploy.mustGetAddress("L1StandardBridge"),
                initCalldata: abi.encodeCall(
                    l1StandardBridge.initialize, (l1CrossDomainMessenger, superchainConfig, systemConfig)
                )
            })
        );
        // L1StandardBridgeProxy
        contracts.push(
            InitializeableContract({
                name: "L1StandardBridgeProxy",
                target: address(l1StandardBridge),
                initCalldata: abi.encodeCall(
                    l1StandardBridge.initialize, (l1CrossDomainMessenger, superchainConfig, systemConfig)
                )
            })
        );
        // L2StandardBridge
        contracts.push(
            InitializeableContract({
                name: "L2StandardBridge",
                target: address(l2StandardBridge),
                initCalldata: abi.encodeCall(l2StandardBridge.initialize, (l1StandardBridge))
            })
        );
        // L2StandardBridgeInterop
        contracts.push(
            InitializeableContract({
                name: "L2StandardBridgeInterop",
                target: address(l2StandardBridge),
                initCalldata: abi.encodeCall(l2StandardBridge.initialize, (l1StandardBridge))
            })
        );
        // L1ERC721BridgeImpl
        contracts.push(
            InitializeableContract({
                name: "L1ERC721Bridge",
                target: deploy.mustGetAddress("L1ERC721Bridge"),
                initCalldata: abi.encodeCall(l1ERC721Bridge.initialize, (l1CrossDomainMessenger, superchainConfig))
            })
        );
        // L1ERC721BridgeProxy
        contracts.push(
            InitializeableContract({
                name: "L1ERC721BridgeProxy",
                target: address(l1ERC721Bridge),
                initCalldata: abi.encodeCall(l1ERC721Bridge.initialize, (l1CrossDomainMessenger, superchainConfig))
            })
        );
        // L2ERC721Bridge
        contracts.push(
            InitializeableContract({
                name: "L2ERC721Bridge",
                target: address(l2ERC721Bridge),
                initCalldata: abi.encodeCall(l2ERC721Bridge.initialize, (payable(address(l1ERC721Bridge))))
            })
        );
        // OptimismMintableERC20FactoryImpl
        contracts.push(
            InitializeableContract({
                name: "OptimismMintableERC20Factory",
                target: deploy.mustGetAddress("OptimismMintableERC20Factory"),
                initCalldata: abi.encodeCall(l1OptimismMintableERC20Factory.initialize, (address(l1StandardBridge)))
            })
        );
        // OptimismMintableERC20FactoryProxy
        contracts.push(
            InitializeableContract({
                name: "OptimismMintableERC20FactoryProxy",
                target: address(l1OptimismMintableERC20Factory),
                initCalldata: abi.encodeCall(l1OptimismMintableERC20Factory.initialize, (address(l1StandardBridge)))
            })
        );
        // DataAvailabilityChallengeImpl
        contracts.push(
            InitializeableContract({
                name: "DataAvailabilityChallenge",
                target: deploy.mustGetAddress("DataAvailabilityChallenge"),
                initCalldata: abi.encodeCall(dataAvailabilityChallenge.initialize, (address(0), 0, 0, 0, 0))
            })
        );
        // DataAvailabilityChallengeProxy
        contracts.push(
            InitializeableContract({
                name: "DataAvailabilityChallengeProxy",
                target: address(dataAvailabilityChallenge),
                initCalldata: abi.encodeCall(dataAvailabilityChallenge.initialize, (address(0), 0, 0, 0, 0))
            })
        );
        // AnchorStateRegistry
        contracts.push(
            InitializeableContract({
                name: "AnchorStateRegistry",
                target: address(anchorStateRegistry),
                initCalldata: abi.encodeCall(
                    anchorStateRegistry.initialize,
                    (new IAnchorStateRegistry.StartingAnchorRoot[](1), ISuperchainConfig(address(0)))
                )
            })
        );
        // AnchorStateRegistryProxy
        contracts.push(
            InitializeableContract({
                name: "AnchorStateRegistryProxy",
                target: address(anchorStateRegistry),
                initCalldata: abi.encodeCall(
                    anchorStateRegistry.initialize,
                    (new IAnchorStateRegistry.StartingAnchorRoot[](1), ISuperchainConfig(address(0)))
                )
            })
        );

        // Nicknamed contracts.
        nicknames["OptimismPortal2Proxy"] = "OptimismPortalProxy";
    }

    /// @notice Tests that:
    ///         1. All `Initializable` contracts in `src/` (except periphery) are accounted for in `contracts`.
    ///         2. The `_initialized` flag of each contract is properly set.
    ///         3. The `initialize()` function of each contract cannot be called again.
    function test_cannotReinitialize_succeeds() public {
        // Collect exclusions.
        string[] memory excludes = new string[](9);
        // TODO: Neither of these contracts are labeled properly in the deployment script. Both are
        //       currently being labeled as their non-interop versions. Remove these exclusions once
        //       the deployment script is fixed.
        excludes[0] = "src/L1/SystemConfigInterop.sol";
        excludes[1] = "src/L1/OptimismPortalInterop.sol";
        // Contract is currently not being deployed as part of the standard deployment script.
        excludes[2] = "src/L2/OptimismSuperchainERC20.sol";
        // Periphery contracts don't get deployed as part of the standard deployment script.
        excludes[3] = "src/periphery/*";
        // TODO: Deployment script is currently "broken" in the sense that it doesn't properly
        //       label the FaultDisputeGame and PermissionedDisputeGame contracts and instead
        //       simply deploys them anonymously. Means that functions like "getInitializedSlot"
        //       don't work properly. Remove these exclusions once the deployment script is fixed.
        excludes[4] = "src/dispute/FaultDisputeGame.sol";
        excludes[5] = "src/dispute/PermissionedDisputeGame.sol";
        // TODO: Eventually remove this exclusion. Same reason as above dispute contracts.
        excludes[6] = "src/L1/OPContractsManager.sol";
        excludes[7] = "src/L1/OPContractsManagerInterop.sol";
        // The L2OutputOracle is not always deployed (and is no longer being modified)
        excludes[8] = "src/L1/L2OutputOracle.sol";

        // Get all contract names in the src directory, minus the excluded contracts.
        string[] memory contractNames = ForgeArtifacts.getContractNames("src/*", excludes);

        // Iterate over all contracts to assert that they are accounted for in the `contracts
        // array. All contracts that have an `initialize()` function must be accounted for in the
        // `contracts` array or an error will be thrown. If the contract is proxied, both the
        // implementation and the proxy must be accounted for in the `contracts` array.
        for (uint256 i; i < contractNames.length; i++) {
            string memory contractName = contractNames[i];
            string memory contractKind = ForgeArtifacts.getContractKind(contractName);

            // Filter out non-contracts.
            if (!LibString.eq(contractKind, "contract")) {
                continue;
            }

            // Construct the query for the initialize function in the contract's ABI.
            string memory cmd = string.concat(
                "echo '",
                ForgeArtifacts.getAbi(contractName),
                "' | jq '.[] | select(.name == \"initialize\" and .type == \"function\")'"
            );

            // If the contract does not have an `initialize()` function, skip it.
            if (bytes(Process.bash(cmd)).length == 0) {
                continue;
            }

            // Check if this contract is in the contracts array.
            assertTrue(
                _hasMatchingContract(contractName), string.concat("Missing ", contractName, " from contracts array")
            );

            // If the contract is proxied, check that the proxy is in the contracts array.
            // Skip predeployed contracts for now since we don't yet keep track of the
            // implementations inside of the deploy script.
            // TODO: We should add support for this in the future so that we can properly check that
            //       the implementations for predeployed contracts are initialized too.
            if (ForgeArtifacts.isProxiedContract(contractName) && !ForgeArtifacts.isPredeployedContract(contractName)) {
                assertTrue(
                    _hasMatchingContract(string.concat(contractName, "Proxy")),
                    string.concat("Missing ", contractName, "Proxy from contracts array")
                );
            }
        }

        // Attempt to re-initialize all contracts within the `contracts` array.
        for (uint256 i; i < contracts.length; i++) {
            InitializeableContract memory _contract = contracts[i];
            string memory name = _getRealContractName(_contract.name);

            // Grab the value of the "initialized" storage slot.
            uint8 initializedSlotVal;
            initializedSlotVal = deploy.loadInitializedSlot(name);

            // Assert that the contract is already initialized.
            assertTrue(
                // Either 1 for initialized or type(uint8).max for initializer disabled.
                initializedSlotVal == 1 || initializedSlotVal == type(uint8).max,
                "Initializable: contract is not initialized"
            );

            // Then, attempt to re-initialize the contract. This should fail.
            (bool success, bytes memory returnData) = _contract.target.call(_contract.initCalldata);
            assertFalse(success);
            assertEq(_extractErrorString(returnData), "Initializable: contract is already initialized");
        }
    }

    /// @dev Returns true if the contract with the given name is in the `contracts` array.
    /// @param _name The name of the contract to check.
    /// @return matching_ True if the contract is in the `contracts` array, false otherwise.
    function _hasMatchingContract(string memory _name) internal view returns (bool matching_) {
        for (uint256 i; i < contracts.length; i++) {
            if (LibString.eq(contracts[i].name, _getRealContractName(_name))) {
                matching_ = true;
            }
        }
    }

    /// @dev Returns the real name of the contract, including any nicknames.
    /// @param _name The name of the contract.
    /// @return real_ The real name of the contract.
    function _getRealContractName(string memory _name) internal view returns (string memory real_) {
        real_ = bytes(nicknames[_name]).length > 0 ? nicknames[_name] : _name;
    }

    /// @dev Extracts the revert string from returndata encoded in the form of `Error(string)`.
    function _extractErrorString(bytes memory _returnData) internal pure returns (string memory error_) {
        // The first 4 bytes of the return data should be the selector for `Error(string)`. If not, revert.
        if (bytes4(_returnData) == 0x08c379a0) {
            // Extract the error string from the returndata. The error string is located 68 bytes after
            // the pointer to `returnData`.
            //
            // 32 bytes: `returnData` length
            // 4 bytes: `Error(string)` selector
            // 32 bytes: ABI encoding metadata; String offset
            // = 68 bytes
            assembly {
                error_ := add(_returnData, 0x44)
            }
        } else {
            revert("Initializer_Test: Invalid returndata format. Expected `Error(string)`");
        }
    }
}
