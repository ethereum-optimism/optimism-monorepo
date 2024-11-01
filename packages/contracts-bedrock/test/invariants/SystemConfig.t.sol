// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { ISystemConfig } from "src/L1/interfaces/ISystemConfig.sol";
import { IProxy } from "src/universal/interfaces/IProxy.sol";
import { Constants } from "src/libraries/Constants.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Types } from "src/libraries/Types.sol";

contract ConfigSetter {
    function setConfig(uint8, bytes calldata) external {
        // noop
    }
}

contract SystemConfig_GasLimitBoundaries_Invariant is Test {
    ISystemConfig public config;

    function setUp() external {
        IProxy proxy = IProxy(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxy.__constructor__, (msg.sender)))
            })
        );
        ISystemConfig configImpl = ISystemConfig(
            DeployUtils.create1({
                _name: "SystemConfig",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(ISystemConfig.__constructor__, ()))
            })
        );
        ConfigSetter setter = new ConfigSetter();

        vm.prank(msg.sender);
        proxy.upgradeToAndCall(
            address(configImpl),
            abi.encodeCall(
                configImpl.initialize,
                (
                    ISystemConfig.Roles({
                        owner: address(0xbeef),
                        feeAdmin: address(0xbeef),
                        unsafeBlockSigner: address(1),
                        batcherHash: bytes32(hex"abcd")
                    }),
                    2100, // overhead
                    1000000, // scalar
                    30_000_000, // gas limit
                    Constants.DEFAULT_RESOURCE_CONFIG(),
                    address(0), // _batchInbox
                    ISystemConfig.Addresses({ // _addrs
                        l1CrossDomainMessenger: address(0),
                        l1ERC721Bridge: address(0),
                        l1StandardBridge: address(0),
                        disputeGameFactory: address(0),
                        optimismPortal: address(setter),
                        optimismMintableERC20Factory: address(0),
                        gasPayingToken: Constants.ETHER
                    }),
                    ISystemConfig.FeeVaultConfigs({
                        baseFeeVaultConfig: Types.FeeVaultConfig({
                            recipient: address(0),
                            min: 0,
                            withdrawalNetwork: Types.WithdrawalNetwork.L1
                        }),
                        sequencerFeeVaultConfig: Types.FeeVaultConfig({
                            recipient: address(0),
                            min: 0,
                            withdrawalNetwork: Types.WithdrawalNetwork.L1
                        }),
                        l1FeeVaultConfig: Types.FeeVaultConfig({
                            recipient: address(0),
                            min: 0,
                            withdrawalNetwork: Types.WithdrawalNetwork.L1
                        })
                    })
                )
            )
        );

        config = ISystemConfig(address(proxy));

        // Set the target contract to the `config`
        targetContract(address(config));
        // Set the target sender to the `config`'s owner (0xbeef)
        targetSender(address(0xbeef));
        // Set the target selector for `setGasLimit`
        // `setGasLimit` is the only function we care about, as it is the only function
        // that can modify the gas limit within the SystemConfig.
        bytes4[] memory selectors = new bytes4[](1);
        selectors[0] = config.setGasLimit.selector;
        FuzzSelector memory selector = FuzzSelector({ addr: address(config), selectors: selectors });
        targetSelector(selector);

        /// Allows the SystemConfig contract to be the target of the invariant test
        /// when it is behind a proxy. Foundry calls this function under the hood to
        /// know the ABI to use when calling the target contract.
        string[] memory artifacts = new string[](1);
        artifacts[0] = "SystemConfig";
        FuzzInterface memory target = FuzzInterface(address(config), artifacts);
        targetInterface(target);
    }

    /// @custom:invariant Gas limit boundaries
    ///
    /// The gas limit of the `SystemConfig` contract can never be lower than the hard-coded lower bound or higher than
    /// the hard-coded upper bound. The lower bound must never be higher than the upper bound.
    function invariant_gasLimitBoundaries() external view {
        assertTrue(config.gasLimit() >= config.minimumGasLimit());
        assertTrue(config.gasLimit() <= config.maximumGasLimit());
        assertTrue(config.minimumGasLimit() <= config.maximumGasLimit());
    }
}
