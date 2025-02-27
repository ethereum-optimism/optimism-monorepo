// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @notice Interface for the OPCM v1.8.0 release contract. This is temporarily required for
/// upgrade 12 so that the deployment of the OPPrestateUpdater can read and reuse the existing
/// permissioned dispute game blueprints.
interface IOPContractsManager180 {
    struct Blueprints {
        address addressManager;
        address proxy;
        address proxyAdmin;
        address l1ChugSplashProxy;
        address resolvedDelegateProxy;
        address anchorStateRegistry;
        address permissionedDisputeGame1;
        address permissionedDisputeGame2;
    }

    function blueprints() external view returns (Blueprints memory);
}
