// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// Interfaces
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions } from "interfaces/L1/IProtocolVersions.sol";

interface IStandardValidatorBase {
    struct ImplementationsBase {
        address superchainConfigImpl;
        address protocolVersionsImpl;
        address l1ERC721BridgeImpl;
        address optimismPortalImpl;
        address systemConfigImpl;
        address optimismMintableERC20FactoryImpl;
        address l1CrossDomainMessengerImpl;
        address l1StandardBridgeImpl;
        address disputeGameFactoryImpl;
        address anchorStateRegistryImpl;
        address delayedWETHImpl;
        address mipsImpl;
    }

    function anchorStateRegistryImpl() external view returns (address);
    function anchorStateRegistryVersion() external pure returns (string memory);
    function challenger() external view returns (address);
    function delayedWETHImpl() external view returns (address);
    function delayedWETHVersion() external pure returns (string memory);
    function disputeGameFactoryImpl() external view returns (address);
    function disputeGameFactoryVersion() external pure returns (string memory);
    function l1CrossDomainMessengerImpl() external view returns (address);
    function l1CrossDomainMessengerVersion() external pure returns (string memory);
    function l1ERC721BridgeImpl() external view returns (address);
    function l1ERC721BridgeVersion() external pure returns (string memory);
    function l1PAOMultisig() external view returns (address);
    function l1StandardBridgeImpl() external view returns (address);
    function l1StandardBridgeVersion() external pure returns (string memory);
    function mips() external view returns (address);
    function mipsImpl() external view returns (address);
    function mipsVersion() external pure returns (string memory);
    function optimismMintableERC20FactoryImpl() external view returns (address);
    function optimismMintableERC20FactoryVersion() external pure returns (string memory);
    function optimismPortalImpl() external view returns (address);
    function optimismPortalVersion() external pure returns (string memory);
    function permissionedDisputeGameVersion() external pure returns (string memory);
    function preimageOracleVersion() external pure returns (string memory);
    function protocolVersions() external view returns (address);
    function protocolVersionsImpl() external view returns (address);
    function protocolVersionsVersion() external pure returns (string memory);
    function superchainConfig() external view returns (address);
    function superchainConfigImpl() external view returns (address);
    function superchainConfigVersion() external pure returns (string memory);
    function systemConfigImpl() external view returns (address);
    function systemConfigVersion() external pure returns (string memory);
}

interface IStandardValidatorV180 is IStandardValidatorBase {
    struct InputV180 {
        address proxyAdmin;
        address sysCfg;
        bytes32 absolutePrestate;
        uint256 l2ChainID;
    }

    function validate(InputV180 memory _input, bool _allowFailure) external view returns (string memory);

    function __constructor__(
        IStandardValidatorBase.ImplementationsBase memory _implementations,
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        address _l1PAOMultisig,
        address _mips,
        address _challenger
    ) external;
}

interface IStandardValidatorV200 is IStandardValidatorBase {
    struct InputV200 {
        address proxyAdmin;
        address sysCfg;
        bytes32 absolutePrestate;
        uint256 l2ChainID;
    }

    function validate(InputV200 memory _input, bool _allowFailure) external view returns (string memory);

    function __constructor__(
        IStandardValidatorBase.ImplementationsBase memory _implementations,
        ISuperchainConfig _superchainConfig,
        IProtocolVersions _protocolVersions,
        address _l1PAOMultisig,
        address _mips,
        address _challenger
    ) external;
}
