// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { SafeSend } from "src/universal/SafeSend.sol";

// Libraries
import { SafeCall } from "src/libraries/SafeCall.sol";

// Interfaces
import { IFaucetAuthModule } from "src/periphery/faucet/authmodules/IFaucetAuthModule.sol";

/// @title  Faucet
/// @notice Faucet contract that drips ETH to users.
contract Faucet {
    /// @notice Emitted on each drip.
    /// @param authModule The type of authentication that was used for verifying the drip.
    /// @param userId     The id of the user that requested the drip.
    /// @param amount     The amount of funds sent.
    /// @param recipient  The recipient of the drip.
    event Drip(string indexed authModule, bytes32 indexed userId, uint256 amount, address indexed recipient);

    /// @notice Parameters for a drip.
    struct DripParameters {
        address payable recipient;
        bytes data;
        bytes32 nonce;
        uint32 gasLimit;
    }

    /// @notice Parameters for authentication.
    struct AuthParameters {
        IFaucetAuthModule module;
        bytes32 id;
        bytes proof;
    }

    /// @notice Configuration for an authentication module.
    struct ModuleConfig {
        string name;
        bool enabled;
        uint256 ttl;
        uint256 amount;
    }

    /// @notice Admin address that can configure the faucet.
    address public immutable ADMIN;

    /// @notice Mapping of authentication modules to their configurations.
    mapping(IFaucetAuthModule => ModuleConfig) public modules;

    /// @notice Mapping of authentication IDs to the next timestamp at which they can be used.
    mapping(IFaucetAuthModule => mapping(bytes32 => uint256)) public timeouts;

    /// @notice Maps from id to nonces to whether or not they have been used.
    mapping(bytes32 => mapping(bytes32 => bool)) public nonces;

    /// @notice Modifier that makes a function admin privileged.
    modifier privileged() {
        require(msg.sender == ADMIN, "Faucet: function can only be called by admin");
        _;
    }

    /// @param _admin Admin address that can configure the faucet.
    constructor(address _admin) {
        ADMIN = _admin;
    }

    /// @notice Allows users to donate ETH to this contract.
    receive() external payable {
        // Thank you!
    }

    /// @notice Allows the admin to withdraw funds.
    /// @param _recipient Address to receive the funds.
    /// @param _amount    Amount of ETH in wei to withdraw.
    function withdraw(address payable _recipient, uint256 _amount) public privileged {
        new SafeSend{ value: _amount }(_recipient);
    }

    /// @notice Allows the admin to configure an authentication module.
    /// @param _module Authentication module to configure.
    /// @param _config Configuration to set for the module.
    function configure(IFaucetAuthModule _module, ModuleConfig memory _config) public privileged {
        modules[_module] = _config;
    }

    /// @notice Drips ETH to a recipient account.
    /// @param _params Drip parameters.
    /// @param _auth   Authentication parameters.
    function drip(DripParameters memory _params, AuthParameters memory _auth) public {
        // Grab the module config once.
        ModuleConfig memory config = modules[_auth.module];

        // Make sure we're using a supported security module.
        require(config.enabled, "Faucet: provided auth module is not supported by this faucet");

        // The issuer's signature commits to a nonce to prevent replay attacks.
        // This checks that the nonce has not been used for this issuer before. The nonces are
        // scoped to the issuer address, so the same nonce can be used by different issuers without
        // clashing.
        require(nonces[_auth.id][_params.nonce] == false, "Faucet: nonce has already been used");

        // Make sure the timeout has elapsed.
        require(
            timeouts[_auth.module][_auth.id] < block.timestamp,
            "Faucet: auth cannot be used yet because timeout has not elapsed"
        );

        // Verify the proof.
        require(
            _auth.module.verify(_params, _auth.id, _auth.proof),
            "Faucet: drip parameters could not be verified by security module"
        );

        // Verify recepient is not the faucet address.
        require(_params.recipient != address(this), "Faucet: cannot drip to itself");

        // Set the next timestamp at which this auth id can be used.
        timeouts[_auth.module][_auth.id] = block.timestamp + config.ttl;

        // Mark the nonce as used.
        nonces[_auth.id][_params.nonce] = true;

        // Execute transfer of ETH to the recipient account.
        bool success = SafeCall.call(_params.recipient, _params.gasLimit, config.amount, _params.data);
        require(success, "Faucet: Failed to execute SafeCall during drip to recipient");

        emit Drip(config.name, _auth.id, config.amount, _params.recipient);
    }

    /// @notice Returns the enable value of a given auth module.
    /// @param _module module to check.
    /// @return bool enabled status of auth modulew.
    function isModuleEnabled(IFaucetAuthModule _module) public view returns (bool) {
        return modules[_module].enabled;
    }
}
