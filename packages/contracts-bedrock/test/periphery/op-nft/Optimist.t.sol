// SPDX-License-Identifier: MIT
pragma solidity >=0.6.2 <0.9.0;

// Testing utilities
import { Test } from "forge-std/Test.sol";
import { IMulticall3 } from "forge-std/interfaces/IMulticall3.sol";
import { AttestationStation } from "src/periphery/op-nft/AttestationStation.sol";
import { Optimist } from "src/periphery/op-nft/Optimist.sol";
import { OptimistAllowlist } from "src/periphery/op-nft/OptimistAllowlist.sol";
import { OptimistInviter } from "src/periphery/op-nft/OptimistInviter.sol";
import { OptimistInviterHelper } from "test/mocks/OptimistInviterHelper.sol";
import { IERC721 } from "@openzeppelin/contracts/token/ERC721/IERC721.sol";

library Multicall {
    bytes internal constant code =
        hex"6080604052600436106100f35760003560e01c80634d2301cc1161008a578063a8b0574e11610059578063a8b0574e1461025a578063bce38bd714610275578063c3077fa914610288578063ee82ac5e1461029b57600080fd5b80634d2301cc146101ec57806372425d9d1461022157806382ad56cb1461023457806386d516e81461024757600080fd5b80633408e470116100c65780633408e47014610191578063399542e9146101a45780633e64a696146101c657806342cbb15c146101d957600080fd5b80630f28c97d146100f8578063174dea711461011a578063252dba421461013a57806327e86d6e1461015b575b600080fd5b34801561010457600080fd5b50425b6040519081526020015b60405180910390f35b61012d610128366004610a85565b6102ba565b6040516101119190610bbe565b61014d610148366004610a85565b6104ef565b604051610111929190610bd8565b34801561016757600080fd5b50437fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0140610107565b34801561019d57600080fd5b5046610107565b6101b76101b2366004610c60565b610690565b60405161011193929190610cba565b3480156101d257600080fd5b5048610107565b3480156101e557600080fd5b5043610107565b3480156101f857600080fd5b50610107610207366004610ce2565b73ffffffffffffffffffffffffffffffffffffffff163190565b34801561022d57600080fd5b5044610107565b61012d610242366004610a85565b6106ab565b34801561025357600080fd5b5045610107565b34801561026657600080fd5b50604051418152602001610111565b61012d610283366004610c60565b61085a565b6101b7610296366004610a85565b610a1a565b3480156102a757600080fd5b506101076102b6366004610d18565b4090565b60606000828067ffffffffffffffff8111156102d8576102d8610d31565b60405190808252806020026020018201604052801561031e57816020015b6040805180820190915260008152606060208201528152602001906001900390816102f65790505b5092503660005b8281101561047757600085828151811061034157610341610d60565b6020026020010151905087878381811061035d5761035d610d60565b905060200281019061036f9190610d8f565b6040810135958601959093506103886020850185610ce2565b73ffffffffffffffffffffffffffffffffffffffff16816103ac6060870187610dcd565b6040516103ba929190610e32565b60006040518083038185875af1925050503d80600081146103f7576040519150601f19603f3d011682016040523d82523d6000602084013e6103fc565b606091505b50602080850191909152901515808452908501351761046d577f08c379a000000000000000000000000000000000000000000000000000000000600052602060045260176024527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060445260846000fd5b5050600101610325565b508234146104e6576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601a60248201527f4d756c746963616c6c333a2076616c7565206d69736d6174636800000000000060448201526064015b60405180910390fd5b50505092915050565b436060828067ffffffffffffffff81111561050c5761050c610d31565b60405190808252806020026020018201604052801561053f57816020015b606081526020019060019003908161052a5790505b5091503660005b8281101561068657600087878381811061056257610562610d60565b90506020028101906105749190610e42565b92506105836020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff166105a66020850185610dcd565b6040516105b4929190610e32565b6000604051808303816000865af19150503d80600081146105f1576040519150601f19603f3d011682016040523d82523d6000602084013e6105f6565b606091505b5086848151811061060957610609610d60565b602090810291909101015290508061067d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060448201526064016104dd565b50600101610546565b5050509250929050565b43804060606106a086868661085a565b905093509350939050565b6060818067ffffffffffffffff8111156106c7576106c7610d31565b60405190808252806020026020018201604052801561070d57816020015b6040805180820190915260008152606060208201528152602001906001900390816106e55790505b5091503660005b828110156104e657600084828151811061073057610730610d60565b6020026020010151905086868381811061074c5761074c610d60565b905060200281019061075e9190610e76565b925061076d6020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff166107906040850185610dcd565b60405161079e929190610e32565b6000604051808303816000865af19150503d80600081146107db576040519150601f19603f3d011682016040523d82523d6000602084013e6107e0565b606091505b506020808401919091529015158083529084013517610851577f08c379a000000000000000000000000000000000000000000000000000000000600052602060045260176024527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060445260646000fd5b50600101610714565b6060818067ffffffffffffffff81111561087657610876610d31565b6040519080825280602002602001820160405280156108bc57816020015b6040805180820190915260008152606060208201528152602001906001900390816108945790505b5091503660005b82811015610a105760008482815181106108df576108df610d60565b602002602001015190508686838181106108fb576108fb610d60565b905060200281019061090d9190610e42565b925061091c6020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff1661093f6020850185610dcd565b60405161094d929190610e32565b6000604051808303816000865af19150503d806000811461098a576040519150601f19603f3d011682016040523d82523d6000602084013e61098f565b606091505b506020830152151581528715610a07578051610a07576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060448201526064016104dd565b506001016108c3565b5050509392505050565b6000806060610a2b60018686610690565b919790965090945092505050565b60008083601f840112610a4b57600080fd5b50813567ffffffffffffffff811115610a6357600080fd5b6020830191508360208260051b8501011115610a7e57600080fd5b9250929050565b60008060208385031215610a9857600080fd5b823567ffffffffffffffff811115610aaf57600080fd5b610abb85828601610a39565b90969095509350505050565b6000815180845260005b81811015610aed57602081850181015186830182015201610ad1565b81811115610aff576000602083870101525b50601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b600082825180855260208086019550808260051b84010181860160005b84811015610bb1578583037fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe001895281518051151584528401516040858501819052610b9d81860183610ac7565b9a86019a9450505090830190600101610b4f565b5090979650505050505050565b602081526000610bd16020830184610b32565b9392505050565b600060408201848352602060408185015281855180845260608601915060608160051b870101935082870160005b82811015610c52577fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffa0888703018452610c40868351610ac7565b95509284019290840190600101610c06565b509398975050505050505050565b600080600060408486031215610c7557600080fd5b83358015158114610c8557600080fd5b9250602084013567ffffffffffffffff811115610ca157600080fd5b610cad86828701610a39565b9497909650939450505050565b838152826020820152606060408201526000610cd96060830184610b32565b95945050505050565b600060208284031215610cf457600080fd5b813573ffffffffffffffffffffffffffffffffffffffff81168114610bd157600080fd5b600060208284031215610d2a57600080fd5b5035919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff81833603018112610dc357600080fd5b9190910192915050565b60008083357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe1843603018112610e0257600080fd5b83018035915067ffffffffffffffff821115610e1d57600080fd5b602001915036819003821315610a7e57600080fd5b8183823760009101908152919050565b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc1833603018112610dc357600080fd5b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffa1833603018112610dc357600080fdfea2646970667358221220bb2b5c71a328032f97c676ae39a1ec2148d3e5d6f73d95e9b17910152d61f16264736f6c634300080c0033";
    address internal constant addr = 0xcA11bde05977b3631167028862bE2a173976CA11;
}

contract Optimist_Initializer is Test {
    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);
    event Initialized(uint8);
    event AttestationCreated(address indexed creator, address indexed about, bytes32 indexed key, bytes val);

    string constant name = "Optimist name";
    string constant symbol = "OPTIMISTSYMBOL";
    string constant base_uri = "https://storageapi.fleek.co/6442819a1b05-bucket/optimist-nft/attributes";
    AttestationStation attestationStation;
    Optimist optimist;
    OptimistAllowlist optimistAllowlist;
    OptimistInviter optimistInviter;

    // Helps with EIP-712 signature generation
    OptimistInviterHelper optimistInviterHelper;

    // To test multicall for claiming and minting in one call
    IMulticall3 multicall3;

    address internal carol_baseURIAttestor;
    address internal alice_allowlistAttestor;
    address internal eve_inviteGranter;
    address internal ted_coinbaseAttestor;
    address internal bob;
    address internal sally;

    /// @notice BaseURI attestor sets the baseURI of the Optimist NFT.
    function _attestBaseURI(string memory _baseUri) internal {
        bytes32 baseURIAttestationKey = optimist.BASE_URI_ATTESTATION_KEY();
        AttestationStation.AttestationData[] memory attestationData = new AttestationStation.AttestationData[](1);
        attestationData[0] =
            AttestationStation.AttestationData(address(optimist), baseURIAttestationKey, bytes(_baseUri));

        vm.expectEmit(true, true, true, true, address(attestationStation));
        emit AttestationCreated(carol_baseURIAttestor, address(optimist), baseURIAttestationKey, bytes(_baseUri));
        vm.prank(carol_baseURIAttestor);
        attestationStation.attest(attestationData);
    }

    /// @notice Allowlist attestor creates an attestation for an address.
    function _attestAllowlist(address _about) internal {
        bytes32 attestationKey = optimistAllowlist.OPTIMIST_CAN_MINT_ATTESTATION_KEY();
        AttestationStation.AttestationData[] memory attestationData = new AttestationStation.AttestationData[](1);
        // we are using true but it can be any non empty value
        attestationData[0] =
            AttestationStation.AttestationData({ about: _about, key: attestationKey, val: bytes("true") });

        vm.expectEmit(true, true, true, true, address(attestationStation));
        emit AttestationCreated(alice_allowlistAttestor, _about, attestationKey, bytes("true"));

        vm.prank(alice_allowlistAttestor);
        attestationStation.attest(attestationData);

        assertTrue(optimist.isOnAllowList(_about));
    }

    /// @notice Coinbase Quest attestor creates an attestation for an address.
    function _attestCoinbaseQuest(address _about) internal {
        bytes32 attestationKey = optimistAllowlist.COINBASE_QUEST_ELIGIBLE_ATTESTATION_KEY();
        AttestationStation.AttestationData[] memory attestationData = new AttestationStation.AttestationData[](1);
        // we are using true but it can be any non empty value
        attestationData[0] =
            AttestationStation.AttestationData({ about: _about, key: attestationKey, val: bytes("true") });

        vm.expectEmit(true, true, true, true, address(attestationStation));
        emit AttestationCreated(ted_coinbaseAttestor, _about, attestationKey, bytes("true"));

        vm.prank(ted_coinbaseAttestor);
        attestationStation.attest(attestationData);

        assertTrue(optimist.isOnAllowList(_about));
    }

    /// @notice Issues invite, then claims it using the claimer's address.
    function _inviteAndClaim(address _about) internal {
        uint256 inviterPrivateKey = 0xbeefbeef;
        address inviter = vm.addr(inviterPrivateKey);

        address[] memory addresses = new address[](1);
        addresses[0] = inviter;

        vm.prank(eve_inviteGranter);

        // grant invites to Inviter;
        optimistInviter.setInviteCounts(addresses, 3);

        // issue a new invite
        OptimistInviter.ClaimableInvite memory claimableInvite =
            optimistInviterHelper.getClaimableInviteWithNewNonce(inviter);

        // EIP-712 sign with Inviter's private key

        (uint8 v, bytes32 r, bytes32 s) = vm.sign(inviterPrivateKey, optimistInviterHelper.getDigest(claimableInvite));
        bytes memory signature = abi.encodePacked(r, s, v);

        bytes32 hashedCommit = keccak256(abi.encode(_about, signature));

        // commit the invite
        vm.prank(_about);
        optimistInviter.commitInvite(hashedCommit);

        // wait minimum commitment period
        vm.warp(optimistInviter.MIN_COMMITMENT_PERIOD() + block.timestamp);

        // reveal and claim the invite
        optimistInviter.claimInvite(_about, claimableInvite, signature);

        assertTrue(optimist.isOnAllowList(_about));
    }

    /// @notice Mocks the allowlistAttestor to always return true for a given address.
    function _mockAllowlistTrueFor(address _claimer) internal {
        vm.mockCall(
            address(optimistAllowlist), abi.encodeCall(OptimistAllowlist.isAllowedToMint, (_claimer)), abi.encode(true)
        );

        assertTrue(optimist.isOnAllowList(_claimer));
    }

    /// @notice Returns address as uint256.
    function _getTokenId(address _owner) internal pure returns (uint256) {
        return uint256(uint160(address(_owner)));
    }

    function setUp() public {
        carol_baseURIAttestor = makeAddr("carol_baseURIAttestor");
        alice_allowlistAttestor = makeAddr("alice_allowlistAttestor");
        eve_inviteGranter = makeAddr("eve_inviteGranter");
        ted_coinbaseAttestor = makeAddr("ted_coinbaseAttestor");
        bob = makeAddr("bob");
        sally = makeAddr("sally");
        _initializeContracts();
    }

    function _initializeContracts() internal {
        attestationStation = new AttestationStation();
        vm.expectEmit(true, true, false, false);
        emit Initialized(1);

        optimistInviter =
            new OptimistInviter({ _inviteGranter: eve_inviteGranter, _attestationStation: attestationStation });

        optimistInviter.initialize("OptimistInviter");

        // Initialize the helper which helps sign EIP-712 signatures
        optimistInviterHelper = new OptimistInviterHelper(optimistInviter, "OptimistInviter");

        optimistAllowlist = new OptimistAllowlist({
            _attestationStation: attestationStation,
            _allowlistAttestor: alice_allowlistAttestor,
            _coinbaseQuestAttestor: ted_coinbaseAttestor,
            _optimistInviter: address(optimistInviter)
        });

        optimist = new Optimist({
            _name: name,
            _symbol: symbol,
            _baseURIAttestor: carol_baseURIAttestor,
            _attestationStation: attestationStation,
            _optimistAllowlist: optimistAllowlist
        });

        multicall3 = IMulticall3(Multicall.addr);
        vm.etch(Multicall.addr, Multicall.code);
    }
}

contract OptimistTest is Optimist_Initializer {
    /// @notice Check that constructor and initializer parameters are correctly set.
    function test_initialize_succeeds() external view {
        // expect name to be set
        assertEq(optimist.name(), name);
        // expect symbol to be set
        assertEq(optimist.symbol(), symbol);
        // expect attestationStation to be set
        assertEq(address(optimist.ATTESTATION_STATION()), address(attestationStation));
        assertEq(optimist.BASE_URI_ATTESTOR(), carol_baseURIAttestor);
    }

    /// @notice Bob should be able to mint an NFT if he is allowlisted
    ///         by the allowlistAttestor and has a balance of 0.
    function test_mint_afterAllowlistAttestation_succeeds() external {
        // bob should start with 0 balance
        assertEq(optimist.balanceOf(bob), 0);

        // allowlist bob
        _attestAllowlist(bob);

        assertTrue(optimistAllowlist.isAllowedToMint(bob));

        // Check that the OptimistAllowlist is checked
        bytes memory data = abi.encodeCall(OptimistAllowlist.isAllowedToMint, (bob));
        vm.expectCall(address(optimistAllowlist), data);

        // mint an NFT and expect mint transfer event to be emitted
        vm.expectEmit(true, true, true, true);
        emit Transfer(address(0), bob, _getTokenId(bob));
        vm.prank(bob);
        optimist.mint(bob);

        // expect the NFT to be owned by bob
        assertEq(optimist.ownerOf(_getTokenId(bob)), bob);
        assertEq(optimist.balanceOf(bob), 1);
    }

    /// @notice Bob should be able to mint an NFT if he claimed an invite through OptimistInviter
    ///         and has a balance of 0.
    function test_mint_afterInviteClaimed_succeeds() external {
        // bob should start with 0 balance
        assertEq(optimist.balanceOf(bob), 0);

        // bob claims an invite
        _inviteAndClaim(bob);

        assertTrue(optimistAllowlist.isAllowedToMint(bob));

        // Check that the OptimistAllowlist is checked
        bytes memory data = abi.encodeCall(OptimistAllowlist.isAllowedToMint, (bob));
        vm.expectCall(address(optimistAllowlist), data);

        // mint an NFT and expect mint transfer event to be emitted
        vm.expectEmit(true, true, true, true);
        emit Transfer(address(0), bob, _getTokenId(bob));
        vm.prank(bob);
        optimist.mint(bob);

        // expect the NFT to be owned by bob
        assertEq(optimist.ownerOf(_getTokenId(bob)), bob);
        assertEq(optimist.balanceOf(bob), 1);
    }

    /// @notice Bob should be able to mint an NFT if he has an attestation from Coinbase Quest
    ///         attestor and has a balance of 0.
    function test_mint_afterCoinbaseQuestAttestation_succeeds() external {
        // bob should start with 0 balance
        assertEq(optimist.balanceOf(bob), 0);

        // bob receives attestation from Coinbase Quest attestor
        _attestCoinbaseQuest(bob);

        assertTrue(optimistAllowlist.isAllowedToMint(bob));

        // Check that the OptimistAllowlist is checked
        bytes memory data = abi.encodeCall(OptimistAllowlist.isAllowedToMint, (bob));
        vm.expectCall(address(optimistAllowlist), data);

        // mint an NFT and expect mint transfer event to be emitted
        vm.expectEmit(true, true, true, true);
        emit Transfer(address(0), bob, _getTokenId(bob));
        vm.prank(bob);
        optimist.mint(bob);

        // expect the NFT to be owned by bob
        assertEq(optimist.ownerOf(_getTokenId(bob)), bob);
        assertEq(optimist.balanceOf(bob), 1);
    }

    /// @notice Multiple valid attestations should allow Bob to mint.
    function test_mint_afterMultipleAttestations_succeeds() external {
        // bob should start with 0 balance
        assertEq(optimist.balanceOf(bob), 0);

        // bob receives attestation from Coinbase Quest attestor
        _attestCoinbaseQuest(bob);

        // allowlist bob
        _attestAllowlist(bob);

        // bob claims an invite
        _inviteAndClaim(bob);

        assertTrue(optimistAllowlist.isAllowedToMint(bob));

        // Check that the OptimistAllowlist is checked
        bytes memory data = abi.encodeCall(OptimistAllowlist.isAllowedToMint, (bob));
        vm.expectCall(address(optimistAllowlist), data);

        // mint an NFT and expect mint transfer event to be emitted
        vm.expectEmit(true, true, true, true);
        emit Transfer(address(0), bob, _getTokenId(bob));
        vm.prank(bob);
        optimist.mint(bob);

        // expect the NFT to be owned by bob
        assertEq(optimist.ownerOf(_getTokenId(bob)), bob);
        assertEq(optimist.balanceOf(bob), 1);
    }

    /// @notice Sally should be able to mint a token on behalf of bob.
    function test_mint_secondaryMinter_succeeds() external {
        _mockAllowlistTrueFor(bob);

        vm.expectEmit(true, true, true, true);
        emit Transfer(address(0), bob, _getTokenId(bob));

        // mint as sally instead of bob
        vm.prank(sally);
        optimist.mint(bob);

        // expect the NFT to be owned by bob
        assertEq(optimist.ownerOf(_getTokenId(bob)), bob);
        assertEq(optimist.balanceOf(bob), 1);
    }

    /// @notice Bob should not be able to mint an NFT if he is not allowlisted.
    function test_mint_forNonAllowlistedClaimer_reverts() external {
        vm.prank(bob);
        vm.expectRevert("Optimist: address is not on allowList");
        optimist.mint(bob);
    }

    /// @notice Bob's tx should revert if he already minted.
    function test_mint_forAlreadyMintedClaimer_reverts() external {
        _attestAllowlist(bob);

        // mint initial nft with bob
        vm.prank(bob);
        optimist.mint(bob);
        // expect the NFT to be owned by bob
        assertEq(optimist.ownerOf(_getTokenId(bob)), bob);
        assertEq(optimist.balanceOf(bob), 1);

        // attempt to mint again
        vm.expectRevert("ERC721: token already minted");
        optimist.mint(bob);
    }

    /// @notice The baseURI should be set by attestation station by the baseURIAttestor.
    function test_baseURI_returnsCorrectBaseURI_succeeds() external {
        _attestBaseURI(base_uri);

        bytes memory data = abi.encodeCall(
            attestationStation.attestations,
            (carol_baseURIAttestor, address(optimist), optimist.BASE_URI_ATTESTATION_KEY())
        );
        vm.expectCall(address(attestationStation), data);
        vm.prank(carol_baseURIAttestor);

        // assert baseURI is set
        assertEq(optimist.baseURI(), base_uri);
    }

    /// @notice tokenURI should return the token uri for a minted token.
    function test_tokenURI_returnsCorrectTokenURI_succeeds() external {
        // we are using true but it can be any non empty value
        _attestBaseURI(base_uri);

        // mint an NFT
        _mockAllowlistTrueFor(bob);
        vm.prank(bob);
        optimist.mint(bob);

        // assert tokenURI is set
        assertEq(optimist.baseURI(), base_uri);
        assertEq(
            optimist.tokenURI(_getTokenId(bob)),
            "https://storageapi.fleek.co/6442819a1b05-bucket/optimist-nft/attributes/0x1d96f2f6bef1202e4ce1ff6dad0c2cb002861d3e.json"
        );
    }

    /// @notice Should return the token id of the owner.
    function test_tokenIdOfAddress_returnsOwnerID_succeeds() external {
        uint256 willTokenId = 1024;
        address will = address(1024);

        _mockAllowlistTrueFor(will);

        optimist.mint(will);

        assertEq(optimist.tokenIdOfAddress(will), willTokenId);
    }

    /// @notice transferFrom should revert since Optimist is a SBT.
    function test_transferFrom_soulbound_reverts() external {
        _mockAllowlistTrueFor(bob);

        // mint as bob
        vm.prank(bob);
        optimist.mint(bob);

        // attempt to transfer to sally
        vm.expectRevert(bytes("Optimist: soul bound token"));
        vm.prank(bob);
        optimist.transferFrom(bob, sally, _getTokenId(bob));

        // attempt to transfer to sally
        vm.expectRevert(bytes("Optimist: soul bound token"));
        vm.prank(bob);
        optimist.safeTransferFrom(bob, sally, _getTokenId(bob));
        // attempt to transfer to sally
        vm.expectRevert(bytes("Optimist: soul bound token"));
        vm.prank(bob);
        optimist.safeTransferFrom(bob, sally, _getTokenId(bob), bytes("0x"));
    }

    /// @notice approve should revert since Optimist is a SBT.
    function test_approve_soulbound_reverts() external {
        _mockAllowlistTrueFor(bob);

        // mint as bob
        vm.prank(bob);
        optimist.mint(bob);

        // attempt to approve sally
        vm.prank(bob);
        vm.expectRevert("Optimist: soul bound token");
        optimist.approve(address(attestationStation), _getTokenId(bob));

        assertEq(optimist.getApproved(_getTokenId(bob)), address(0));
    }

    /// @notice setApprovalForAll should revert since Optimist is a SBT.
    function test_setApprovalForAll_soulbound_reverts() external {
        _mockAllowlistTrueFor(bob);

        // mint as bob
        vm.prank(bob);
        optimist.mint(bob);
        vm.prank(alice_allowlistAttestor);
        vm.expectRevert(bytes("Optimist: soul bound token"));
        optimist.setApprovalForAll(alice_allowlistAttestor, true);

        // expect approval amount to stil be 0
        assertEq(optimist.getApproved(_getTokenId(bob)), address(0));
        // isApprovedForAll should return false
        assertEq(optimist.isApprovedForAll(alice_allowlistAttestor, alice_allowlistAttestor), false);
    }

    /// @notice Only owner should be able to burn token.
    function test_burn_byOwner_succeeds() external {
        _mockAllowlistTrueFor(bob);

        // mint as bob
        vm.prank(bob);
        optimist.mint(bob);

        // burn as bob
        vm.prank(bob);
        optimist.burn(_getTokenId(bob));

        // expect bob to have no balance now
        assertEq(optimist.balanceOf(bob), 0);
    }

    /// @notice Non-owner attempting to burn token should revert.
    function test_burn_byNonOwner_reverts() external {
        _mockAllowlistTrueFor(bob);

        // mint as bob
        vm.prank(bob);
        optimist.mint(bob);

        vm.expectRevert("ERC721: caller is not token owner nor approved");
        // burn as Sally
        vm.prank(sally);
        optimist.burn(_getTokenId(bob));

        // expect bob to have still have the token
        assertEq(optimist.balanceOf(bob), 1);
    }

    /// @notice Should support ERC-721 interface.
    function test_supportsInterface_returnsCorrectInterfaceForERC721_succeeds() external view {
        bytes4 iface721 = type(IERC721).interfaceId;
        // check that it supports ERC-721 interface
        assertEq(optimist.supportsInterface(iface721), true);
    }

    /// @notice Checking that multi-call using the invite & claim flow works correctly, since the
    ///         frontend will be making multicalls to improve UX. The OptimistInviter.claimInvite
    ///         and Optimist.mint will be batched
    function test_multicall_batchingClaimAndMint_succeeds() external {
        uint256 inviterPrivateKey = 0xbeefbeef;
        address inviter = vm.addr(inviterPrivateKey);

        address[] memory addresses = new address[](1);
        addresses[0] = inviter;

        vm.prank(eve_inviteGranter);

        // grant invites to Inviter;
        optimistInviter.setInviteCounts(addresses, 3);

        // issue a new invite
        OptimistInviter.ClaimableInvite memory claimableInvite =
            optimistInviterHelper.getClaimableInviteWithNewNonce(inviter);

        // EIP-712 sign with Inviter's private key

        (uint8 v, bytes32 r, bytes32 s) = vm.sign(inviterPrivateKey, optimistInviterHelper.getDigest(claimableInvite));
        bytes memory signature = abi.encodePacked(r, s, v);

        bytes32 hashedCommit = keccak256(abi.encode(bob, signature));

        // commit the invite
        vm.prank(bob);
        optimistInviter.commitInvite(hashedCommit);

        // wait minimum commitment period
        vm.warp(optimistInviter.MIN_COMMITMENT_PERIOD() + block.timestamp);

        IMulticall3.Call3[] memory calls = new IMulticall3.Call3[](2);

        // First call is to claim the invite, receiving the attestation
        calls[0] = IMulticall3.Call3({
            target: address(optimistInviter),
            callData: abi.encodeCall(OptimistInviter.claimInvite, (bob, claimableInvite, signature)),
            allowFailure: false
        });

        // Second call is to mint the Optimist NFT
        calls[1] = IMulticall3.Call3({
            target: address(optimist),
            callData: abi.encodeCall(Optimist.mint, (bob)),
            allowFailure: false
        });

        multicall3.aggregate3(calls);

        assertTrue(optimist.isOnAllowList(bob));
        assertEq(optimist.ownerOf(_getTokenId(bob)), bob);
        assertEq(optimist.balanceOf(bob), 1);
    }
}
