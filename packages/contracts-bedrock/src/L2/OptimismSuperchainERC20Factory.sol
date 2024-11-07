// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { OptimismSuperchainERC20 } from "src/L2/OptimismSuperchainERC20.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { BeaconProxy } from "@openzeppelin/contracts-v5/proxy/beacon/BeaconProxy.sol";
import { CREATE3 } from "@rari-capital/solmate/src/utils/CREATE3.sol";

/// @custom:proxied
/// @custom:predeployed 0x4200000000000000000000000000000000000026
/// @title OptimismSuperchainERC20Factory
/// @notice OptimismSuperchainERC20Factory is a factory contract that deploys OptimismSuperchainERC20 Beacon Proxies
///         using CREATE3.
contract OptimismSuperchainERC20Factory is ISemver {
    /// @notice Emitted when an OptimismSuperchainERC20 is deployed.
    /// @param superchainToken  Address of the OptimismSuperchainERC20 deployment.
    /// @param remoteToken      Address of the corresponding token on the remote chain.
    /// @param deployer         Address of the account that deployed the token.
    event OptimismSuperchainERC20Created(
        address indexed superchainToken, address indexed remoteToken, address deployer
    );

    /// @notice Semantic version.
    /// @custom:semver 1.0.0-beta.4
    string public constant version = "1.0.0-beta.4";

    /// @notice Mapping of the deployed OptimismSuperchainERC20 to the remote token address.
    ///         This is used to keep track of the token deployments.
    mapping(address _localToken => address remoteToken_) public deployments;

    /// @notice Deploys a OptimismSuperchainERC20 Beacon Proxy using CREATE3.
    /// @param _remoteToken      Address of the remote token.
    /// @param _name             Name of the OptimismSuperchainERC20.
    /// @param _symbol           Symbol of the OptimismSuperchainERC20.
    /// @param _decimals         Decimals of the OptimismSuperchainERC20.
    /// @return superchainERC20_ Address of the OptimismSuperchainERC20 deployment.
    function deploy(
        address _remoteToken,
        string memory _name,
        string memory _symbol,
        uint8 _decimals
    )
        external
        returns (address superchainERC20_)
    {
        bytes memory initCallData =
            abi.encodeCall(OptimismSuperchainERC20.initialize, (_remoteToken, _name, _symbol, _decimals));

        bytes memory creationCode = bytes.concat(
            type(BeaconProxy).creationCode, abi.encode(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_BEACON, initCallData)
        );

        bytes32 salt = keccak256(abi.encode(_remoteToken, _name, _symbol, _decimals));
        superchainERC20_ = CREATE3.deploy({ salt: salt, creationCode: creationCode, value: 0 });

        deployments[superchainERC20_] = _remoteToken;

        emit OptimismSuperchainERC20Created(superchainERC20_, _remoteToken, msg.sender);
    }
}
