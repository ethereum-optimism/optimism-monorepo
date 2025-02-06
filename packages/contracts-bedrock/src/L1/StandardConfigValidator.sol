// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { Blueprint } from "src/libraries/Blueprint.sol";
import { Constants } from "src/libraries/Constants.sol";
import { Bytes } from "src/libraries/Bytes.sol";
import { Claim, Hash, Duration, GameType, GameTypes, OutputRoot } from "src/dispute/lib/Types.sol";

// Interfaces
import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";
import { IPermissionedDisputeGame } from "interfaces/dispute/IPermissionedDisputeGame.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IProtocolVersions, ProtocolVersion } from "interfaces/L1/IProtocolVersions.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IL1StandardBridge } from "interfaces/L1/IL1StandardBridge.sol";
import { IOptimismMintableERC20Factory } from "interfaces/universal/IOptimismMintableERC20Factory.sol";

contract StandardConfigValidator is ISemver {
    // Hardcoded standard config values. This means for chains that are not exactly on the standard
    // config (e.g. a different L1ProxyAdminOwner), they will need a different validator contract,
    // so we may need to parameterize this via the OPCM constructor.
    address public constant l1PAO = 0x5a0Aae59D09fccBdDb6C6CcEB07B7279367C3d2A;
    address public constant guardian = 0x09f7150D8c019BeF34450d6920f6B3608ceFdAf2;
    address public constant challenger = 0x9BA6e03D8B90dE867373Db8cF1A58d2F7F006b3A;
    address public constant proposer = 0x473300df21D047806A082244b417f96b32f13A33;

    IOPContractsManager public immutable opcm;
    IOPContractsManager.Implementations impls;

    error AddressHasNoCode(address who, bytes32 contractName);
    error ValidationFailed(bytes32 reason);

    constructor() {
        opcm = IOPContractsManager(msg.sender);
        impls = opcm.implementations();
    }

    /// @custom:semver 0.0.1
    function version() public pure virtual returns (string memory) {
        return "0.0.1";
    }

    function validateOpChain(ISystemConfig _sc, IProxyAdmin _pa) public view {
        ISystemConfig.Addresses memory addrs = _sc.getAddresses();

        // assertValidFaultProofConfig(_sc, _pa, addrs);
        // assertValidL1CrossDomainMessengerImpl(_sc, _pa, addrs);
        // assertValidL1ERC721BridgeImpl(_sc, _pa, addrs);
        // assertValidL1StandardBridgeImpl(_sc, _pa, addrs);
        // assertValidOptimismMintableERC20FactoryImpl(_sc, _pa, addrs);
        // assertValidOptimismPortalImpl(_sc, _pa, addrs);
        // assertValidSystemConfigImpl(_sc, _pa, addrs);
    }

    function validateSuperchain(
        ISuperchainConfig _sc, // SuperchainConfig
        IProtocolVersions _pv, // ProtocolVersions
        IProxyAdmin _spa // SuperchainProxyAdmin
    )
        public
        view
    {
        validateSuperchainProxyAdmin(_spa);
        validateSuperchainConfig(_sc, _spa);
        validateProtocolVersions(_pv, _spa);
    }

    function validateSuperchainProxyAdmin(IProxyAdmin _spa) internal view {
        assertContractAddress(address(_spa), "SuperchainProxyAdmin");
        if (_spa.owner() != l1PAO) revert ValidationFailed("SuperchainProxyAdmin-100");
    }

    function validateSuperchainConfig(ISuperchainConfig _sc, IProxyAdmin _spa) internal view {
        address scImpl = _spa.getProxyImplementation(address(_sc));
        assertContractAddress(address(_sc), "SuperchainConfigProxy");
        assertContractAddress(scImpl, "SuperchainConfigImpl");

        if (scImpl != address(impls.superchainConfigImpl)) revert ValidationFailed("SuperchainConfig-100");
        if (_sc.guardian() != guardian) revert ValidationFailed("SuperchainConfig-200");
        if (_sc.paused() != false) revert ValidationFailed("SuperchainConfig-300");
    }

    function validateProtocolVersions(IProtocolVersions _pv, IProxyAdmin _spa) internal view {
        address pvImpl = _spa.getProxyImplementation(address(_pv));
        assertContractAddress(address(_pv), "ProtocolVersionsProxy");
        assertContractAddress(pvImpl, "ProtocolVersionsImpl");

        if (pvImpl != address(impls.protocolVersionsImpl)) revert ValidationFailed("ProxyAdmin-100");
        if (ProtocolVersion.unwrap(_pv.required()) == 0) revert ValidationFailed("ProxyAdmin-200");
        if (ProtocolVersion.unwrap(_pv.recommended()) == 0) revert ValidationFailed("ProxyAdmin-300");
    }

    function assertContractAddress(address _who, bytes32 _contractName) internal view {
        if (_who.code.length == 0) revert AddressHasNoCode(_who, _contractName);
    }
}
