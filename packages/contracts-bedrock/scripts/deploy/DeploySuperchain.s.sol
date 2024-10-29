// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { stdToml } from "forge-std/StdToml.sol";

import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IProtocolVersions, ProtocolVersion } from "src/L1/interfaces/IProtocolVersions.sol";
import { IProxyAdmin } from "src/universal/interfaces/IProxyAdmin.sol";
import { IProxy } from "src/universal/interfaces/IProxy.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Solarray } from "scripts/libraries/Solarray.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";

// This comment block defines the requirements and rationale for the architecture used in this forge
// script, along with other scripts that are being written as new Superchain-first deploy scripts to
// complement the OP Contracts Manager. The script architecture is a bit different than a standard forge
// deployment script.
//
// There are three categories of users that are expected to interact with the scripts:
//   1. End users that want to run live contract deployments. These users are expected to run these scripts via
//      'op-deployer' which uses a go interface to interact with the scripts.
//   2. Solidity developers that want to use or test these scripts in a standard forge test environment.
//   3. Go developers that want to run the deploy scripts as part of e2e testing with other aspects of the OP Stack.
//
// We want each user to interact with the scripts in the way that's simplest for their use case:
//   1. Solidity developers: Direct calls to the script, with the input and output contracts configured.
//   2. Go developers: The forge scripts can be executed directly in Go.
//
// The following architecture is used to meet the requirements of each user. We use this file's
// `DeploySuperchain` script as an example, but it applies to other scripts as well.
//
// This `DeploySuperchain.s.sol` file contains three contracts:
//   1. `DeploySuperchainInput`: Responsible for parsing, storing, and exposing the input data.
//   2. `DeploySuperchainOutput`: Responsible for storing and exposing the output data.
//   3. `DeploySuperchain`: The core script that executes the deployment. It reads inputs from the
//      input contract, and writes outputs to the output contract.
//
// Because the core script performs calls to the input and output contracts, Go developers can
// intercept calls to these addresses (analogous to how forge intercepts calls to the `Vm` address
// to execute cheatcodes), to avoid the need for hardcoding the input/output values.
//
// Public getter methods on the input and output contracts allow individual fields to be accessed
// in a strong, type-safe manner (as opposed to a single struct getter where the caller may
// inadvertently transpose two addresses, for example).
//
// Each deployment step in the core deploy script is modularized into its own function that performs
// the deploy and sets the output on the Output contract, allowing for easy composition and testing
// of deployment steps. The output setter methods requires keying off the four-byte selector of
// each output field's getter method, ensuring that the output is set for the correct field and
// minimizing the amount of boilerplate needed for each output field.
//
// This script doubles as a reference for documenting the pattern used and therefore contains
// comments explaining the patterns used. Other scripts are not expected to have this level of
// documentation.
//
// Additionally, we intentionally use "Input" and "Output" terminology to clearly distinguish these
// scripts from the existing ones that use the "Config" and "Artifacts" terminology. Within scripts
// we use variable names that are shorthand for the full contract names, for example:
//   - `dsi` for DeploySuperchainInput
//   - `dso` for DeploySuperchainOutput
//   - `dio` for DeployImplementationsInput
//   - `dio` for DeployImplementationsOutput
//   - `doo` for DeployOPChainInput
//   - `doo` for DeployOPChainOutput
//   - etc.

// All contracts of the form `Deploy<X>Input` should inherit from `BaseDeployIO`, as it provides
// shared functionality for all deploy scripts, such as access to cheat codes.
contract DeploySuperchainInput is BaseDeployIO {
    using stdToml for string;

    // All inputs are set in storage individually. We put any roles first, followed by the remaining
    // inputs. Inputs are internal and prefixed with an underscore, because we will expose a getter
    // method that returns the input value. We use a getter method to allow us to make assertions on
    // the input to ensure it's valid before returning it. We also intentionally do not use a struct
    // to hold all inputs, because as features are developed the set of inputs will change, and
    // modifying structs in Solidity is not very simple.

    // Role inputs.
    address internal _guardian;
    address internal _protocolVersionsOwner;
    address internal _superchainProxyAdminOwner;

    // Other inputs.
    bool internal _paused;
    ProtocolVersion internal _recommendedProtocolVersion;
    ProtocolVersion internal _requiredProtocolVersion;

    // These `set` methods let each input be set individually. The selector of an input's getter method
    // is used to determine which field to set.
    function set(bytes4 _sel, address _address) public {
        require(_address != address(0), "DeploySuperchainInput: cannot set zero address");
        if (_sel == this.guardian.selector) _guardian = _address;
        else if (_sel == this.protocolVersionsOwner.selector) _protocolVersionsOwner = _address;
        else if (_sel == this.superchainProxyAdminOwner.selector) _superchainProxyAdminOwner = _address;
        else revert("DeploySuperchainInput: unknown selector");
    }

    function set(bytes4 _sel, bool _value) public {
        if (_sel == this.paused.selector) _paused = _value;
        else revert("DeploySuperchainInput: unknown selector");
    }

    function set(bytes4 _sel, ProtocolVersion _value) public {
        require(ProtocolVersion.unwrap(_value) != 0, "DeploySuperchainInput: cannot set null protocol version");
        if (_sel == this.recommendedProtocolVersion.selector) _recommendedProtocolVersion = _value;
        else if (_sel == this.requiredProtocolVersion.selector) _requiredProtocolVersion = _value;
        else revert("DeploySuperchainInput: unknown selector");
    }

    // Each input field is exposed via it's own getter method. Using public storage variables here
    // would be less verbose, but would also be more error-prone, as it would require the caller to
    // validate that each input is set before accessing it. With getter methods, we can automatically
    // validate that each input is set before allowing any field to be accessed.

    function superchainProxyAdminOwner() public view returns (address) {
        require(_superchainProxyAdminOwner != address(0), "DeploySuperchainInput: superchainProxyAdminOwner not set");
        return _superchainProxyAdminOwner;
    }

    function protocolVersionsOwner() public view returns (address) {
        require(_protocolVersionsOwner != address(0), "DeploySuperchainInput: protocolVersionsOwner not set");
        return _protocolVersionsOwner;
    }

    function guardian() public view returns (address) {
        require(_guardian != address(0), "DeploySuperchainInput: guardian not set");
        return _guardian;
    }

    function paused() public view returns (bool) {
        return _paused;
    }

    function requiredProtocolVersion() public view returns (ProtocolVersion) {
        require(
            ProtocolVersion.unwrap(_requiredProtocolVersion) != 0,
            "DeploySuperchainInput: requiredProtocolVersion not set"
        );
        return _requiredProtocolVersion;
    }

    function recommendedProtocolVersion() public view returns (ProtocolVersion) {
        require(
            ProtocolVersion.unwrap(_recommendedProtocolVersion) != 0,
            "DeploySuperchainInput: recommendedProtocolVersion not set"
        );
        return _recommendedProtocolVersion;
    }
}

// All contracts of the form `Deploy<X>Output` should inherit from `BaseDeployIO`, as it provides
// shared functionality for all deploy scripts, such as access to cheat codes.
contract DeploySuperchainOutput is BaseDeployIO {
    // All outputs are stored in storage individually, with the same rationale as doing so for
    // inputs, and the same pattern is used below to expose the outputs.
    IProtocolVersions internal _protocolVersionsImpl;
    IProtocolVersions internal _protocolVersionsProxy;
    ISuperchainConfig internal _superchainConfigImpl;
    ISuperchainConfig internal _superchainConfigProxy;
    IProxyAdmin internal _superchainProxyAdmin;

    // This method lets each field be set individually. The selector of an output's getter method
    // is used to determine which field to set.
    function set(bytes4 _sel, address _address) public {
        require(_address != address(0), "DeploySuperchainOutput: cannot set zero address");
        if (_sel == this.superchainProxyAdmin.selector) _superchainProxyAdmin = IProxyAdmin(_address);
        else if (_sel == this.superchainConfigImpl.selector) _superchainConfigImpl = ISuperchainConfig(_address);
        else if (_sel == this.superchainConfigProxy.selector) _superchainConfigProxy = ISuperchainConfig(_address);
        else if (_sel == this.protocolVersionsImpl.selector) _protocolVersionsImpl = IProtocolVersions(_address);
        else if (_sel == this.protocolVersionsProxy.selector) _protocolVersionsProxy = IProtocolVersions(_address);
        else revert("DeploySuperchainOutput: unknown selector");
    }

    // This function can be called to ensure all outputs are correct.
    // It fetches the output values using external calls to the getter methods for safety.
    function checkOutput(DeploySuperchainInput _dsi) public {
        address[] memory addrs = Solarray.addresses(
            address(this.superchainProxyAdmin()),
            address(this.superchainConfigImpl()),
            address(this.superchainConfigProxy()),
            address(this.protocolVersionsImpl()),
            address(this.protocolVersionsProxy())
        );
        DeployUtils.assertValidContractAddresses(addrs);

        // To read the implementations we prank as the zero address due to the proxyCallIfNotAdmin modifier.
        vm.startPrank(address(0));
        address actualSuperchainConfigImpl = IProxy(payable(address(_superchainConfigProxy))).implementation();
        address actualProtocolVersionsImpl = IProxy(payable(address(_protocolVersionsProxy))).implementation();
        vm.stopPrank();

        require(actualSuperchainConfigImpl == address(_superchainConfigImpl), "100"); // nosemgrep:
            // sol-style-malformed-require
        require(actualProtocolVersionsImpl == address(_protocolVersionsImpl), "200"); // nosemgrep:
            // sol-style-malformed-require

        assertValidDeploy(_dsi);
    }

    function superchainProxyAdmin() public view returns (IProxyAdmin) {
        // This does not have to be a contract address, it could be an EOA.
        return _superchainProxyAdmin;
    }

    function superchainConfigImpl() public view returns (ISuperchainConfig) {
        DeployUtils.assertValidContractAddress(address(_superchainConfigImpl));
        return _superchainConfigImpl;
    }

    function superchainConfigProxy() public view returns (ISuperchainConfig) {
        DeployUtils.assertValidContractAddress(address(_superchainConfigProxy));
        return _superchainConfigProxy;
    }

    function protocolVersionsImpl() public view returns (IProtocolVersions) {
        DeployUtils.assertValidContractAddress(address(_protocolVersionsImpl));
        return _protocolVersionsImpl;
    }

    function protocolVersionsProxy() public view returns (IProtocolVersions) {
        DeployUtils.assertValidContractAddress(address(_protocolVersionsProxy));
        return _protocolVersionsProxy;
    }

    // -------- Deployment Assertions --------
    function assertValidDeploy(DeploySuperchainInput _dsi) public {
        assertValidSuperchainProxyAdmin(_dsi);
        assertValidSuperchainConfig(_dsi);
        assertValidProtocolVersions(_dsi);
    }

    function assertValidSuperchainProxyAdmin(DeploySuperchainInput _dsi) internal view {
        require(superchainProxyAdmin().owner() == _dsi.superchainProxyAdminOwner(), "SPA-10");
    }

    function assertValidSuperchainConfig(DeploySuperchainInput _dsi) internal {
        // Proxy checks.
        ISuperchainConfig superchainConfig = superchainConfigProxy();
        DeployUtils.assertInitialized({ _contractAddress: address(superchainConfig), _slot: 0, _offset: 0 });
        require(superchainConfig.guardian() == _dsi.guardian(), "SUPCON-10");
        require(superchainConfig.paused() == _dsi.paused(), "SUPCON-20");

        vm.startPrank(address(0));
        require(
            IProxy(payable(address(superchainConfig))).implementation() == address(superchainConfigImpl()), "SUPCON-30"
        );
        require(IProxy(payable(address(superchainConfig))).admin() == address(superchainProxyAdmin()), "SUPCON-40");
        vm.stopPrank();

        // Implementation checks
        superchainConfig = superchainConfigImpl();
        require(superchainConfig.guardian() == address(0), "SUPCON-50");
        require(superchainConfig.paused() == false, "SUPCON-60");
    }

    function assertValidProtocolVersions(DeploySuperchainInput _dsi) internal {
        // Proxy checks.
        IProtocolVersions pv = protocolVersionsProxy();
        DeployUtils.assertInitialized({ _contractAddress: address(pv), _slot: 0, _offset: 0 });
        require(pv.owner() == _dsi.protocolVersionsOwner(), "PV-10");
        require(
            ProtocolVersion.unwrap(pv.required()) == ProtocolVersion.unwrap(_dsi.requiredProtocolVersion()), "PV-20"
        );
        require(
            ProtocolVersion.unwrap(pv.recommended()) == ProtocolVersion.unwrap(_dsi.recommendedProtocolVersion()),
            "PV-30"
        );

        vm.startPrank(address(0));
        require(IProxy(payable(address(pv))).implementation() == address(protocolVersionsImpl()), "PV-40");
        require(IProxy(payable(address(pv))).admin() == address(superchainProxyAdmin()), "PV-50");
        vm.stopPrank();

        // Implementation checks.
        pv = protocolVersionsImpl();
        require(pv.owner() == address(0xdead), "PV-60");
        require(ProtocolVersion.unwrap(pv.required()) == 0, "PV-70");
        require(ProtocolVersion.unwrap(pv.recommended()) == 0, "PV-80");
    }
}

// For all broadcasts in this script we explicitly specify the deployer as `msg.sender` because for
// testing we deploy this script from a test contract. If we provide no argument, the foundry
// default sender would be the broadcaster during test, but the broadcaster needs to be the deployer
// since they are set to the initial proxy admin owner.
contract DeploySuperchain is Script {
    // -------- Core Deployment Methods --------

    function run(DeploySuperchainInput _dsi, DeploySuperchainOutput _dso) public {
        // Notice that we do not do any explicit verification here that inputs are set. This is because
        // the verification happens elsewhere:
        //   - Getter methods on the input contract provide sanity checks that values are set, when applicable.
        //   - The individual methods below that we use to compose the deployment are responsible for handling
        //     their own verification.
        // This pattern ensures that other deploy scripts that might compose these contracts and
        // methods in different ways are still protected from invalid inputs without need to implement
        // additional verification logic.

        // Deploy the proxy admin, with the owner set to the deployer.
        deploySuperchainProxyAdmin(_dsi, _dso);

        // Deploy and initialize the superchain contracts.
        deploySuperchainImplementationContracts(_dsi, _dso);
        deployAndInitializeSuperchainConfig(_dsi, _dso);
        deployAndInitializeProtocolVersions(_dsi, _dso);

        // Transfer ownership of the ProxyAdmin from the deployer to the specified owner.
        transferProxyAdminOwnership(_dsi, _dso);

        // Output assertions, to make sure outputs were assigned correctly.
        _dso.checkOutput(_dsi);
    }

    // -------- Deployment Steps --------

    function deploySuperchainProxyAdmin(DeploySuperchainInput, DeploySuperchainOutput _dso) public {
        // Deploy the proxy admin, with the owner set to the deployer.
        // We explicitly specify the deployer as `msg.sender` because for testing we deploy this script from a test
        // contract. If we provide no argument, the foundry default sender would be the broadcaster during test, but the
        // broadcaster needs to be the deployer since they are set to the initial proxy admin owner.
        vm.broadcast(msg.sender);
        IProxyAdmin superchainProxyAdmin = IProxyAdmin(
            DeployUtils.create1({
                _name: "ProxyAdmin",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProxyAdmin.__constructor__, (msg.sender)))
            })
        );

        vm.label(address(superchainProxyAdmin), "SuperchainProxyAdmin");
        _dso.set(_dso.superchainProxyAdmin.selector, address(superchainProxyAdmin));
    }

    function deploySuperchainImplementationContracts(DeploySuperchainInput, DeploySuperchainOutput _dso) public {
        // Deploy implementation contracts.
        vm.startBroadcast(msg.sender);
        ISuperchainConfig superchainConfigImpl = ISuperchainConfig(
            DeployUtils.create1({
                _name: "SuperchainConfig",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(ISuperchainConfig.__constructor__, ()))
            })
        );
        IProtocolVersions protocolVersionsImpl = IProtocolVersions(
            DeployUtils.create1({
                _name: "ProtocolVersions",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IProtocolVersions.__constructor__, ()))
            })
        );
        vm.stopBroadcast();

        vm.label(address(superchainConfigImpl), "SuperchainConfigImpl");
        vm.label(address(protocolVersionsImpl), "ProtocolVersionsImpl");

        _dso.set(_dso.superchainConfigImpl.selector, address(superchainConfigImpl));
        _dso.set(_dso.protocolVersionsImpl.selector, address(protocolVersionsImpl));
    }

    function deployAndInitializeSuperchainConfig(DeploySuperchainInput _dsi, DeploySuperchainOutput _dso) public {
        address guardian = _dsi.guardian();
        bool paused = _dsi.paused();

        IProxyAdmin superchainProxyAdmin = _dso.superchainProxyAdmin();
        ISuperchainConfig superchainConfigImpl = _dso.superchainConfigImpl();

        vm.startBroadcast(msg.sender);
        ISuperchainConfig superchainConfigProxy = ISuperchainConfig(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IProxy.__constructor__, (address(superchainProxyAdmin)))
                )
            })
        );
        superchainProxyAdmin.upgradeAndCall(
            payable(address(superchainConfigProxy)),
            address(superchainConfigImpl),
            abi.encodeCall(ISuperchainConfig.initialize, (guardian, paused))
        );
        vm.stopBroadcast();

        vm.label(address(superchainConfigProxy), "SuperchainConfigProxy");
        _dso.set(_dso.superchainConfigProxy.selector, address(superchainConfigProxy));
    }

    function deployAndInitializeProtocolVersions(DeploySuperchainInput _dsi, DeploySuperchainOutput _dso) public {
        address protocolVersionsOwner = _dsi.protocolVersionsOwner();
        ProtocolVersion requiredProtocolVersion = _dsi.requiredProtocolVersion();
        ProtocolVersion recommendedProtocolVersion = _dsi.recommendedProtocolVersion();

        IProxyAdmin superchainProxyAdmin = _dso.superchainProxyAdmin();
        IProtocolVersions protocolVersionsImpl = _dso.protocolVersionsImpl();

        vm.startBroadcast(msg.sender);
        IProtocolVersions protocolVersionsProxy = IProtocolVersions(
            DeployUtils.create1({
                _name: "Proxy",
                _args: DeployUtils.encodeConstructor(
                    abi.encodeCall(IProxy.__constructor__, (address(superchainProxyAdmin)))
                )
            })
        );
        superchainProxyAdmin.upgradeAndCall(
            payable(address(protocolVersionsProxy)),
            address(protocolVersionsImpl),
            abi.encodeCall(
                IProtocolVersions.initialize,
                (protocolVersionsOwner, requiredProtocolVersion, recommendedProtocolVersion)
            )
        );
        vm.stopBroadcast();

        vm.label(address(protocolVersionsProxy), "ProtocolVersionsProxy");
        _dso.set(_dso.protocolVersionsProxy.selector, address(protocolVersionsProxy));
    }

    function transferProxyAdminOwnership(DeploySuperchainInput _dsi, DeploySuperchainOutput _dso) public {
        address superchainProxyAdminOwner = _dsi.superchainProxyAdminOwner();

        IProxyAdmin superchainProxyAdmin = _dso.superchainProxyAdmin();
        DeployUtils.assertValidContractAddress(address(superchainProxyAdmin));

        vm.broadcast(msg.sender);
        superchainProxyAdmin.transferOwnership(superchainProxyAdminOwner);
    }

    // -------- Utilities --------

    // This etches the IO contracts into memory so that we can use them in tests.
    // When interacting with the script programmatically (e.g. in a Solidity test), this must be called.
    function etchIOContracts() public returns (DeploySuperchainInput dsi_, DeploySuperchainOutput dso_) {
        (dsi_, dso_) = getIOContracts();
        vm.etch(address(dsi_), type(DeploySuperchainInput).runtimeCode);
        vm.etch(address(dso_), type(DeploySuperchainOutput).runtimeCode);
        vm.allowCheatcodes(address(dsi_));
        vm.allowCheatcodes(address(dso_));
    }

    // This returns the addresses of the IO contracts for this script.
    function getIOContracts() public view returns (DeploySuperchainInput dsi_, DeploySuperchainOutput dso_) {
        dsi_ = DeploySuperchainInput(DeployUtils.toIOAddress(msg.sender, "optimism.DeploySuperchainInput"));
        dso_ = DeploySuperchainOutput(DeployUtils.toIOAddress(msg.sender, "optimism.DeploySuperchainOutput"));
    }
}
