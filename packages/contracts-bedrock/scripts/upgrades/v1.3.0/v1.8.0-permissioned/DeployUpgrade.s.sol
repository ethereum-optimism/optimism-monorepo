// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Forge
import { StdAssertions } from "forge-std/StdAssertions.sol";
import { console2 as console } from "forge-std/console2.sol";

// Scripts
import { Deploy } from "scripts/deploy/Deploy.s.sol";
import { ChainAssertions } from "scripts/deploy/ChainAssertions.sol";
import { Types } from "scripts/libraries/Types.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Contracts
import { Proxy } from "src/universal/Proxy.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { IDisputeGameFactory } from "src/dispute/interfaces/IDisputeGameFactory.sol";
import { PermissionedDisputeGame } from "src/dispute/PermissionedDisputeGame.sol";
import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";
import { IPreimageOracle } from "src/cannon/interfaces/IPreimageOracle.sol";
import { MIPS } from "src/cannon/MIPS.sol";
import { IBigStepper } from "src/dispute/interfaces/IBigStepper.sol";
import { IDisputeGame } from "src/dispute/interfaces/IDisputeGame.sol";
import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { Blueprint } from "src/libraries/Blueprint.sol";
import { GameTypes } from "src/dispute/lib/Types.sol";
import { Duration } from "src/dispute/lib/LibUDT.sol";

// Libraries
import { GameTypes, OutputRoot, Hash } from "src/dispute/lib/Types.sol";
import { Constants } from "src/libraries/Constants.sol";

/// @title DeployUpgrade
/// @notice Script for deploying contracts required to upgrade from v1.8.0 L2OO to v1.8.0 in a
///         PERMISSIONED configuration.
contract DeployUpgrade is Deploy, StdAssertions {
    /// @notice Address of the ProxyAdmin contract.
    address public proxyAdmin;

    /// @notice Address of the SystemOwnerSafe contract.
    address public systemOwnerSafe;

    /// @notice Address of the SuperchainConfigProxy contract.
    address public superchainConfigProxy;

    /// @notice Deployment bytecode of the AnchorStateRegistry implementation for v1.6.0.
    bytes public constant ANCHOR_STATE_REGISTRY_BYTECODE = hex"60a060405234801561001057600080fd5b506040516111d73803806111d783398101604081905261002f9161010a565b6001600160a01b03811660805261004461004a565b5061013a565b600054610100900460ff16156100b65760405162461bcd60e51b815260206004820152602760248201527f496e697469616c697a61626c653a20636f6e747261637420697320696e697469604482015266616c697a696e6760c81b606482015260840160405180910390fd5b60005460ff9081161015610108576000805460ff191660ff9081179091556040519081527f7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb38474024989060200160405180910390a15b565b60006020828403121561011c57600080fd5b81516001600160a01b038116811461013357600080fd5b9392505050565b608051611074610163600039600081816101830152818161033c01526108f801526110746000f3fe608060405234801561001057600080fd5b506004361061007d5760003560e01c80635e05fbd01161005b5780635e05fbd01461012a5780637258a8071461013d578063838c2d1e14610179578063f2b4e6171461018157600080fd5b806317cf21a91461008257806335e80ab31461009757806354fd4d50146100e1575b600080fd5b610095610090366004610b4c565b6101a7565b005b6002546100b79073ffffffffffffffffffffffffffffffffffffffff1681565b60405173ffffffffffffffffffffffffffffffffffffffff90911681526020015b60405180910390f35b61011d6040518060400160405280600581526020017f322e302e3000000000000000000000000000000000000000000000000000000081525081565b6040516100d89190610bea565b610095610138366004610cc6565b61061c565b61016461014b366004610df0565b6001602081905260009182526040909120805491015482565b604080519283526020830191909152016100d8565b610095610853565b7f00000000000000000000000000000000000000000000000000000000000000006100b7565b600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663452a93206040518163ffffffff1660e01b8152600401602060405180830381865afa158015610214573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906102389190610e0d565b73ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161461029c576040517f82b4290000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60008060008373ffffffffffffffffffffffffffffffffffffffff1663fa24f7436040518163ffffffff1660e01b8152600401600060405180830381865afa1580156102ec573d6000803e3d6000fd5b505050506040513d6000823e601f3d9081017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe01682016040526103329190810190610e2a565b92509250925060007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16635f0150cb8585856040518463ffffffff1660e01b815260040161039793929190610efb565b6040805180830381865afa1580156103b3573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906103d79190610f29565b5090508473ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161461043f576040517f6b0f689100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60028573ffffffffffffffffffffffffffffffffffffffff1663200d2ed26040518163ffffffff1660e01b8152600401602060405180830381865afa15801561048c573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906104b09190610f9f565b60028111156104c1576104c1610f70565b146104f8576040517f8f8af25f00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60405180604001604052806105788773ffffffffffffffffffffffffffffffffffffffff1663bcef3b556040518163ffffffff1660e01b8152600401602060405180830381865afa158015610551573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906105759190610fc0565b90565b81526020018673ffffffffffffffffffffffffffffffffffffffff16638b85902b6040518163ffffffff1660e01b8152600401602060405180830381865afa1580156105c8573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906105ec9190610fc0565b905263ffffffff909416600090815260016020818152604090922086518155959091015194019390935550505050565b600054610100900460ff161580801561063c5750600054600160ff909116105b806106565750303b158015610656575060005460ff166001145b6106e6576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602e60248201527f496e697469616c697a61626c653a20636f6e747261637420697320616c72656160448201527f647920696e697469616c697a6564000000000000000000000000000000000000606482015260840160405180910390fd5b600080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00166001179055801561074457600080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00ff166101001790555b60005b83518110156107aa57600084828151811061076457610764610fd9565b60209081029190910181015180820151905163ffffffff1660009081526001808452604090912082518155919092015191015550806107a281611008565b915050610747565b50600280547fffffffffffffffffffffffff00000000000000000000000000000000000000001673ffffffffffffffffffffffffffffffffffffffff8416179055801561084e57600080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00ff169055604051600181527f7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb38474024989060200160405180910390a15b505050565b600033905060008060008373ffffffffffffffffffffffffffffffffffffffff1663fa24f7436040518163ffffffff1660e01b8152600401600060405180830381865afa1580156108a8573d6000803e3d6000fd5b505050506040513d6000823e601f3d9081017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe01682016040526108ee9190810190610e2a565b92509250925060007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16635f0150cb8585856040518463ffffffff1660e01b815260040161095393929190610efb565b6040805180830381865afa15801561096f573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906109939190610f29565b5090508473ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff16146109fb576040517f6b0f689100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600160008563ffffffff1663ffffffff168152602001908152602001600020600101548573ffffffffffffffffffffffffffffffffffffffff16638b85902b6040518163ffffffff1660e01b8152600401602060405180830381865afa158015610a69573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610a8d9190610fc0565b11610a99575050505050565b60028573ffffffffffffffffffffffffffffffffffffffff1663200d2ed26040518163ffffffff1660e01b8152600401602060405180830381865afa158015610ae6573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610b0a9190610f9f565b6002811115610b1b57610b1b610f70565b146104f8575050505050565b73ffffffffffffffffffffffffffffffffffffffff81168114610b4957600080fd5b50565b600060208284031215610b5e57600080fd5b8135610b6981610b27565b9392505050565b60005b83811015610b8b578181015183820152602001610b73565b83811115610b9a576000848401525b50505050565b60008151808452610bb8816020860160208601610b70565b601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b602081526000610b696020830184610ba0565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b6040805190810167ffffffffffffffff81118282101715610c4f57610c4f610bfd565b60405290565b604051601f82017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe016810167ffffffffffffffff81118282101715610c9c57610c9c610bfd565b604052919050565b63ffffffff81168114610b4957600080fd5b8035610cc181610b27565b919050565b6000806040808486031215610cda57600080fd5b833567ffffffffffffffff80821115610cf257600080fd5b818601915086601f830112610d0657600080fd5b8135602082821115610d1a57610d1a610bfd565b610d28818360051b01610c55565b8281528181019350606092830285018201928a841115610d4757600080fd5b948201945b83861015610dd457858b0381811215610d655760008081fd5b610d6d610c2c565b8735610d7881610ca4565b81527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08201891315610daa5760008081fd5b610db2610c2c565b8886013581528989013586820152818601528652509485019493820193610d4c565b509650610de2888201610cb6565b955050505050509250929050565b600060208284031215610e0257600080fd5b8135610b6981610ca4565b600060208284031215610e1f57600080fd5b8151610b6981610b27565b600080600060608486031215610e3f57600080fd5b8351610e4a81610ca4565b60208501516040860151919450925067ffffffffffffffff80821115610e6f57600080fd5b818601915086601f830112610e8357600080fd5b815181811115610e9557610e95610bfd565b610ec660207fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0601f84011601610c55565b9150808252876020828501011115610edd57600080fd5b610eee816020840160208601610b70565b5080925050509250925092565b63ffffffff84168152826020820152606060408201526000610f206060830184610ba0565b95945050505050565b60008060408385031215610f3c57600080fd5b8251610f4781610b27565b602084015190925067ffffffffffffffff81168114610f6557600080fd5b809150509250929050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602160045260246000fd5b600060208284031215610fb157600080fd5b815160038110610b6957600080fd5b600060208284031215610fd257600080fd5b5051919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b60007fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8203611060577f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b506001019056fea164736f6c634300080f000a";

    /// @notice Deploys the contracts required to upgrade from v1.8.0 L2OO to v1.8.0 in a
    ///         PERMISSIONED configuration.
    /// @param _proxyAdmin Address of the ProxyAdmin contract.
    /// @param _systemOwnerSafe Address of the SystemOwnerSafe contract.
    /// @param _superchainConfigProxy Address of the SuperchainConfigProxy contract.
    /// @param _disputeGameFactoryImpl Address of the DisputeGameFactory implementation contract.
    /// @param _delayedWethImpl Address of the DelayedWETH implementation contract.
    /// @param _preimageOracleImpl Address of the PreimageOracle implementation contract.
    /// @param _mipsImpl Address of the MIPS implementation contract.
    /// @param _optimismPortal2Impl Address of the OptimismPortal2 implementation contract.
    function deploy(
        address _proxyAdmin,
        address _systemOwnerSafe,
        address _superchainConfigProxy,
        address _disputeGameFactoryImpl,
        address _delayedWethImpl,
        address _preimageOracleImpl,
        address _mipsImpl,
        address _optimismPortal2Impl
    )
        public
    {
        console.log("Deploying contracts required to upgrade from v1.8.0 L2OO to v1.8.0 permissioned");
        console.log("Using PERMISSIONED proof system");

        // Set address variables.
        proxyAdmin = _proxyAdmin;
        systemOwnerSafe = _systemOwnerSafe;
        superchainConfigProxy = _superchainConfigProxy;

        // Prank admin contracts.
        prankDeployment("ProxyAdmin", msg.sender);
        prankDeployment("SystemOwnerSafe", msg.sender);

        // Prank shared contracts.
        prankDeployment("SuperchainConfigProxy", superchainConfigProxy);
        prankDeployment("DisputeGameFactory", _disputeGameFactoryImpl);
        prankDeployment("DelayedWETH", _delayedWethImpl);
        prankDeployment("PreimageOracle", _preimageOracleImpl);
        prankDeployment("OptimismPortal2", _optimismPortal2Impl);

        // Deploy proxy contracts.
        deployERC1967Proxy("DisputeGameFactoryProxy");
        deployERC1967Proxy("AnchorStateRegistryProxy");
        deployERC1967Proxy("PermissionedDelayedWETHProxy");

        // Deploy AnchorStateRegistry implementation contract.
        // We can't use a pre-created implementation because the ASR implementation holds an
        // immutable variable that points at the DisputeGameFactoryProxy.
        deployAnchorStateRegistry2();

        // Re-use the existing MIPS implementation, but ensure its address is available
        save("Mips", _mipsImpl);

        // Initialize proxy contracts.
        initializeDisputeGameFactoryProxy();
        initializeAnchorStateRegistryProxy();
        initializePermissionedDelayedWETHProxy();

        // ONLY deploy and set up the PermissionedDisputeGame.
        // We can't use a pre-created implementation because the PermissionedDisputeGame holds an
        // immutable variable that refers to the L2 chain ID.
        setPermissionedCannonFaultGameImplementation();

        // Transfer contract ownership to ProxyAdmin.
        transferPermissionedWETHOwnershipFinal();
        transferDGFOwnershipFinal();
        transferAnchorStateOwnershipFinal();

        // Run post-deployment assertions.
        postDeployAssertions();

        // Print config summary.
        printConfigSummary();

        // Print deployment summary.
        printDeploymentSummary();
    }

    /// @notice Deploys the AnchorStateRegistry implementation.
    function deployAnchorStateRegistry2() internal broadcast {
        console.log("Deploying AnchorStateRegistry");
        bytes memory initCode = abi.encodePacked(
            ANCHOR_STATE_REGISTRY_BYTECODE,
            DeployUtils.encodeConstructor(
                abi.encodeCall(
                    IAnchorStateRegistry.__constructor__,
                    (IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy")))
                )
            )
        );
        bytes32 salt = _implSalt();
        address preComputedAddress = vm.computeCreate2Address(salt, keccak256(initCode));
        require(preComputedAddress.code.length == 0, "DeployUtils: contract already deployed");
        address addr_;
        assembly {
            addr_ := create2(0, add(initCode, 0x20), mload(initCode), salt)
            if iszero(addr_) {
                let size := returndatasize()
                returndatacopy(0, 0, size)
                revert(0, size)
            }
        }
        DeployUtils.assertValidContractAddress(addr_);
        save("AnchorStateRegistry", addr_);
        console.log("AnchorStateRegistry deployed at %s", addr_);
    }

    /// @notice Sets the implementation for the `PERMISSIONED_CANNON` game type in the `DisputeGameFactory`
    function setPermissionedCannonFaultGameImplementation() public broadcast {
        console.log("Setting Cannon PermissionedDisputeGame implementation");
        IDisputeGameFactory factory = IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy"));
        IDelayedWETH weth = IDelayedWETH(mustGetAddress("PermissionedDelayedWETHProxy"));

        PermissionedDisputeGame impl = new PermissionedDisputeGame(
            GameTypes.PERMISSIONED_CANNON,
            loadMipsAbsolutePrestate(),
            cfg.faultGameMaxDepth(),
            cfg.faultGameSplitDepth(),
            Duration.wrap(uint64(cfg.faultGameClockExtension())),
            Duration.wrap(uint64(cfg.faultGameMaxClockDuration())),
            IBigStepper(mustGetAddress("Mips")),
            weth,
            IAnchorStateRegistry(mustGetAddress("AnchorStateRegistryProxy")),
            cfg.l2ChainID(),
            cfg.l2OutputOracleProposer(),
            cfg.l2OutputOracleChallenger()
        );
        factory.setImplementation(GameTypes.PERMISSIONED_CANNON, IDisputeGame(address(impl)));
        save("PermissionedDisputeGame", address(impl));
        console.log(
            "DisputeGameFactoryProxy: set `FaultDisputeGame` implementation (Backend: Cannon | GameType: PERMISSIONED_CANNON)"
        );
    }

    /// @notice Initializes the DisputeGameFactory proxy.
    function initializeDisputeGameFactoryProxy() internal broadcast {
        console.log("Initializing DisputeGameFactory proxy");
        Proxy(payable(mustGetAddress("DisputeGameFactoryProxy"))).upgradeToAndCall(
            mustGetAddress("DisputeGameFactory"), abi.encodeCall(IDisputeGameFactory.initialize, msg.sender)
        );

        // We don't need to set the initialization bond for PermissionedDisputeGame because the
        // initialization bond is meant to be zero anyway. We assert that this bond is zero in the
        // post-checks that we perform either way, so no need for an explicit transaction.
    }

    /// @notice Initializes the AnchorStateRegistry proxy.
    function initializeAnchorStateRegistryProxy() internal broadcast {
        // Set up the anchor state root array.
        IAnchorStateRegistry.StartingAnchorRoot[] memory roots = new IAnchorStateRegistry.StartingAnchorRoot[](2);
        roots[0] = IAnchorStateRegistry.StartingAnchorRoot({
            gameType: GameTypes.PERMISSIONED_CANNON,
            outputRoot: OutputRoot({
                root: Hash.wrap(cfg.faultGameGenesisOutputRoot()),
                l2BlockNumber: cfg.faultGameGenesisBlock()
            })
        });

        // Initialize AnchorStateRegistry proxy.
        console.log("Initializing AnchorStateRegistry proxy");
        Proxy(payable(mustGetAddress("AnchorStateRegistryProxy"))).upgradeToAndCall(
            mustGetAddress("AnchorStateRegistry"),
            abi.encodeCall(
                IAnchorStateRegistry.initialize, (roots, ISuperchainConfig(mustGetAddress("SuperchainConfigProxy")))
            )
        );
    }

    /// @notice Initializes the permissioned DelayedWETH proxy.
    function initializePermissionedDelayedWETHProxy() internal broadcast {
        // Initialize permissioned DelayedWETH proxy.
        console.log("Initializing permissioned DelayedWETH proxy");
        Proxy(payable(mustGetAddress("PermissionedDelayedWETHProxy"))).upgradeToAndCall(
            mustGetAddress("DelayedWETH"),
            abi.encodeCall(
                IDelayedWETH.initialize, (msg.sender, ISuperchainConfig(mustGetAddress("SuperchainConfigProxy")))
            )
        );
    }

    /// @notice Transfers ownership of the permissioned DelayedWETH proxy to the ProxyAdmin and
    ///         transfers ownership of the underlying DelayedWETH contract to the SystemOwnerSafe.
    function transferPermissionedWETHOwnershipFinal() internal broadcast {
        // Transfer ownership of permissioned DelayedWETH to SystemOwnerSafe.
        console.log("Transferring ownership of underlying permissioned DelayedWETH");
        IDelayedWETH weth = IDelayedWETH(mustGetAddress("PermissionedDelayedWETHProxy"));
        weth.transferOwnership(systemOwnerSafe);

        // Transfer ownership of permissioned DelayedWETH proxy to ProxyAdmin.
        console.log("Transferring ownership of permissioned DelayedWETH proxy");
        Proxy prox = Proxy(payable(address(weth)));
        prox.changeAdmin(proxyAdmin);
    }

    /// @notice Transfers ownership of the DisputeGameFactory proxy to the ProxyAdmin and transfers
    ///         ownership of the underlying DisputeGameFactory contract to the SystemOwnerSafe.
    function transferDGFOwnershipFinal() internal broadcast {
        // Transfer ownership of DisputeGameFactory to SystemOwnerSafe.
        console.log("Transferring ownership of underlying DisputeGameFactory");
        IDisputeGameFactory dgf = IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy"));
        dgf.transferOwnership(systemOwnerSafe);

        // Transfer ownership of DisputeGameFactory proxy to ProxyAdmin.
        console.log("Transferring ownership of DisputeGameFactory proxy");
        Proxy prox = Proxy(payable(address(dgf)));
        prox.changeAdmin(proxyAdmin);
    }

    /// @notice Transfers ownership of the AnchorStateRegistry proxy to the ProxyAdmin.
    function transferAnchorStateOwnershipFinal() internal broadcast {
        // Transfer ownership of AnchorStateRegistry proxy to ProxyAdmin.
        console.log("Transferring ownership of AnchorStateRegistry proxy");
        IAnchorStateRegistry asr = IAnchorStateRegistry(mustGetAddress("AnchorStateRegistryProxy"));
        Proxy prox = Proxy(payable(address(asr)));
        prox.changeAdmin(proxyAdmin);
    }

    /// @notice Checks that the deployed system is configured correctly.
    function postDeployAssertions() internal view {
        Types.ContractSet memory contracts = _proxies();
        contracts.OptimismPortal2 = mustGetAddress("OptimismPortal2");

        // Ensure that `useFaultProofs` is set to `true`.
        assertTrue(cfg.useFaultProofs(), "DeployUpgrade: useFaultProofs is not set to true");

        // Verify that the DGF is owned by the ProxyAdmin.
        address dgfProxyAddr = mustGetAddress("DisputeGameFactoryProxy");
        assertEq(
            address(uint160(uint256(vm.load(dgfProxyAddr, Constants.PROXY_OWNER_ADDRESS)))),
            proxyAdmin,
            "DeployUpgrade: DGF is not owned by ProxyAdmin"
        );

        // Verify that permissioned DelayedWETH is owned by the ProxyAdmin.
        address soyWethProxyAddr = mustGetAddress("PermissionedDelayedWETHProxy");
        assertEq(
            address(uint160(uint256(vm.load(soyWethProxyAddr, Constants.PROXY_OWNER_ADDRESS)))),
            proxyAdmin,
            "DeployUpgrade: Permissioned DelayedWETH is not owned by ProxyAdmin"
        );

        // Run standard assertions.
        ChainAssertions.checkDisputeGameFactory(contracts, systemOwnerSafe, true);
        ChainAssertions.checkPermissionedDelayedWETH(contracts, cfg, true, systemOwnerSafe);
        ChainAssertions.checkOptimismPortal2(contracts, cfg, false);

        // Verify PreimageOracle configuration.
        IPreimageOracle oracle = IPreimageOracle(mustGetAddress("PreimageOracle"));
        assertEq(
            oracle.minProposalSize(),
            cfg.preimageOracleMinProposalSize(),
            "DeployUpgrade: PreimageOracle minProposalSize is not set correctly"
        );
        assertEq(
            oracle.challengePeriod(),
            cfg.preimageOracleChallengePeriod(),
            "DeployUpgrade: PreimageOracle challengePeriod is not set correctly"
        );

        // Verify MIPS configuration.
        MIPS mips = MIPS(mustGetAddress("Mips"));
        assertEq(address(mips.oracle()), address(oracle), "DeployUpgrade: MIPS oracle is not set correctly");

        // Verify AnchorStateRegistry configuration.
        IAnchorStateRegistry asr = IAnchorStateRegistry(mustGetAddress("AnchorStateRegistryProxy"));
        (Hash root1, uint256 l2BlockNumber1) = asr.anchors(GameTypes.PERMISSIONED_CANNON);
        assertEq(
            root1.raw(),
            cfg.faultGameGenesisOutputRoot(),
            "DeployUpgrade: AnchorStateRegistry root is not set correctly"
        );
        assertEq(
            l2BlockNumber1,
            cfg.faultGameGenesisBlock(),
            "DeployUpgrade: AnchorStateRegistry l2BlockNumber is not set correctly"
        );

        // Verify DisputeGameFactory configuration.
        IDisputeGameFactory dgf = IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy"));
        assertEq(
            dgf.initBonds(GameTypes.PERMISSIONED_CANNON),
            0 ether,
            "DeployUpgrade: DisputeGameFactory initBonds is not set correctly"
        );
        assertEq(
            address(dgf.gameImpls(GameTypes.CANNON)),
            address(0),
            "DeployUpgrade: DisputeGameFactory gameImpls CANNON is not set correctly"
        );
        assertEq(
            address(dgf.gameImpls(GameTypes.PERMISSIONED_CANNON)),
            mustGetAddress("PermissionedDisputeGame"),
            "DeployUpgrade: DisputeGameFactory gameImpls PERMISSIONED_CANNON is not set correctly"
        );

        // Verify security override yoke configuration.
        address soyGameAddr = address(dgf.gameImpls(GameTypes.PERMISSIONED_CANNON));
        PermissionedDisputeGame soyGameImpl = PermissionedDisputeGame(payable(soyGameAddr));
        assertEq(
            soyGameImpl.proposer(),
            cfg.l2OutputOracleProposer(),
            "DeployUpgrade: PermissionedDisputeGame proposer is not set correctly"
        );
        assertEq(
            soyGameImpl.challenger(),
            cfg.l2OutputOracleChallenger(),
            "DeployUpgrade: PermissionedDisputeGame challenger is not set correctly"
        );
        assertEq(
            soyGameImpl.maxGameDepth(),
            cfg.faultGameMaxDepth(),
            "DeployUpgrade: PermissionedDisputeGame maxGameDepth is not set correctly"
        );
        assertEq(
            soyGameImpl.splitDepth(),
            cfg.faultGameSplitDepth(),
            "DeployUpgrade: PermissionedDisputeGame splitDepth is not set correctly"
        );
        assertEq(
            soyGameImpl.clockExtension().raw(),
            cfg.faultGameClockExtension(),
            "DeployUpgrade: PermissionedDisputeGame clockExtension is not set correctly"
        );
        assertEq(
            soyGameImpl.maxClockDuration().raw(),
            cfg.faultGameMaxClockDuration(),
            "DeployUpgrade: PermissionedDisputeGame maxClockDuration is not set correctly"
        );
        assertEq(
            soyGameImpl.absolutePrestate().raw(),
            bytes32(cfg.faultGameAbsolutePrestate()),
            "DeployUpgrade: PermissionedDisputeGame absolutePrestate is not set correctly"
        );
        assertEq(
            address(soyGameImpl.weth()),
            soyWethProxyAddr,
            "DeployUpgrade: PermissionedDisputeGame weth is not set correctly"
        );
        assertEq(
            address(soyGameImpl.anchorStateRegistry()),
            address(asr),
            "DeployUpgrade: PermissionedDisputeGame anchorStateRegistry is not set correctly"
        );
        assertEq(
            address(soyGameImpl.vm()), address(mips), "DeployUpgrade: PermissionedDisputeGame vm is not set correctly"
        );
    }

    /// @notice Prints a summary of the configuration used to deploy this system.
    function printConfigSummary() internal view {
        console.log("Configuration Summary (chainid: %d)", block.chainid);
        console.log("    0. Use Fault Proofs: %s", cfg.useFaultProofs() ? "true" : "false");
        console.log("    1. Absolute Prestate: %x", cfg.faultGameAbsolutePrestate());
        console.log("    2. Max Depth: %d", cfg.faultGameMaxDepth());
        console.log("    3. Output / Execution split Depth: %d", cfg.faultGameSplitDepth());
        console.log("    4. Clock Extension (seconds): %d", cfg.faultGameClockExtension());
        console.log("    5. Max Clock Duration (seconds): %d", cfg.faultGameMaxClockDuration());
        console.log("    6. L2 Genesis block number: %d", cfg.faultGameGenesisBlock());
        console.log("    7. L2 Genesis output root: %x", uint256(cfg.faultGameGenesisOutputRoot()));
        console.log("    8. Proof Maturity Delay (seconds): %d", cfg.proofMaturityDelaySeconds());
        console.log("    9. Dispute Game Finality Delay (seconds): %d", cfg.disputeGameFinalityDelaySeconds());
        console.log("   10. Respected Game Type: %d", cfg.respectedGameType());
        console.log("   11. Preimage Oracle Min Proposal Size (bytes): %d", cfg.preimageOracleMinProposalSize());
        console.log("   12. Preimage Oracle Challenge Period (seconds): %d", cfg.preimageOracleChallengePeriod());
        console.log("   13. ProxyAdmin: %s", proxyAdmin);
        console.log("   14. SystemOwnerSafe: %s", systemOwnerSafe);
        console.log("   15. SuperchainConfigProxy: %s", superchainConfigProxy);
    }

    /// @notice Prints a summary of the contracts deployed during this script.
    function printDeploymentSummary() internal view {
        console.log("Deployment Summary (chainid: %d)", block.chainid);
        console.log("    0. DisputeGameFactoryProxy: %s", mustGetAddress("DisputeGameFactoryProxy"));
        console.log("    1. AnchorStateRegistryProxy: %s", mustGetAddress("AnchorStateRegistryProxy"));
        console.log("    2. AnchorStateRegistryImpl: %s", mustGetAddress("AnchorStateRegistry"));
        console.log("    3. PermissionedDelayedWETHProxy: %s", mustGetAddress("PermissionedDelayedWETHProxy"));
        console.log(
            "    4. PermissionedDisputeGame: %s",
            address(
                IDisputeGameFactory(mustGetAddress("DisputeGameFactoryProxy")).gameImpls(GameTypes.PERMISSIONED_CANNON)
            )
        );
    }
}
