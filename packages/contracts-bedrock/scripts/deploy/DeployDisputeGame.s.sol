// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { GameType, Claim, Duration } from "src/dispute/lib/Types.sol";
import { LibString } from "@solady/utils/LibString.sol";

// Interfaces
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";
import { IPermissionedDisputeGame } from "src/dispute/interfaces/IPermissionedDisputeGame.sol";
import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";
import { IBigStepper } from "src/dispute/interfaces/IBigStepper.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";
import { IMIPS } from "src/cannon/interfaces/IMIPS.sol";

/// @title DeployDisputeGameInput
contract DeployDisputeGameInput is BaseDeployIO {
    // Common inputs.
    string internal _release;
    string internal _standardVersionsToml;

    // Specify which MIPS version to use.
    uint256 internal _mipsVersion;

    // All inputs required to deploy PreimageOracle.
    uint256 internal _minProposalSizeBytes;
    uint256 internal _challengePeriodSeconds;

    // Specify which game kind is being deployed here.
    string internal _gameKind;

    // All inputs required to deploy FaultDisputeGame.
    uint256 internal _gameType;
    bytes32 internal _absolutePrestate;
    uint256 internal _maxGameDepth;
    uint256 internal _splitDepth;
    uint256 internal _clockExtension;
    uint256 internal _maxClockDuration;
    IDelayedWETH internal _delayedWethProxy;
    IAnchorStateRegistry internal _anchorStateRegistryProxy;
    uint256 internal _l2ChainId;

    // Additional inputs required to deploy PermissionedDisputeGame.
    address internal _proposer;
    address internal _challenger;

    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.mipsVersion.selector) {
            require(_value == 1 || _value == 2, "DeployDisputeGame: unknown mips version");
            _mipsVersion = _value;
        } else if (_sel == this.minProposalSizeBytes.selector) {
            require(_value != 0, "DeployDisputeGame: minProposalSizeBytes cannot be zero");
            _minProposalSizeBytes = _value;
        } else if (_sel == this.challengePeriodSeconds.selector) {
            require(_value != 0, "DeployDisputeGame: challengePeriodSeconds cannot be zero");
            _challengePeriodSeconds = _value;
        } else if (_sel == this.gameType.selector) {
            require(_value <= type(uint32).max, "DeployDisputeGame: gameType must fit inside uint32");
            _gameType = _value;
        } else if (_sel == this.maxGameDepth.selector) {
            require(_value != 0, "DeployDisputeGame: maxGameDepth cannot be zero");
            _maxGameDepth = _value;
        } else if (_sel == this.splitDepth.selector) {
            require(_value != 0, "DeployDisputeGame: splitDepth cannot be zero");
            _splitDepth = _value;
        } else if (_sel == this.clockExtension.selector) {
            require(_value <= type(uint64).max, "DeployDisputeGame: clockExtension must fit inside uint64");
            require(_value != 0, "DeployDisputeGame: clockExtension cannot be zero");
            _clockExtension = _value;
        } else if (_sel == this.maxClockDuration.selector) {
            require(_value <= type(uint64).max, "DeployDisputeGame: maxClockDuration must fit inside uint64");
            require(_value != 0, "DeployDisputeGame: maxClockDuration cannot be zero");
            _maxClockDuration = _value;
        } else if (_sel == this.l2ChainId.selector) {
            require(_value != 0, "DeployDisputeGame: l2ChainId cannot be zero");
            _l2ChainId = _value;
        } else {
            revert("DeployDisputeGame: unknown selector");
        }
    }

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.delayedWethProxy.selector) {
            require(_value != address(0), "DeployDisputeGame: delayedWethProxy cannot be zero address");
            _delayedWethProxy = IDelayedWETH(payable(_value));
        } else if (_sel == this.anchorStateRegistryProxy.selector) {
            require(_value != address(0), "DeployDisputeGame: anchorStateRegistryProxy cannot be zero address");
            _anchorStateRegistryProxy = IAnchorStateRegistry(payable(_value));
        } else if (_sel == this.proposer.selector) {
            require(_value != address(0), "DeployDisputeGame: proposer cannot be zero address");
            _proposer = _value;
        } else if (_sel == this.challenger.selector) {
            require(_value != address(0), "DeployDisputeGame: challenger cannot be zero address");
            _challenger = _value;
        } else {
            revert("DeployDisputeGame: unknown selector");
        }
    }

    function set(bytes4 _sel, string memory _value) public {
        if (_sel == this.gameKind.selector) {
            require(
                LibString.eq(_value, "FaultDisputeGame") || LibString.eq(_value, "PermissionedDisputeGame"),
                "DeployDisputeGame: unknown game kind"
            );
            _gameKind = _value;
        } else if (_sel == this.release.selector) {
            require(!LibString.eq(_value, ""), "DeployDisputeGame: release cannot be empty");
            _release = _value;
        } else if (_sel == this.standardVersionsToml.selector) {
            require(!LibString.eq(_value, ""), "DeployDisputeGame: standardVersionsToml cannot be empty");
            _standardVersionsToml = _value;
        } else {
            revert("DeployDisputeGame: unknown selector");
        }
    }

    function release() public view returns (string memory) {
        require(!LibString.eq(_release, ""), "DeployDisputeGame: release not set");
        return _release;
    }

    function standardVersionsToml() public view returns (string memory) {
        require(!LibString.eq(_standardVersionsToml, ""), "DeployDisputeGame: standardVersionsToml not set");
        return _standardVersionsToml;
    }

    function mipsVersion() public view returns (uint256) {
        require(_mipsVersion != 0, "DeployDisputeGame: mipsVersion not set");
        require(_mipsVersion == 1 || _mipsVersion == 2, "DeployDisputeGame: unknown mips version");
        return _mipsVersion;
    }

    function minProposalSizeBytes() public view returns (uint256) {
        require(_minProposalSizeBytes != 0, "DeployDisputeGame: minProposalSizeBytes not set");
        return _minProposalSizeBytes;
    }

    function challengePeriodSeconds() public view returns (uint256) {
        require(_challengePeriodSeconds != 0, "DeployDisputeGame: challengePeriodSeconds not set");
        return _challengePeriodSeconds;
    }

    function gameKind() public view returns (string memory) {
        require(
            LibString.eq(_gameKind, "FaultDisputeGame") || LibString.eq(_gameKind, "PermissionedDisputeGame"),
            "DeployDisputeGame: unknown game kind"
        );
        return _gameKind;
    }

    function gameType() public view returns (uint256) {
        require(_gameType <= type(uint32).max, "DeployDisputeGame: gameType must fit inside uint32");
        return _gameType;
    }

    function absolutePrestate() public view returns (bytes32) {
        require(_absolutePrestate != bytes32(0), "DeployDisputeGame: absolutePrestate not set");
        return _absolutePrestate;
    }

    function maxGameDepth() public view returns (uint256) {
        require(_maxGameDepth != 0, "DeployDisputeGame: maxGameDepth not set");
        return _maxGameDepth;
    }

    function splitDepth() public view returns (uint256) {
        require(_splitDepth != 0, "DeployDisputeGame: splitDepth not set");
        return _splitDepth;
    }

    function clockExtension() public view returns (uint256) {
        require(_clockExtension <= type(uint64).max, "DeployDisputeGame: clockExtension must fit inside uint64");
        require(_clockExtension != 0, "DeployDisputeGame: clockExtension not set");
        return _clockExtension;
    }

    function maxClockDuration() public view returns (uint256) {
        require(_maxClockDuration <= type(uint64).max, "DeployDisputeGame: maxClockDuration must fit inside uint64");
        require(_maxClockDuration != 0, "DeployDisputeGame: maxClockDuration not set");
        return _maxClockDuration;
    }

    function delayedWethProxy() public view returns (IDelayedWETH) {
        require(address(_delayedWethProxy) != address(0), "DeployDisputeGame: delayedWethProxy not set");
        return _delayedWethProxy;
    }

    function anchorStateRegistryProxy() public view returns (IAnchorStateRegistry) {
        require(address(_anchorStateRegistryProxy) != address(0), "DeployDisputeGame: anchorStateRegistryProxy not set");
        return _anchorStateRegistryProxy;
    }

    function l2ChainId() public view returns (uint256) {
        require(_l2ChainId != 0, "DeployDisputeGame: l2ChainId not set");
        return _l2ChainId;
    }

    function proposer() public view returns (address) {
        if (LibString.eq(_gameKind, "FaultDisputeGame")) {
            require(_proposer == address(0), "DeployDisputeGame: proposer must be empty");
        } else {
            require(_proposer != address(0), "DeployDisputeGame: proposer not set");
        }
        return _proposer;
    }

    function challenger() public view returns (address) {
        if (LibString.eq(_gameKind, "FaultDisputeGame")) {
            require(_challenger == address(0), "DeployDisputeGame: challenger must be empty");
        } else {
            require(_challenger != address(0), "DeployDisputeGame: challenger not set");
        }
        return _challenger;
    }
}

/// @title DeployDisputeGameOutput
contract DeployDisputeGameOutput is BaseDeployIO {
    // PermissionedDisputeGame is used as the type here because it has all of the same functions as
    // FaultDisputeGame but with the added proposer and challenger fields.
    IPermissionedDisputeGame internal _disputeGameImpl;
    IMIPS internal _mipsSingleton;
    IPreimageOracle internal _preimageOracleSingleton;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.disputeGameImpl.selector) {
            require(_value != address(0), "DeployDisputeGame: disputeGameImpl cannot be zero address");
            _disputeGameImpl = IPermissionedDisputeGame(_value);
        } else if (_sel == this.mipsSingleton.selector) {
            require(_value != address(0), "DeployDisputeGame: mipsSingleton cannot be zero address");
            _mipsSingleton = IMIPS(_value);
        } else if (_sel == this.preimageOracleSingleton.selector) {
            require(_value != address(0), "DeployDisputeGame: preimageOracleSingleton cannot be zero address");
            _preimageOracleSingleton = IPreimageOracle(_value);
        } else {
            revert("DeployDisputeGame: unknown selector");
        }
    }

    function checkOutput(DeployDisputeGameInput _dgi) public view {
        DeployUtils.assertValidContractAddress(address(_preimageOracleSingleton));
        DeployUtils.assertValidContractAddress(address(_mipsSingleton));
        DeployUtils.assertValidContractAddress(address(_disputeGameImpl));
        assertValidDeploy(_dgi);
    }

    function preimageOracleSingleton() public view returns (IPreimageOracle) {
        DeployUtils.assertValidContractAddress(address(_preimageOracleSingleton));
        return _preimageOracleSingleton;
    }

    function mipsSingleton() public view returns (IMIPS) {
        DeployUtils.assertValidContractAddress(address(_mipsSingleton));
        return _mipsSingleton;
    }

    function disputeGameImpl() public view returns (IPermissionedDisputeGame) {
        DeployUtils.assertValidContractAddress(address(_disputeGameImpl));
        return _disputeGameImpl;
    }

    function assertValidDeploy(DeployDisputeGameInput _dgi) public view {
        assertValidPreimageOracleSingleton(_dgi);
        assertValidMipsSingleton(_dgi);
        assertValidDisputeGameImpl(_dgi);
    }

    function assertValidPreimageOracleSingleton(DeployDisputeGameInput _dgi) internal view {
        IPreimageOracle oracle = preimageOracleSingleton();

        require(oracle.minProposalSize() == _dgi.minProposalSizeBytes(), "PO-10");
        require(oracle.challengePeriod() == _dgi.challengePeriodSeconds(), "PO-20");
    }

    function assertValidMipsSingleton(DeployDisputeGameInput) internal view {
        IMIPS mips = mipsSingleton();

        require(address(mips.oracle()) == address(preimageOracleSingleton()), "MIPS-10");
    }

    function assertValidDisputeGameImpl(DeployDisputeGameInput _dgi) internal view {
        IPermissionedDisputeGame game = disputeGameImpl();

        require(game.gameType().raw() == uint32(_dgi.gameType()), "DG-10");
        require(game.maxGameDepth() == _dgi.maxGameDepth(), "DG-20");
        require(game.splitDepth() == _dgi.splitDepth(), "DG-30");
        require(game.clockExtension().raw() == uint64(_dgi.clockExtension()), "DG-40");
        require(game.maxClockDuration().raw() == uint64(_dgi.maxClockDuration()), "DG-50");
        require(game.vm() == IBigStepper(address(mipsSingleton())), "DG-60");
        require(game.weth() == _dgi.delayedWethProxy(), "DG-70");
        require(game.anchorStateRegistry() == _dgi.anchorStateRegistryProxy(), "DG-80");
        require(game.l2ChainId() == _dgi.l2ChainId(), "DG-90");

        if (LibString.eq(_dgi.gameKind(), "PermissionedDisputeGame")) {
            require(game.proposer() == _dgi.proposer(), "DG-100");
            require(game.challenger() == _dgi.challenger(), "DG-110");
        }
    }
}

/// @title DeployDisputeGame
contract DeployDisputeGame is Script {
    /// We need a struct for constructor args to avoid stack-too-deep errors.
    struct DisputeGameConstructorArgs {
        GameType gameType;
        Claim absolutePrestate;
        uint256 maxGameDepth;
        uint256 splitDepth;
        Duration clockExtension;
        Duration maxClockDuration;
        IBigStepper gameVm;
        IDelayedWETH delayedWethProxy;
        IAnchorStateRegistry anchorStateRegistryProxy;
        uint256 l2ChainId;
        address proposer;
        address challenger;
    }

    function run(DeployDisputeGameInput _dgi, DeployDisputeGameOutput _dgo) public {
        deployPreimageOracleSingleton(_dgi, _dgo);
        deployMipsSingleton(_dgi, _dgo);
        deployDisputeGameImpl(_dgi, _dgo);
        _dgo.checkOutput(_dgi);
    }

    function deployPreimageOracleSingleton(DeployDisputeGameInput _dgi, DeployDisputeGameOutput _dgo) internal {
        string memory release = _dgi.release();
        string memory stdVerToml = _dgi.standardVersionsToml();
        string memory contractName = "preimage_oracle";
        IPreimageOracle singleton;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            singleton = IPreimageOracle(payable(existingImplementation));
        } else if (isDevelopRelease(release)) {
            uint256 minProposalSizeBytes = _dgi.minProposalSizeBytes();
            uint256 challengePeriodSeconds = _dgi.challengePeriodSeconds();
            vm.broadcast(msg.sender);
            singleton = IPreimageOracle(
                DeployUtils.create1({
                    _name: "PreimageOracle",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(IPreimageOracle.__constructor__, (minProposalSizeBytes, challengePeriodSeconds))
                    )
                })
            );
        } else {
            revert(string.concat("DeployImplementations: failed to deploy release ", release));
        }

        vm.label(address(singleton), "PreimageOracleSingleton");
        _dgo.set(_dgo.preimageOracleSingleton.selector, address(singleton));
    }

    function deployMipsSingleton(DeployDisputeGameInput _dgi, DeployDisputeGameOutput _dgo) internal {
        string memory release = _dgi.release();
        string memory stdVerToml = _dgi.standardVersionsToml();
        string memory contractName = "mips";
        IMIPS singleton;

        address existingImplementation = getReleaseAddress(release, contractName, stdVerToml);
        if (existingImplementation != address(0)) {
            singleton = IMIPS(payable(existingImplementation));
        } else if (isDevelopRelease(release)) {
            uint256 mipsVersion = _dgi.mipsVersion();
            IPreimageOracle preimageOracle = IPreimageOracle(address(_dgo.preimageOracleSingleton()));
            vm.broadcast(msg.sender);
            singleton = IMIPS(
                DeployUtils.create1({
                    _name: mipsVersion == 1 ? "MIPS" : "MIPS2",
                    _args: DeployUtils.encodeConstructor(abi.encodeCall(IMIPS.__constructor__, (preimageOracle)))
                })
            );
        } else {
            revert(string.concat("DeployImplementations: failed to deploy release ", release));
        }

        vm.label(address(singleton), "MIPSSingleton");
        _dgo.set(_dgo.mipsSingleton.selector, address(singleton));
    }

    function deployDisputeGameImpl(DeployDisputeGameInput _dgi, DeployDisputeGameOutput _dgo) internal {
        // Shove the arguments into a struct to avoid stack-too-deep errors.
        DisputeGameConstructorArgs memory args = DisputeGameConstructorArgs({
            gameType: GameType.wrap(uint32(_dgi.gameType())),
            absolutePrestate: Claim.wrap(_dgi.absolutePrestate()),
            maxGameDepth: _dgi.maxGameDepth(),
            splitDepth: _dgi.splitDepth(),
            clockExtension: Duration.wrap(uint64(_dgi.clockExtension())),
            maxClockDuration: Duration.wrap(uint64(_dgi.maxClockDuration())),
            gameVm: IBigStepper(address(_dgo.mipsSingleton())),
            delayedWethProxy: _dgi.delayedWethProxy(),
            anchorStateRegistryProxy: _dgi.anchorStateRegistryProxy(),
            l2ChainId: _dgi.l2ChainId(),
            proposer: _dgi.proposer(),
            challenger: _dgi.challenger()
        });

        // PermissionedDisputeGame is used as the type here because it is a superset of
        // FaultDisputeGame. If the user requests to deploy a FaultDisputeGame, the user will get a
        // FaultDisputeGame (and not a PermissionedDisputeGame).
        vm.broadcast(msg.sender);
        IPermissionedDisputeGame impl;
        if (LibString.eq(_dgi.gameKind(), "FaultDisputeGame")) {
            impl = IPermissionedDisputeGame(
                DeployUtils.create1({
                    _name: "FaultDisputeGame",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(
                            IFaultDisputeGame.__constructor__,
                            (
                                args.gameType,
                                args.absolutePrestate,
                                args.maxGameDepth,
                                args.splitDepth,
                                args.clockExtension,
                                args.maxClockDuration,
                                args.gameVm,
                                args.delayedWethProxy,
                                args.anchorStateRegistryProxy,
                                args.l2ChainId
                            )
                        )
                    )
                })
            );
        } else {
            impl = IPermissionedDisputeGame(
                DeployUtils.create1({
                    _name: "PermissionedDisputeGame",
                    _args: DeployUtils.encodeConstructor(
                        abi.encodeCall(
                            IPermissionedDisputeGame.__constructor__,
                            (
                                args.gameType,
                                args.absolutePrestate,
                                args.maxGameDepth,
                                args.splitDepth,
                                args.clockExtension,
                                args.maxClockDuration,
                                args.gameVm,
                                args.delayedWethProxy,
                                args.anchorStateRegistryProxy,
                                args.l2ChainId,
                                args.proposer,
                                args.challenger
                            )
                        )
                    )
                })
            );
        }

        vm.label(address(impl), string.concat(_dgi.gameKind(), "Impl"));
        _dgo.set(_dgo.disputeGameImpl.selector, address(impl));
    }

    // Zero address is returned if the address is not found in '_standardVersionsToml'.
    function getReleaseAddress(
        string memory _version,
        string memory _contractName,
        string memory _standardVersionsToml
    )
        internal
        pure
        returns (address addr_)
    {
        string memory baseKey = string.concat('.releases["', _version, '"].', _contractName);
        string memory implAddressKey = string.concat(baseKey, ".implementation_address");
        string memory addressKey = string.concat(baseKey, ".address");
        try vm.parseTomlAddress(_standardVersionsToml, implAddressKey) returns (address parsedAddr_) {
            addr_ = parsedAddr_;
        } catch {
            try vm.parseTomlAddress(_standardVersionsToml, addressKey) returns (address parsedAddr_) {
                addr_ = parsedAddr_;
            } catch {
                addr_ = address(0);
            }
        }
    }

    // A release is considered a 'develop' release if it does not start with 'op-contracts'.
    function isDevelopRelease(string memory _release) internal pure returns (bool) {
        return !LibString.startsWith(_release, "op-contracts");
    }
}
