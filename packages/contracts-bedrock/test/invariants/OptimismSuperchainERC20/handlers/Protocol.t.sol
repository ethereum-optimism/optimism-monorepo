// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import { TestBase } from "forge-std/Base.sol";
import { StdUtils } from "forge-std/StdUtils.sol";

import { ERC1967Proxy } from "@openzeppelin/contracts-v5/proxy/ERC1967/ERC1967Proxy.sol";
import { EnumerableMap } from "@openzeppelin/contracts/utils/structs/EnumerableMap.sol";
import { OptimismSuperchainERC20 } from "src/L2/OptimismSuperchainERC20.sol";
import { OptimismSuperchainERC20ForToBProperties } from "../helpers/OptimismSuperchainERC20ForToBProperties.t.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { MockL2ToL2CrossDomainMessenger } from "../helpers/MockL2ToL2CrossDomainMessenger.t.sol";
import { Actors } from "../helpers/Actors.t.sol";

contract ProtocolHandler is TestBase, StdUtils, Actors {
    using EnumerableMap for EnumerableMap.Bytes32ToUintMap;

    uint8 public constant MAX_CHAINS = 4;
    uint8 internal constant INITIAL_TOKENS = 1;
    uint8 internal constant INITIAL_SUPERTOKENS = 1;
    uint8 internal constant SUPERTOKEN_INITIAL_MINT = 100;
    address internal constant BRIDGE = Predeploys.L2_STANDARD_BRIDGE;
    MockL2ToL2CrossDomainMessenger internal constant MESSENGER =
        MockL2ToL2CrossDomainMessenger(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);
    OptimismSuperchainERC20 internal superchainERC20Impl;
    // NOTE: having more options for this enables the fuzzer to configure
    // different supertokens for the same remote token
    string[] internal WORDS = ["TOKENS"];
    uint8[] internal DECIMALS = [6, 18];

    struct TokenDeployParams {
        uint8 remoteTokenIndex;
        uint8 nameIndex;
        uint8 symbolIndex;
        uint8 decimalsIndex;
    }

    address[] internal remoteTokens;
    address[] internal allSuperTokens;

    /// @notice  'real' deploy salt => total supply sum across chains
    EnumerableMap.Bytes32ToUintMap internal ghost_totalSupplyAcrossChains;
    /// @notice  'real' deploy salt => tokens sendERC20'd but not yet relayERC20'd
    EnumerableMap.Bytes32ToUintMap internal ghost_tokensInTransit;

    constructor() {
        vm.etch(address(MESSENGER), address(new MockL2ToL2CrossDomainMessenger()).code);
        superchainERC20Impl = new OptimismSuperchainERC20ForToBProperties();
        for (uint256 remoteTokenIndex; remoteTokenIndex < INITIAL_TOKENS; remoteTokenIndex++) {
            _deployRemoteToken();
            for (uint256 supertokenChainId; supertokenChainId < INITIAL_SUPERTOKENS; supertokenChainId++) {
                _deploySupertoken(remoteTokens[remoteTokenIndex], WORDS[0], WORDS[0], DECIMALS[0], supertokenChainId);
            }
        }
        // integrate with all ToB properties using address(this) as the sender
        addActor(address(this));
    }

    /// @notice the deploy params are _indexes_ to pick from a pre-defined array of options and limit
    /// the amount of supertokens for a given remoteAsset that are incompatible between them, as
    /// two supertokens have to share decimals, name, symbol and remoteAsset to be considered
    /// the same asset, and therefore bridgable.
    modifier validateTokenDeployParams(TokenDeployParams memory params) {
        params.remoteTokenIndex = uint8(bound(params.remoteTokenIndex, 0, remoteTokens.length - 1));
        params.nameIndex = uint8(bound(params.nameIndex, 0, WORDS.length - 1));
        params.symbolIndex = uint8(bound(params.symbolIndex, 0, WORDS.length - 1));
        params.decimalsIndex = uint8(bound(params.decimalsIndex, 0, DECIMALS.length - 1));
        _;
    }

    function handler_mockNewRemoteToken() external {
        _deployRemoteToken();
    }

    /// @notice pick one already-deployed supertoken and mint an arbitrary amount of it
    /// necessary so there is something to be bridged :D
    /// TODO: will be replaced when testing the factories and `convert()`
    function handler_mintSupertoken(uint256 index, uint96 amount) external withActor(msg.sender) {
        index = bound(index, 0, allSuperTokens.length - 1);
        address addr = allSuperTokens[index];
        vm.prank(BRIDGE);
        OptimismSuperchainERC20(addr).crosschainMint(currentActor(), amount);
        // currentValue will be zero if key is not present
        (, uint256 currentValue) = ghost_totalSupplyAcrossChains.tryGet(MESSENGER.superTokenInitDeploySalts(addr));
        ghost_totalSupplyAcrossChains.set(MESSENGER.superTokenInitDeploySalts(addr), currentValue + amount);
    }

    /// @notice The ToB properties don't preclude the need for this since they
    /// always use address(this) as the caller, which won't get any balance
    /// until it's transferred to it somehow
    function handler_supERC20Transfer(
        uint256 tokenIndex,
        uint256 toIndex,
        uint256 amount
    )
        external
        withActor(msg.sender)
    {
        vm.prank(currentActor());
        OptimismSuperchainERC20(allSuperTokens[bound(tokenIndex, 0, allSuperTokens.length)]).transfer(
            getActorByRawIndex(toIndex), amount
        );
    }

    function handler_supERC20TransferFrom(
        uint256 tokenIndex,
        uint256 fromIndex,
        uint256 toIndex,
        uint256 amount
    )
        external
        withActor(msg.sender)
    {
        vm.prank(currentActor());
        OptimismSuperchainERC20(allSuperTokens[bound(tokenIndex, 0, allSuperTokens.length)]).transferFrom(
            getActorByRawIndex(fromIndex), getActorByRawIndex(toIndex), amount
        );
    }

    function handler_supERC20Approve(
        uint256 tokenIndex,
        uint256 spenderIndex,
        uint256 amount
    )
        external
        withActor(msg.sender)
    {
        vm.prank(currentActor());
        OptimismSuperchainERC20(allSuperTokens[bound(tokenIndex, 0, allSuperTokens.length)]).approve(
            getActorByRawIndex(spenderIndex), amount
        );
    }

    /// @notice deploy a remote token, that supertokens will be a representation of. They are  never called, so there
    /// is no need to actually deploy a contract for them
    function _deployRemoteToken() internal {
        // make sure they don't conflict with predeploys/preinstalls/precompiles/other tokens
        remoteTokens.push(address(uint160(1000 + remoteTokens.length)));
    }

    /// @notice deploy a new supertoken representing remoteToken
    /// remoteToken, name, symbol and decimals determine the 'real' deploy salt
    /// and supertokens sharing it are interoperable between them
    /// we however use the chainId as part of the deploy salt to mock the ability of
    /// supertokens to exist on different chains on a single EVM.
    function _deploySupertoken(
        address remoteToken,
        string memory name,
        string memory symbol,
        uint8 decimals,
        uint256 chainId
    )
        internal
        returns (OptimismSuperchainERC20 supertoken_)
    {
        // this salt would be used in production. Tokens sharing it will be bridgable with each other
        bytes32 realSalt = keccak256(abi.encode(remoteToken, name, symbol, decimals));
        // Foundry invariant erroneously show other unrelated invariant breaking
        // when this deployment fails due to a create2 collision, so we revert eagerly instead
        require(
            MESSENGER.superTokenAddresses(chainId, realSalt) == address(0), "ProtocolHandler: skip duplicate deployment"
        );

        // what we use in the tests to walk around two contracts needing two different addresses
        // tbf we could be using CREATE1, but this feels more verbose
        bytes32 hackySalt = keccak256(abi.encode(remoteToken, name, symbol, decimals, chainId));
        supertoken_ = OptimismSuperchainERC20(
            address(
                // TODO: Use the OptimismSuperchainERC20 Beacon Proxy
                new ERC1967Proxy{ salt: hackySalt }(
                    address(superchainERC20Impl),
                    abi.encodeCall(OptimismSuperchainERC20.initialize, (remoteToken, name, symbol, decimals))
                )
            )
        );
        MESSENGER.registerSupertoken(realSalt, chainId, address(supertoken_));
        allSuperTokens.push(address(supertoken_));
    }
}
