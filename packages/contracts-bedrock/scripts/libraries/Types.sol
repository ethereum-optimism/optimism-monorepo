// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

library Types {
    /// @notice Represents a set of L1 contracts. Used to represent a set of proxies.
    /// This is not an exhaustive list of all contracts on L1, but rather a subset.
    struct ContractSet {
        address L1CrossDomainMessenger;
        address L1StandardBridge;
        address L2OutputOracle;
        address DisputeGameFactory;
        address DelayedWETH;
        address PermissionedDelayedWETH;
        address AnchorStateRegistry;
        address OptimismMintableERC20Factory;
        address OptimismPortal;
        address SystemConfig;
        address L1ERC721Bridge;
        address ProtocolVersions;
        address SuperchainConfig;
    }
}
